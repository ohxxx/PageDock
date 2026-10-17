// [INPUT]: Original Windows backend and updater source from the pinned MyGo version.
// [OUTPUT]: Exact replacement text and supplemental source-file definitions.
// [POS]: Windows target patch definitions read by build preparation on every host, not host runtime code.
// [PROTOCOL]: Update patches/mygo/windows/AGENTS.md, the preparation tool, and platform behavior tests.
package windows

var Replacements = []struct{ File, Before, After string }{
	{"internal/windows/titlebar.go", "dark := w.b.isDark()", `dark := w.b.isDark()
	if bg := w.bg; bg != nil && bg.A == 255 {
		dark = captionBackgroundIsDark(bg.R, bg.G, bg.B)
	}`},
	{"internal/windows/window.go", "w.bg = &c", `w.bg = &c
	if w.caption != nil {
		w.caption.paint()
	}`},
	{"internal/windows/dialog.go", "text := o.Message", `const mbDefaultButtonOffset = 0x100
	style |= uintptr(o.DefaultID) * mbDefaultButtonOffset
	text := o.Message`},
	{"internal/windows/surface.go", "s.keyTaken = false\n\t\tmodifiers := mods()", `s.keyTaken = false
		if cmd, ok := s.w.accels[accelKey{vk: uint32(wp), mods: currentModifiers()}]; ok {
			s.w.b.menuCommand(cmd, s.w)
			s.keyTaken = true
			return 0, true
		}
		modifiers := mods()`},
	{"internal/windows/menu.go", "e := b.menus.entries[cmd]\n\tif e == nil {\n\t\treturn\n\t}", `e := b.menus.entries[cmd]
	if e == nil {
		return
	}
	if state, _, _ := procGetMenuState.Call(e.menu, uintptr(cmd), mfByCommand); state&mfGrayed != 0 {
		return
	}`},
	{"internal/windows/webview.go", "func (w *window) becomeReady() {", "func (w *window) finishReady() {"},
	{"internal/windows/window.go", "ready        bool", "ready        bool\n\tpermissionCancel func()"},
	{"internal/windows/webview.go", "func (w *window) failPending(err error) {", `func (w *window) failPending(err error) {
	if cancel := w.permissionCancel; cancel != nil {
		w.permissionCancel = nil
		cancel()
	}`},
	{"internal/windows/webview.go", "const allow, deny = 1, 2 // COREWEBVIEW2_PERMISSION_STATE", `if err := transientPermission(args); err != nil {
		log.Print(err)
		comCall(args, permPutState, 2) // DENY: Reject persistent grants that cannot be reauthorized.
		return
	}
	const allow, deny = 1, 2 // COREWEBVIEW2_PERMISSION_STATE`},
	{"updater.go", `olds, _ := filepath.Glob(filepath.Join(filepath.Dir(startExe), ".*.old-*"))`, `if runtime.GOOS == "windows" {
		ctx, cancel := context.WithTimeout(context.Background(), updateCleanupTimeout)
		defer cancel()
		if err := cleanUpdateExecutables(ctx, startExe, os.Remove); err != nil {
			log.Printf("mygo: cleaning old executables: %v", err)
		}
		return
	}
	olds, _ := filepath.Glob(filepath.Join(filepath.Dir(startExe), ".*.old-*"))`},
}

var Files = []struct{ Source, Target string }{
	{"color.go", "internal/windows/pagedock_caption_color.go"},
	{"windows/permissions.go.in", "internal/windows/pagedock_permissions.go"},
	{"updater/cleanup.go", "pagedock_update_cleanup.go"},
}
