# Windows application integration

`window_test.go` checks native shortcuts, focus, menus, and clipboard behavior. `media_test.go` checks explicitly supplied file devices, permission changes, and local WebRTC. Both retain `desktop && windows`; packages.json keeps them in pagedock with the `internal/` working directory.

`window_resources_test.go` measures real kernel handle recovery after forty native UI window lifetimes without WebView2, following ten warm-up windows and GC. It is an explicit `desktop && pagedock_soak && windows` environment diagnostic: the current VM also reproduces hardware swap-chain handle growth without Go/MyGo, so an ordinary desktop correctness run must not be mistaken for this native-resource acceptance check. Run it through the soak EXE with `windows.ps1 -TestPattern '^TestDesktopWindowsNativeWindowHandles$'`, serially with other desktop cases. Keep failing hardware evidence visible alongside software controls.

Do not expose application locks or private state. Shared desktop sessions and local-site fixtures stay in the parent directory. Skip media cases without explicit test inputs; never automatically open physical cameras. Use `tests/playground/windows/media.mjs`. Update the parent map, product rules, and platform validation scope.
