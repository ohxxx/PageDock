//go:build desktop

// [INPUT]: Isolated config, local websites, the real MyGo main thread, and locale events.
// [OUTPUT]: Immediate language switching after save, menu/title refresh, failure preservation, and system-language restoration after full reset.
// [POS]: Preserve system locale preferences; shared TestMain uses MYGO_LOCALE to prevent MyGo preference writes.
// [PROTOCOL]: Update this header, internal/AGENTS.md, and tests/AGENTS.md, including Validation records.
package pagedock

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/i18n"
	prefs "github.com/ohxxx/PageDock/internal/config"
	l10n "github.com/ohxxx/PageDock/internal/locale"
)

func TestDesktopLanguageSaveAndFailure(t *testing.T) {
	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "<!doctype html><title>Language test</title><p>Local page</p>")
	}))
	defer site.Close()
	defer applyLanguage(l10n.Chinese)
	store := prefs.Store{Path: filepath.Join(t.TempDir(), prefs.Filename)}
	config := prefs.Config{URL: site.URL + "/", AllowHTTP: true, Language: l10n.Chinese}
	if err := store.Save(config); err != nil {
		t.Fatal(err)
	}
	a := &application{store: store}
	off := mygo.App.OnLocaleChanged(func(string) { go a.refreshLanguage() })
	defer off()
	a.start()
	a.showSettings()
	defer func() {
		a.mu.Lock()
		a.restarting = true
		a.stopSiteLocked()
		a.settings.Destroy()
		a.mu.Unlock()
	}()
	config.Language = l10n.English
	a.mu.Lock()
	err := a.saveSettingsLocked(config)
	a.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool {
		return mygo.App.Locale() == l10n.English && a.settings.Title() == "PageDock · Settings"
	})
	if got, err := store.Load(); err != nil || got != config {
		t.Fatalf("saved language: %+v, %v", got, err)
	}
	eventually(t, func() bool {
		for _, item := range mygo.App.Menu().Items() {
			if item.Label == "Website" {
				return true
			}
		}
		return false
	})
	// A real config replacement failure must not change the active language or saved in-memory state early.
	blocked := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocked, []byte("blocked"), 0600); err != nil {
		t.Fatal(err)
	}
	a.mu.Lock()
	a.store = prefs.Store{Path: filepath.Join(blocked, prefs.Filename)}
	next := config
	next.Language = l10n.Chinese
	err = a.saveSettingsLocked(next)
	got := a.config
	a.store = store
	a.mu.Unlock()
	if err == nil || got != config || mygo.App.Locale() != l10n.English {
		t.Fatalf("failed save changed the language: %+v, %v, %s", got, err, mygo.App.Locale())
	}
	if err := store.PrepareReset(func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	reset, err := store.CompleteReset(func() error { applyLanguage(""); return nil })
	if err != nil || !reset {
		t.Fatalf("reset: %v, %v", reset, err)
	}
	want := i18n.Match(mygo.App.PreferredLanguages(), l10n.Catalog.Languages())
	if want == "" {
		want = l10n.English
	}
	if mygo.App.Locale() != want {
		t.Fatalf("reset did not follow system: %s, want %s", mygo.App.Locale(), want)
	}
}
