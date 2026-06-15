package service

import (
	"context"
	"io"

	"github.com/vahan-sahakyan/distributed-social-network/media-service/internal/storage"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/broker"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/id"
)

type Image struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

type Service struct {
	store    *storage.MinioStorage
	producer *broker.Producer
}

func New(store *storage.MinioStorage, producer *broker.Producer) *Service {
	return &Service{store: store, producer: producer}
}

func (s *Service) Upload(ctx context.Context, reader io.Reader, size int64, contentType string) (*Image, error) {
	imageID := id.New()

	url, err := s.store.Upload(ctx, imageID, reader, size, contentType)
	if err != nil {
		return nil, err
	}

	img := &Image{ID: imageID, URL: url}
	_ = s.producer.Publish(ctx, "image.uploaded", img)

	return img, nil
}

func (s *Service) GetURL(ctx context.Context, imageID string) (string, error) {
	return s.store.GetURL(ctx, imageID)
}
