package media

import (
	"bytes"
	"context"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/indieinfra/scribble/config"
)

// readCloserWrapper wraps a bytes.Reader to implement multipart.File
type readCloserWrapper struct {
	*bytes.Reader
}

func (r *readCloserWrapper) Close() error {
	return nil
}

// Helper to create a multipart file from bytes
func createMultipartFile(t *testing.T, filename string, contentType string, data []byte) (*multipart.File, *multipart.FileHeader) {
	t.Helper()

	// Create a ReadCloser to use as the multipart.File
	reader := &readCloserWrapper{bytes.NewReader(data)}
	file := multipart.File(reader)

	// Create the header
	header := &multipart.FileHeader{
		Filename: filename,
		Size:     int64(len(data)),
		Header:   make(map[string][]string),
	}
	if contentType != "" {
		header.Header.Set("Content-Type", contentType)
	}

	return &file, header
}

func setupFilesystemMediaTest(t *testing.T) (*FilesystemMediaStore, string, func()) {
	t.Helper()

	tmpDir := t.TempDir()

	cfg := &config.FilesystemMediaStrategy{
		Path:      tmpDir,
		PublicUrl: "https://media.example.com/",
	}

	store, err := NewFilesystemMediaStore(cfg)
	if err != nil {
		t.Fatalf("NewFilesystemMediaStore: %v", err)
	}

	cleanup := func() {
		// t.TempDir handles cleanup
	}

	return store, tmpDir, cleanup
}

func TestNewFilesystemMediaStore(t *testing.T) {
	t.Run("creates directory if missing", func(t *testing.T) {
		tmpDir := t.TempDir()
		nestedPath := filepath.Join(tmpDir, "media", "uploads")

		cfg := &config.FilesystemMediaStrategy{
			Path:      nestedPath,
			PublicUrl: "https://media.example.com/",
		}

		store, err := NewFilesystemMediaStore(cfg)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if _, err := os.Stat(nestedPath); os.IsNotExist(err) {
			t.Fatal("expected directory to be created")
		}

		if store.basePath != nestedPath {
			t.Errorf("basePath = %q, want %q", store.basePath, nestedPath)
		}
	})

	t.Run("nil config returns error", func(t *testing.T) {
		_, err := NewFilesystemMediaStore(nil)
		if err == nil {
			t.Error("expected error for nil config")
		}
	})

	t.Run("uses custom path pattern", func(t *testing.T) {
		tmpDir := t.TempDir()

		cfg := &config.FilesystemMediaStrategy{
			Path:        tmpDir,
			PublicUrl:   "https://media.example.com/",
			PathPattern: "uploads/{year}/{month}/{filename}",
		}

		_, err := NewFilesystemMediaStore(cfg)
		if err != nil {
			t.Fatalf("NewFilesystemMediaStore: %v", err)
		}
	})
}

