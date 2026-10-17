// [INPUT]: The starting EXE path, a deadline context, and a single-file deletion operation.
// [OUTPUT]: Remove committed old images of the same executable with bounded busy-file retries, retaining files and reporting failures.
// [POS]: MyGo Windows startup cleanup patch, copied into the dependency root package and using its existing background cleanup entry.
// [PROTOCOL]: Update patches/mygo/AGENTS.md, build preparation, and file behavior tests.
package mygo

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const updateCleanupTimeout = 15 * time.Second
const updateCleanupRetry = 100 * time.Millisecond

func cleanUpdateExecutables(ctx context.Context, executable string, remove func(string) error) error {
	// Use the same path as EXE appearance transactions, including symlink and Windows short-name launches.
	executable, err := filepath.EvalSymlinks(executable)
	if err != nil {
		return err
	}
	dir := filepath.Dir(executable)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	prefix := "." + filepath.Base(executable) + ".old-"
	var pending []string
	for _, entry := range entries {
		name := entry.Name()
		if len(name) > len(prefix) && strings.EqualFold(name[:len(prefix)], prefix) {
			pending = append(pending, filepath.Join(dir, name))
		}
	}
	// Retry only copies present at startup; do not scan new transaction files created during this run.
	for len(pending) > 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		var failures []error
		retry := pending[:0]
		for _, path := range pending {
			info, err := os.Lstat(path)
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err == nil {
				if !info.Mode().IsRegular() {
					continue
				}
				err = remove(path)
			}
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				retry = append(retry, path)
				failures = append(failures, err)
			}
		}
		pending = retry
		if len(pending) == 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return errors.Join(append(failures, ctx.Err())...)
		case <-time.After(updateCleanupRetry):
		}
	}
	return nil
}
