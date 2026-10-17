# macOS appearance

Every Go file explicitly declares `//go:build darwin`; filenames do not repeat the platform suffix.

| File | Responsibility |
|---|---|
| `bundle.go` | `Apply`, `Relaunch`; complete bundle staging, ad-hoc signing, atomic exchange, registration, Dock synchronization, and relaunch preserving the config directory |
| `name.go` | Localized-name snapshots, restoration, and legacy-config migration |
| `cache.go` | IconServices refresh and current-user Notification Center reload |

Transactions retain this order: resource update, signing, exchange, registration, cache refresh, config save. Failures restore the original bundle and caches; failed rollback retains recoverable copies. Reuse `appearance.EncodeICNS` instead of duplicating encoding. The parent package does not import this package; application orchestration selects it by build target.

Tests live in `tests/internal/appearance/macos/`. Run `make test` and `make test-desktop`; modify temporary bundles only and unregister them afterward. Correct Dock icons or bundle resources do not prove visible notification icons. Update headers, local/parent maps, and Validation records in tests/AGENTS.md.
