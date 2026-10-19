package storage

import (
	"context"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type LocalStore struct {
	Dir       string // filesystem base dir (e.g. public/static/uploads)
	PublicURL string // url base (e.g. /static/uploads)
}

func NewLocalStore(dir, publicURL string) *LocalStore {
	return &LocalStore{Dir: dir, PublicURL: publicURL}
}

func (s *LocalStore) Put(ctx context.Context, key string, body io.Reader, contentType string) (string, error) {
	_ = ctx
	_ = contentType
	key = strings.TrimLeft(key, "/")
	targetPath := filepath.Join(s.Dir, filepath.FromSlash(key))

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return "", err
	}

	f, err := os.Create(targetPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	if _, err := io.Copy(f, body); err != nil {
		return "", err
	}

	base := strings.TrimRight(s.PublicURL, "/")
	return base + "/" + strings.ReplaceAll(key, "\\", "/"), nil
}

// DeleteURL removes a file under this store's public URL base (path-only or absolute URL).
func (s *LocalStore) DeleteURL(ctx context.Context, objectURL string) error {
	_ = ctx
	raw := strings.TrimSpace(objectURL)
	if raw == "" {
		return nil
	}
	if i := strings.Index(raw, "?"); i >= 0 {
		raw = raw[:i]
	}
	path := raw
	if u, err := url.Parse(raw); err == nil && u.Path != "" {
		path = u.Path
	}
	base := strings.TrimRight(s.PublicURL, "/")
	if base != "" && !strings.HasPrefix(path, base+"/") && path != base {
		return nil
	}
	rel := strings.TrimPrefix(path, base)
	rel = strings.TrimLeft(rel, "/")
	if rel == "" {
		return nil
	}
	target := filepath.Join(s.Dir, filepath.FromSlash(rel))
	dirClean := filepath.Clean(s.Dir)
	tgtClean := filepath.Clean(target)
	if !strings.HasPrefix(tgtClean, dirClean+string(os.PathSeparator)) && tgtClean != dirClean {
		return nil
	}
	_ = os.Remove(target)
	return nil
}
