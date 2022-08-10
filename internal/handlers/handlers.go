package handlers

import (
	"fmt"
	"github.com/CloudyKit/jet/v6"
	"github.com/fasthttp/websocket"
	"log"
	"math/rand"
	"net/http"
	"reflect"
	"sort"
)

var wsChan = make(chan WsPayload)

var clients = make(map[WebSocketConnection]string)

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
	err := renderPage(w, "home.jet", nil)
	if err != nil {
		log.Println(err)
	}
}

type WebSocketConnection struct {
	*websocket.Conn
}

// defini a resposta enviada para o websocket
type WsJsonResponse struct {
	Action         string   `json:"action"`
	Message        string   `json:"message"`
	MessageType    string   `json:"message_type"`
	Type           string   `json:"type"`
	ConnectedUsers []string `json:"connected_users"`
}

type WsPayload struct {
	Action   string              `json:"action"`
	Username string              `json:"username"`
	Message  string              `json:"message"`
	Type     string              `json:"type"`
	Conn     WebSocketConnection `json:"-"`
}

//realiza o upgrade da conexão para websocket
func WsEndpoint(w http.ResponseWriter, r *http.Request) {
	ws, err := upgradeConnection.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
	}
	log.Println("EVA CONECTADO")
	log.Println(r)
	var response WsJsonResponse
	response.Message = `<em><small>Conexao bem sucedida<small><em>`

	conn := WebSocketConnection{Conn: ws}
	clients[conn] = ""
	log.Println(conn)
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
		log.Println(payload)
		if err != nil {
		} else {
			payload.Conn = *conn
			wsChan <- payload
		}
	}
}

func ListenToWsChannel() {
	var response WsJsonResponse
	for {
		e := <-wsChan
		switch e.Action {
		case "username":
			//obtem a lista de todos os usuarios e envia via broadcast
			clients[e.Conn] = e.Username
			users := getUserList()
			response.Action = "list_users"
			response.ConnectedUsers = users
			broadcastToAll(response)

		case "left":
			response.Action = "list_users"
			delete(clients, e.Conn)
			users := getUserList()
			response.ConnectedUsers = users
			broadcastToAll(response)

		case "broadcast":
			response.Action = "broadcast"
			response.Message = fmt.Sprintf("<strong>%s</strong>: %s", e.Username, e.Message)
			toOne(response)
		}
		//response.Action = "Aqui"
		//response.Message = fmt.Sprintf("Alguma mensagem, e acao %s", e.Action)
		//broadcastToAll(response)
	}
}

func getUserList() []string {
	var userList []string
	for _, x := range clients {
		if x != "" {
			userList = append(userList, x)
		}
	}
	sort.Strings(userList)
	return userList
}
func MapRandomKeyGet(mapI interface{}) interface{} {
	keys := reflect.ValueOf(mapI).MapKeys()

	return keys[rand.Intn(len(keys))].Interface()
}

func toOne(response WsJsonResponse) {
	log.Printf("Numero usuarios %d", len(clients))
	teste := clients[MapRandomKeyGet(clients).(WebSocketConnection)]
	log.Println(teste)
	for client, value := range clients {
		if value == teste {
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
	log.Printf("Numero usuarios %d", len(clients))
	for client := range clients {
		err := client.WriteJSON(response)
		if err != nil {
			log.Println("websocket erro")
			_ = client.Close()
			delete(clients, client)
		}
	}
}

func renderPage(w http.ResponseWriter, tmpl string, data jet.VarMap) error {
	view, err := views.GetTemplate(tmpl)
	if err != nil {
		log.Println(err)
		return err
	}

	err = view.Execute(w, data, nil)
	if err != nil {
		log.Println(err)
		return err
	}

	return nil
}
