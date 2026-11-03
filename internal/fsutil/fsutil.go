package fsutil

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// ReadJSON reads a JSON file into T. A missing file returns def without error.
func ReadJSON[T any](path string, def T) (T, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return def, nil
		}
		return def, err
	}
	var v T
	if err := json.Unmarshal(data, &v); err != nil {
		return def, fmt.Errorf("parse %s: %w", path, err)
	}
	return v, nil
}

// WriteJSON writes v as indented JSON using an atomic replace.
func WriteJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return AtomicWrite(path, data, 0o644)
}

// AtomicWrite writes data to a temporary file in the same directory and renames it.
func AtomicWrite(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

// CopyProgress describes the state of a tree copy.
type CopyProgress struct {
	Files      int64
	Bytes      int64
	TotalFiles int64
	TotalBytes int64
	Current    string
}

// CopyTree copies src to dst, preserving modes, symlinks and mod times.
func CopyTree(ctx context.Context, src, dst string, onProgress func(CopyProgress)) error {
	totalFiles, totalBytes, err := treeStats(ctx, src)
	if err != nil {
		return err
	}
	progress := CopyProgress{TotalFiles: totalFiles, TotalBytes: totalBytes}
	emit := func(current string) {
		progress.Current = current
		if onProgress != nil {
			onProgress(progress)
		}
	}
	return filepath.WalkDir(src, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(dst, 0o755)
		}
		target := filepath.Join(dst, rel)
		info, err := d.Info()
		if err != nil {
			return err
		}
		switch {
		case d.Type()&os.ModeSymlink != 0:
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			if err := os.Symlink(link, target); err != nil {
				return fmt.Errorf("create symlink %s: %w", target, err)
			}
			progress.Files++
			emit(rel)
		case d.IsDir():
			if err := os.MkdirAll(target, info.Mode().Perm()); err != nil {
				return err
			}
		default:
			if err := copyFile(ctx, path, target, info, &progress); err != nil {
				return err
			}
			emit(rel)
		}
		return nil
	})
}

func copyFile(ctx context.Context, src, dst string, info os.FileInfo, progress *CopyProgress) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return err
	}
	n, copyErr := io.Copy(out, &ctxReader{ctx: ctx, r: in})
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	progress.Files++
	progress.Bytes += n
	_ = os.Chtimes(dst, time.Now(), info.ModTime())
	return nil
}

func treeStats(ctx context.Context, root string) (files int64, bytes int64, err error) {
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		files++
		if d.Type()&os.ModeSymlink == 0 {
			bytes += info.Size()
		}
		return nil
	})
	return files, bytes, err
}

type ctxReader struct {
	ctx context.Context
	r   io.Reader
}

func (c *ctxReader) Read(p []byte) (int, error) {
	select {
	case <-c.ctx.Done():
		return 0, c.ctx.Err()
	default:
	}
	return c.r.Read(p)
}

// DirSize returns the total size of regular files under root.
func DirSize(root string) (int64, error) {
	var total int64
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			if errors.Is(walkErr, os.ErrNotExist) {
				return nil
			}
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if info.Mode().IsRegular() {
			total += info.Size()
		}
		return nil
	})
	return total, err
}
