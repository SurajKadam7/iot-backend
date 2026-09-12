package storage

import (
	"bytes"
	"context"
	"io"
	"sort"
	"strings"
	"sync"
)

// Memory is an in-process Store for tests.
type Memory struct {
	mu   sync.RWMutex
	blob map[string][]byte
}

func NewMemory() *Memory {
	return &Memory{blob: map[string][]byte{}}
}

func (m *Memory) Put(_ context.Context, key string, body io.Reader, _ string) error {
	key, err := normalizeKey(key)
	if err != nil {
		return err
	}
	b, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	cp := make([]byte, len(b))
	copy(cp, b)
	m.mu.Lock()
	m.blob[key] = cp
	m.mu.Unlock()
	return nil
}

func (m *Memory) Get(_ context.Context, key string) (io.ReadCloser, error) {
	key, err := normalizeKey(key)
	if err != nil {
		return nil, err
	}
	m.mu.RLock()
	b, ok := m.blob[key]
	m.mu.RUnlock()
	if !ok {
		return nil, ErrNotFound
	}
	cp := make([]byte, len(b))
	copy(cp, b)
	return io.NopCloser(bytes.NewReader(cp)), nil
}

func (m *Memory) List(_ context.Context, prefix string) ([]string, error) {
	prefix = strings.TrimSpace(prefix)
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]string, 0)
	for k := range m.blob {
		if prefix == "" || strings.HasPrefix(k, prefix) {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out, nil
}

var _ Store = (*Memory)(nil)
