package storage

import (
	"context"
	"io"
	"time"
)

type PutOptions struct {
	ContentType string
}

type ObjectMeta struct {
	Key         string
	SizeBytes   int64
	ContentType string
	UpdatedAt   time.Time
}

type ObjectStorage interface {
	Put(ctx context.Context, key string, reader io.Reader, sizeBytes int64, opts PutOptions) (ObjectMeta, error)
	Open(ctx context.Context, key string) (io.ReadCloser, ObjectMeta, error)
	Delete(ctx context.Context, key string) error
	Stat(ctx context.Context, key string) (ObjectMeta, error)
}
