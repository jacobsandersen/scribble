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
