# Configuration and local data

| Path | Responsibility |
|---|---|
| `config.go` | `Config.Validate/DisplayName`, shared name validation, and atomic JSON storage through `Store.Load/Save` |
| `image.go` | Bounded PNG/JPEG import, visible-content sizing with transparent margins/rounded opaque corners, unchanged 512-pixel persistence decoding, and platform icon capability |
| `permissions.go` | Exact origins and permission combinations; reject unauthorized/unknown requests |
| `reset.go` | Persist both cleanup requests, enforce order/retries, and limit config/window-state deletion |
| `browser.go` | Shared profile deletion: bounded directories, link rejection, busy-file retries, and deadlines |
| `windows/name.go` | Windows filename, reserved-name, and trailing-period validation |
| `windows/browser.go` | WebView2 profile discovery, external-override rejection, and a 15-second cleanup context |
| `native_windows.go` / `native_other.go` | Compile-time selection linking validation and shared deletion; other platforms retain MyGo native cleanup |

Platform implementations use short filenames and explicit Windows build tags. Root `native_*.go` files contain only necessary package-level selection/wiring. Native profile entry points receive the existing deletion function; this package owns the safe file algorithm, avoiding reverse dependencies and cycles.

Depend on `site` URL policy, `locale` messages, and MyGo; do not import views or appearance. Persist image contents, never user paths. Preserve `Store.Path`, JSON fields, atomic writes, original files on error, and browsing-before-config reset order. Windows profiles follow the fixed MyGo internal name, not the display name or `PAGEDOCK_DATA_DIR`; reject external overrides and directory links.

Icon imports center the visible alpha bounds within an approximately 80% content area, preserving aspect ratio and replacing any existing transparent margins. Round fully opaque rectangular artwork with antialiased corners; retain existing alpha silhouettes, holes, shadows, and translucent pixels. Empty transparent images remain transparent. Avoid resampling at the same size, so re-imported normalized PNGs keep their pixels. This shared format serves previews and macOS/Windows resources; DecodeIcon never normalizes again or migrates old saved images.

Shared tests live in `tests/internal/config/`; Windows file-lock/profile cases are organized in its `windows/` directory. `tests/packages.json` maps those cases back to config, retaining private access and working directories. Cross-package image tests live in `tests/internal/image_test.go`. Run `make test`, plus native Windows cases on Windows. Update headers, this map, and platform validation scope.
