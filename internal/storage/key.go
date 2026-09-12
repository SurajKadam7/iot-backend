package storage

import (
	"path"
	"strings"
)

func normalizeKey(key string) (string, error) {
	key = strings.TrimSpace(key)
	key = strings.TrimPrefix(key, "/")
	if key == "" {
		return "", ErrInvalidKey
	}
	if strings.Contains(key, "\\") || strings.Contains(key, "..") {
		return "", ErrInvalidKey
	}
	cleaned := path.Clean("/" + key)
	cleaned = strings.TrimPrefix(cleaned, "/")
	if cleaned == "." || cleaned == "" {
		return "", ErrInvalidKey
	}
	for _, part := range strings.Split(cleaned, "/") {
		if part == ".." || part == "." || part == "" {
			return "", ErrInvalidKey
		}
	}
	return cleaned, nil
}
