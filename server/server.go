package server

import (
	"fmt"
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

func StartServer(cfg *config.Config) {
	mux := http.NewServeMux()
	mux.Handle("GET /", middleware.ValidateTokenMiddleware(cfg, get.DispatchGet(cfg)))
	mux.Handle("POST /", middleware.ValidateTokenMiddleware(cfg, post.DispatchPost(cfg)))
	mux.Handle("POST /media", middleware.ValidateTokenMiddleware(cfg, upload.HandleMediaUpload(cfg)))

	content.ActiveContentStore = &content.NoopContentStore{}
	media.ActiveMediaStore = &media.NoopMediaStore{}

	bindAddress := fmt.Sprintf("%v:%v", cfg.Server.Address, cfg.Server.Port)
	log.Printf("serving http requests on %q", bindAddress)
	log.Fatal(http.ListenAndServe(bindAddress, mux))
}
