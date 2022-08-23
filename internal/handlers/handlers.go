package handlers

import (
	"context"
	"fmt"
	"github.com/CloudyKit/jet/v6"
	"github.com/fasthttp/websocket"
	"github.com/go-redis/redis/v8"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"
)

var Ctx = context.TODO()

var client_redis = redis.NewClient(&redis.Options{
	Addr:     "localhost:8379",
	Password: "",
	DB:       0,
})

var wsChan = make(chan WsPayload)

//structs sempre tem que ter as variáveis começando com letra maiúscula
type usuario struct {
	id   string
	Name string
	Tipo string
}

var clients = make(map[WebSocketConnection]usuario)

var views = jet.NewSet(
	jet.NewOSFileSystemLoader("./html"),
	jet.InDevelopmentMode())

var upgradeConnection = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// Renderiza página home
func Home(w http.ResponseWriter, r *http.Request) {
	id := r.PostFormValue("identificao")
	err := renderPage(w, "home.jet", nil)
	if err != nil {
		log.Println(err)
	}
	w.Write([]byte("<label for=\"teste\" id=\"teste\" hidden = true >" + id + "</label>"))
	//js.Global.Get("document").Call("write", "Capivara")
}

func Login(w http.ResponseWriter, r *http.Request) {

	err := renderPage(w, "login.jet", nil)
	if err != nil {
		log.Println(err)
	}

}

func Valida_login(w http.ResponseWriter, r *http.Request) {

	id := r.PostFormValue("identificao")
	password := r.PostFormValue("password")
	plataforma := r.PostFormValue("plataforma")
	key := "login:" + id
	teste, err := client_redis.HGet(Ctx, key, "senha").Result()
	if err != nil {
		log.Println("usuário não encontrado")
	}
	if plataforma == "A" {
		if err == nil && teste == password {
			w.Write([]byte("T"))
		} else {
			w.Write([]byte("Errado"))
		}
	} else {
		if teste == password {
			Home(w, r)
		} else {
			Login(w, r)
		}
	}
}

type WebSocketConnection struct {
	*websocket.Conn
}

// defini a resposta enviada no websocket
type WsJsonResponse struct {
	Action         string   `json:"action"`
	Remetente      string   `json:"remetente"`
	Message        string   `json:"message"`
	MessageType    string   `json:"message_type"`
	Plataforma     string   `json:"plataforma"`
	ConnectedUsers []string `json:"connected_users"`
	Messages       []string `json:"messages_slice"`
}

// Estrututra recebida pelo Servidor dos clientes
type WsPayload struct {
	Action     string              `json:"action"`
	Id         string              `json:"id"`
	Username   string              `json:"username"`
	Message    string              `json:"message"`
	Receiver   string              `json:"receiver"`
	Plataforma string              `json:"plataforma"`
	Conn       WebSocketConnection `json:"-"`
}

//realiza o upgrade da conexão para websocket
func WsEndpoint(w http.ResponseWriter, r *http.Request) {
	ws, err := upgradeConnection.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
	}
	var response WsJsonResponse
	//response.Message = `<em><small>Conexao bem sucedida<small><em>`
	conn := WebSocketConnection{Conn: ws}
	u := usuario{"",
		"", ""}
	clients[conn] = u
	err = ws.WriteJSON(response)
	if err != nil {
		log.Println(err)
	}

	go ListenForWs(&conn)
}

func ListenForWs(conn *WebSocketConnection) {
	defer func() {
		if r := recover(); r != nil {
			log.Println("Error", fmt.Sprintf("%v", r))
		}
	}()

	var payload WsPayload

	for {
		err := conn.ReadJSON(&payload)
		//log.Println(payload)
		if err != nil {
		} else {
			payload.Conn = *conn
			wsChan <- payload
		}
	}
}

