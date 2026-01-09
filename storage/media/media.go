package media

import (
	"context"
	"net/textproto"
	"time"
)

var ActiveMediaStore MediaStore

type UploadedFile struct {
	Filename string
	Header   textproto.MIMEHeader
	Path     string
	Size     int64
}

type MediaObject struct {
	ID       string
	Filename string
	MIMEType string
	Size     int64
	Location string
	Created  time.Time
}

type MediaStore interface {
	Upload(ctx context.Context, file *UploadedFile) (string, error)
	Delete(ctx context.Context, url string) error
}
