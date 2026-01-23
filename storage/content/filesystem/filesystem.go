package filesystem

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/indieinfra/scribble/config"
	"github.com/indieinfra/scribble/server/util"
	"github.com/indieinfra/scribble/storage/content"
	storageutil "github.com/indieinfra/scribble/storage/util"
)

// StoreImpl stores mf2 documents as JSON files in a local directory.
type StoreImpl struct {
	cfg        *config.FilesystemContentStrategy
	basePath   string
	publicURL  string
	pattern    *storageutil.PathPattern
	slugToPath map[string]string // Maps slug to relative file path
	pathToSlug map[string]string // Maps relative file path to slug
	mu         sync.RWMutex      // Protects the maps and file operations
}

// NewFilesystemContentStore creates a new filesystem-based content store.
func NewFilesystemContentStore(cfg *config.FilesystemContentStrategy) (*StoreImpl, error) {
	if cfg == nil {
		return nil, fmt.Errorf("filesystem content config is nil")
	}

	// Ensure base path exists
	if err := os.MkdirAll(cfg.Path, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base directory: %w", err)
	}

	// Set up path pattern
	pattern := storageutil.DefaultContentPattern()
	if cfg.PathPattern != "" {
		pattern = storageutil.NewPathPattern(cfg.PathPattern)
	}

	store := &StoreImpl{
		cfg:        cfg,
		basePath:   cfg.Path,
		publicURL:  storageutil.NormalizeBaseURL(cfg.PublicUrl),
		pattern:    pattern,
		slugToPath: make(map[string]string),
		pathToSlug: make(map[string]string),
	}

	// Build initial index
	if err := store.rebuildIndex(); err != nil {
		return nil, fmt.Errorf("failed to build index: %w", err)
	}

	return store, nil
}

// rebuildIndex scans the filesystem and builds the slug-to-path index.
func (fs *StoreImpl) rebuildIndex() error {
	fs.slugToPath = make(map[string]string)
	fs.pathToSlug = make(map[string]string)

	return filepath.WalkDir(fs.basePath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || filepath.Ext(path) != ".json" {
			return nil
		}

		relPath, err := filepath.Rel(fs.basePath, path)
		if err != nil {
			return err
		}

		// Read file to extract slug
		data, err := os.ReadFile(path)
		if err != nil {
			log.Printf("warning: failed to read %s during index rebuild: %v", relPath, err)
			return nil
		}

		var doc util.Mf2Document
		if err := json.Unmarshal(data, &doc); err != nil {
			log.Printf("warning: failed to unmarshal %s during index rebuild: %v", relPath, err)
			return nil
		}

		slug, err := content.ExtractSlug(doc)
		if err != nil {
			log.Printf("warning: no slug in %s during index rebuild: %v", relPath, err)
			return nil
		}

		fs.slugToPath[slug] = relPath
		fs.pathToSlug[relPath] = slug

		return nil
	})
}

func (fs *StoreImpl) Create(ctx context.Context, doc util.Mf2Document) (string, bool, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	slug, err := content.ExtractSlug(doc)
	if err != nil {
		return "", false, err
	}

	// Ensure slug is unique (using unlocked check since we already hold the lock)
	uniqueSlug, err := fs.ensureUniqueSlugUnlocked(slug, "")
	if err != nil {
		return "", false, err
	}

	// Update the document with the unique slug if it changed
	if uniqueSlug != slug {
		doc.Properties["slug"] = []any{uniqueSlug}
	}

	// Generate file path
	now := time.Now()
	relPath, err := fs.pattern.Generate(uniqueSlug, now, "")
	if err != nil {
		return "", false, err
	}

	absPath := filepath.Join(fs.basePath, relPath)

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
		return "", false, fmt.Errorf("failed to create directory: %w", err)
	}

	// Marshal and write
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return "", false, err
	}

	if err := os.WriteFile(absPath, data, 0644); err != nil {
		return "", false, err
	}

	// Update index
	fs.slugToPath[uniqueSlug] = relPath
	fs.pathToSlug[relPath] = uniqueSlug

	return fs.publicURL + uniqueSlug, true, nil
}

