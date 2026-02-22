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

	"github.com/jacobsandersen/scribble/config"
	"github.com/jacobsandersen/scribble/server/micropub/micropubget"
	"github.com/jacobsandersen/scribble/server/micropub/micropubpost"
	"github.com/jacobsandersen/scribble/server/micropub/micropubupload"
	"github.com/jacobsandersen/scribble/server/middleware"
	"github.com/jacobsandersen/scribble/server/query"
	"github.com/jacobsandersen/scribble/server/state"
	"github.com/jacobsandersen/scribble/storage/content/factory"
	mediafactory "github.com/jacobsandersen/scribble/storage/media/factory"
	"github.com/jacobsandersen/scribble/storage/util"
)

func StartServer(cfg *config.Config) error {
	log.Println("initializing...")
	st, err := initialize(&state.ScribbleState{Cfg: cfg})
	if err != nil {
		return fmt.Errorf("initialization failed: %w", err)
	}

	server, errChan := runServer(st)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigChan:
		log.Printf("received signal %v, shutting down...", sig)
		shutdownServerGracefully(server)
		return nil
	case err := <-errChan:
		log.Printf("server error: %v, shutting down...", err)
		shutdownServerGracefully(server)
		return err
	}
}

func runServer(st *state.ScribbleState) (*http.Server, chan error) {
	mux := http.NewServeMux()
	mux.Handle("GET /micropub", middleware.ValidateTokenMiddleware(st.Cfg, micropubget.DispatchGet(st)))
	mux.Handle("POST /micropub", middleware.ValidateTokenMiddleware(st.Cfg, micropubpost.DispatchPost(st)))
	mux.Handle("POST /micropub/media", middleware.ValidateTokenMiddleware(st.Cfg, micropubupload.HandleMediaUpload(st)))
	mux.Handle("GET /query/find", query.HandleFind(st))
	mux.Handle("GET /query/list", query.HandleList(st))
	mux.Handle("GET /query/list-categories", query.HandleListCategories(st))

	srv := &http.Server{
		Addr:    st.Cfg.Server.Binding.AddressPort(),
		Handler: mux,
	}

	errChan := make(chan error, 1)

	go func() {
		log.Printf("serving http requests on %q", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- err
		}
	}()

	return srv, errChan
}

func shutdownServerGracefully(srv *http.Server) {
	if srv == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
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
