package factory

import (
	"fmt"
	"sync"

	"github.com/indieinfra/scribble/config"
	"github.com/indieinfra/scribble/storage/content"
	"github.com/indieinfra/scribble/storage/content/d1"
	"github.com/indieinfra/scribble/storage/content/filesystem"
	"github.com/indieinfra/scribble/storage/content/git"
	"github.com/indieinfra/scribble/storage/content/sql"
)

// Factory builds a content store for the provided content config.
type Factory func(*config.Content) (content.Store, error)

var (
	mu       sync.RWMutex
	registry = map[string]Factory{}
)

// Register adds or replaces a content store factory for the given strategy name.
func Register(strategy string, factory Factory) {
	mu.Lock()
	registry[strategy] = factory
	mu.Unlock()
}

// Get retrieves a factory for the given strategy.
func Get(strategy string) (Factory, bool) {
	mu.RLock()
	f, ok := registry[strategy]
	mu.RUnlock()
	return f, ok
}

// Create builds a content store using the registered factory for the configured strategy.
func Create(cfg *config.Content) (content.Store, error) {
	f, ok := Get(cfg.Strategy)
	if !ok {
		return nil, fmt.Errorf("unknown content strategy %q", cfg.Strategy)
	}
	return f(cfg)
}

func init() {
	Register("git", func(cfg *config.Content) (content.Store, error) {
		return git.NewGitContentStore(cfg.Git)
	})

	Register("sql", func(cfg *config.Content) (content.Store, error) {
		return sql.NewSQLContentStore(cfg.SQL)
	})

	Register("d1", func(cfg *config.Content) (content.Store, error) {
		return d1.NewD1ContentStore(cfg.D1)
	})

	Register("filesystem", func(cfg *config.Content) (content.Store, error) {
		return filesystem.NewFilesystemContentStore(cfg.Filesystem)
	})
}
