//go:build desktop

// [INPUT]: Real native windows and embedded WebViews with weak references.
// [OUTPUT]: Closed windows and individually destroyed pages become unreachable after pending UI work drains.
// [POS]: Cross-platform regression for MyGo window registry teardown.
// [PROTOCOL]: Update tests/internal/AGENTS.md and patches/mygo/AGENTS.md with lifecycle changes.
package pagedock

import (
	"runtime"
	"testing"
	"time"
	"weak"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func TestDesktopClosedWindowReclaimed(t *testing.T) {
	ref := closedWindowReference(t)
	assertReclaimed(t, ref, "closed window")
}

//go:noinline
func closedWindowReference(t *testing.T) weak.Pointer[mygo.Window] {
	t.Helper()
	win := mygo.NewWindow(mygo.WindowOptions{Hidden: true, Content: ui.View(func(c *ui.Context) {})})
	if win == nil {
		t.Fatal("create window")
	}
	ref := weak.Make(win)
	win.Destroy()
	return ref
}

func TestDesktopDestroyedWebViewReclaimed(t *testing.T) {
	win := mygo.NewWindow(mygo.WindowOptions{Hidden: true, Content: ui.View(func(c *ui.Context) {})})
	if win == nil {
		t.Fatal("create window")
	}
	defer win.Destroy()
	ref := closedPageReference(t, win)
	assertReclaimed(t, ref, "destroyed page with live host")
	runtime.KeepAlive(win)
}

//go:noinline
func closedPageReference(t *testing.T, host *mygo.Window) weak.Pointer[mygo.Page] {
	t.Helper()
	web, err := host.NewWebView(mygo.WebViewOptions{})
	if err != nil {
		t.Fatal(err)
	}
	ref := weak.Make(web.Page())
	web.Destroy()
	return ref
}

func assertReclaimed[T any](t *testing.T, ref weak.Pointer[T], name string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		mygo.RunOnMain(func() {})
		runtime.GC()
		if ref.Value() == nil {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("%s still retained after UI drain and GC", name)
}
