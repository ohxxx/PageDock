// [INPUT]: Saved branding, light/dark pages, narrow windows, text scaling, and native input events.
// [OUTPUT]: Click/keyboard activation, correct menu anchors, and long names that do not overlap caption buttons.
// [POS]: Shared title bar behavior tests without source-text assertions.
// [PROTOCOL]: Update internal/view/AGENTS.md, tests/AGENTS.md, and Windows device validation records.
package view

import (
	"fmt"
	"image"
	"image/color"
	"strings"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	prefs "github.com/ohxxx/PageDock/internal/config"
	l10n "github.com/ohxxx/PageDock/internal/locale"
)

const (
	titleMenuIconSize  = 18
	titleMenuDragSpace = 48
	titleMenuInset     = 6
)

func TestTitleMenuInputAndLayout(t *testing.T) {
	for _, width := range []float32{420, 1180} {
		for _, background := range []ui.Color{ui.Hex("#ffffff"), ui.Hex("#121212")} {
			calls := 0
			var x, y int
			menu := &Menu{name: strings.Repeat("应用名称", 16), onOpen: func(px, py int) { calls++; x, y = px, py }}
			web := &mygo.WebView{}
			v := &Browser{Menu: menu, Web: web, background: background}
			tt := ui.NewTester(v.Render, int(width), 400)
			tt.SetTitleBar(ui.TitleBar{Height: 32, Right: 138})
			tt.SetPreferences(ui.Preferences{TextScale: 2, HighContrast: true})
			label := fmt.Sprintf(l10n.Text("appMenu"), menu.name)
			r, ok := tt.Find(label)
			if !ok || r.W <= titleMenuIconSize || r.X < 0 || r.X+r.W > width-138-titleMenuDragSpace+titleMenuInset || r.Y+r.H > 32 {
				t.Fatalf("menu overlaps window controls or has no target: %+v", r)
			}
			if err := tt.Click(label); err != nil {
				t.Fatal(err)
			}
			if calls != 1 || x != int(r.X) || y != int(r.Y+r.H) {
				t.Fatalf("click did not open below button: calls=%d anchor=(%d,%d) bounds=%+v", calls, x, y, r)
			}
			tt.Key(0, ui.KeyEnter)
			tt.Key(0, ui.KeySpace)
			if calls != 3 {
				t.Fatalf("keyboard did not activate button: %d", calls)
			}
			if tt.WebViewAt(20, 33) != web {
				t.Fatal("menu changed page layout")
			}
			tt.SetTitleBar(ui.TitleBar{})
			if _, ok := tt.Find(label); ok || tt.WebViewAt(20, 1) != web {
				t.Fatal("full screen retained title menu space")
			}
		}
	}
}

func TestTitleMenuSavedAppearance(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 18, 18))
	for y := 0; y < 18; y++ {
		for x := 0; x < 18; x++ {
			img.SetRGBA(x, y, color.RGBA{R: 255, A: 255})
		}
	}
	menu := &Menu{icon: ui.NewBitmap(img)}
	v := &Settings{Menu: menu, Draft: prefs.Config{AppName: "未保存名称"}}
	tt := ui.NewTester(v.Render, 860, 700)
	tt.SetTitleBar(ui.TitleBar{Height: 32, Right: 138})
	r, ok := tt.Find(fmt.Sprintf(l10n.Text("appMenu"), l10n.Text("appName")))
	if !ok {
		t.Fatal("default name does not follow current language")
	}
	pixel := tt.Image().RGBAAt(int(r.X+8+titleMenuIconSize/2), int(r.Y+r.H/2))
	if pixel.R < 240 || pixel.G > 10 || pixel.B > 10 {
		t.Fatalf("saved logo is not visible: %v", pixel)
	}
	if _, ok := tt.Find(fmt.Sprintf(l10n.Text("appMenu"), v.Draft.AppName)); ok {
		t.Fatal("unsaved name changed title menu")
	}
}
