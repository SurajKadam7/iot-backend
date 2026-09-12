package storage

import (
	"context"
	"errors"
	"io"
	"time"
)

var (
	ErrNotFound    = errors.New("object not found")
	ErrInvalidKey  = errors.New("invalid object key")
	ErrUnavailable = errors.New("object store unavailable")
)

// Store is the shared blob API for archive CSV and assembled exports.
// Filesystem, in-memory, and S3 all implement this so export jobs never
// talk to a vendor SDK directly.
type Store interface {
	Put(ctx context.Context, key string, body io.Reader, contentType string) error
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	List(ctx context.Context, prefix string) ([]string, error)
}

// Presigner is an optional capability. S3 implements it; filesystem does not.
type Presigner interface {
	PresignGet(ctx context.Context, key string, expiry time.Duration) (string, error)
}
