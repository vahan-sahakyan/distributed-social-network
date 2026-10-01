package storage

import (
	"context"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/retry"
)

type MinioStorage struct {
	client *minio.Client
	bucket string
}

// NewMinio ensures the bucket exists and is public-read, retrying until MinIO answers or ctx is done.
func NewMinio(ctx context.Context, endpoint, accessKey, secretKey, bucket string) (*MinioStorage, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: false,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create minio client: %w", err)
	}

	if err := retry.Do(ctx, "setting up minio bucket", func(ctx context.Context) error {
		return setupBucket(ctx, client, bucket)
	}); err != nil {
		return nil, fmt.Errorf("setting up bucket %s: %w", bucket, err)
	}
	return &MinioStorage{client: client, bucket: bucket}, nil
}

func setupBucket(ctx context.Context, client *minio.Client, bucket string) error {
	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		return fmt.Errorf("checking bucket: %w", err)
	}
	if !exists {
		if err := client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
			return fmt.Errorf("creating bucket: %w", err)
		}
	}

	policy := fmt.Sprintf(`{
		"Version": "2012-10-17",
		"Statement": [{
			"Effect": "Allow",
			"Principal": {"AWS": ["*"]},
			"Action": ["s3:GetObject"],
			"Resource": ["arn:aws:s3:::%s/*"]
		}]
	}`, bucket)
	if err := client.SetBucketPolicy(ctx, bucket, policy); err != nil {
		return fmt.Errorf("setting bucket policy: %w", err)
	}
	return nil
}

func (s *MinioStorage) Upload(ctx context.Context, objectName string, reader io.Reader, size int64, contentType string) (string, error) {
	_, err := s.client.PutObject(ctx, s.bucket, objectName, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload: %w", err)
	}

	return fmt.Sprintf("/%s/%s", s.bucket, objectName), nil
}

func (s *MinioStorage) GetURL(_ context.Context, objectName string) (string, error) {
	return fmt.Sprintf("/%s/%s", s.bucket, objectName), nil
}
