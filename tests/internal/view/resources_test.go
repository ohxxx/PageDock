//go:build desktop

// [INPUT]: A static loopback page and actual native window visibility/navigation events.
// [OUTPUT]: Reused sampling resources, suspension while hidden, and recovery without focus events.
// [POS]: Real WebView resource regression; no external websites or device inputs.
// [PROTOCOL]: Update tests/internal/view/AGENTS.md and resource validation records.
package view

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync/atomic"
	"testing"
	"time"
	"weak"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func TestDesktopBrowserSamplingResources(t *testing.T) {
	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!doctype html><style>body{background:#121212}</style><p>Sampling</p><script>
window.createdCanvases=0;window.sampleReads=0;
const create=document.createElement.bind(document);document.createElement=function(name,...args){if(name==='canvas')createdCanvases++;return create(name,...args)};
const read=CanvasRenderingContext2D.prototype.getImageData;CanvasRenderingContext2D.prototype.getImageData=function(...args){sampleReads++;return read.apply(this,args)};
window.ready=true;</script>`)
	}))
	defer site.Close()
	v := &Browser{}
	win := mygo.NewWindow(mygo.WindowOptions{Width: 600, Height: 400, Content: ui.View(v.Render)})
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
	eventually(t, func() bool { value, _ := mygo.EvalAs[bool](page, "window.sampleReads > 0"); return value })
	count := func(name string) int {
		t.Helper()
		value, err := mygo.EvalAs[int](page, name)
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	// Let a stable page back off, then count actual canvas reads rather than source or timer constants.
	time.Sleep(4 * time.Second)
	before := count("sampleReads")
	time.Sleep(4 * time.Second)
	after := count("sampleReads")
	canvases := count("createdCanvases")
	t.Logf("idle: canvases=%d reads/4s=%d", canvases, after-before)
	if canvases != 1 || after-before > 3 {
		t.Errorf("idle sampling: canvases=%d reads/4s=%d; want one canvas and at most 3 reads", canvases, after-before)
	}
	win.Hide()
	time.Sleep(250 * time.Millisecond)
	before = count("sampleReads")
	time.Sleep(time.Second)
	after = count("sampleReads")
	if after != before {
		t.Fatalf("hidden page sampled %d times", after-before)
	}
	if _, err := page.Eval("document.body.style.background='#e8edf2'"); err != nil {
		t.Fatal(err)
	}
	win.Show()
	eventually(t, func() bool {
		color := make(chan ui.Color, 1)
		win.Update(func() { color <- v.background })
		return <-color == ui.Hex("#e8edf2")
	})
}

func TestDesktopBrowserFollowerStops(t *testing.T) {
	win := mygo.NewWindow(mygo.WindowOptions{Width: 400, Height: 300, Content: ui.View(func(c *ui.Context) { ui.Text(c, "Follower lifetime") })})
	defer win.Destroy()
	web, err := win.NewWebView(mygo.WebViewOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	reference := followWithReference(win, web.Page(), &calls)
	win.Show()
	eventually(t, func() bool { return calls.Load() > 1 })
	win.Hide()
	// Allow the current sample to finish before checking suspension.
	time.Sleep(100 * time.Millisecond)
	before := calls.Load()
	time.Sleep(200 * time.Millisecond)
	if calls.Load() != before {
		t.Fatal("follower kept sampling a hidden window")
	}
	win.Show()
	eventually(t, func() bool { return calls.Load() > before })
	win.Minimize()
	eventually(t, win.IsMinimized)
	time.Sleep(100 * time.Millisecond)
	before = calls.Load()
	time.Sleep(200 * time.Millisecond)
	if calls.Load() != before {
		t.Fatal("follower kept sampling a minimized window")
	}
	win.Restore()
	eventually(t, func() bool { return calls.Load() > before })
	win.Destroy()
	eventually(t, func() bool { runtime.GC(); return reference.Value() == nil })
}

func followWithReference(win *mygo.Window, page *mygo.Page, calls *atomic.Int32) weak.Pointer[[4096]byte] {
	owner := new([4096]byte)
	followBrowser(win, page, func(context.Context, bool) time.Duration {
		calls.Add(1)
		runtime.KeepAlive(owner)
		return 25 * time.Millisecond
	})
	return weak.Make(owner)
}

func TestDesktopBrowserFollowerAppVisibility(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("application hiding is macOS-only")
	}
	win := mygo.NewWindow(mygo.WindowOptions{Width: 400, Height: 300, Content: ui.View(func(c *ui.Context) { ui.Text(c, "App visibility") })})
	defer win.Destroy()
	web, err := win.NewWebView(mygo.WebViewOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	reference := followWithReference(win, web.Page(), &calls)
	win.Show()
	eventually(t, func() bool { return calls.Load() > 1 })
	other := mygo.NewWindow(mygo.WindowOptions{Width: 300, Height: 200, Content: ui.View(func(c *ui.Context) { ui.Text(c, "Other window") })})
	defer other.Destroy()
	focus(other)
	defer mygo.App.Show()
	for range 5 {
		mygo.App.Hide()
		eventually(t, mygo.App.IsHidden)
		time.Sleep(100 * time.Millisecond)
		before := calls.Load()
		time.Sleep(200 * time.Millisecond)
		if calls.Load() != before {
			t.Fatal("follower kept sampling a hidden application")
		}
		mygo.App.Show()
		// App unhide must wake the unfocused website without another focus call.
		eventually(t, func() bool { return calls.Load() > before })
	}
	win.Destroy()
	eventually(t, func() bool { runtime.GC(); return reference.Value() == nil })
}
