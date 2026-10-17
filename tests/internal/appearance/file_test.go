// [INPUT]: Temporary executable files, real config writes, and file errors injected at each step.
// [OUTPUT]: Successful replacement, save-failure rollback, and preservation after replacement failures.
// [POS]: Portable tests for Windows icon file transactions.
// [PROTOCOL]: Update internal/appearance/AGENTS.md and tests/AGENTS.md.
package appearance

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	prefs "github.com/ohxxx/PageDock/internal/config"
)

func TestIconExecutableTransaction(t *testing.T) {
	for _, failure := range []string{"", "first rename", "second rename", "save"} {
		t.Run(failure, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "应用.exe")
			if err := os.WriteFile(path, []byte("original"), 0o700); err != nil {
				t.Fatal(err)
			}
			store := prefs.Store{Path: filepath.Join(dir, prefs.Filename)}
			old := prefs.Config{URL: "https://example.com/"}
			if err := store.Save(old); err != nil {
				t.Fatal(err)
			}
			wantErr := errors.New("write denied")
			calls, saves := 0, 0
			rename := func(from, to string) error {
				calls++
				if (failure == "first rename" && calls == 1) || (failure == "second rename" && calls == 2) {
					return wantErr
				}
				return os.Rename(from, to)
			}
			err := ReplaceExecutable(path, []byte("updated"), rename, func() error {
				saves++
				if failure == "save" {
					return wantErr
				}
				next := old
				next.AllowCamera = true
				return store.Save(next)
			})
			if failure == "" && err != nil {
				t.Fatal(err)
			}
			if failure != "" && !errors.Is(err, wantErr) {
				t.Fatalf("lost failure: %v", err)
			}
			data, err := ReadExecutable(path)
			if err != nil {
				t.Fatal(err)
			}
			want := "original"
			if failure == "" {
				want = "updated"
			}
			if string(data) != want {
				t.Fatalf("program = %q, want %q", data, want)
			}
			got, err := store.Load()
			if err != nil || got.AllowCamera != (failure == "") {
				t.Fatalf("config = %+v, %v", got, err)
			}
			if (failure == "first rename" || failure == "second rename") && saves != 0 {
				t.Fatal("saved before program replacement")
			}
			entries, err := os.ReadDir(dir)
			if err != nil || len(entries) != 2 {
				t.Fatalf("left temporary files: %v, %v", entries, err)
			}
		})
	}
}

func TestIconExecutableKeepsRecoveryOnRollbackFailure(t *testing.T) {
	for _, failedRename := range []int{3, 4} {
		t.Run(string(rune('0'+failedRename)), func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "app.exe")
			if err := os.WriteFile(path, []byte("original"), 0o700); err != nil {
				t.Fatal(err)
			}
			calls := 0
			err := ReplaceExecutable(path, []byte("updated"), func(from, to string) error {
				calls++
				if calls == failedRename {
					return errors.New("rename denied")
				}
				return os.Rename(from, to)
			}, func() error { return errors.New("save denied") })
			if err == nil {
				t.Fatal("rollback failure hidden")
			}
			backups, err := filepath.Glob(filepath.Join(dir, ".pagedock-icon-recovery-*.exe"))
			if err != nil || len(backups) != 1 {
				t.Fatalf("recovery copy lost: %v, %v", backups, err)
			}
			data, err := os.ReadFile(backups[0])
			if err != nil || string(data) != "original" {
				t.Fatalf("recovery is not original: %q, %v", data, err)
			}
			if _, err := os.Stat(path); err != nil {
				t.Fatal("no executable left at launch path")
			}
		})
	}
}
