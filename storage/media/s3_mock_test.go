package media

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/textproto"
	"strings"
	"testing"

	"github.com/minio/minio-go/v7"

	"github.com/indieinfra/scribble/config"
)

type stubS3Client struct {
	bucketExists  bool
	bucketErr     error
	putCalled     bool
	removeCalled  bool
	lastPutKey    string
	lastRemoveKey string
	putErr        error
	removeErr     error
}

func (c *stubS3Client) BucketExists(ctx context.Context, bucketName string) (bool, error) {
	return c.bucketExists, c.bucketErr
}

func (c *stubS3Client) PutObject(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error) {
	c.putCalled = true
	c.lastPutKey = objectName
	if c.putErr != nil {
		return minio.UploadInfo{}, c.putErr
	}
	return minio.UploadInfo{}, nil
}

func (c *stubS3Client) RemoveObject(ctx context.Context, bucketName, objectName string, opts minio.RemoveObjectOptions) error {
	c.removeCalled = true
	c.lastRemoveKey = objectName
	if c.removeErr != nil {
		return c.removeErr
	}
	return nil
}

func withStubClient(t *testing.T, stub *stubS3Client) func() {
	prev := newMinioClient
	newMinioClient = func(endpoint string, opts *minio.Options) (s3Client, error) {
		return stub, nil
	}

	return func() { newMinioClient = prev }
}

func baseMediaConfig() *config.Media {
	return &config.Media{
		Strategy: "s3",
		S3: &config.S3MediaStrategy{
			AccessKeyId: "key",
			SecretKeyId: "secret",
			Bucket:      "bucket",
			Endpoint:    "https://s3.example.com",
			PublicUrl:   "https://cdn.example.com",
		},
	}
}

func TestNewS3MediaStore_ClientError(t *testing.T) {
	prev := newMinioClient
	newMinioClient = func(endpoint string, opts *minio.Options) (s3Client, error) {
		return nil, errors.New("boom")
	}
	t.Cleanup(func() { newMinioClient = prev })

	if _, err := NewS3MediaStore(baseMediaConfig()); err == nil {
		t.Fatalf("expected error when client creation fails")
	}
}

func TestNewS3MediaStore_BucketExistsError(t *testing.T) {
	stub := &stubS3Client{bucketExists: false, bucketErr: errors.New("check failed")}
	defer withStubClient(t, stub)()

	if _, err := NewS3MediaStore(baseMediaConfig()); err == nil {
		t.Fatalf("expected error when bucket check fails")
	}
}

