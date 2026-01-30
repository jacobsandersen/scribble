package media

import (
	"context"
	"mime/multipart"
)

type Store interface {
	Upload(ctx context.Context, file *multipart.File, header *multipart.FileHeader, key string) (string, error)
	Delete(ctx context.Context, url string) error
}
