package media

import (
	"context"
	"mime/multipart"
)

var ActiveMediaStore MediaStore

type MediaStore interface {
	Upload(ctx context.Context, file *multipart.File, header *multipart.FileHeader) (string, error)
	Delete(ctx context.Context, url string) error
}
