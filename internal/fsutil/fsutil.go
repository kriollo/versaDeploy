package fsutil

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// CalculateDirSize calculates the total size of all files in a directory recursively.
func CalculateDirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

// CopyDirReplace replaces dst entirely with a copy of src: dst is removed first
// (if it exists) and every file, directory, and symlink under src is recreated
// under dst, preserving file permissions.
func CopyDirReplace(src, dst string) error {
	if err := os.RemoveAll(dst); err != nil {
		return fmt.Errorf("failed to clear destination %s: %w", dst, err)
	}
	if err := os.MkdirAll(dst, 0775); err != nil {
		return fmt.Errorf("failed to create destination %s: %w", dst, err)
	}

	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		target := filepath.Join(dst, rel)

		if d.Type()&os.ModeSymlink != 0 {
			linkTarget, err := os.Readlink(path)
			if err != nil {
				return fmt.Errorf("failed to read symlink %s: %w", path, err)
			}
			return os.Symlink(linkTarget, target)
		}

		if d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			return os.MkdirAll(target, info.Mode().Perm())
		}

		return copyFile(path, target, d)
	})
}

func copyFile(src, dst string, d os.DirEntry) error {
	info, err := d.Info()
	if err != nil {
		return err
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
