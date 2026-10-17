//go:build desktop && windows

// [INPUT]: A real Windows desktop, page colors opposite to the system theme, and dynamic background changes.
// [OUTPUT]: Inspect button glyphs on screen, ensuring native caption buttons are checked as well as container backgrounds.
// [POS]: Windows-only desktop tests using isolated windows and preserving user system theme settings.
// [PROTOCOL]: Update internal/view/AGENTS.md and tests/AGENTS.md, including Validation records.
package view

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"syscall"
	"testing"
	"unsafe"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func TestDesktopWindowsCaptionFollowsPage(t *testing.T) {
	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `<!doctype html><body style="margin:0;background:#121212"></body>`)
	}))
	defer site.Close()
	previous := mygo.Theme.Source()
	defer mygo.Theme.SetSource(previous)
	mygo.Theme.SetSource(mygo.ThemeLight)
	v := &Browser{}
	win := mygo.NewWindow(mygo.WindowOptions{
		Width: 800, Height: 600, TitleBarStyle: mygo.TitleBarHidden,
		BackgroundColor: WindowBackground, Content: ui.View(v.Render),
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
	focus(win)
	hwnd := win.NativeHandle()
	var diagnostic string
	defer func() {
		if t.Failed() {
			t.Log(diagnostic)
		}
	}()
	for _, tc := range []struct {
		background string
		theme      mygo.ThemeSource
		white      bool
	}{
		{"#121212", mygo.ThemeLight, true},
		{"#f8f8f8", mygo.ThemeDark, false},
		{"#121212", mygo.ThemeDark, true},
	} {
		mygo.Theme.SetSource(tc.theme)
		pageValue(t, page, fmt.Sprintf("document.body.style.background=%q; return 'done'", tc.background))
		eventually(t, func() bool {
			result := make(chan bool, 1)
			win.Update(func() {
				want := ui.Hex(tc.background)
				glyphs := windowsCaptionHasGlyphs(hwnd, tc.white, &diagnostic)
				if v.background != want {
					diagnostic = fmt.Sprintf("background=%v want=%v; %s", v.background, want, diagnostic)
				}
				result <- v.background == want && glyphs
			})
			return <-result
		})
	}
}

// Inspect glyphs inside all three screen buttons; native Content screenshots omit these separate child windows.
func windowsCaptionHasGlyphs(hwnd uintptr, white bool, diagnostic *string) bool {
	user := syscall.NewLazyDLL("user32.dll")
	gdi := syscall.NewLazyDLL("gdi32.dll")
	foreground, _, _ := user.NewProc("GetForegroundWindow").Call()
	if foreground != hwnd {
		*diagnostic = fmt.Sprintf("foreground=%x want=%x", foreground, hwnd)
		return false
	}
	var bounds struct{ left, top, right, bottom int32 }
	var found bool
	const childWindow, nextWindow = 5, 2
	child, _, _ := user.NewProc("GetWindow").Call(hwnd, childWindow)
	for child != 0 {
		var name [64]uint16
		user.NewProc("GetClassNameW").Call(child, uintptr(unsafe.Pointer(&name[0])), uintptr(len(name)))
		if syscall.UTF16ToString(name[:]) == "MyGoCaption" {
			user.NewProc("GetWindowRect").Call(child, uintptr(unsafe.Pointer(&bounds)))
			// The top resize edge has the same name but contains no buttons.
			if bounds.bottom-bounds.top > 16 && bounds.right-bounds.left > 0 {
				found = true
				break
			}
		}
		child, _, _ = user.NewProc("GetWindow").Call(child, nextWindow)
	}
	if !found {
		*diagnostic = "caption child not found"
		return false
	}
	syscall.NewLazyDLL("dwmapi.dll").NewProc("DwmFlush").Call()
	dc, _, _ := user.NewProc("GetDC").Call(0)
	if dc == 0 {
		return false
	}
	defer user.NewProc("ReleaseDC").Call(0, dc)
	// Copy the button area once; per-pixel GetPixel would synchronize the whole virtual-GPU screen each time.
	memoryDC, _, _ := gdi.NewProc("CreateCompatibleDC").Call(dc)
	if memoryDC == 0 {
		return false
	}
	defer gdi.NewProc("DeleteDC").Call(memoryDC)
	var header struct {
		Size                        uint32
		Width, Height               int32
		Planes, BitCount            uint16
		Compression, SizeImage      uint32
		XPels, YPels                int32
		ColorsUsed, ColorsImportant uint32
	}
	header.Size, header.Width, header.Height = uint32(unsafe.Sizeof(header)), bounds.right-bounds.left, bounds.top-bounds.bottom
	header.Planes, header.BitCount = 1, 32
	var bits unsafe.Pointer
	bitmap, _, _ := gdi.NewProc("CreateDIBSection").Call(dc, uintptr(unsafe.Pointer(&header)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if bitmap == 0 {
		return false
	}
	defer gdi.NewProc("DeleteObject").Call(bitmap)
	previous, _, _ := gdi.NewProc("SelectObject").Call(memoryDC, bitmap)
	defer gdi.NewProc("SelectObject").Call(memoryDC, previous)
	const captureCopy = 0x00CC0020 | 0x40000000 // SRCCOPY | CAPTUREBLT
	if ok, _, _ := gdi.NewProc("BitBlt").Call(memoryDC, 0, 0, uintptr(header.Width), uintptr(-header.Height), dc, uintptr(bounds.left), uintptr(bounds.top), captureCopy); ok == 0 {
		return false
	}
	gdi.NewProc("GdiFlush").Call()
	image := unsafe.Slice((*byte)(bits), int(header.Width*-header.Height*4))
	width, height := (bounds.right-bounds.left)/3, bounds.bottom-bounds.top
	for i := int32(0); i < 3; i++ {
		pixels := 0
		cx, cy := bounds.left+width*i+width/2, bounds.top+height/2
		for y := cy - height/3; y <= cy+height/3; y++ {
			for x := cx - width/4; x <= cx+width/4; x++ {
				offset := ((y-bounds.top)*header.Width + x - bounds.left) * 4
				b, g, r := image[offset], image[offset+1], image[offset+2]
				if white && min(r, g, b) > 160 || !white && max(r, g, b) < 100 {
					pixels++
				}
			}
		}
		if pixels < 4 {
			*diagnostic = fmt.Sprintf("button=%d matching pixels=%d white=%v bounds=%+v", i, pixels, white, bounds)
			return false
		}
	}
	return true
}
