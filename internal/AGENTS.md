# Desktop application orchestration

This directory is the `pagedock` package, called directly by root `main.go`; there is no extra `app/` layer. Keep only cross-module application state and operation order here.

| Path | Responsibility |
|---|---|
| `run.go` | Fixed MyGo identity, single instance, user directories, event registration, and pending cleanup before the first window |
| `session.go` | `application` state/lock, startup recovery, activation, and error presentation |
| `window.go` | Settings/website windows, navigation, exact-origin authorization, load failures, and refresh |
| `settings.go` | Action dispatch, draft saves, image selection, test notifications, cleanup confirmation, and restart |
| `menu.go` | System menus/shared actions targeting the embedded WebView; view/windows owns Windows About presentation |
| `language.go` | Main-thread locale application and menu/window title refresh |
| `notification.go` | `PWA.Receive/Close`, origin validation, native notifications, and click events |
| `native_darwin.go` / `native_windows.go` / `native_other.go` | Compile-time binding of existing platform operations; platform packages depend on shared appearance resources |
| `config/` | Shared config, permissions, and safe file cleanup; windows/ locates profiles and validates names |
| `appearance/` | Shared resources/file transactions; macos/ and windows/ encapsulate native appearance/taskbar implementations |
| `brand/` | One default-logo PNG and native bitmap shared by packaging, settings, title bars, README, and Playground |
| `site/` | URLs, origins, and website notification adapter scripts |
| `view/` | Shared native settings, website container, and state; windows/ holds platform menu presentation |
| `locale/` | Native translation catalogs and registration |

Do not expose `application`, its lock, or window ownership across packages. Background work holds the lock for serialized transactions; main-thread events only dispatch goroutines and never wait for it. Read/write `view` fields on the UI main thread only. Pass drafts by value and return results through `Window.Update`.

Save order: validation, appearance/config transaction, language application, then restart or page recreation. Permission-only changes retain valid session URLs; full reset clears browsing data first. Directory changes must preserve fixed identity, JSON fields, and website service names.

Tests live under `tests/internal/`; tests/packages.json preserves private package access and working directories for macos/windows/other groups. Default commands are `make test`, `make test-desktop`, and `make check`. Record compilation separately from desktop validation. Update file headers, this map, and affected child maps; global structure changes also update root AGENTS.md.
