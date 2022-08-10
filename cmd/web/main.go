package main

import (
	"log"
	"net/http"
	"ws/internal/handlers"
)

func main() {
	mux := routes()

	log.Println("Ouvindo o canal")
	go handlers.ListenToWsChannel()

	log.Println("WEB SERVICE ON")

	_ = http.ListenAndServe(":8083", mux)
}
