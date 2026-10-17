//go:build windows

// [INPUT]: The current MyGo locale, fixed MyGo internal name, and Windows local app directory, before any WebView exists.
// [OUTPUT]: Clear the MyGo 0.5.1 app-specific WebView2 directory, waiting up to 15 seconds for the old process to release it.
// [POS]: Windows cleanup after restart; paths must match the pinned dependency's internal/windows/app.go.
// [PROTOCOL]: Recheck profile paths when upgrading MyGo; update the owning AGENTS.md and validation records.
package windows

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/egoist/mygo"
	l10n "github.com/ohxxx/PageDock/internal/locale"
)

const (
	webView2ProfileDirectory = "WebView2"
	browserCleanupTimeout    = 15 * time.Second
)

func ClearBrowsingData(remove func(context.Context, string) error) error {
	// An external override may point to shared browser data; never guess its path or falsely report successful cleanup.
	if os.Getenv("WEBVIEW2_USER_DATA_FOLDER") != "" {
		return errors.New(l10n.Text("profileExternal"))
	}
	dir := os.Getenv("LOCALAPPDATA")
	if dir == "" {
		dir = os.TempDir()
	}
	ctx, cancel := context.WithTimeout(context.Background(), browserCleanupTimeout)
	defer cancel()
	return remove(ctx, filepath.Join(dir, mygo.App.Name(), webView2ProfileDirectory))
}
