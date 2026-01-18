package server

import (
	"context"
	"errors"
	"mime/multipart"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/indieinfra/scribble/config"
	"github.com/indieinfra/scribble/server/state"
	"github.com/indieinfra/scribble/server/util"
	contentpkg "github.com/indieinfra/scribble/storage/content"
	contentfactory "github.com/indieinfra/scribble/storage/content/factory"
	"github.com/indieinfra/scribble/storage/media"
	mediafactory "github.com/indieinfra/scribble/storage/media/factory"
)

type stubContentStore struct{}
type stubMediaStore struct{}

func (stubContentStore) Create(context.Context, util.Mf2Document) (string, bool, error) {
	return "", false, nil
}
func (stubContentStore) Update(context.Context, string, map[string][]any, map[string][]any, any) (string, error) {
	return "", nil
}
func (stubContentStore) Delete(context.Context, string) error { return nil }
func (stubContentStore) Undelete(context.Context, string) (string, bool, error) {
	return "", false, nil
}
func (stubContentStore) Get(context.Context, string) (*util.Mf2Document, error) {
	return &util.Mf2Document{}, nil
}
func (stubContentStore) ExistsBySlug(context.Context, string) (bool, error) { return false, nil }

func (stubMediaStore) Upload(context.Context, *multipart.File, *multipart.FileHeader) (string, error) {
	return "", nil
}
func (stubMediaStore) Delete(context.Context, string) error { return nil }

func TestInitializeContentStore_UsesRegisteredFactory(t *testing.T) {
	strategy := "stub-content"
	contentfactory.Register(strategy, func(cfg *config.Content) (contentpkg.ContentStore, error) {
		return stubContentStore{}, nil
	})

	store, err := initializeContentStore(&config.Content{Strategy: strategy})
	if err != nil {
		t.Fatalf("expected store, got error %v", err)
	}
	if _, ok := store.(stubContentStore); !ok {
		t.Fatalf("unexpected store type: %T", store)
	}
}

func TestInitializeContentStore_Error(t *testing.T) {
	strategy := "error-content"
	contentfactory.Register(strategy, func(cfg *config.Content) (contentpkg.ContentStore, error) {
		return nil, errors.New("failed")
	})

	if _, err := initializeContentStore(&config.Content{Strategy: strategy}); err == nil {
		t.Fatalf("expected error for failing factory")
	}
}

func TestInitializeMediaStore_UsesRegisteredFactory(t *testing.T) {
	strategy := "stub-media"
	mediafactory.Register(strategy, func(cfg *config.Media) (media.MediaStore, error) {
		return stubMediaStore{}, nil
	})

	store, err := initializeMediaStore(&config.Media{Strategy: strategy})
	if err != nil {
		t.Fatalf("expected media store, got %v", err)
	}
	if _, ok := store.(stubMediaStore); !ok {
		t.Fatalf("unexpected media store type: %T", store)
	}
}

func TestInitializeMediaStore_Error(t *testing.T) {
	strategy := "error-media"
	mediafactory.Register(strategy, func(cfg *config.Media) (media.MediaStore, error) {
		return nil, errors.New("failed")
	})

	if _, err := initializeMediaStore(&config.Media{Strategy: strategy}); err == nil {
		t.Fatalf("expected error for failing media factory")
	}
}

func TestCleanupAllowsEmptyGitStore(t *testing.T) {
	st := &state.ScribbleState{ContentStore: &contentpkg.GitContentStore{}}

	cleanup(st)
}

func TestStartServer_FailsWhenInitializationFails(t *testing.T) {
	cfg := &config.Config{
		Content: config.Content{Strategy: "unknown"},
		Media:   config.Media{Strategy: "noop"},
	}

	if err := StartServer(cfg); err == nil {
		t.Fatalf("expected StartServer to fail for unknown strategy")
	}
}

func TestStartServer_ShutsDownOnSignal(t *testing.T) {
	contentfactory.Register("stub-start", func(cfg *config.Content) (contentpkg.ContentStore, error) {
		return stubContentStore{}, nil
	})
	mediafactory.Register("stub-start", func(cfg *config.Media) (media.MediaStore, error) {
		return stubMediaStore{}, nil
	})

	cfg := &config.Config{
		Server: config.Server{Address: "127.0.0.1", Port: 0},
		Micropub: config.Micropub{
			MeUrl:         "https://example.org",
			TokenEndpoint: "https://example.org/token",
		},
		Content: config.Content{Strategy: "stub-start"},
		Media:   config.Media{Strategy: "stub-start"},
	}

	done := make(chan struct{})
	go func() {
		if err := StartServer(cfg); err != nil {
			t.Errorf("StartServer returned error: %v", err)
		}
		close(done)
	}()

	time.Sleep(200 * time.Millisecond)
	proc, _ := os.FindProcess(os.Getpid())
	_ = proc.Signal(syscall.SIGINT)

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatalf("server did not shut down after signal")
	}
}
