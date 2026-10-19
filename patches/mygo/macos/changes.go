// [INPUT]: Pinned MyGo window/WebView creation, backend lifetime, and application visibility events.
// [OUTPUT]: Bounded native window workers, one default WKProcessPool, and explicit visibility callbacks.
// [POS]: macOS target patches read by build preparation on every host.
// [PROTOCOL]: Update patches/mygo/AGENTS.md and native pool/endurance validation on dependency upgrades.
package macos

var Replacements = []struct{ File, Before, After string }{
	{"internal/darwin/window.go", `send(w.win, "setTabbingMode:", 2) // NSWindowTabbingModeDisallowed`, `send(w.win, "setTabbingMode:", 2) // NSWindowTabbingModeDisallowed
	// AppKit automatic ordering animations can strand a dispatch worker for
	// each show/hide on macOS 27.2. Eventually all workers are occupied and
	// WebKit cannot start the next navigation. Use the public per-window
	// policy before the first show; do not change system animation settings.
	const noAutomaticWindowAnimation = 2 // NSWindowAnimationBehaviorNone
	send(w.win, "setAnimationBehavior:", noAutomaticWindowAnimation)`},
	{"app.go", "onDidResignActive listeners[func()]", "onDidResignActive listeners[func()]\n\tonDidHide listeners[func()]\n\tonDidUnhide listeners[func()]"},
	{"app.go", "// OnOpenURL is called when the application is asked to open a URL of a", `// OnDidHide is called after the application becomes hidden on macOS.
func (a *Application) OnDidHide(fn func()) (off func()) {
	return a.onDidHide.add(fn, false)
}

// OnDidUnhide is called after the application becomes visible on macOS.
func (a *Application) OnDidUnhide(fn func()) (off func()) {
	return a.onDidUnhide.add(fn, false)
}

// OnOpenURL is called when the application is asked to open a URL of a`},
	{"app.go", "func (appHandler) DidResignActive() { fire(&App.onDidResignActive) }", `func (appHandler) DidResignActive() { fire(&App.onDidResignActive) }
func (appHandler) DidHide() { fire(&App.onDidHide) }
func (appHandler) DidUnhide() { fire(&App.onDidUnhide) }`},
	{"internal/platform/platform.go", "\tDidResignActive()", "\tDidResignActive()\n\tDidHide()\n\tDidUnhide()"},
	{"internal/darwin/app.go", `method("applicationDidBecomeActive:",`, `method("applicationDidHide:", func(self id, _ objc.SEL, n id) {
			theBackend.h.DidHide()
		}),
		method("applicationDidUnhide:", func(self id, _ objc.SEL, n id) {
			theBackend.h.DidUnhide()
		}),
		method("applicationDidBecomeActive:",`},
	{"internal/darwin/app.go", "notifyDelegate id", "notifyDelegate id\n\twebProcessPool id // Owned for the app lifetime, main thread only."},
	{"internal/darwin/app.go", "func (b *Backend) Run() error {\n\tsend(b.app, \"run\")", `func (b *Backend) Run() error {
	defer func() {
		release(b.webProcessPool)
		b.webProcessPool = 0
	}()
	send(b.app, "run")`},
	{"internal/darwin/window.go", `cfg = alloc("WKWebViewConfiguration")`, `cfg = alloc("WKWebViewConfiguration")
		// Reuse process resources: a fresh pool per view accumulates native
		// power observers across closed windows. Page handlers,
		// scripts, preferences and permissions remain independently configured.
		if w.b.webProcessPool == 0 {
			w.b.webProcessPool = alloc("WKProcessPool")
		}
		send(cfg, "setProcessPool:", uintptr(w.b.webProcessPool))`},
}
