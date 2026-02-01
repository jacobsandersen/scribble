package s3

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/indieinfra/scribble/config"
	"github.com/indieinfra/scribble/server/util"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type StoreImpl struct {
	client       *minio.Client
	bucket       string
	endpointHost string
	region       string
	mediaUrl     string
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

	lookup := minio.BucketLookupAuto

	client, err := minio.New(endpointHost, &minio.Options{
		Creds:        credentials.NewStaticV4(s3cfg.AccessKeyId, s3cfg.SecretKeyId, ""),
		Secure:       true,
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
		client:       client,
		bucket:       s3cfg.Bucket,
		endpointHost: endpointHost,
		region:       s3cfg.Region,
		mediaUrl:     cfg.MediaUrl,
	}, nil
}

func (s *StoreImpl) Upload(ctx context.Context, data *util.MultipartFile, key string) error {
	if data.File == nil || data.Header == nil {
		return fmt.Errorf("file and header are required")
	}

	opts := minio.PutObjectOptions{ContentType: data.DetectedMimeType}

	if _, err := s.client.PutObject(ctx, s.bucket, key, data.File, data.Header.Size, opts); err != nil {
		return fmt.Errorf("upload to s3 failed: %w", err)
	}

	return nil
}

func (s *StoreImpl) Delete(ctx context.Context, urlStr string) error {
	if err := s.client.RemoveObject(ctx, s.bucket, s.keyFromURL(urlStr), minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("delete from s3 failed: %w", err)
	}

	return nil
}

func (s *StoreImpl) keyFromURL(urlStr string) string {
	return strings.TrimPrefix(urlStr, s.mediaUrl)
}