func TestFilesystemMediaStore_Upload(t *testing.T) {
	t.Run("uploads file with default pattern", func(t *testing.T) {
		store, tmpDir, cleanup := setupFilesystemMediaTest(t)
		defer cleanup()

		content := []byte("test image data")
		file, header := createMultipartFile(t, "test.jpg", "image/jpeg", content)

		url, err := store.Upload(context.Background(), file, header)
		if err != nil {
			t.Fatalf("Upload: %v", err)
		}

		if !strings.HasPrefix(url, "https://media.example.com/") {
			t.Errorf("url = %q, expected to start with public URL", url)
		}

		// Extract relative path and verify file exists
		relPath := strings.TrimPrefix(url, "https://media.example.com/")
		absPath := filepath.Join(tmpDir, filepath.FromSlash(relPath))

		data, err := os.ReadFile(absPath)
		if err != nil {
			t.Fatalf("ReadFile: %v", err)
		}

		if !bytes.Equal(data, content) {
			t.Error("file content mismatch")
		}
	})

	t.Run("handles duplicate filename", func(t *testing.T) {
		store, _, cleanup := setupFilesystemMediaTest(t)
		defer cleanup()

		content1 := []byte("first upload")
		file1, header1 := createMultipartFile(t, "duplicate.txt", "text/plain", content1)

		url1, err := store.Upload(context.Background(), file1, header1)
		if err != nil {
			t.Fatalf("Upload 1: %v", err)
		}

		content2 := []byte("second upload")
		file2, header2 := createMultipartFile(t, "duplicate.txt", "text/plain", content2)

		url2, err := store.Upload(context.Background(), file2, header2)
		if err != nil {
			t.Fatalf("Upload 2: %v", err)
		}

		if url1 == url2 {
			t.Error("expected different URLs for duplicate filename")
		}
	})

	t.Run("derives extension from content type", func(t *testing.T) {
		store, _, cleanup := setupFilesystemMediaTest(t)
		defer cleanup()

		content := []byte("plain text")
		file, header := createMultipartFile(t, "noext", "text/plain", content)

		url, err := store.Upload(context.Background(), file, header)
		if err != nil {
			t.Fatalf("Upload: %v", err)
		}

		// Should have derived an extension from content type
		// (text/plain could map to .txt, .conf, or other extensions)
		hasExt := strings.Contains(url, ".")
		if !hasExt {
			t.Errorf("expected URL to have an extension, got %q", url)
		}
	})

	t.Run("generates filename if missing", func(t *testing.T) {
		store, _, cleanup := setupFilesystemMediaTest(t)
		defer cleanup()

		content := []byte("data")
		file, header := createMultipartFile(t, ".jpg", "image/jpeg", content)

		url, err := store.Upload(context.Background(), file, header)
		if err != nil {
			t.Fatalf("Upload: %v", err)
		}

		// Should have generated a UUID-based filename
		relPath := strings.TrimPrefix(url, "https://media.example.com/")
		filename := filepath.Base(relPath)
		if filename == ".jpg" || filename == "" {
			t.Errorf("expected generated filename, got %q", filename)
		}
	})

	t.Run("organizes by date pattern", func(t *testing.T) {
		tmpDir := t.TempDir()

		cfg := &config.FilesystemMediaStrategy{
			Path:        tmpDir,
			PublicUrl:   "https://media.example.com/",
			PathPattern: "{year}/{month}/{filename}",
		}

		store, err := NewFilesystemMediaStore(cfg)
		if err != nil {
			t.Fatalf("NewFilesystemMediaStore: %v", err)
		}

		content := []byte("test data")
		file, header := createMultipartFile(t, "test.jpg", "image/jpeg", content)

		url, err := store.Upload(context.Background(), file, header)
		if err != nil {
			t.Fatalf("Upload: %v", err)
		}

		// URL should contain year and month
		relPath := strings.TrimPrefix(url, "https://media.example.com/")
		parts := strings.Split(relPath, "/")
		if len(parts) < 3 {
			t.Errorf("expected at least 3 path parts (year/month/file), got %d", len(parts))
		}
	})
}

func TestFilesystemMediaStore_Delete(t *testing.T) {
	t.Run("deletes existing file", func(t *testing.T) {
		store, tmpDir, cleanup := setupFilesystemMediaTest(t)
		defer cleanup()

		content := []byte("to be deleted")
		file, header := createMultipartFile(t, "delete-me.txt", "text/plain", content)

		url, err := store.Upload(context.Background(), file, header)
		if err != nil {
			t.Fatalf("Upload: %v", err)
		}

		// Verify file exists
		relPath := strings.TrimPrefix(url, "https://media.example.com/")
		absPath := filepath.Join(tmpDir, filepath.FromSlash(relPath))
		if _, err := os.Stat(absPath); os.IsNotExist(err) {
			t.Fatal("file should exist before delete")
		}

		// Delete it
		if err := store.Delete(context.Background(), url); err != nil {
			t.Fatalf("Delete: %v", err)
		}

		// Verify file is gone
		if _, err := os.Stat(absPath); !os.IsNotExist(err) {
			t.Error("file should not exist after delete")
		}
	})

	t.Run("succeeds for non-existent file", func(t *testing.T) {
		store, _, cleanup := setupFilesystemMediaTest(t)
		defer cleanup()

		// Try to delete a file that doesn't exist
		err := store.Delete(context.Background(), "https://media.example.com/2024/01/missing.jpg")
		if err != nil {
			t.Errorf("Delete of non-existent file should succeed, got: %v", err)
		}
	})

	t.Run("rejects URL with wrong prefix", func(t *testing.T) {
		store, _, cleanup := setupFilesystemMediaTest(t)
		defer cleanup()

		err := store.Delete(context.Background(), "https://wrong-domain.com/file.jpg")
		if err == nil {
			t.Error("expected error for mismatched URL prefix")
		}
	})
}

func TestFilesystemMediaStore_ConcurrentUploads(t *testing.T) {
	t.Run("handles concurrent uploads", func(t *testing.T) {
		store, _, cleanup := setupFilesystemMediaTest(t)
		defer cleanup()

		done := make(chan string)

		for i := 0; i < 5; i++ {
			go func(n int) {
				content := []byte("concurrent upload")
				file, header := createMultipartFile(t, "concurrent.txt", "text/plain", content)

				url, err := store.Upload(context.Background(), file, header)
				if err != nil {
					t.Errorf("Upload %d: %v", n, err)
				}
				done <- url
			}(i)
		}

		urls := make(map[string]bool)
		for i := 0; i < 5; i++ {
			url := <-done
			if urls[url] {
				t.Errorf("duplicate URL: %s", url)
			}
			urls[url] = true
		}
	})
}
