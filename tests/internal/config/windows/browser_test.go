//go:build windows

// [INPUT]: Temporary Windows WebView2 profiles and Win32 handles that deny shared deletion.
// [OUTPUT]: Cleanup succeeds after locks release, times out honestly, and locates data by the internal name.
// [POS]: Native Windows file cleanup tests confined to temporary directories.
// [PROTOCOL]: Update internal/config/AGENTS.md, tests/AGENTS.md, and validation records.
package config

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/egoist/mygo"
)

func TestWindowsBrowserCleanupWaitsForFileRelease(t *testing.T) {
	profile := filepath.Join(t.TempDir(), "WebView2")
	if err := os.Mkdir(profile, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(profile, "Cookies")
	if err := os.WriteFile(path, []byte("login"), 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := syscall.CreateFile(p, syscall.GENERIC_READ, syscall.FILE_SHARE_READ, nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if handle != 0 {
			syscall.CloseHandle(handle)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	err = removeBrowsingProfile(ctx, profile)
	cancel()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("locked profile should time out: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("locked data disappeared", err)
	}
	closed := make(chan struct{})
	go func() { time.Sleep(browserCleanupRetry); syscall.CloseHandle(handle); close(closed) }()
	ctx, cancel = context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := removeBrowsingProfile(ctx, profile); err != nil {
		t.Fatal(err)
	}
	<-closed
	handle = 0
	if _, err := os.Stat(profile); !os.IsNotExist(err) {
		t.Fatalf("profile remains: %v", err)
	}
}

func TestWindowsBrowserCleanupUsesLocalProfile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LOCALAPPDATA", dir)
	t.Setenv("WEBVIEW2_USER_DATA_FOLDER", "")
	profile := filepath.Join(dir, mygo.App.Name(), "WebView2")
	if err := os.MkdirAll(profile, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(profile, "Cookies"), []byte("login"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ClearBrowsingData(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(profile); !os.IsNotExist(err) {
		t.Fatalf("profile remains: %v", err)
	}
	t.Setenv("WEBVIEW2_USER_DATA_FOLDER", filepath.Join(dir, "external"))
	if err := ClearBrowsingData(); err == nil {
		t.Fatal("external profile override silently accepted")
	}
}
