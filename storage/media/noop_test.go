package media

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/textproto"
	"testing"
)

func TestNoopMediaStore(t *testing.T) {
	store := &NoopMediaStore{}
	ctx := context.Background()

	data := []byte("data")
	mf := multipart.File(testFile{bytes.NewReader(data)})
	header := &multipart.FileHeader{Filename: "file.txt", Size: int64(len(data)), Header: textproto.MIMEHeader{"Content-Type": []string{"text/plain"}}}

	url, err := store.Upload(ctx, &mf, header)
	if err != nil || url == "" {
		t.Fatalf("unexpected upload result: url=%q err=%v", url, err)
	}

	if err := store.Delete(ctx, url); err != nil {
		t.Fatalf("delete returned error: %v", err)
	}
}
