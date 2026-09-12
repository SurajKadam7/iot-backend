package storage

import (
	"bytes"
	"context"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

func TestMemoryRoundTrip(t *testing.T) {
	m := NewMemory()
	ctx := context.Background()
	if err := m.Put(ctx, "org/a/file.csv", strings.NewReader("hello"), "text/csv"); err != nil {
		t.Fatal(err)
	}
	rc, err := m.Get(ctx, "org/a/file.csv")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(rc)
	_ = rc.Close()
	if string(b) != "hello" {
		t.Fatalf("got %q", b)
	}
	keys, err := m.List(ctx, "org/a/")
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 || keys[0] != "org/a/file.csv" {
		t.Fatalf("list %#v", keys)
	}
	if _, err := m.Get(ctx, "missing"); err != ErrNotFound {
		t.Fatalf("missing: %v", err)
	}
}

func TestFilesystemRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	fs, err := NewFilesystem(root)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := fs.Put(ctx, "../escape.csv", strings.NewReader("x"), "text/csv"); err != ErrInvalidKey {
		t.Fatalf("expected invalid key, got %v", err)
	}
	if err := fs.Put(ctx, "org/dev/a.csv", bytes.NewReader([]byte("row")), "text/csv"); err != nil {
		t.Fatal(err)
	}
	keys, err := fs.List(ctx, "org/")
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 || keys[0] != "org/dev/a.csv" {
		t.Fatalf("list %#v", keys)
	}
	if _, err := fs.Get(ctx, filepath.ToSlash(filepath.Join("..", "x"))); err != ErrInvalidKey {
		t.Fatalf("get traversal: %v", err)
	}
}

func TestNormalizeKey(t *testing.T) {
	k, err := normalizeKey("/org/a/b.csv")
	if err != nil || k != "org/a/b.csv" {
		t.Fatalf("%q %v", k, err)
	}
	if _, err := normalizeKey(".."); err != ErrInvalidKey {
		t.Fatalf("got %v", err)
	}
}
