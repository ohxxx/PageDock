# Native Windows UI differences

`menu.go` renders saved branding and a keyboard-accessible title bar menu, falling back to the `internal/brand` logo for an empty image. `about.go` configures the About menu, opens the native MyGo dialog in the background, and reports errors to the application.

This package owns neither the application lock nor settings drafts. Shared callers supply display values and callbacks. Portable MyGo/UI controls support real rendering/input tests on macOS/Linux, so do not add Windows-only compilation constraints. Callers retain existing platform selection and preserve other platforms' menus.

Tests in `tests/internal/view/windows/` map back to view through packages.json to retain Browser/Settings integration assertions. `menu_test.go` runs on every host; `caption_test.go` explicitly requires `desktop && windows`. Update parent/test maps.
