# Native views

| Path | Responsibility |
|---|---|
| `settings.go` | `Settings` draft, action entry, navigation, and save bar |
| `sections.go` | Shared settings sections, platform capability, default/custom icon preview, and contextual hints |
| `browser.go` | `Browser` embedded WebView, thin loading indicator, adaptive color/loading followers, and cancellation/unsubscription on close |
| `menu.go` | Shared saved name/image state, main-thread callback binding, and image previews |
| `theme.go` | Shared light/dark colors, dimensions, text scaling, and high-contrast tokens |
| `windows/menu.go` | Actual Windows title bar button drawing, layout, mouse, and keyboard behavior |
| `windows/about.go` | Native Windows About dialog and error propagation |

The Windows subpackage uses portable MyGo/UI interfaces, retaining compilation/rendering tests on every host. Callers still select it through existing platform conditions. Native system API implementations require matching build tags; directory names do not filter Go files.

Without a custom image, settings previews and the Windows title bar display the default logo from `internal/brand`. This fallback affects drawing only, preserving empty draft values, custom-image flags, and default-restoration transactions.

`Settings`, `Browser`, and `Menu` state belongs to the UI main thread. Background operations receive drafts by value and return through `Window.Update`; menu callbacks dispatch a goroutine before taking the application lock. The Windows package receives display values/callbacks without owning app state/locks, saving config, or initiating file transactions.

Both browser followers stop their timers while hidden/minimized and wake on window visibility/navigation and explicit macOS application DidHide/DidUnhide events. Application unhide need not activate the app or focus the website, so activation callbacks are insufficient. Loading uses native IsLoading with a 100 ms active cadence and a 1 s idle fallback, since MyGo does not report native LoadURL/Reload starts. Stable color sampling backs off from 500 ms to 2 s, reusing one offscreen 1-pixel canvas per document. Sampling never changes page layout; closing releases timers, event subscriptions, and the view callback.

Settings remain native MyGo Content; only website views host WebViews. Shared control tests live in `tests/internal/view/`; Windows menu/caption tests live in its windows/ group and still run in view through package mapping. Portable menu tests run on every host; screen-pixel tests require Windows desktop. Run `make test-desktop` serially and update headers, maps, and test notes.
