# Desktop integration and module tests

This level holds cross-module tests for the pagedock package. `tests/packages.json` maps platform groups back to their original package; module tests mirror product directories by default. There is no app/ layer.

| Path | Coverage |
|---|---|
| `desktop_test.go` / `testmain_test.go` | Mutually exclusive desktop/ordinary entries with isolated config and locale |
| `browser_test.go` / `playground_test.go` | Login redirects, permission changes, and the real local test site; retain the desktop tag |
| `notification_test.go` / `pwa_test.go` | Content sanitization, notification IDs, and the real WebView PWA bridge |
| `notifications_test.go` | Native record limits/expiry, replacement/failure/cancellation, and weak-reference proof of page callback release |
| `soak_test.go` / `soak_metrics_test.go` | Opt-in macOS endurance workload, real notification registry/bridge, closed-object references, GC checkpoints, and profiles; run through tests/soak/run.py |
| `language_test.go` / `reset_test.go` | Save failures, locale changes, and window state after cleanup |
| `image_test.go` | Image import, visible sizing, rounded corners, alpha/aspect preservation, repeat-import stability, storage, and native preview |
| `macos/notification_test.go` | Explicitly enabled macOS system banner validation |
| `windows/window_test.go` / `windows/media_test.go` | Shortcuts/focus, explicit device inputs, and media permissions |
| `other/appearance_test.go` | Retain/restore icon config outside macOS/Windows |
| `config/` | Persistence, names, permissions, reset order, and safe profile cleanup |
| `appearance/` | Shared file/resources and macOS/Windows appearance transactions |
| `site/` / `locale/` | URL/origin policy and bilingual catalogs |
| `view/` | Native controls, menus, layout, loading/backgrounds, and actual button pixels |

Platform groups preserve package ownership. App integration still executes in `internal/`, with the test site at `../playground`. macOS appearance evidence still resolves through `../../../.scratch`. Explicit build tags determine platform/desktop eligibility; short filenames must retain those constraints.

Access only temporary files and loopback services; never clear real user data. Real notification/media cases require explicit opt-in. Run desktop packages serially to isolate focus. Update headers, tests/AGENTS.md, and owning module maps.
