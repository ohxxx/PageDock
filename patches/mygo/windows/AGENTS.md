# Windows target patches

`changes.go` contains exact replacements and supplemental source definitions for caption buttons, menu shortcuts, default confirmation buttons, permission cache migration, and old EXE cleanup. Build preparation reads these target definitions on every host, so this file has no host Windows build tag.

`permissions.go.in` is native source injected into MyGo's Windows backend. Preserve its `windows && (amd64 || arm64)` constraint and permission migration, cancellation, and failure behavior. The `.in` file is not compiled as a separate PageDock package.

Portable color and file cleanup algorithms stay in the parent `color.go` and `updater/cleanup.go`. Definitions use repository-relative source paths and upstream backend target filenames. Preparation reapplies changes to pinned upstream source without touching the module cache; version/context drift must fail. Check generated copies and run behavior tests/platform builds after changes.
