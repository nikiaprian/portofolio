package storage

import (
	"context"
	"io"
)

// ObjectStore stores binary objects and returns a public URL.
// For development we use a public R2 bucket or local static dir.
type ObjectStore interface {
	// Put uploads an object at key and returns its public URL.
	Put(ctx context.Context, key string, body io.Reader, contentType string) (string, error)
	// DeleteURL removes the object whose URL was returned from Put (same base URL).
	// URLs that do not belong to this store are ignored (no error).
	DeleteURL(ctx context.Context, objectURL string) error
}
