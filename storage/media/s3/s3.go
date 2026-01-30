package s3

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/url"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/indieinfra/scribble/config"
)

// StoreImpl uploads media to S3 or any compatible service (R2, Backblaze, MinIO).
type s3Client interface {
	BucketExists(ctx context.Context, bucketName string) (bool, error)
	PutObject(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error)
	RemoveObject(ctx context.Context, bucketName, objectName string, opts minio.RemoveObjectOptions) error
}

var newMinioClient = func(endpoint string, opts *minio.Options) (s3Client, error) {
	return minio.New(endpoint, opts)
}

type StoreImpl struct {
	client         s3Client
	bucket         string
	prefix         string
	publicBase     string
	forcePathStyle bool
	endpointHost   string
	secure         bool
	region         string
}

func NewS3MediaStore(cfg *config.Media) (*StoreImpl, error) {
	if cfg == nil || cfg.S3 == nil {
		return nil, fmt.Errorf("s3 media config is nil")
	}

	s3cfg := cfg.S3
	region := strings.TrimSpace(s3cfg.Region)
	if strings.EqualFold(region, "auto") {
		region = ""
	}
	endpointHost := strings.TrimSpace(s3cfg.Endpoint)
	if endpointHost == "" {
		if region == "" {
			endpointHost = "s3.amazonaws.com"
		} else {
			endpointHost = fmt.Sprintf("s3.%s.amazonaws.com", region)
		}
	} else {
		if parsed, err := url.Parse(endpointHost); err == nil && parsed.Host != "" {
			endpointHost = parsed.Host
		}
	}

	secure := !s3cfg.DisableSSL
	lookup := minio.BucketLookupAuto
	if s3cfg.ForcePathStyle {
		lookup = minio.BucketLookupPath
	}

	client, err := newMinioClient(endpointHost, &minio.Options{
		Creds:        credentials.NewStaticV4(s3cfg.AccessKeyId, s3cfg.SecretKeyId, ""),
		Secure:       secure,
		Region:       region,
		BucketLookup: lookup,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create s3 client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	exists, err := client.BucketExists(ctx, s3cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("failed to verify s3 bucket %q: %w", s3cfg.Bucket, err)
	}
	if !exists {
		return nil, fmt.Errorf("s3 bucket %q does not exist or is not accessible", s3cfg.Bucket)
	}

	return &StoreImpl{
		client:         client,
		bucket:         s3cfg.Bucket,
		prefix:         strings.TrimPrefix(s3cfg.Prefix, "/"),
		publicBase:     strings.TrimSuffix(s3cfg.PublicUrl, "/"),
		forcePathStyle: s3cfg.ForcePathStyle,
		endpointHost:   endpointHost,
		secure:         secure,
		region:         s3cfg.Region,
	}, nil
}

func (s *StoreImpl) Upload(ctx context.Context, file *multipart.File, header *multipart.FileHeader, key string) (string, error) {
	if file == nil || header == nil {
		return "", fmt.Errorf("file and header are required")
	}

	opts := minio.PutObjectOptions{ContentType: header.Header.Get("Content-Type")}

	if _, err := s.client.PutObject(ctx, s.bucket, key, *file, header.Size, opts); err != nil {
		return "", fmt.Errorf("upload to s3 failed: %w", err)
	}

	return s.objectURL(key), nil
}

func (s *StoreImpl) Delete(ctx context.Context, urlStr string) error {
	key, err := s.keyFromURL(urlStr)
	if err != nil {
		return err
	}

	if err := s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("delete from s3 failed: %w", err)
	}

	return nil
}

func (s *StoreImpl) objectURL(key string) string {
	if s.publicBase != "" {
		return s.publicBase + "/" + key
	}

	scheme := "https"
	if !s.secure {
		scheme = "http"
	}

	if s.forcePathStyle {
		return fmt.Sprintf("%s://%s/%s/%s", scheme, s.endpointHost, s.bucket, key)
	}

	return fmt.Sprintf("%s://%s.%s/%s", scheme, s.bucket, s.endpointHost, key)
}

func (s *StoreImpl) keyFromURL(urlStr string) (string, error) {
	u, err := url.Parse(urlStr)
	if err != nil {
		return "", fmt.Errorf("invalid media url: %w", err)
	}

	key := strings.TrimPrefix(u.Path, "/")
	if strings.HasPrefix(key, s.bucket+"/") {
		key = strings.TrimPrefix(key, s.bucket+"/")
	}

	if key == "" {
		return "", fmt.Errorf("could not derive object key from url")
	}

	return key, nil
}
