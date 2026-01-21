package factory

import (
	"fmt"
	"sync"

	"github.com/indieinfra/scribble/config"
	"github.com/indieinfra/scribble/storage/content"
)

// Factory builds a content store for the provided content config.
type Factory func(*config.Content) (content.ContentStore, error)

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
func Create(cfg *config.Content) (content.ContentStore, error) {
	f, ok := Get(cfg.Strategy)
	if !ok {
		return nil, fmt.Errorf("unknown content strategy %q", cfg.Strategy)
	}
	return f(cfg)
}

func init() {
	Register("git", func(cfg *config.Content) (content.ContentStore, error) {
		return content.NewGitContentStore(cfg.Git)
	})

	Register("sql", func(cfg *config.Content) (content.ContentStore, error) {
		return content.NewSQLContentStore(cfg.SQL)
	})

	Register("d1", func(cfg *config.Content) (content.ContentStore, error) {
		return content.NewD1ContentStore(cfg.D1)
	})

	Register("filesystem", func(cfg *config.Content) (content.ContentStore, error) {
		return content.NewFilesystemContentStore(cfg.Filesystem)
	})
}
