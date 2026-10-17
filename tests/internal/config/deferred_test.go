// [INPUT]: Temporary config, window state, simulated persistent browsing data, and deferred cleanup results.
// [OUTPUT]: Restart without waiting for browser callbacks, then clear browsing data first; cover both cleanup scopes, failure retention, and retries.
// [POS]: Cold-start reset behavior tests without real user browsers.
// [PROTOCOL]: Update internal/config/AGENTS.md, tests/AGENTS.md, and validation records.
package config

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestDeferredResetClearsBrowserBeforeSettings(t *testing.T) {
	for _, all := range []bool{false, true} {
		t.Run(map[bool]string{false: "site", true: "all"}[all], func(t *testing.T) {
			dir := t.TempDir()
			store := Store{Path: filepath.Join(dir, Filename)}
			want := Config{URL: "https://example.com/", AppName: "Custom", AllowNotifications: true}
			if err := store.Save(want); err != nil {
				t.Fatal(err)
			}
			profile := filepath.Join(dir, "browser-profile")
			if err := os.Mkdir(profile, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(profile, "Cookies"), []byte("session"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := store.DeferReset(all); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(profile); err != nil {
				t.Fatal("browser removed before restart", err)
			}
			if got, err := store.Load(); err != nil || got != want {
				t.Fatalf("config changed before restart: %+v, %v", got, err)
			}
			window := filepath.Join(dir, windowStateFilename)
			if err := os.WriteFile(window, []byte("old process exit state"), 0o600); err != nil {
				t.Fatal(err)
			}
			failure := errors.New("profile locked")
			if err := store.CompleteBrowsingReset(func() error { return failure }); !errors.Is(err, failure) {
				t.Fatalf("cleanup failure lost: %v", err)
			}
			if got, err := store.Load(); err != nil || got != want {
				t.Fatalf("failed cleanup lost config: %+v, %v", got, err)
			}
			if _, err := os.Stat(filepath.Join(dir, resetFilename)); err != nil {
				t.Fatal("retry request lost", err)
			}
			if err := store.CompleteBrowsingReset(func() error {
				return removeBrowsingProfile(context.Background(), profile)
			}); err != nil {
				t.Fatal(err)
			}
			restored := false
			reset, err := store.CompleteReset(func() error {
				if _, err := os.Stat(profile); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("appearance restored before browser cleanup", err)
				}
				restored = true
				return nil
			})
			if err != nil || reset != all || restored != all {
				t.Fatalf("reset=%v restored=%v err=%v", reset, restored, err)
			}
			if all {
				want = Config{}
			}
			if got, err := store.Load(); err != nil || got != want {
				t.Fatalf("wrong config after reset: %+v, %v", got, err)
			}
			if _, err := os.Stat(window); all != errors.Is(err, os.ErrNotExist) {
				t.Fatalf("wrong window state scope: %v", err)
			}
			if err := store.CompleteBrowsingReset(func() error { t.Fatal("cleanup repeated"); return nil }); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestDeferredResetRejectsInvalidRequest(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), Filename)}
	if err := os.WriteFile(filepath.Join(filepath.Dir(store.Path), resetFilename), []byte("unknown"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := store.CompleteBrowsingReset(func() error { t.Fatal("invalid request deleted data"); return nil }); err == nil {
		t.Fatal("invalid request accepted")
	}
}