func ListenToWsChannel() {
	var response WsJsonResponse
	var Registrado bool
	for {
		e := <-wsChan
		switch e.Action {

		// consulta e envia o nome do usuario após se conectar
		case "eu":
			response.Action = "eu"
			e.Username = retorna_nome(client_redis, Ctx, e)
			response.Message = fmt.Sprintf("%s", e.Username)
			toOne(response, e)
		//Cria um map com a concexao do websocket e o Struct do usuário
		case "conexao":
			igual := false
			Registrado = id_existe(client_redis, Ctx, e.Id)
			if !Registrado {
				add_redis(client_redis, Ctx, e)
			}
			e.Username = retorna_nome(client_redis, Ctx, e)
			users := getUserList()
			for _, te := range users {
				teste := strings.Split(te, ":")
				if teste[0] == e.Id {
					igual = true
				}
			}
			if igual {
				response.Action = "duplo"
				response.Message = fmt.Sprintf("Id Já Online")
				duplo(response, e.Conn)
			} else {
				clients[e.Conn] = usuario{e.Id, e.Username, e.Plataforma}
				users = getUserList()
				response.Action = "list_users"
				response.ConnectedUsers = users
				broadcastToAll(response)
			}

			// Desconceta o usuario
		case "left":
			response.Action = "list_users"
			delete(clients, e.Conn)
			users := getUserList()
			response.ConnectedUsers = users
			broadcastToAll(response)

			// envia mensagem para todos os conectados
		case "broadcast":
			response.Action = "broadcast"
			response.Message = fmt.Sprintf("<strong>%s</strong>: %s <br>", e.Username, e.Message)
			broadcastToAll(response)

			// envia mensagem somente para o destinatário especificado
		case "unicast":
			response.Action = "unicast"
			key := ""
			if e.Id < e.Receiver {
				key = "chat:" + e.Id + ":" + e.Receiver
			} else {
				key = "chat:" + e.Receiver + ":" + e.Id
			}
			response.Message = fmt.Sprintf("<strong>%s</strong>: %s <br>", e.Username, e.Message)
			dados := &redis.Z{float64(time.Now().Unix()), response.Message}
			//adicionando conversas no redis
			err := client_redis.ZAdd(Ctx, key, dados).Err()
			if err != nil {
				log.Println(err)
			}
			response.Remetente = e.Id
			toOne(response, e)

		case "duplo":
			response.Action = "duplo"
			response.Message = fmt.Sprintf("Id já online ")
			toOne(response, e)

		case "chat":
			response.Action = "chat"
			log.Println(e)
			messages_list := retornar_chat_redis(client_redis, Ctx, e)
			response.Messages = messages_list
			retorna_chat(response, e)
			response.Messages = nil
		}
		//response.Action = "Aqui"
		//response.Message = fmt.Sprintf("Alguma mensagem, e acao %s", e.Action)
		//broadcastToAll(response)
	}
}

func add_redis(client *redis.Client, Ctx context.Context, e WsPayload) {
	key := "ws:site:" + e.Id
	teste := client.HSet(Ctx, key, "name", e.Username, "Tipo", e.Plataforma)
	fmt.Println(teste)

}

func retornar_chat_redis(client *redis.Client, Ctx context.Context, e WsPayload) []string {
	key := ""
	if e.Id < e.Receiver {
		key = "chat:" + e.Id + ":" + e.Receiver
	} else {
		key = "chat:" + e.Receiver + ":" + e.Id
	}
	teste, err := client_redis.ZRange(Ctx, key, 0, 100).Result()
	if err != nil {
		log.Println(err)
		return nil
	}
	var message []string
	message = teste
	return message
}

func id_existe(client *redis.Client, Ctx context.Context, id string) bool {

	key := "ws:site:" + id
	_, err := client.HGet(Ctx, key, "name").Result()
	if err != nil {
		log.Println("Id não encnontrado")
		return false
	}
	return true
}

func retorna_nome(client *redis.Client, Ctx context.Context, e WsPayload) string {

	key := "ws:site:" + e.Id
	teste, err := client.HGet(Ctx, key, "name").Result()
	if err != nil {
		log.Println("Usuário não encontrado")
		return teste
	}
	return teste
}

func getUserList() []string {
	var userList []string
	for _, x := range clients {
		if x.Name != "" {
			userList = append(userList, x.id+":"+x.Name)
		}
	}
	sort.Strings(userList)
	return userList
}

func retorna_chat(response WsJsonResponse, e WsPayload) {

	for client, value := range clients {
		if value.id == e.Id {
			log.Printf("Numero usuarios %d", e.Id)
			err := client.WriteJSON(response)
			if err != nil {
				log.Println(err)
			}
		}
	}
}

func toOne(response WsJsonResponse, e WsPayload) {
	for client, value := range clients {
		if value.id == e.Receiver || value.id == e.Id {
			err := client.WriteJSON(response)
			if err != nil {
				log.Println("websocket erro")
				_ = client.Close()
				delete(clients, client)
			}
		}
	}
}

func broadcastToAll(response WsJsonResponse) {
	for client := range clients {
		err := client.WriteJSON(response)
		if err != nil {
			log.Println("websocket erro")
			_ = client.Close()
			delete(clients, client)
		}
	}
}

func duplo(response WsJsonResponse, client WebSocketConnection) {
	err := client.WriteJSON(response)
	delete(clients, client)
	if err != nil {
		log.Println("websocket erro")
		_ = client.Close()
		delete(clients, client)
	}
}

func renderPage(w http.ResponseWriter, tmpl string, data jet.VarMap) error {
	view, err := views.GetTemplate(tmpl)
	if err != nil {
		log.Println(err)
		return err
	}
	teste := jet.VarMap{}
	teste.Set("outro", 123)
	err = view.Execute(w, data, teste)
	if err != nil {
		log.Println(err)
		return err
	}

	return nil
}
