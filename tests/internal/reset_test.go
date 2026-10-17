//go:build desktop

// [INPUT]: Temporary config with a pending reset and real native windows.
// [OUTPUT]: Return to empty settings with permissions disabled after cleanup; stale actions cannot write during restart.
// [POS]: Desktop state tests with a browser-cleanup substitute; preserve this machine's WebView data.
// [PROTOCOL]: Update this header, internal/AGENTS.md, and tests/AGENTS.md; update the root AGENTS.md when directory boundaries change.
package pagedock

import (
	"path/filepath"
	"testing"
	"time"

	prefs "github.com/ohxxx/PageDock/internal/config"
	views "github.com/ohxxx/PageDock/internal/view"
)

func TestDesktopResetReturnsToEmptySettings(t *testing.T) {
	store := prefs.Store{Path: filepath.Join(t.TempDir(), prefs.Filename)}
	old := prefs.Config{URL: "https://example.com/", AllowHTTP: true, AllowNotifications: true, AllowCamera: true, AllowMicrophone: true, AllowGeolocation: true}
	if err := store.Save(old); err != nil {
		t.Fatal(err)
	}
	if err := store.PrepareReset(func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if reset, err := store.CompleteReset(func() error { return nil }); err != nil || !reset {
		t.Fatalf("startup reset: %v, %v", reset, err)
	}
	a := &application{store: store}
	a.start()
	defer a.settings.Destroy()
	if a.main != nil || a.settings == nil || a.settings.Page() != nil {
		t.Fatal("reset did not open only native settings")
	}
	type state struct {
		draft             prefs.Config
		savedURL, problem string
		busy              bool
	}
	result := make(chan state, 1)
	a.settings.Update(func() {
		v := a.settingsUI
		result <- state{v.Draft, v.SavedURL, v.Problem, v.Busy}
	})
	select {
	case got := <-result:
		if got != (state{}) {
			t.Fatalf("settings retained old data: %+v", got)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("empty settings did not render")
	}
	// Simulate stale save, menu, or activation events queued after restart begins.
	a.mu.Lock()
	a.restarting = true
	a.mu.Unlock()
	a.runSettings(a.settingsUI, views.Save, old)
	a.home()
	a.activate()
	if got, err := store.Load(); err != nil || got != (prefs.Config{}) || a.main != nil {
		t.Fatalf("stale action restored old state: %+v, %v", got, err)
	}
}
