package factory

import (
	"context"
	"errors"
	"mime/multipart"
	"testing"

	"github.com/indieinfra/scribble/config"
	"github.com/indieinfra/scribble/storage/media"
)

type fakeMediaStore struct{}

func (fakeMediaStore) Upload(context.Context, *multipart.File, *multipart.FileHeader) (string, error) {
	return "", nil
}
func (fakeMediaStore) Delete(context.Context, string) error { return nil }

func TestRegisterAndGetMediaFactory(t *testing.T) {
	Register("fake-media", func(cfg *config.Media) (media.MediaStore, error) {
		return fakeMediaStore{}, nil
	})

	factory, ok := Get("fake-media")
	if !ok {
		t.Fatalf("expected media factory to be registered")
	}

	store, err := factory(&config.Media{})
	if err != nil {
		t.Fatalf("factory returned error: %v", err)
	}
	if _, ok := store.(fakeMediaStore); !ok {
		t.Fatalf("unexpected store type: %T", store)
	}
}

func TestCreateMediaUnknownStrategy(t *testing.T) {
	cfg := &config.Media{Strategy: "missing"}
	if _, err := Create(cfg); err == nil {
		t.Fatalf("expected error for unknown media strategy")
	}
}

func TestCreateMediaUsesRegisteredFactory(t *testing.T) {
	Register("fake-media-create", func(cfg *config.Media) (media.MediaStore, error) {
		return fakeMediaStore{}, nil
	})

	store, err := Create(&config.Media{Strategy: "fake-media-create"})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if _, ok := store.(fakeMediaStore); !ok {
		t.Fatalf("unexpected store type: %T", store)
	}
}

func TestRegisterMediaReplacesFactory(t *testing.T) {
	Register("replace-media", func(cfg *config.Media) (media.MediaStore, error) {
		return nil, errors.New("first")
	})
	Register("replace-media", func(cfg *config.Media) (media.MediaStore, error) {
		return fakeMediaStore{}, nil
	})

	factory, _ := Get("replace-media")
	store, err := factory(&config.Media{})
	if err != nil {
		t.Fatalf("expected replaced media factory to succeed: %v", err)
	}
	if _, ok := store.(fakeMediaStore); !ok {
		t.Fatalf("unexpected store type: %T", store)
	}
}

func TestBuiltinMediaStrategiesRegistered(t *testing.T) {
	strategies := []string{"noop", "s3", "filesystem"}

	for _, strategy := range strategies {
		t.Run("strategy_"+strategy, func(t *testing.T) {
			factory, ok := Get(strategy)
			if !ok {
				t.Fatalf("expected %q strategy to be registered", strategy)
			}
			if factory == nil {
				t.Fatalf("expected non-nil factory for %q", strategy)
			}
		})
	}
}

func TestCreateNoopMediaStore(t *testing.T) {
	cfg := &config.Media{Strategy: "noop"}
	store, err := Create(cfg)
	if err != nil {
		t.Fatalf("expected noop store to be created, got error: %v", err)
	}
	if store == nil {
		t.Fatal("expected non-nil store")
	}
	// Verify it's a NoopMediaStore
	if _, ok := store.(*media.NoopMediaStore); !ok {
		t.Fatalf("expected NoopMediaStore, got %T", store)
	}
}

func TestCreateS3MediaStore_MissingConfig(t *testing.T) {
	cfg := &config.Media{
		Strategy: "s3",
		S3:       nil,
	}

	_, err := Create(cfg)
	if err == nil {
		t.Fatal("expected error when S3 config is nil")
	}
}

func TestCreateFilesystemMediaStore_MissingConfig(t *testing.T) {
	cfg := &config.Media{
		Strategy:   "filesystem",
		Filesystem: nil,
	}

	_, err := Create(cfg)
	if err == nil {
		t.Fatal("expected error when Filesystem config is nil")
	}
}

func TestCreateFilesystemMediaStore_Success(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Media{
		Strategy: "filesystem",
		Filesystem: &config.FilesystemMediaStrategy{
			Path:      tmpDir,
			PublicUrl: "https://example.org/media",
		},
	}

	store, err := Create(cfg)
	if err != nil {
		t.Fatalf("expected filesystem media store to be created, got error: %v", err)
	}

	if store == nil {
		t.Fatal("expected non-nil store")
	}

	// Verify the store implements MediaStore interface
	var _ media.MediaStore = store
}
