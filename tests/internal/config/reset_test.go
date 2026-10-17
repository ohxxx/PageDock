// [INPUT]: Temporary config directories, simulated browser cleanup results, and window state rewritten at exit.
// [OUTPUT]: Real storage results for full reset, icon-restoration failure/retry, no-op without a marker, and preservation of unrelated files.
// [POS]: Reset persistence tests without system WebView cleanup or user data access.
// [PROTOCOL]: Update this header, internal/config/AGENTS.md, and tests/AGENTS.md; update the root AGENTS.md when directory boundaries change.
package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResetClearsStateOnNextLaunch(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "data[1]")
	store := Store{Path: filepath.Join(dir, Filename)}
	want := Config{URL: "https://example.com/", AllowHTTP: true, AllowNotifications: true, AllowCamera: true, AllowMicrophone: true, AllowGeolocation: true}
	if err := store.Save(want); err != nil {
		t.Fatal(err)
	}
	write := func(name string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte("saved data"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("unrelated.txt")
	write(".config-interrupted.json")
	if reset, err := store.CompleteReset(func() error { return nil }); err != nil || reset {
		t.Fatalf("unrequested reset: %v, %v", reset, err)
	}
	if got, err := store.Load(); err != nil || got != want {
		t.Fatalf("normal launch lost config: %+v, %v", got, err)
	}
	cleared := false
	if err := store.PrepareReset(func() error { cleared = true; return nil }); err != nil || !cleared {
		t.Fatalf("prepare reset: %v, cleared=%v", err, cleared)
	}
	// MyGo rewrites window state when the old process exits; delete it in the new process before creating windows.
	write(windowStateFilename)
	write(windowStateFilename + ".tmp")
	if reset, err := store.CompleteReset(func() error { return nil }); err != nil || !reset {
		t.Fatalf("complete reset: %v, %v", reset, err)
	}
	if got, err := store.Load(); err != nil || got != (Config{}) {
		t.Fatalf("first-launch config: %+v, %v", got, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 || entries[0].Name() != "unrelated.txt" {
		t.Fatalf("reset scope: %v, %v", entries, err)
	}
	// A subsequent launch after saving new settings must not repeat an old reset request.
	if err := store.Save(want); err != nil {
		t.Fatal(err)
	}
	if reset, err := store.CompleteReset(func() error { return nil }); err != nil || reset {
		t.Fatalf("reset repeated: %v, %v", reset, err)
	}
	if got, err := store.Load(); err != nil || got != want {
		t.Fatalf("new config removed: %+v, %v", got, err)
	}
}

func TestResetDoesNotDiscardSettingsBeforeDeferredBrowserCleanup(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), Filename)}
	want := Config{URL: "https://example.com/", AppName: "Test name", AllowNotifications: true}
	if err := store.Save(want); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(filepath.Dir(store.Path), resetFilename)
	if err := os.WriteFile(marker, []byte("all"), 0o600); err != nil {
		t.Fatal(err)
	}
	restored := false
	reset, err := store.CompleteReset(func() error { restored = true; return nil })
	if err == nil || reset || restored {
		t.Fatalf("unfinished browser cleanup discarded settings: reset=%v restored=%v err=%v", reset, restored, err)
	}
	if got, err := store.Load(); err != nil || got != want {
		t.Fatalf("pending browser cleanup lost settings: %+v, %v", got, err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("pending request lost: %v", err)
	}
}

func TestResetClearFailurePreservesSettings(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), Filename)}
	want := Config{URL: "https://example.com/", AllowNotifications: true}
	if err := store.Save(want); err != nil {
		t.Fatal(err)
	}
	failure := errors.New("browser cleanup failed")
	if err := store.PrepareReset(func() error { return failure }); !errors.Is(err, failure) {
		t.Fatalf("lost cleanup error: %v", err)
	}
	if reset, err := store.CompleteReset(func() error { return nil }); err != nil || reset {
		t.Fatalf("failed cleanup scheduled reset: %v, %v", reset, err)
	}
	if got, err := store.Load(); err != nil || got != want {
		t.Fatalf("failed cleanup lost settings: %+v, %v", got, err)
	}
}

func TestResetMarkerFailureDoesNotScheduleReset(t *testing.T) {
	dir := t.TempDir()
	store := Store{Path: filepath.Join(dir, Filename)}
	want := Config{URL: "https://example.com/"}
	if err := store.Save(want); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(dir, resetFilename)
	if err := os.Mkdir(marker, 0o700); err != nil {
		t.Fatal(err)
	}
	err := store.PrepareReset(func() error { return nil })
	if err == nil || !strings.Contains(err.Error(), "网站数据已清除") {
		t.Fatalf("partial cleanup not reported: %v", err)
	}
	if reset, err := store.CompleteReset(func() error { return nil }); err == nil || reset {
		t.Fatalf("invalid marker accepted: %v, %v", reset, err)
	}
	if got, err := store.Load(); err != nil || got != want {
		t.Fatalf("marker failure lost config: %+v, %v", got, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 2 {
		t.Fatalf("temporary marker leaked: %v, %v", entries, err)
	}
}

func TestResetCompletionRetriesAfterFailure(t *testing.T) {
	dir := t.TempDir()
	store := Store{Path: filepath.Join(dir, Filename)}
	if err := store.Save(Config{URL: "https://example.com/"}); err != nil {
		t.Fatal(err)
	}
	if err := store.PrepareReset(func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	blocked := filepath.Join(dir, windowStateFilename)
	if err := os.Mkdir(blocked, 0o700); err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(blocked, "keep")
	if err := os.WriteFile(child, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if reset, err := store.CompleteReset(func() error { return nil }); err == nil || reset {
		t.Fatalf("partial reset reported success: %v, %v", reset, err)
	}
	if _, err := os.Stat(filepath.Join(dir, resetFilename)); err != nil {
		t.Fatalf("retry marker lost: %v", err)
	}
	if err := os.Remove(child); err != nil {
		t.Fatal(err)
	}
	if reset, err := store.CompleteReset(func() error { return nil }); err != nil || !reset {
		t.Fatalf("retry failed: %v, %v", reset, err)
	}
	if got, err := store.Load(); err != nil || got != (Config{}) {
		t.Fatalf("retry did not restore first launch: %+v, %v", got, err)
	}
}

func TestResetWaitsForIconRestore(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), Filename)}
	want := Config{URL: "https://example.com/"}
	if err := store.Save(want); err != nil {
		t.Fatal(err)
	}
	called := false
	if reset, err := store.CompleteReset(func() error { called = true; return nil }); err != nil || reset || called {
		t.Fatal("unrequested icon reset")
	}
	if err := store.PrepareReset(func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	failure := errors.New("application is read only")
	if reset, err := store.CompleteReset(func() error { return failure }); reset || !errors.Is(err, failure) {
		t.Fatalf("lost icon failure: %v", err)
	}
	if got, err := store.Load(); err != nil || got != want {
		t.Fatal("settings cleared before icon restoration")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(store.Path), resetFilename)); err != nil {
		t.Fatal("retry marker lost")
	}
	if reset, err := store.CompleteReset(func() error { return nil }); !reset || err != nil {
		t.Fatalf("retry: %v", err)
	}
}
