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

func TestBuiltinStrategiesRegistered(t *testing.T) {
	strategies := []string{"git", "sql", "d1", "filesystem"}

	for _, strategy := range strategies {
		t.Run(strategy, func(t *testing.T) {
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

func TestCreateGitContentStore_InvalidConfig(t *testing.T) {
	cfg := &config.Content{
		Strategy: "git",
		Git: &config.GitContentStrategy{
			Repository: "not-a-valid-url",
			Path:       "content",
			PublicUrl:  "https://example.org",
			Auth: config.GitContentStrategyAuth{
				Method: "plain",
				Plain: &config.UsernamePasswordAuth{
					Username: "user",
					Password: "pass",
				},
			},
		},
	}

	_, err := Create(cfg)
	if err == nil {
		t.Fatal("expected error when Git config has invalid repository URL")
	}
}

func TestCreateSQLContentStore_InvalidDSN(t *testing.T) {
	cfg := &config.Content{
		Strategy: "sql",
		SQL: &config.SQLContentStrategy{
			Driver:    "postgres",
			DSN:       "invalid-dsn",
			PublicUrl: "https://example.org",
		},
	}

	_, err := Create(cfg)
	if err == nil {
		t.Fatal("expected error when SQL config has invalid DSN")
	}
}

func TestCreateD1ContentStore_MissingConfig(t *testing.T) {
	cfg := &config.Content{
		Strategy: "d1",
		D1:       nil,
	}

	_, err := Create(cfg)
	if err == nil {
		t.Fatal("expected error when D1 config is nil")
	}
}

func TestCreateFilesystemContentStore_MissingConfig(t *testing.T) {
	cfg := &config.Content{
		Strategy:   "filesystem",
		Filesystem: nil,
	}

	_, err := Create(cfg)
	if err == nil {
		t.Fatal("expected error when Filesystem config is nil")
	}
}

func TestCreateFilesystemContentStore_Success(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Content{
		Strategy: "filesystem",
		Filesystem: &config.FilesystemContentStrategy{
			Path:      tmpDir,
			PublicUrl: "https://example.org/content",
		},
	}

	store, err := Create(cfg)
	if err != nil {
		t.Fatalf("expected filesystem store to be created, got error: %v", err)
	}

	if store == nil {
		t.Fatal("expected non-nil store")
	}

	// Verify the store implements ContentStore interface
	var _ content.ContentStore = store
}
