//go:build desktop

// [INPUT]: A locale-isolated local desktop session, temporary config, and httptest websites.
// [OUTPUT]: Native settings, WebView cross-site login callbacks, persistence, and settings-operation isolation tests.
// [POS]: Explicitly enabled desktop integration tests; never contact remote services or clear real browsing data.
// [PROTOCOL]: Update this header, internal/AGENTS.md, and tests/AGENTS.md; update the root AGENTS.md when directory boundaries change.
package pagedock

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo"
	prefs "github.com/ohxxx/PageDock/internal/config"
	l10n "github.com/ohxxx/PageDock/internal/locale"
	views "github.com/ohxxx/PageDock/internal/view"
)

var desktopApp *application

func TestMain(m *testing.M) {
	// Isolate MyGo locale persistence; existing interaction tests always use Simplified Chinese.
	os.Setenv("MYGO_LOCALE", l10n.Chinese)
	dir, err := os.MkdirTemp("", "pagedock-desktop-test-")
	if err != nil {
		panic(err)
	}
	mygo.App.SetName("PageDockTests")
	mygo.App.SetPath(mygo.PathUserData, dir)
	desktopApp = &application{store: prefs.Store{Path: filepath.Join(dir, prefs.Filename)}}
	mygo.App.OnLocaleChanged(func(string) { go desktopApp.refreshLanguage() })
	mygo.Bind(&PWA{app: desktopApp})
	mygo.App.OnWindowAllClosed(func() {})
	exitCode := make(chan int, 1)
	mygo.App.WhenReady(func() {
		go func() {
			exitCode <- m.Run()
			mygo.App.Quit()
		}()
	})
	if err := mygo.App.Run(); err != nil {
		panic(err)
	}
	code := <-exitCode
	os.RemoveAll(dir)
	os.Exit(code)
}

func eventually(t *testing.T, check func() bool) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("timed out waiting for desktop state")
}

func pageValue(t *testing.T, page *mygo.Page, script string) string {
	t.Helper()
	value, err := mygo.EvalAs[string](page, script)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func desktopWindows() (*mygo.Window, *mygo.Window) {
	desktopApp.mu.Lock()
	defer desktopApp.mu.Unlock()
	return desktopApp.settings, desktopApp.main
}

func desktopPage() *mygo.Page {
	desktopApp.mu.Lock()
	defer desktopApp.mu.Unlock()
	return desktopApp.mainWeb.Page()
}

// Use the same begin entry as native controls; access UI fields only through main-thread Update calls.
func nativeSettingsUpdate(t *testing.T, fn func(*views.Settings)) {
	t.Helper()
	desktopApp.mu.Lock()
	win, view := desktopApp.settings, desktopApp.settingsUI
	desktopApp.mu.Unlock()
	done := make(chan struct{})
	win.Update(func() { fn(view); close(done) })
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("native settings UI did not respond")
	}
}

func TestDesktopSettingsAndSiteBoundary(t *testing.T) {
	var foreignRequests atomic.Int32
	var siteURL string
	foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		foreignRequests.Add(1)
		if r.URL.Path == "/authorize" {
			http.Redirect(w, r, siteURL+"/callback?code=test-code&state=test-state", http.StatusFound)
			return
		}
		fmt.Fprint(w, "<h1>Other site</h1>")
	}))
	defer foreign.Close()
	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, foreign.URL+"/authorize", http.StatusFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, "<!doctype html><title>Local PageDock test</title><h1>%s</h1>", r.URL.Path)
	}))
	defer site.Close()
	siteURL = site.URL
	desktopApp.start()
	settings, _ := desktopWindows()
	if settings.Page() != nil {
		t.Fatal("settings created a WebView instead of native content")
	}
	// Native settings validation failures must neither dispatch a save nor write files.
	var status string
	nativeSettingsUpdate(t, func(v *views.Settings) {
		v.Draft.URL = site.URL
		v.Begin(views.Save)
		status = v.Status
	})
	if !strings.Contains(status, "HTTPS") {
		t.Fatalf("HTTP rejection: %s", status)
	}
	if _, err := os.Stat(desktopApp.store.Path); !os.IsNotExist(err) {
		t.Fatalf("invalid form wrote configuration: %v", err)
	}
	nativeSettingsUpdate(t, func(v *views.Settings) {
		v.Draft.AllowHTTP, v.Draft.AllowNotifications = true, true
		v.Begin(views.Save)
	})
	var main *mygo.Window
	eventually(t, func() bool {
		_, main = desktopWindows()
		if main == nil {
			return false
		}
		body, _ := mygo.EvalAs[string](desktopPage(), `document.querySelector('h1')?.textContent`)
		return body == "/"
	})
	value := pageValue(t, desktopPage(), `try { await mygo.call('Settings.State'); return 'allowed'; } catch { return 'denied'; }`)
	if value != "denied" {
		t.Fatalf("remote page reached Go settings: %s", value)
	}
	_, _ = desktopPage().Eval(`location.href = '/next'`)
	eventually(t, func() bool { return desktopPage().URL() == site.URL+"/next" })
	foreignURL, _ := json.Marshal(foreign.URL + "/escape")
	_, _ = desktopPage().Eval("location.href = " + string(foreignURL))
	eventually(t, func() bool {
		body, _ := mygo.EvalAs[string](desktopPage(), `document.querySelector('h1')?.textContent`)
		return desktopPage().URL() == foreign.URL+"/escape" && body == "Other site"
	})
	value = pageValue(t, desktopPage(), `try { await mygo.call('Settings.State'); return 'allowed'; } catch { return 'denied'; }`)
	if value != "denied" {
		t.Fatal("cross-origin navigation granted access to Go settings")
	}
	redirectURL, _ := json.Marshal(site.URL + "/redirect")
	_, _ = desktopPage().Eval("location.href = " + string(redirectURL))
	eventually(t, func() bool {
		body, _ := mygo.EvalAs[string](desktopPage(), `document.querySelector('h1')?.textContent`)
		return desktopPage().URL() == site.URL+"/callback?code=test-code&state=test-state" && body == "/callback"
	})
	if foreignRequests.Load() < 2 {
		t.Fatal("the login redirect did not reach the other origin")
	}
	persisted, err := desktopApp.store.Load()
	if err != nil || persisted.URL != site.URL+"/" {
		t.Fatalf("login callback replaced the saved home URL: %+v, %v", persisted, err)
	}
	// Changing sites closes the old window and applies the new policy together with config.
	eventually(t, func() bool {
		var idle bool
		nativeSettingsUpdate(t, func(v *views.Settings) { idle = !v.Busy })
		return idle
	})
	nativeSettingsUpdate(t, func(v *views.Settings) {
		v.Draft = prefs.Config{URL: foreign.URL + "/", AllowHTTP: true}
		v.Begin(views.Save)
	})
	eventually(t, main.IsDestroyed)
	got, err := desktopApp.store.Load()
	if err != nil || got.URL != foreign.URL+"/" {
		t.Fatalf("persisted site: %+v, %v", got, err)
	}
	settings, main = desktopWindows()
	main.Destroy()
	settings.Destroy()
	// Repeat startup: restore the URL from disk and create the website window directly.
	desktopApp.start()
	_, main = desktopWindows()
	eventually(t, func() bool {
		body, _ := mygo.EvalAs[string](desktopPage(), `document.querySelector('h1')?.textContent`)
		return body == "Other site"
	})
	if desktopPage().URL() != foreign.URL+"/" {
		t.Fatal("startup did not restore the saved site")
	}
	main.Destroy()
}
