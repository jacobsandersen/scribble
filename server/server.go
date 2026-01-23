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
	"github.com/indieinfra/scribble/server/handler/get"
	"github.com/indieinfra/scribble/server/handler/post"
	"github.com/indieinfra/scribble/server/handler/upload"
	"github.com/indieinfra/scribble/server/middleware"
	"github.com/indieinfra/scribble/server/state"
	"github.com/indieinfra/scribble/storage/content"
	"github.com/indieinfra/scribble/storage/content/factory"
	"github.com/indieinfra/scribble/storage/content/git"
	"github.com/indieinfra/scribble/storage/media"
	mediafactory "github.com/indieinfra/scribble/storage/media/factory"
)

func StartServer(cfg *config.Config) error {
	log.Println("initializing...")
	st, err := initialize(&state.ScribbleState{Cfg: cfg})
	if err != nil {
		if st != nil && st.ContentStore != nil {
			cleanup(st)
		}
		return fmt.Errorf("initialization failed: %w", err)
	}

	log.Println("configuring routes...")
	mux := http.NewServeMux()
	mux.Handle("GET /", middleware.ValidateTokenMiddleware(st.Cfg, get.DispatchGet(st)))
	mux.Handle("POST /", middleware.ValidateTokenMiddleware(st.Cfg, post.DispatchPost(st)))
	mux.Handle("POST /media", middleware.ValidateTokenMiddleware(st.Cfg, upload.HandleMediaUpload(st)))

	srv := &http.Server{
		Addr:    fmt.Sprintf("%v:%v", st.Cfg.Server.Address, st.Cfg.Server.Port),
		Handler: mux,
	}

	// Start serving in background to support graceful shutdown.
	errChan := make(chan error, 1)
	go func() {
		log.Printf("serving http requests on %q", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- err
		}
	}()

	// Listen for termination signals.
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigChan:
		log.Printf("received signal %v, shutting down...", sig)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("graceful shutdown failed: %v", err)
		}
		cleanup(st)
		return nil
	case err := <-errChan:
		cleanup(st)
		return err
	}
}

func initialize(st *state.ScribbleState) (*state.ScribbleState, error) {
	contentStore, err := initializeContentStore(&st.Cfg.Content)
	if err != nil {
		return nil, err
	}
	st.ContentStore = contentStore

	mediaStore, err := initializeMediaStore(&st.Cfg.Media)
	if err != nil {
		if gitStore, ok := st.ContentStore.(*git.StoreImpl); ok {
			_ = gitStore.Cleanup()
		}
		return nil, err
	}
	st.MediaStore = mediaStore

	return st, nil
}

func initializeContentStore(cfg *config.Content) (content.Store, error) {
	return factory.Create(cfg)
}

func initializeMediaStore(cfg *config.Media) (media.Store, error) {
	return mediafactory.Create(cfg)
}

func cleanup(state *state.ScribbleState) {
	// Cleanup git content store if applicable
	if gitStore, ok := state.ContentStore.(*git.StoreImpl); ok {
		if err := gitStore.Cleanup(); err != nil {
			log.Printf("error during cleanup: %v", err)
		}
	}
}
