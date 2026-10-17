// [INPUT]: Caption button areas, loading state, page background, fullscreen state, and window size.
// [OUTPUT]: Visible loading feedback without blocked clicks or residue; correct fullscreen and scaled layouts.
// [POS]: Native layout behavior tests requiring neither real websites nor source-string checks.
// [PROTOCOL]: Update this header, internal/view/AGENTS.md, and tests/AGENTS.md.
package view

import (
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func TestBrowserReservesWindowControls(t *testing.T) {
	web := &mygo.WebView{}
	v := &Browser{Web: web}
	tt := ui.NewTester(v.Render, 800, 600)
	for _, height := range []float32{28, 40, 0} {
		tt.SetTitleBar(ui.TitleBar{Height: height, Left: 80})
		if height > 0 && tt.WebViewAt(20, height/2) != nil {
			t.Fatal("web page covers window controls or drag region")
		}
		if tt.WebViewAt(20, height+1) != web || tt.WebViewAt(799, 599) != web {
			t.Fatalf("page does not fill the available content area with title bar height %v", height)
		}
	}
	tt.SetSize(420, 400)
	if tt.WebViewAt(1, 1) != web || tt.WebViewAt(419, 399) != web {
		t.Fatal("full-screen layout left a gap after resizing")
	}
}

func TestBrowserLoadingIndicator(t *testing.T) {
	web := &mygo.WebView{}
	v := &Browser{Web: web}
	tt := ui.NewTester(v.Render, 800, 600)
	for _, background := range []ui.Color{ui.Hex("#121212"), ui.Hex("#f7f7f7")} {
		v.background = background
		for _, height := range []float32{28, 40, 0} {
			tt.SetTitleBar(ui.TitleBar{Height: height, Left: 80})
			for _, reduceMotion := range []bool{false, true} {
				v.loading = true
				tt.SetPreferences(ui.Preferences{ReduceMotion: reduceMotion})
				tt.Frame()
				r, ok := tt.Find("正在加载页面")
				if !ok || r.X != 0 || r.W != 800 || r.H != browserLoadingHeight || r.Y != max(0, height-browserLoadingHeight) {
					t.Fatalf("incorrect loading indicator bounds: %+v", r)
				}
				if tt.WebViewAt(400, height+1) != web || tt.WebViewAt(400, 599) != web {
					t.Fatal("loading indicator moved the page or blocked clicks")
				}
				if reduceMotion {
					r, g, b, _ := tt.Image().At(400, int(r.Y)).RGBA()
					if ui.RGB(uint8(r>>8), uint8(g>>8), uint8(b>>8)) == background {
						t.Fatal("loading indicator is invisible against the page background")
					}
				}
				v.loading = false
				tt.Frame()
				if _, ok := tt.Find("正在加载页面"); ok {
					t.Fatal("loading indicator remained after completion")
				}
				if height > 0 {
					r, g, b, _ := tt.Image().At(400, int(height)-1).RGBA()
					if ui.RGB(uint8(r>>8), uint8(g>>8), uint8(b>>8)) != background {
						t.Fatal("loading indicator left a border or shadow")
					}
				}
			}
		}
	}
}
