package main

import (
	"github.com/bmizerany/pat"
	"net/http"
	"ws/internal/handlers"
)

func routes() http.Handler {
	mux := pat.New()

	mux.Get("/login", http.HandlerFunc(handlers.Login))
	mux.Post("/", http.HandlerFunc(handlers.Valida_login))
	mux.Post("/", http.HandlerFunc(handlers.Home))
	mux.Get("/ws", http.HandlerFunc(handlers.WsEndpoint))

	fileServer := http.FileServer(http.Dir(("./static/")))
	mux.Get("/static/", http.StripPrefix("/static", fileServer))
	return mux
}
