# Native names, icons, and system identity

Shared resource processing and file transactions stay here; native implementations live in `macos/` and `windows/`. Application orchestration binds `Apply`, `Relaunch`, `PrepareTaskbar`, and `NewWindow` at compile time through `internal/native_*.go`, preserving call order and error handling. Platform packages depend on this package; it never imports them back.

| Path | Responsibility |
|---|---|
| `identity.go` | Fixed `InternalName`, independent of the user-visible name |
| `icns.go` | `EncodeICNS`: PNG to multiresolution ICNS |
| `file.go` | `ReadExecutable`, `ReplaceExecutable`: validation, staging, replacement, rollback, and recovery-copy retention |
| `pe.go` | PE icon resources, default-icon snapshots, and non-icon resource preservation |
| `version.go` | `RewritePE`, `PEIdentity`, `DisplayNameID`: combined name/icon updates, fixed identity, and multilingual snapshots; `MaxNameStateBytes` bounds resource snapshots and registry reads |
| `window_other.go` | Shared macOS/Linux native window creation without taskbar migration |
| `other.go` | Retain saved images on Linux and other unsupported platforms while allowing ordinary saves, default restoration, and restart |
| `macos/` | Bundles, signing, system names, Dock/notification icon caches, and relaunch |
| `windows/` | EXE updates, Shell links, system names, registry, taskbar, and relaunch |

Resource algorithms remain in the shared package because they can be tested across hosts. Export only operations needed by real platform callers; keep COM, cache, and bundle transaction details private. Depend on `config` values/validation and `locale` messages, never application state or UI packages.

Go does not select platforms by directory name. Short filenames in platform directories require `//go:build darwin` or `//go:build windows`. Tests mirror this layout under `tests/internal/appearance/` with matching constraints; macOS desktop cases also require `desktop`. Update headers, local/root maps, and validation records together.
