// [INPUT]: Pinned MyGo registries for native windows and embedded WebViews.
// [OUTPUT]: Deletions that clear obsolete references in slice backing arrays.
// [POS]: Cross-platform dependency patches; closed UI objects must not remain GC roots.
// [PROTOCOL]: Update patches/mygo/AGENTS.md and the real desktop lifetime tests.
package shared

var Replacements = []struct{ File, Before, After string }{
	{"window.go", "\"runtime\"", "\"runtime\"\n\t\"slices\""},
	{"window.go", "windows.list = append(windows.list[:i:i], windows.list[i+1:]...)", "windows.list = slices.Delete(windows.list, i, i+1)"},
	{"webview.go", "\"log\"", "\"log\"\n\t\"slices\""},
	{"webview.go", "h.webViews = append(h.webViews[:i:i], h.webViews[i+1:]...)", "h.webViews = slices.Delete(h.webViews, i, i+1)"},
}
