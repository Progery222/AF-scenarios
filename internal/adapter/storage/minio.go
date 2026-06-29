package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/mobilefarm/af/scenarios/internal/config"
)

type MinIO struct {
	client *minio.Client
	bucket string
}

func NewMinIO(ctx context.Context, cfg config.Config) (*MinIO, error) {
	client, err := minio.New(cfg.MinioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.MinioAccessKey, cfg.MinioSecretKey, ""),
		Secure: cfg.MinioUseSSL,
	})
	if err != nil {
		return nil, err
	}
	m := &MinIO{client: client, bucket: cfg.MinioBucket}
	if err := m.ensureBucket(ctx); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *MinIO) ensureBucket(ctx context.Context) error {
	var lastErr error
	for attempt := 0; attempt < 20; attempt++ {
		if err := pingMinIOReady(m.client.EndpointURL().Scheme, m.client.EndpointURL().Host); err != nil {
			lastErr = err
			time.Sleep(500 * time.Millisecond)
			continue
		}
		exists, err := m.client.BucketExists(ctx, m.bucket)
		if err != nil {
			lastErr = err
			time.Sleep(500 * time.Millisecond)
			continue
		}
		if !exists {
			if err := m.client.MakeBucket(ctx, m.bucket, minio.MakeBucketOptions{}); err != nil {
				lastErr = err
				time.Sleep(500 * time.Millisecond)
				continue
			}
		}
		return nil
	}
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("minio not ready")
}

func pingMinIOReady(scheme, host string) error {
	if scheme == "" {
		scheme = "http"
	}
	url := fmt.Sprintf("%s://%s/minio/health/ready", scheme, host)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("minio health %d", resp.StatusCode)
	}
	return nil
}

func (m *MinIO) Put(ctx context.Context, key string, data []byte, contentType string) error {
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	_, err := m.client.PutObject(ctx, m.bucket, key, bytes.NewReader(data), int64(len(data)),
		minio.PutObjectOptions{ContentType: contentType})
	return err
}

func (m *MinIO) Get(ctx context.Context, key string) ([]byte, error) {
	obj, err := m.client.GetObject(ctx, m.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	defer obj.Close()
	return io.ReadAll(obj)
}

func (m *MinIO) Delete(ctx context.Context, key string) error {
	return m.client.RemoveObject(ctx, m.bucket, key, minio.RemoveObjectOptions{})
}

func (m *MinIO) ListPrefix(ctx context.Context, prefix string) ([]string, error) {
	ch := m.client.ListObjects(ctx, m.bucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	})
	var keys []string
	for obj := range ch {
		if obj.Err != nil {
			return nil, obj.Err
		}
		keys = append(keys, obj.Key)
	}
	return keys, nil
}

func (m *MinIO) Ping(ctx context.Context) error {
	_, err := m.client.ListBuckets(ctx)
	return err
}

func (m *MinIO) Bucket() string { return m.bucket }
