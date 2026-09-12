package storage

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Filesystem stores objects under a local directory. Used when S3_BUCKET is empty
// so local development does not require AWS.
type Filesystem struct {
	root string
}

func NewFilesystem(root string) (*Filesystem, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		root = "data/archive"
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, err
	}
	return &Filesystem{root: abs}, nil
}

func (f *Filesystem) resolve(key string) (string, error) {
	key, err := normalizeKey(key)
	if err != nil {
		return "", err
	}
	full := filepath.Join(f.root, filepath.FromSlash(key))
	rel, err := filepath.Rel(f.root, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", ErrInvalidKey
	}
	return full, nil
}

func (f *Filesystem) Put(_ context.Context, key string, body io.Reader, _ string) error {
	full, err := f.resolve(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(full), ".put-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(tmpName)
		}
	}()
	if _, err := io.Copy(tmp, body); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, full); err != nil {
		return err
	}
	ok = true
	return nil
}

func (f *Filesystem) Get(_ context.Context, key string) (io.ReadCloser, error) {
	full, err := f.resolve(key)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(full)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return file, nil
}

func (f *Filesystem) List(_ context.Context, prefix string) ([]string, error) {
	prefix, err := normalizeListPrefix(prefix)
	if err != nil {
		return nil, err
	}
	var out []string
	err = filepath.WalkDir(f.root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(f.root, path)
		if err != nil {
			return err
		}
		key := filepath.ToSlash(rel)
		if prefix == "" || strings.HasPrefix(key, prefix) {
			out = append(out, key)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if out == nil {
		out = []string{}
	}
	sort.Strings(out)
	return out, nil
}

func normalizeListPrefix(prefix string) (string, error) {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return "", nil
	}
	prefix = strings.TrimPrefix(prefix, "/")
	if strings.HasSuffix(prefix, "/") {
		base := strings.TrimSuffix(prefix, "/")
		if base == "" {
			return "", nil
		}
		cleaned, err := normalizeKey(base)
		if err != nil {
			return "", err
		}
		return cleaned + "/", nil
	}
	return normalizeKey(prefix)
}

var _ Store = (*Filesystem)(nil)
