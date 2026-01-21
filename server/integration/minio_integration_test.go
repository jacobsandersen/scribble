//go:build testcontainers
// +build testcontainers

package integration

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/indieinfra/scribble/config"
	"github.com/indieinfra/scribble/server/handler/upload"
	"github.com/indieinfra/scribble/server/state"
	"github.com/indieinfra/scribble/storage/content"
	"github.com/indieinfra/scribble/storage/media"
)

func newMinioState(t *testing.T) *state.ScribbleState {
	t.Helper()

	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "minio/minio:RELEASE.2024-01-16T16-07-38Z",
		ExposedPorts: []string{"9000/tcp"},
		Env: map[string]string{
			"MINIO_ROOT_USER":     "minioadmin",
			"MINIO_ROOT_PASSWORD": "minioadmin",
		},
		Cmd:        []string{"server", "/data"},
		WaitingFor: wait.ForLog("API:").WithStartupTimeout(60 * time.Second),
	}

	cont, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("failed to start minio container: %v", err)
	}

	t.Cleanup(func() {
		_ = cont.Terminate(ctx)
	})

	host, err := cont.Host(ctx)
	if err != nil {
		t.Fatalf("failed to get host: %v", err)
	}

	mapped, err := cont.MappedPort(ctx, "9000")
	if err != nil {
		t.Fatalf("failed to get mapped port: %v", err)
	}

	endpoint := host + ":" + mapped.Port()

	// Create bucket before wiring store
	cli, err := minio.New(endpoint, &minio.Options{
		Creds:        credentials.NewStaticV4("minioadmin", "minioadmin", ""),
		Secure:       false,
		BucketLookup: minio.BucketLookupPath,
	})
	if err != nil {
		t.Fatalf("failed to init minio client: %v", err)
	}

	bucket := "test-media"
	ctxTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := cli.MakeBucket(ctxTimeout, bucket, minio.MakeBucketOptions{Region: "us-east-1"}); err != nil {
		exists, errExists := cli.BucketExists(ctxTimeout, bucket)
		if errExists != nil || !exists {
			t.Fatalf("failed to ensure bucket exists: %v", err)
		}
	}

	cfg := &config.Config{
		Debug:  false,
		Server: config.Server{Limits: config.ServerLimits{MaxPayloadSize: 1 << 20, MaxFileSize: 1 << 20, MaxMultipartMem: 1 << 20}},
		Micropub: config.Micropub{
			MeUrl:         "https://example.test/me",
			TokenEndpoint: "https://example.test/token",
		},
		Content: config.Content{Strategy: "noop"},
		Media: config.Media{
			Strategy: "s3",
			S3: &config.S3MediaStrategy{
				Endpoint:       "http://" + endpoint,
				Region:         "us-east-1",
				Bucket:         bucket,
				AccessKeyId:    "minioadmin",
				SecretKeyId:    "minioadmin",
				ForcePathStyle: true,
				DisableSSL:     true,
			},
		},
	}

	mediaStore, err := media.NewS3MediaStore(&cfg.Media)
	if err != nil {
		t.Fatalf("failed to create s3 media store: %v", err)
	}

	return &state.ScribbleState{
		Cfg:          cfg,
		ContentStore: &content.NoopContentStore{},
		MediaStore:   mediaStore,
	}
}

func minioObjectKeyFromLocation(t *testing.T, bucket, loc string) string {
	u, err := url.Parse(loc)
	if err != nil {
		t.Fatalf("invalid location url: %v", err)
	}
	key := strings.TrimPrefix(u.Path, "/")
	key = strings.TrimPrefix(key, bucket+"/")
	return key
}

func TestMinio_UploadPhoto(t *testing.T) {
	st := newMinioState(t)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", `form-data; name="file"; filename="test.jpg"`)
	h.Set("Content-Type", "image/jpeg")

	part, err := writer.CreatePart(h)
	if err != nil {
		t.Fatal(err)
	}
	jpegData := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, []byte("fake image data")...)
	part.Write(jpegData)
	writer.Close()

	req := httptest.NewRequest("POST", "/media", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	rec := httptest.NewRecorder()
	withToken(st.Cfg, upload.HandleMediaUpload(st)).ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	loc := rec.Header().Get("Location")
	if loc == "" {
		t.Fatal("expected location header")
	}

	key := minioObjectKeyFromLocation(t, st.Cfg.Media.S3.Bucket, loc)

	ctx := context.Background()
	cli, err := minio.New(strings.TrimPrefix(st.Cfg.Media.S3.Endpoint, "http://"), &minio.Options{
		Creds:        credentials.NewStaticV4(st.Cfg.Media.S3.AccessKeyId, st.Cfg.Media.S3.SecretKeyId, ""),
		Secure:       false,
		BucketLookup: minio.BucketLookupPath,
	})
	if err != nil {
		t.Fatalf("minio client: %v", err)
	}

	_, err = cli.StatObject(ctx, st.Cfg.Media.S3.Bucket, key, minio.StatObjectOptions{})
	if err != nil {
		t.Fatalf("uploaded object not found: %v", err)
	}
}

func TestMinio_UploadVideo(t *testing.T) {
	st := newMinioState(t)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", `form-data; name="file"; filename="test.mp4"`)
	h.Set("Content-Type", "video/mp4")

	part, err := writer.CreatePart(h)
	if err != nil {
		t.Fatal(err)
	}
	mp4Data := append([]byte{0x00, 0x00, 0x00, 0x20, 0x66, 0x74, 0x79, 0x70}, []byte("fake video data")...)
	part.Write(mp4Data)
	writer.Close()

	req := httptest.NewRequest("POST", "/media", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	rec := httptest.NewRecorder()
	withToken(st.Cfg, upload.HandleMediaUpload(st)).ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	if rec.Header().Get("Location") == "" {
		t.Fatal("expected location header")
	}
}

func TestMinio_MultipleUploads(t *testing.T) {
	st := newMinioState(t)

	for i := 0; i < 3; i++ {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		h := textproto.MIMEHeader{}
		h.Set("Content-Disposition", `form-data; name="file"; filename="test.jpg"`)
		h.Set("Content-Type", "image/jpeg")

		part, _ := writer.CreatePart(h)
		part.Write([]byte{0xFF, 0xD8, 0xFF, 0xE0})
		writer.Close()

		req := httptest.NewRequest("POST", "/media", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())

		rec := httptest.NewRecorder()
		withToken(st.Cfg, upload.HandleMediaUpload(st)).ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("upload %d: expected 201, got %d", i+1, rec.Code)
		}

		if rec.Header().Get("Location") == "" {
			t.Fatalf("upload %d: expected location header", i+1)
		}
	}
}
