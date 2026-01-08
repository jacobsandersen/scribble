package storage

import (
	"context"
	"io"
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
	Upload(ctx context.Context, filename string, contentType string, r io.Reader) (*MediaObject, error)
	Get(ctx context.Context, id string) (*MediaObject, error)
	Delete(ctx context.Context, id string) error
	URL(ctx context.Context, id string) (string, error)
}
