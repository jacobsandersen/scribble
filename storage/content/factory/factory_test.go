package factory

import (
	"context"
	"errors"
	"testing"

	"github.com/indieinfra/scribble/config"
	"github.com/indieinfra/scribble/server/util"
	"github.com/indieinfra/scribble/storage/content"
)

type fakeContentStore struct{}

func (fakeContentStore) Create(context.Context, util.Mf2Document) (string, bool, error) {
	return "", false, nil
}
func (fakeContentStore) Update(context.Context, string, map[string][]any, map[string][]any, any) (string, error) {
	return "", nil
}
func (fakeContentStore) Delete(context.Context, string) error { return nil }
func (fakeContentStore) Undelete(context.Context, string) (string, bool, error) {
	return "", false, nil
}
func (fakeContentStore) Get(context.Context, string) (*util.Mf2Document, error) {
	return &util.Mf2Document{}, nil
}
func (fakeContentStore) ExistsBySlug(context.Context, string) (bool, error) { return true, nil }

func TestRegisterAndGet(t *testing.T) {
	Register("fake", func(cfg *config.Content) (content.ContentStore, error) {
		return fakeContentStore{}, nil
	})

	factory, ok := Get("fake")
	if !ok {
		t.Fatalf("expected factory to be registered")
	}

	store, err := factory(&config.Content{})
	if err != nil {
		t.Fatalf("factory returned error: %v", err)
	}
	if _, ok := store.(fakeContentStore); !ok {
		t.Fatalf("unexpected store type: %T", store)
	}
}

func TestCreateUnknownStrategy(t *testing.T) {
	cfg := &config.Content{Strategy: "missing"}
	if _, err := Create(cfg); err == nil {
		t.Fatalf("expected error for unknown strategy")
	}
}

func TestCreateUsesRegisteredFactory(t *testing.T) {
	Register("fake-create", func(cfg *config.Content) (content.ContentStore, error) {
		return fakeContentStore{}, nil
	})

	store, err := Create(&config.Content{Strategy: "fake-create"})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if _, ok := store.(fakeContentStore); !ok {
		t.Fatalf("unexpected store type: %T", store)
	}
}

func TestRegisterCanReplaceFactory(t *testing.T) {
	Register("replace", func(cfg *config.Content) (content.ContentStore, error) {
		return nil, errors.New("first")
	})
	Register("replace", func(cfg *config.Content) (content.ContentStore, error) {
		return fakeContentStore{}, nil
	})

	factory, _ := Get("replace")
	store, err := factory(&config.Content{})
	if err != nil {
		t.Fatalf("expected replaced factory to succeed: %v", err)
	}
	if _, ok := store.(fakeContentStore); !ok {
		t.Fatalf("unexpected store type: %T", store)
	}
}
