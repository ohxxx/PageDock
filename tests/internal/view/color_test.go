//go:build desktop

// [INPUT]: Local light/dark pages, dynamic themes, cross-origin navigation, and real WebViews.
// [OUTPUT]: Actual native title bar pixels follow the page top while preserving page layout and permission isolation.
// [POS]: Top white-strip regression tests inspect rendered pixels instead of source strings.
// [PROTOCOL]: Update this header, internal/view/AGENTS.md, and tests/AGENTS.md.
package view

import (
	"bytes"
	"fmt"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func TestDesktopBrowserBackgroundFollowsPage(t *testing.T) {
	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><style>body{margin:0;background:#121212}header{height:60px}</style><header><button onclick="this.textContent='clicked'">click</button></header>`)
	}))
	defer site.Close()
	foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<!doctype html><style>body{background:#165a53}</style><p>new origin</p>`)
	}))
	defer foreign.Close()
	v := &Browser{}
	win := mygo.NewWindow(mygo.WindowOptions{
		Width: 800, Height: 600, TitleBarStyle: mygo.TitleBarHidden,
		Content: ui.View(v.Render),
	})
	defer win.Destroy()
	web, err := win.NewWebView(mygo.WebViewOptions{})
	if err != nil {
		t.Fatal(err)
	}
	win.Update(func() { v.Web = web })
	page := web.Page()
	v.FollowBackground(win, page)
	if err := page.LoadURL(site.URL); err != nil {
		t.Fatal(err)
	}
	win.Show()
	waitColor := func(want ui.Color) {
		t.Helper()
		eventually(t, func() bool {
			data, err := win.CapturePage()
			if err != nil {
				t.Fatal(err)
			}
			image, err := png.Decode(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			r, g, b, _ := image.At(image.Bounds().Dx()/2, 5).RGBA()
			return ui.RGB(uint8(r>>8), uint8(g>>8), uint8(b>>8)) == want
		})
	}
	waitColor(ui.Hex("#121212"))
	pageValue(t, page, `document.body.style.background='#e8edf2'; return 'done'`)
	waitColor(ui.Hex("#e8edf2"))
	pageValue(t, page, `document.querySelector('header').style.background='#2c3035'; return 'done'`)
	waitColor(ui.Hex("#2c3035"))
	if got := pageValue(t, page, `document.querySelector('button').click(); return document.querySelector('button').textContent`); got != "clicked" {
		t.Fatal("page action stopped working")
	}
	if err := page.LoadURL(foreign.URL); err != nil {
		t.Fatal(err)
	}
	waitColor(ui.Hex("#165a53"))
}
