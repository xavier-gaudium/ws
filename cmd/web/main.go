package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"ws/internal/handlers"
)

var (
	newFile *os.File
	err     error
)

func main() {
	mux := routes()

	log.Println("Ouvindo o canal")
	go handlers.ListenToWsChannel()

	log.Println("WEB SERVICE ON")

	fmt.Println("We are testing Go-Redis")
	_ = http.ListenAndServe(":8099", mux)

	//_ = http.ListenAndServeTLS(":443", "certs/Certificate.crt", "certs/Key.key", mux)
	//log.Println(err)
}
