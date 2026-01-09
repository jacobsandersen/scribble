package media

import (
	"context"
	"log"
	"os"
)

type NoopMediaStore struct{}

func (ms *NoopMediaStore) Upload(ctx context.Context, file *UploadedFile) (string, error) {
	log.Println("Received no-op media upload request - dumping request information")
	log.Printf("Filename: %v", file.Filename)
	log.Printf("Header: %v", file.Header)
	log.Printf("Size: %v", file.Size)
	log.Printf("Path: %v", file.Path)

	log.Println("Deleting uploaded file from disk...")
	err := os.Remove(file.Path)
	if err != nil {
		log.Printf("Error deleting file on disk, please do it manually: %v", err)
	}

	return "https://noop.example.org/noop", nil
}

func (ms *NoopMediaStore) Delete(ctx context.Context, url string) error {
	log.Println("Received no-op media delete request - dumping request information")
	log.Printf("Url: %v", url)
	return nil
}