func (fs *StoreImpl) Update(ctx context.Context, url string, replacements map[string][]any, additions map[string][]any, deletions any) (string, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	oldSlug, err := util.SlugFromURL(url)
	if err != nil {
		return url, err
	}

	oldPath, exists := fs.slugToPath[oldSlug]
	if !exists {
		return url, content.ErrNotFound
	}

	absOldPath := filepath.Join(fs.basePath, oldPath)

	// Read existing document
	data, err := os.ReadFile(absOldPath)
	if err != nil {
		if os.IsNotExist(err) {
			return url, content.ErrNotFound
		}
		return url, err
	}

	var doc util.Mf2Document
	if err := json.Unmarshal(data, &doc); err != nil {
		return url, err
	}

	// Apply mutations
	content.ApplyMutations(&doc, replacements, additions, deletions)

	// Check if slug needs recomputation
	var newSlug string
	if content.ShouldRecomputeSlug(replacements, additions) {
		proposedSlug, err := content.ComputeNewSlug(&doc, replacements)
		if err != nil {
			return url, err
		}

		// Ensure uniqueness (using unlocked check since we already hold the lock)
		newSlug, err = fs.ensureUniqueSlugUnlocked(proposedSlug, oldSlug)
		if err != nil {
			return url, err
		}

		doc.Properties["slug"] = []any{newSlug}
	} else {
		newSlug = oldSlug
	}

	newURL := fs.publicURL + newSlug

	// Marshal document
	data, err = json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return url, err
	}

	// Handle slug change (requires file rename)
	if newSlug != oldSlug {
		now := time.Now()
		newRelPath, err := fs.pattern.Generate(newSlug, now, "")
		if err != nil {
			return url, err
		}

		absNewPath := filepath.Join(fs.basePath, newRelPath)

		// Ensure parent directory exists
		if err := os.MkdirAll(filepath.Dir(absNewPath), 0755); err != nil {
			return url, fmt.Errorf("failed to create directory: %w", err)
		}

		// Write new file first
		if err := os.WriteFile(absNewPath, data, 0644); err != nil {
			return url, err
		}

		// Remove old file
		if err := os.Remove(absOldPath); err != nil {
			// Attempt cleanup
			_ = os.Remove(absNewPath)
			return url, fmt.Errorf("failed to remove old file: %w", err)
		}

		// Update index
		delete(fs.slugToPath, oldSlug)
		delete(fs.pathToSlug, oldPath)
		fs.slugToPath[newSlug] = newRelPath
		fs.pathToSlug[newRelPath] = newSlug

		return newURL, nil
	}

	// No slug change - update in place
	if err := os.WriteFile(absOldPath, data, 0644); err != nil {
		return url, err
	}

	return url, nil
}

func (fs *StoreImpl) Delete(ctx context.Context, url string) error {
	_, err := fs.setDeletedStatus(ctx, url, true)
	return err
}

func (fs *StoreImpl) Undelete(ctx context.Context, url string) (string, bool, error) {
	newURL, err := fs.setDeletedStatus(ctx, url, false)
	return newURL, false, err
}

func (fs *StoreImpl) setDeletedStatus(ctx context.Context, url string, deleted bool) (string, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	slug, err := util.SlugFromURL(url)
	if err != nil {
		return url, err
	}

	relPath, exists := fs.slugToPath[slug]
	if !exists {
		return url, content.ErrNotFound
	}

	absPath := filepath.Join(fs.basePath, relPath)

	data, err := os.ReadFile(absPath)
	if err != nil {
		return url, err
	}

	var doc util.Mf2Document
	if err := json.Unmarshal(data, &doc); err != nil {
		return url, err
	}

	if deleted {
		// Set deleted property to true
		doc.Properties["deleted"] = []any{true}
	} else {
		// Remove deleted property entirely
		delete(doc.Properties, "deleted")
	}

	data, err = json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return url, err
	}

	if err := os.WriteFile(absPath, data, 0644); err != nil {
		return url, err
	}

	return fs.publicURL + slug, nil
}

func (fs *StoreImpl) Get(ctx context.Context, url string) (*util.Mf2Document, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	slug, err := util.SlugFromURL(url)
	if err != nil {
		return nil, err
	}

	relPath, exists := fs.slugToPath[slug]
	if !exists {
		return nil, content.ErrNotFound
	}

	absPath := filepath.Join(fs.basePath, relPath)

	data, err := os.ReadFile(absPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, content.ErrNotFound
		}
		return nil, err
	}

	var doc util.Mf2Document
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}

	return &doc, nil
}

func (fs *StoreImpl) ExistsBySlug(ctx context.Context, slug string) (bool, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	return fs.existsBySlugUnlocked(slug), nil
}

// existsBySlugUnlocked checks if a slug exists WITHOUT acquiring the mutex.
// Must be called only when the caller already holds the mutex (read or write lock).
func (fs *StoreImpl) existsBySlugUnlocked(slug string) bool {
	_, exists := fs.slugToPath[slug]
	return exists
}

// ensureUniqueSlugUnlocked checks if the proposed slug already exists (excluding the old slug).
// If it does, appends a UUID suffix to make it unique. Returns the final unique slug.
// Must be called only when the caller already holds the mutex (write lock).
func (fs *StoreImpl) ensureUniqueSlugUnlocked(proposedSlug, oldSlug string) (string, error) {
	// If the slug didn't actually change, no collision possible
	if proposedSlug == oldSlug {
		return proposedSlug, nil
	}

	// Check if the proposed slug already exists
	if !fs.existsBySlugUnlocked(proposedSlug) {
		return proposedSlug, nil
	}

	// Collision detected - append UUID to make it unique
	uniqueSlug := fmt.Sprintf("%s-%s", proposedSlug, uuid.New().String())

	// Sanity check: verify the UUID-suffixed slug doesn't exist either
	// (extremely unlikely but theoretically possible)
	if fs.existsBySlugUnlocked(uniqueSlug) {
		// This should never happen in practice, but if it does, fail safely
		return "", fmt.Errorf("slug collision persists even after UUID suffix: %s", uniqueSlug)
	}

	return uniqueSlug, nil
}
