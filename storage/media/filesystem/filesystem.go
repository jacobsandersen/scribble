package filesystem

import (
	"context"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/indieinfra/scribble/config"
	storageutil "github.com/indieinfra/scribble/storage/util"
)

// StoreImpl stores uploaded media files in a local directory.
type StoreImpl struct {
	basePath  string
	publicURL string
	pattern   *storageutil.PathPattern
	mu        sync.RWMutex // Protects file operations
}

// NewFilesystemMediaStore creates a new filesystem-based media store.
func NewFilesystemMediaStore(cfg *config.FilesystemMediaStrategy) (*StoreImpl, error) {
	if cfg == nil {
		return nil, fmt.Errorf("filesystem media config is nil")
	}

	// Ensure base path exists
	if err := os.MkdirAll(cfg.Path, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base directory: %w", err)
	}

	// Set up path pattern
	pattern := storageutil.DefaultMediaPattern()
	if cfg.PathPattern != "" {
		pattern = storageutil.NewPathPattern(cfg.PathPattern)
	}

	return &StoreImpl{
		basePath:  cfg.Path,
		publicURL: storageutil.NormalizeBaseURL(cfg.PublicUrl),
		pattern:   pattern,
	}, nil
}

// Upload saves the provided file to the filesystem and returns its public URL.
func (fs *StoreImpl) Upload(ctx context.Context, file *multipart.File, header *multipart.FileHeader) (string, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	filename := header.Filename
	contentType := header.Header.Get("Content-Type")

	// Determine file extension
	ext := filepath.Ext(filename)
	if ext == "" && contentType != "" {
		// Try to derive extension from content type
		exts, err := mime.ExtensionsByType(contentType)
		if err == nil && len(exts) > 0 {
			ext = exts[0]
		}
	}

	// Generate a unique filename if needed
	baseFilename := strings.TrimSuffix(filename, ext)
	if baseFilename == "" {
		baseFilename = uuid.New().String()
	}

	// Generate file path using pattern
	now := time.Now()
	relPath, err := fs.pattern.Generate(baseFilename, now, ext)
	if err != nil {
		return "", fmt.Errorf("failed to generate path: %w", err)
	}

	absPath := filepath.Join(fs.basePath, relPath)

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// Check if file already exists - if so, make filename unique
	if _, err := os.Stat(absPath); err == nil {
		// File exists - append UUID to make unique
		baseFilename = fmt.Sprintf("%s-%s", baseFilename, uuid.New().String()[:8])
		relPath, err = fs.pattern.Generate(baseFilename, now, ext)
		if err != nil {
			return "", fmt.Errorf("failed to generate unique path: %w", err)
		}
		absPath = filepath.Join(fs.basePath, relPath)

		// Ensure directory still exists (pattern may have changed)
		if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
			return "", fmt.Errorf("failed to create unique directory: %w", err)
		}
	}

	// Create the file
	outFile, err := os.Create(absPath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer outFile.Close()

	// Copy data from multipart file to output file
	if _, err := io.Copy(outFile, *file); err != nil {
		// Attempt to clean up partial file
		_ = os.Remove(absPath)
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	// Construct public URL
	// relPath uses OS-specific separators; convert to URL path separators
	urlPath := filepath.ToSlash(relPath)
	publicURL := fs.publicURL + urlPath

	return publicURL, nil
}

// Delete removes a media file from the filesystem.
func (fs *StoreImpl) Delete(ctx context.Context, url string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	// Extract relative path from URL
	if !strings.HasPrefix(url, fs.publicURL) {
		return fmt.Errorf("url %q does not match public URL prefix %q", url, fs.publicURL)
	}

	relPath := strings.TrimPrefix(url, fs.publicURL)
	// Convert URL path separators to OS-specific separators
	relPath = filepath.FromSlash(relPath)

	absPath := filepath.Join(fs.basePath, relPath)

	// Check if file exists
	if _, err := os.Stat(absPath); err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist - consider this successful
			return nil
		}
		return fmt.Errorf("failed to stat file: %w", err)
	}

	// Remove the file
	if err := os.Remove(absPath); err != nil {
		return fmt.Errorf("failed to remove file: %w", err)
	}

	// Optionally clean up empty parent directories
	// (commented out to avoid accidentally removing user-created directories)
	// _ = cleanupEmptyDirs(filesystem.basePath, filepath.Dir(absPath))

	return nil
}
