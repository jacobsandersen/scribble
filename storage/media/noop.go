package media

import (
	"context"
	"log"
	"mime/multipart"
)

type NoopMediaStore struct{}

func (ms *NoopMediaStore) Upload(ctx context.Context, file *multipart.File, header *multipart.FileHeader) (string, error) {
	log.Println("Received no-op media upload request - dumping request information")
	log.Printf("Filename: %v", header.Filename)
	log.Printf("Header: %v", header.Header)
	log.Printf("Size: %v", header.Size)

	return "https://noop.example.org/noop", nil
}

func (ms *NoopMediaStore) Delete(ctx context.Context, url string) error {
	log.Println("Received no-op media delete request - dumping request information")
	log.Printf("Url: %v", url)
	return nil
}
