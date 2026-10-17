//go:build desktop

// [INPUT]: Delayed local responses and real WebView load, reload, navigation, stop, and network failure.
// [OUTPUT]: The native loading indicator appears before the first response and disappears on completion, cancellation, or failure.
// [POS]: Loading feedback integration tests without online requests or simulated completion callbacks.
// [PROTOCOL]: Update this header, internal/view/AGENTS.md, and tests/AGENTS.md.
package view

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func TestDesktopBrowserLoading(t *testing.T) {
	requests := make(chan chan struct{}, 8)
	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/favicon.ico" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		release := make(chan struct{})
		select {
		case requests <- release:
		case <-r.Context().Done():
			return
		}
		select {
		case <-release:
		case <-r.Context().Done():
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		fmt.Fprint(w, `<!doctype html><style>body{background:#121212}</style><a href="/next">Next</a>`)
	}))
	defer site.Close()
	v := &Browser{}
	win := mygo.NewWindow(mygo.WindowOptions{Width: 800, Height: 600, TitleBarStyle: mygo.TitleBarHidden, Content: ui.View(v.Render)})
	defer win.Destroy()
	web, err := win.NewWebView(mygo.WebViewOptions{})
	if err != nil {
		t.Fatal(err)
	}
	win.Update(func() { v.Web = web })
	page := web.Page()
	v.FollowLoading(win, page)
	win.Show()
	waitLoading := func(want bool) {
		t.Helper()
		eventually(t, func() bool {
			state := make(chan bool, 1)
			win.Update(func() { state <- v.loading })
			return <-state == want
		})
	}
	waitRequest := func() chan struct{} {
		t.Helper()
		select {
		case release := <-requests:
			return release
		case <-time.After(10 * time.Second):
			t.Fatal("web view did not start a request")
			return nil
		}
	}
	load := func(action func()) {
		t.Helper()
		action()
		release := waitRequest()
		waitLoading(true)
		close(release)
		waitLoading(false)
	}
	load(func() {
		if err := page.LoadURL(site.URL); err != nil {
			t.Fatal(err)
		}
	})
	load(page.Reload)
	load(func() {
		pageValue(t, page, `setTimeout(() => document.querySelector('a').click(), 0); return 'scheduled'`)
	})
	page.Reload()
	release := waitRequest()
	waitLoading(true)
	page.Stop()
	waitLoading(false)
	close(release)

	// A closed local listening port causes a real network failure; HTTP 404 is not a navigation failure.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	failedURL := "http://" + listener.Addr().String()
	listener.Close()
	var failed atomic.Bool
	page.OnDidFailLoad(func(*mygo.LoadError) { failed.Store(true) })
	if err := page.LoadURL(failedURL); err != nil {
		t.Fatal(err)
	}
	eventually(t, failed.Load)
	waitLoading(false)
}
