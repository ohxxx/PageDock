// [INPUT]: Temporary EXEs, recovery copies, neighboring files, controlled deletion failures, and deadlines.
// [OUTPUT]: Cleanup scope, busy-file retries, retention on timeout, and cleanup continuation at the next launch.
// [POS]: Portable file tests sharing the production patch implementation; never read real desktop folders.
// [PROTOCOL]: Update patches/mygo/AGENTS.md and validation records.
package mygo

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestUpdateCleanupScope(t *testing.T) {
	dir := t.TempDir()
	// Match brackets in names literally rather than treating them as glob syntax.
	executable := filepath.Join(dir, "泊页 [test].exe")
	old := "." + filepath.Base(executable) + ".old-"
	preserved := []string{
		filepath.Base(executable), ".pagedock-icon-recovery-original.exe",
		".pagedock-icon-staged.exe", ".other.exe.old-original", old,
		".泊页 t.exe.old-original", "config.json",
	}
	for _, name := range append(preserved, old+"first", old+"second") {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	nested := filepath.Join(dir, old+"directory", "keep")
	if err := os.MkdirAll(filepath.Dir(nested), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(nested, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, old+"link")
	linkCreated := os.Symlink(executable, link) == nil // Check other cleanup boundaries even when Windows disallows symlink creation.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := cleanUpdateExecutables(ctx, executable, os.Remove); err != nil {
		t.Fatal(err)
	}
	for _, name := range preserved {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || string(data) != name {
			t.Fatalf("unrelated file changed: %s: %q, %v", name, data, err)
		}
	}
	for _, name := range []string{old + "first", old + "second"} {
		if _, err := os.Stat(filepath.Join(dir, name)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("old executable remains: %s, %v", name, err)
		}
	}
	if data, err := os.ReadFile(nested); err != nil || string(data) != "keep" {
		t.Fatalf("directory contents changed: %q, %v", data, err)
	}
	if linkCreated {
		if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("link changed: %v, %v", info, err)
		}
	}
}

func TestUpdateCleanupFromExecutableLink(t *testing.T) {
	dir := t.TempDir()
	executable := filepath.Join(dir, "app.exe")
	old := filepath.Join(dir, ".app.exe.old-previous")
	for _, path := range []string{executable, old} {
		if err := os.WriteFile(path, []byte("program"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	link := filepath.Join(t.TempDir(), "launch.exe")
	if err := os.Symlink(executable, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := cleanUpdateExecutables(context.Background(), link, os.Remove); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(old); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("old executable at resolved launch path remains: %v", err)
	}
	if data, err := os.ReadFile(link); err != nil || string(data) != "program" {
		t.Fatalf("launch target changed: %q, %v", data, err)
	}
}

func TestUpdateCleanupRetriesBusyFiles(t *testing.T) {
	dir := t.TempDir()
	executable := filepath.Join(dir, "app.exe")
	busy := filepath.Join(dir, ".APP.EXE.old-busy")
	ready := filepath.Join(dir, ".app.exe.old-ready")
	newTransaction := filepath.Join(dir, ".app.exe.old-new")
	for _, path := range []string{executable, busy, ready} {
		if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	calls := 0
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err := cleanUpdateExecutables(ctx, executable, func(path string) error {
		if filepath.Base(path) == filepath.Base(busy) {
			calls++
			if calls == 1 {
				if err := os.WriteFile(newTransaction, []byte("new"), 0o600); err != nil {
					t.Fatal(err)
				}
				return &os.PathError{Op: "remove", Path: path, Err: os.ErrPermission}
			}
			if _, err := os.Stat(ready); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("busy file blocked cleanup of an available file")
			}
		}
		return os.Remove(path)
	})
	if err != nil || calls < 2 {
		t.Fatalf("did not retry transient lock: %d, %v", calls, err)
	}
	if _, err := os.Stat(busy); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("busy file not removed after release: %v", err)
	}
	if data, err := os.ReadFile(newTransaction); err != nil || string(data) != "new" {
		t.Fatalf("cleanup reached a later transaction: %q, %v", data, err)
	}
}

func TestUpdateCleanupTimeoutThenNextLaunch(t *testing.T) {
	dir := t.TempDir()
	executable := filepath.Join(dir, "app.exe")
	old := filepath.Join(dir, ".app.exe.old-previous")
	if err := os.WriteFile(executable, []byte("current"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(old, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	err := cleanUpdateExecutables(ctx, executable, func(path string) error {
		return &os.PathError{Op: "remove", Path: path, Err: os.ErrPermission}
	})
	cancel()
	if !errors.Is(err, context.DeadlineExceeded) || !errors.Is(err, os.ErrPermission) {
		t.Fatalf("cleanup did not report timeout and file error: %v", err)
	}
	if data, err := os.ReadFile(old); err != nil || string(data) != "old" {
		t.Fatalf("timeout damaged old copy: %q, %v", data, err)
	}
	if err := cleanUpdateExecutables(context.Background(), executable, os.Remove); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(old); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("next launch did not remove old copy: %v", err)
	}
	if err := cleanUpdateExecutables(context.Background(), executable, os.Remove); err != nil {
		t.Fatalf("empty cleanup failed: %v", err)
	}
}
