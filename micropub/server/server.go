package server

import (
	"log"
	"net/http"

	"github.com/indieinfra/scribble/config"
	"github.com/indieinfra/scribble/micropub/auth"
	"github.com/indieinfra/scribble/micropub/server/handler"
)

func StartServer() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", handler.HandleMicropub)
	mux.Handle("POST /", auth.ValidateTokenMiddleware(http.HandlerFunc(handler.HandleMicropub)))

	bindAddress := config.BindAddress()
	log.Printf("serving http requests on %q", bindAddress)
	log.Fatal(http.ListenAndServe(bindAddress, mux))
}
