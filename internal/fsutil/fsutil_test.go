package fsutil

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestCalculateDirSize(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("123"), 0644); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0775); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "b.txt"), []byte("45"), 0644); err != nil {
		t.Fatal(err)
	}

	size, err := CalculateDirSize(dir)
	if err != nil {
		t.Fatalf("CalculateDirSize failed: %v", err)
	}
	if size != 5 {
		t.Errorf("expected 5, got %d", size)
	}
}

func TestCopyDirReplace(t *testing.T) {
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "file.txt"), []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(src, "nested"), 0775); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "nested", "inner.txt"), []byte("inner"), 0644); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(t.TempDir(), "dst")
	if err := CopyDirReplace(src, dst); err != nil {
		t.Fatalf("CopyDirReplace failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dst, "file.txt"))
	if err != nil || string(data) != "content" {
		t.Errorf("expected file.txt with 'content', got %q err=%v", data, err)
	}
	data, err = os.ReadFile(filepath.Join(dst, "nested", "inner.txt"))
	if err != nil || string(data) != "inner" {
		t.Errorf("expected nested/inner.txt with 'inner', got %q err=%v", data, err)
	}
}

func TestCopyDirReplace_ReplacesExistingContent(t *testing.T) {
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "new.txt"), []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}

	dst := t.TempDir()
	stale := filepath.Join(dst, "stale.txt")
	if err := os.WriteFile(stale, []byte("stale"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := CopyDirReplace(src, dst); err != nil {
		t.Fatalf("CopyDirReplace failed: %v", err)
	}

	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("expected stale.txt to be removed, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "new.txt")); err != nil {
		t.Errorf("expected new.txt to be present: %v", err)
	}
}

func TestCopyDirReplace_PreservesSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires elevated privileges on windows")
	}

	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "target.txt"), []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("target.txt", filepath.Join(src, "link.txt")); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(t.TempDir(), "dst")
	if err := CopyDirReplace(src, dst); err != nil {
		t.Fatalf("CopyDirReplace failed: %v", err)
	}

	linkTarget, err := os.Readlink(filepath.Join(dst, "link.txt"))
	if err != nil {
		t.Fatalf("expected link.txt to be a symlink: %v", err)
	}
	if linkTarget != "target.txt" {
		t.Errorf("expected symlink target 'target.txt', got %q", linkTarget)
	}
}
