package server

import (
	"log"
	"net/http"

	"github.com/indieinfra/scribble/config"
	"github.com/indieinfra/scribble/server/handler/get"
	"github.com/indieinfra/scribble/server/handler/post"
	"github.com/indieinfra/scribble/server/handler/upload"
	"github.com/indieinfra/scribble/server/middleware"
)

func StartServer() {
	mux := http.NewServeMux()
	mux.Handle("GET /", http.HandlerFunc(get.DispatchGet))
	mux.Handle("POST /", middleware.ValidateTokenMiddleware(http.HandlerFunc(post.DispatchPost)))
	mux.Handle("POST /media", middleware.ValidateTokenMiddleware(http.HandlerFunc(upload.HandleMediaUpload)))

	bindAddress := config.BindAddress()
	log.Printf("serving http requests on %q", bindAddress)
	log.Fatal(http.ListenAndServe(bindAddress, mux))
}
