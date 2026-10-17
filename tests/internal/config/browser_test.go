// [INPUT]: Temporary browser profiles, neighboring files, and directory links.
// [OUTPUT]: Delete only browsing data without traversing links; missing directories are idempotent.
// [POS]: Actual deletion-scope tests without user profiles.
// [PROTOCOL]: Update internal/config/AGENTS.md, tests/AGENTS.md, and validation records.
package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRemoveBrowsingProfileScope(t *testing.T) {
	dir := t.TempDir()
	profile := filepath.Join(dir, "WebView2")
	for _, name := range []string{"Default/Network/Cookies", "Default/Local Storage/leveldb/token", "Default/IndexedDB/data", "Default/Service Worker/CacheStorage/data"} {
		path := filepath.Join(profile, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("saved browser data"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	neighbor := filepath.Join(dir, "keep.txt")
	if err := os.WriteFile(neighbor, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := removeBrowsingProfile(context.Background(), profile); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(profile); !os.IsNotExist(err) {
			t.Fatalf("profile remains: %v", err)
		}
		if got, err := os.ReadFile(neighbor); err != nil || string(got) != "keep" {
			t.Fatalf("neighbor changed: %q %v", got, err)
		}
	}
}

func TestRemoveBrowsingProfileRejectsLinks(t *testing.T) {
	dir := t.TempDir()
	external := t.TempDir()
	if err := os.WriteFile(filepath.Join(external, "keep"), []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "linked")
	if err := os.Symlink(external, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	for _, path := range []string{link, filepath.Join(link, "WebView2")} {
		if err := removeBrowsingProfile(context.Background(), path); err == nil {
			t.Fatalf("linked path accepted: %s", path)
		}
	}
	if got, err := os.ReadFile(filepath.Join(external, "keep")); err != nil || string(got) != "keep" {
		t.Fatalf("external data changed: %q %v", got, err)
	}
}
