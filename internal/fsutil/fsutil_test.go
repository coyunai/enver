package fsutil

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestJSONRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.json")
	if err := WriteJSON(path, map[string]string{"hello": "world"}); err != nil {
		t.Fatal(err)
	}
	got, err := ReadJSON(path, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	if got["hello"] != "world" {
		t.Fatalf("unexpected value: %#v", got)
	}
}

func TestReadJSONMissingReturnsDefault(t *testing.T) {
	got, err := ReadJSON(filepath.Join(t.TempDir(), "missing.json"), map[string]string{"a": "b"})
	if err != nil {
		t.Fatal(err)
	}
	if got["a"] != "b" {
		t.Fatalf("unexpected default: %#v", got)
	}
}

func TestCopyTree(t *testing.T) {
	src := filepath.Join(t.TempDir(), "src")
	dst := filepath.Join(t.TempDir(), "dst")
	if err := os.MkdirAll(filepath.Join(src, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "bin", "tool.txt"), []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	var last CopyProgress
	if err := CopyTree(context.Background(), src, dst, func(p CopyProgress) { last = p }); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dst, "bin", "tool.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "payload" {
		t.Fatalf("unexpected content: %q", data)
	}
	if last.TotalFiles != 1 || last.Files != 1 {
		t.Fatalf("unexpected progress: %#v", last)
	}
}

func TestCopyTreeCanceled(t *testing.T) {
	src := filepath.Join(t.TempDir(), "src")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := CopyTree(ctx, src, filepath.Join(t.TempDir(), "dst"), nil); err == nil {
		t.Fatal("expected canceled context error")
	}
}
