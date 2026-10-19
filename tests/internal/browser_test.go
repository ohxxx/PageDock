//go:build desktop

// [INPUT]: Local websites, native permission settings, and real embedded WebViews.
// [OUTPUT]: Permission recovery, notification-record cleanup, navigation/reload, persistence, and unchanged state on failure.
// [POS]: End-to-end settings permission tests using only temporary desktop config and local services.
// [PROTOCOL]: Update this header, internal/AGENTS.md, and tests/AGENTS.md; update the root AGENTS.md when directory boundaries change.
package pagedock

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo"
	prefs "github.com/ohxxx/PageDock/internal/config"
	views "github.com/ohxxx/PageDock/internal/view"
)

func TestDesktopSettingsPermissionRecovery(t *testing.T) {
	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><title>Permission test</title><h1>Browser permissions</h1><script>window.permissionAtLoad=Notification.permission;</script>`)
	}))
	defer site.Close()
	config := prefs.Config{URL: site.URL + "/home", AllowHTTP: true}
	if err := desktopApp.store.Save(config); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(desktopApp.store.Path)
	desktopApp.start()
	desktopApp.showSettings()
	settings, _ := desktopWindows()
	defer settings.Destroy()
	defer func() {
		_, win := desktopWindows()
		if win != nil {
			win.Destroy()
		}
	}()
	_, main := desktopWindows()
	if main.Page() != nil {
		t.Fatal("website container is not native UI")
	}
	waitPermission := func(want string) {
		t.Helper()
		eventually(t, func() bool {
			v, _ := mygo.EvalAs[string](desktopPage(), `window.permissionAtLoad || ''`)
			return v == want
		})
	}
	waitPermission("denied")
	var loads atomic.Int32
	desktopPage().OnDidFinishLoad(func() { loads.Add(1) })
	loadPage := func(action func(), want string) {
		t.Helper()
		before := loads.Load()
		action()
		eventually(t, func() bool { return loads.Load() > before && desktopPage().URL() == want })
	}
	currentURL := site.URL + "/conversation?code=test-session&view=unread#message"
	loadPage(func() {
		if err := desktopPage().LoadURL(currentURL); err != nil {
			t.Fatal(err)
		}
	}, currentURL)
	waitPermission("denied")
	// Menu actions must still target the embedded WebView after pages move into the native container.
	loadPage(func() { desktopApp.history(false) }, config.URL)
	loadPage(func() { desktopApp.history(true) }, currentURL)
	var reload *mygo.MenuItem
	for _, top := range desktopApp.menu().Items() {
		for _, item := range top.Submenu {
			if item.Label == "刷新" {
				reload = item
			}
		}
	}
	if reload == nil || reload.Click == nil {
		t.Fatal("browser reload action missing")
	}
	loadPage(func() { reload.Click(reload, main) }, currentURL)
	waitPermission("denied")
	nativeSettingsUpdate(t, func(v *views.Settings) {
		v.Draft.AllowNotifications = true
		v.Draft.AllowCamera = true
		v.Draft.AllowMicrophone = true
		v.Begin(views.Save)
	})
	eventually(t, main.IsDestroyed)
	waitPermission("granted")
	if desktopPage().URL() != currentURL {
		t.Fatal("changing permission lost current conversation URL")
	}
	saved, err := desktopApp.store.Load()
	if err != nil || !saved.AllowNotifications || !saved.AllowCamera || !saved.AllowMicrophone || saved.URL != config.URL {
		t.Fatalf("permission persistence: %+v, %v", saved, err)
	}

	received := make(chan mygo.NotificationOptions, 1)
	present, dismiss := showPWANotification, closePWANotification
	showPWANotification = func(opts mygo.NotificationOptions, _ func()) error { received <- opts; return nil }
	closePWANotification = func(string) {}
	defer func() { appNotifications.clear(); showPWANotification, closePWANotification = present, dismiss }()
	if got := pageValue(t, desktopPage(), `await new Promise((resolve,reject)=>{const n=new Notification("恢复通知",{body:"权限已允许"});n.onshow=resolve;n.onerror=reject;});return "shown";`); got != "shown" {
		t.Fatal(got)
	}
	select {
	case opts := <-received:
		if opts.Title != "恢复通知" {
			t.Fatal(opts)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("permission recovery did not reach native notifications")
	}

	_, main = desktopWindows()
	eventually(t, func() bool {
		var idle bool
		nativeSettingsUpdate(t, func(v *views.Settings) { idle = !v.Busy })
		return idle
	})
	nativeSettingsUpdate(t, func(v *views.Settings) {
		v.Draft.AllowNotifications = false
		v.Draft.AllowCamera = false
		v.Draft.AllowMicrophone = false
		v.Begin(views.Save)
	})
	eventually(t, main.IsDestroyed)
	waitPermission("denied")
	if desktopPage().URL() != currentURL {
		t.Fatal("blocking notifications changed the current URL")
	}
	got := pageValue(t, desktopPage(), `try{await mygo.call("PWA.Receive",{title:"blocked"});return "allowed";}catch{return "denied";}`)
	if got != "denied" {
		t.Fatal("blocked notification reached Go")
	}

	// A config write failure must not grant website permissions or replace the current WebView.
	desktopApp.mu.Lock()
	store, oldWeb := desktopApp.store, desktopApp.mainWeb
	desktopApp.store = prefs.Store{Path: t.TempDir()}
	next := desktopApp.config
	next.AllowNotifications, next.AllowCamera = true, true
	err = desktopApp.saveSettingsLocked(next)
	desktopApp.store = store
	unchanged := !desktopApp.config.AllowNotifications && !desktopApp.config.AllowCamera && desktopApp.mainWeb == oldWeb
	desktopApp.mu.Unlock()
	if err == nil || !unchanged {
		t.Fatal("failed save changed the browser permission")
	}
	_, main = desktopWindows()
	main.Destroy()
	desktopApp.start()
	waitPermission("denied")
	if desktopPage().URL() != config.URL {
		t.Fatal("restart did not keep saved home URL")
	}
}
