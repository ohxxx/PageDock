# Website policy and page adaptation

- `url.go`: `TargetURL` validates the persisted home page; `NavigationURL` allows temporary login callbacks; `ParseURL/Origin/SameOrigin` normalize and compare exact origins. Default to HTTPS and never persist credentials or tokens.
- `preload.go`: Generate the saved-origin-only, top-level Notification/PWA adapter. Call only `PWA.Receive/Close`; preserve the click protocol and page-local data.

Depend only on the standard library and `locale`. `config.AllowsPermissions` enforces browser permissions; `internal/` owns Go notification reception and application windows. Add no remote service or configuration bridge.

URL tests live in `tests/internal/site/`; real WebView/notification-bridge integration lives in `tests/internal/`. Run `make test` and `make test-desktop`. Update headers, this map, and related behavior tests.