func TestNewS3MediaStore_DefaultsPathStyle(t *testing.T) {
	stub := &stubS3Client{bucketExists: true}
	restore := withStubClient(t, stub)
	defer restore()

	cfg := baseMediaConfig()
	cfg.S3.Endpoint = ""
	cfg.S3.Region = "auto"
	cfg.S3.ForcePathStyle = true
	cfg.S3.DisableSSL = true
	cfg.S3.PublicUrl = ""

	store, err := NewS3MediaStore(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	url := store.objectURL("k")
	if url != "http://s3.amazonaws.com/bucket/k" {
		t.Fatalf("unexpected path-style url: %s", url)
	}

	if !store.forcePathStyle || store.secure || store.endpointHost != "s3.amazonaws.com" || store.region != "auto" {
		t.Fatalf("store defaults not set as expected: %+v", store)
	}
}

func TestNewS3MediaStore_ErrWhenBucketMissing(t *testing.T) {
	stub := &stubS3Client{bucketExists: false}
	defer withStubClient(t, stub)()

	if _, err := NewS3MediaStore(baseMediaConfig()); err == nil {
		t.Fatalf("expected error when bucket does not exist")
	}
}

func TestNewS3MediaStore_SetsFields(t *testing.T) {
	stub := &stubS3Client{bucketExists: true}
	restore := withStubClient(t, stub)
	defer restore()

	store, err := NewS3MediaStore(baseMediaConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if store.bucket != "bucket" || store.publicBase != "https://cdn.example.com" {
		t.Fatalf("store fields not populated correctly: %+v", store)
	}
}

func TestS3MediaStore_UploadAndDelete(t *testing.T) {
	stub := &stubS3Client{bucketExists: true}
	restore := withStubClient(t, stub)
	defer restore()

	store, err := NewS3MediaStore(baseMediaConfig())
	if err != nil {
		t.Fatalf("unexpected error creating store: %v", err)
	}

	data := []byte("hello")
	mf := multipart.File(testFile{bytes.NewReader(data)})
	header := &multipart.FileHeader{Filename: "file.txt", Size: int64(len(data)), Header: textproto.MIMEHeader{"Content-Type": []string{"text/plain"}}}

	url, err := store.Upload(context.Background(), &mf, header)
	if err != nil {
		t.Fatalf("upload failed: %v", err)
	}

	if !stub.putCalled || stub.lastPutKey == "" || url == "" {
		t.Fatalf("expected PutObject to be invoked and url returned")
	}

	if err := store.Delete(context.Background(), url); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if !stub.removeCalled || stub.lastRemoveKey == "" {
		t.Fatalf("expected RemoveObject to be invoked")
	}
}

func TestS3MediaStore_UploadError(t *testing.T) {
	stub := &stubS3Client{bucketExists: true, putErr: errors.New("put fail")}
	restore := withStubClient(t, stub)
	defer restore()

	store, err := NewS3MediaStore(baseMediaConfig())
	if err != nil {
		t.Fatalf("unexpected error creating store: %v", err)
	}

	mf := multipart.File(testFile{bytes.NewReader([]byte("bad"))})
	header := &multipart.FileHeader{Filename: "file.txt", Size: 3, Header: textproto.MIMEHeader{"Content-Type": []string{"text/plain"}}}

	if _, err := store.Upload(context.Background(), &mf, header); err == nil {
		t.Fatalf("expected upload to fail")
	}
}

func TestS3MediaStore_ObjectKeyDefaults(t *testing.T) {
	store := &S3MediaStore{prefix: "pre"}

	key := store.objectKey("")
	if !strings.HasPrefix(key, "pre/") || !strings.Contains(key, "upload") {
		t.Fatalf("unexpected key: %s", key)
	}
}

func TestS3MediaStore_UploadValidation(t *testing.T) {
	store := &S3MediaStore{}

	if _, err := store.Upload(context.Background(), nil, nil); err == nil {
		t.Fatalf("expected error when file and header missing")
	}
}

func TestS3MediaStore_DeleteInvalidURL(t *testing.T) {
	stub := &stubS3Client{bucketExists: true}
	store := &S3MediaStore{client: stub, bucket: "bucket"}

	if err := store.Delete(context.Background(), "::://bad url"); err == nil {
		t.Fatalf("expected error for invalid url")
	}
}

func TestS3MediaStore_DeleteEmptyPath(t *testing.T) {
	stub := &stubS3Client{bucketExists: true}
	store := &S3MediaStore{client: stub, bucket: "bucket"}

	if err := store.Delete(context.Background(), "https://example.com"); err == nil {
		t.Fatalf("expected error for empty path")
	}
}

func TestS3MediaStore_DeletePathStyleKey(t *testing.T) {
	stub := &stubS3Client{bucketExists: true}
	store := &S3MediaStore{client: stub, bucket: "bucket"}

	if err := store.Delete(context.Background(), "https://example.com/bucket/prefix/object.txt"); err != nil {
		t.Fatalf("delete should succeed: %v", err)
	}

	if stub.lastRemoveKey != "prefix/object.txt" {
		t.Fatalf("expected bucket prefix to be trimmed, got %q", stub.lastRemoveKey)
	}
}

func TestS3MediaStore_DeleteError(t *testing.T) {
	stub := &stubS3Client{bucketExists: true, removeErr: errors.New("remove fail")}
	store := &S3MediaStore{client: stub, bucket: "bucket"}

	if err := store.Delete(context.Background(), "https://example.com/bucket/prefix/object.txt"); err == nil {
		t.Fatalf("expected delete to fail")
	}
}

func TestS3MediaStore_objectURL(t *testing.T) {
	store := &S3MediaStore{bucket: "bucket", endpointHost: "s3.example.com", secure: true}

	if got := store.objectURL("path/to/key"); got != "https://bucket.s3.example.com/path/to/key" {
		t.Fatalf("unexpected virtual-host url: %s", got)
	}

	store.forcePathStyle = true
	if got := store.objectURL("path/to/key"); got != "https://s3.example.com/bucket/path/to/key" {
		t.Fatalf("unexpected path-style url: %s", got)
	}

	store.publicBase = "https://cdn.example.com/media"
	if got := store.objectURL("path/to/key"); got != "https://cdn.example.com/media/path/to/key" {
		t.Fatalf("unexpected public url: %s", got)
	}
}

func TestS3MediaStore_keyFromURL(t *testing.T) {
	store := &S3MediaStore{bucket: "bucket"}

	cases := []struct {
		name   string
		url    string
		expect string
	}{
		{"virtual host", "https://bucket.s3.example.com/path/to/key", "path/to/key"},
		{"path style", "https://s3.example.com/bucket/path/to/key", "path/to/key"},
		{"plain path", "https://example.com/path/to/key", "path/to/key"},
	}

	for _, tc := range cases {
		got, err := store.keyFromURL(tc.url)
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", tc.name, err)
		}
		if got != tc.expect {
			t.Fatalf("%s: expected %s, got %s", tc.name, tc.expect, got)
		}
	}
}
