package server

import (
	"log"
	"net/http"

	"github.com/indieinfra/scribble/config"
	"github.com/indieinfra/scribble/server/handler/get"
	"github.com/indieinfra/scribble/server/handler/post"
	"github.com/indieinfra/scribble/server/handler/upload"
	"github.com/indieinfra/scribble/server/middleware"
	"github.com/indieinfra/scribble/storage/content"
	"github.com/indieinfra/scribble/storage/media"
)

func StartServer() {
	mux := http.NewServeMux()
	mux.Handle("GET /", middleware.ValidateTokenMiddleware(http.HandlerFunc(get.DispatchGet)))
	mux.Handle("POST /", middleware.ValidateTokenMiddleware(http.HandlerFunc(post.DispatchPost)))
	mux.Handle("POST /media", middleware.ValidateTokenMiddleware(http.HandlerFunc(upload.HandleMediaUpload)))

	content.ActiveContentStore = &content.NoopContentStore{}
	media.ActiveMediaStore = &media.NoopMediaStore{}

	bindAddress := config.BindAddress()
	log.Printf("serving http requests on %q", bindAddress)
	log.Fatal(http.ListenAndServe(bindAddress, mux))
}
