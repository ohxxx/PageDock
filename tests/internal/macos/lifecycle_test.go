//go:build desktop && darwin

// [INPUT]: Two real WKWebViews and their public configuration objects.
// [OUTPUT]: Shared process resources with separate per-page scripts and message handlers.
// [POS]: Native regression for MyGo's default WebKit process pool ownership.
// [PROTOCOL]: Update tests/internal/AGENTS.md and patches/mygo/AGENTS.md.
package pagedock

import (
	"testing"

	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo"
)

func TestDesktopWebKitPoolLifetime(t *testing.T) {
	var pools, controllers [2]objc.ID
	for i := range pools {
		win := mygo.NewWindow(mygo.WindowOptions{Hidden: true})
		if win == nil {
			t.Fatal("create window")
		}
		defer win.Destroy()
		mygo.RunOnMain(func() {
			pool := objc.ID(objc.GetClass("NSAutoreleasePool")).Send(objc.RegisterName("new"))
			defer pool.Send(objc.RegisterName("drain"))
			web := findWKView(objc.ID(win.NativeHandle()).Send(objc.RegisterName("contentView")))
			if web == 0 {
				return
			}
			config := web.Send(objc.RegisterName("configuration"))
			pools[i] = config.Send(objc.RegisterName("processPool"))
			controllers[i] = config.Send(objc.RegisterName("userContentController"))
		})
	}
	if pools[0] == 0 || pools[1] != pools[0] {
		t.Fatalf("default WebViews create separate process pools: %x, %x", pools[0], pools[1])
	}
	if controllers[0] == 0 || controllers[1] == 0 || controllers[0] == controllers[1] {
		t.Fatal("page script/message controllers must stay independent")
	}
}

func findWKView(view objc.ID) objc.ID {
	if view.Send(objc.RegisterName("isKindOfClass:"), objc.ID(objc.GetClass("WKWebView"))) != 0 {
		return view
	}
	children := view.Send(objc.RegisterName("subviews"))
	for i := uintptr(0); i < uintptr(children.Send(objc.RegisterName("count"))); i++ {
		if web := findWKView(children.Send(objc.RegisterName("objectAtIndex:"), i)); web != 0 {
			return web
		}
	}
	return 0
}
