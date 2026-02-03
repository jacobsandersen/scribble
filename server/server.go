package server

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/indieinfra/scribble/config"
	micropubget "github.com/indieinfra/scribble/server/handler/micropub/get"
	micropubpost "github.com/indieinfra/scribble/server/handler/micropub/post"
	micropubupload "github.com/indieinfra/scribble/server/handler/micropub/upload"
	queryget "github.com/indieinfra/scribble/server/handler/query/get"
	"github.com/indieinfra/scribble/server/middleware"
	"github.com/indieinfra/scribble/server/state"
	"github.com/indieinfra/scribble/storage/content/factory"
	mediafactory "github.com/indieinfra/scribble/storage/media/factory"
	"github.com/indieinfra/scribble/storage/util"
)

func StartServer(cfg *config.Config) error {
	log.Println("initializing...")
	st, err := initialize(&state.ScribbleState{Cfg: cfg})
	if err != nil {
		return fmt.Errorf("initialization failed: %w", err)
	}

	errChan := make(chan error, 1)
	micropubServer := runMicropubServer(st, errChan)
	queryServer := runQueryServer(st, errChan)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigChan:
		log.Printf("received signal %v, shutting down...", sig)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		for _, srv := range []*http.Server{micropubServer, queryServer} {
			if srv == nil {
				continue
			}
			if err := srv.Shutdown(ctx); err != nil {
				log.Printf("graceful shutdown failed: %v", err)
			}
		}

		return nil
	case err := <-errChan:
		return err
	}
}

func runMicropubServer(st *state.ScribbleState, errChan chan error) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("GET /", middleware.ValidateTokenMiddleware(st.Cfg, micropubget.DispatchGet(st)))
	mux.Handle("POST /", middleware.ValidateTokenMiddleware(st.Cfg, micropubpost.DispatchPost(st)))
	mux.Handle("POST /media", middleware.ValidateTokenMiddleware(st.Cfg, micropubupload.HandleMediaUpload(st)))

	micropubBinding := st.Cfg.Server.Micropub.Server

	srv := &http.Server{
		Addr:    fmt.Sprintf("%v:%v", micropubBinding.Address, micropubBinding.Port),
		Handler: mux,
	}

	return runServer("micropub", mux, srv, errChan)
}

func runQueryServer(st *state.ScribbleState, errChan chan error) *http.Server {
	if !st.Cfg.Server.QueryServer.Enabled {
		return nil
	}

	mux := http.NewServeMux()
	mux.Handle("GET /", queryget.DispatchGet(st))

	queryBinding := st.Cfg.Server.QueryServer.Server

	srv := &http.Server{
		Addr:    fmt.Sprintf("%v:%v", queryBinding.Address, queryBinding.Port),
		Handler: mux,
	}

	return runServer("query", mux, srv, errChan)
}

func runServer(typ string, mux *http.ServeMux, srv *http.Server, errChan chan error) *http.Server {
	go func() {
		log.Printf("serving %s http requests on %q", typ, srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- err
		}
	}()

	return srv
}

func initialize(st *state.ScribbleState) (*state.ScribbleState, error) {
	pat, err := util.NewContentPathPattern(st.Cfg.Content.ContentUrl)
	if err != nil {
		return nil, fmt.Errorf("invalid content path pattern: %w", err)
	}
	st.ContentPathPattern = pat

	pat, err = util.NewMediaPathPattern(st.Cfg.Media.MediaUrl)
	if err != nil {
		return nil, fmt.Errorf("invalid media path pattern: %w", err)
	}
	st.MediaPathPattern = pat

	contentStore, err := factory.Create(&st.Cfg.Content)
	if err != nil {
		return nil, err
	}
	st.ContentStore = contentStore

	mediaStore, err := mediafactory.Create(&st.Cfg.Media)
	if err != nil {
		return nil, err
	}
	st.MediaStore = mediaStore

	return st, nil
}
