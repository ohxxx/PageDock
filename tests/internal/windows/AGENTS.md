# Windows application integration

`window_test.go` checks native shortcuts, focus, menus, and clipboard behavior. `media_test.go` checks explicitly supplied file devices, permission changes, and local WebRTC. Both retain `desktop && windows`; packages.json keeps them in pagedock with the `internal/` working directory.

Do not expose application locks or private state. Shared desktop sessions and local-site fixtures stay in the parent directory. Skip media cases without explicit test inputs; never automatically open physical cameras. Use `tests/playground/windows/media.mjs`. Update the parent map, product rules, and platform validation scope.
