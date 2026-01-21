package factory

import (
	"fmt"
	"sync"

	"github.com/indieinfra/scribble/config"
	"github.com/indieinfra/scribble/storage/media"
)

// Factory builds a media store for the provided media config.
type Factory func(*config.Media) (media.MediaStore, error)

var (
	mu       sync.RWMutex
	registry = map[string]Factory{}
)

// Register adds or replaces a media store factory for the given strategy name.
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

// Create builds a media store using the registered factory for the configured strategy.
func Create(cfg *config.Media) (media.MediaStore, error) {
	if f, ok := Get(cfg.Strategy); ok {
		return f(cfg)
	}

	return nil, fmt.Errorf("unknown media strategy %q", cfg.Strategy)
}

func init() {
	Register("noop", func(cfg *config.Media) (media.MediaStore, error) {
		return &media.NoopMediaStore{}, nil
	})
	Register("s3", func(cfg *config.Media) (media.MediaStore, error) {
		return media.NewS3MediaStore(cfg)
	})
	Register("filesystem", func(cfg *config.Media) (media.MediaStore, error) {
		return media.NewFilesystemMediaStore(cfg.Filesystem)
	})
}
