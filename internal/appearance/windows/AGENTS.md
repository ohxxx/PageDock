# Windows appearance

Native API files explicitly declare `//go:build windows`; filenames do not repeat the platform suffix. The appearance cache is portable and tested on every host.

| File | Responsibility |
|---|---|
| `apply.go` | `Apply`, `Relaunch`; combined name/icon/shortcut/registry/config transactions and running-EXE rename retries |
| `cache.go` | Serialize appearance synchronization and bypass EXE processing for ordinary saves after a successful name/icon transaction |
| `links.go` | COM/Shell link snapshots, updates, and rollback, preserving targets, arguments, and pinning |
| `name.go` | System display names, directory metadata, and recovery |
| `registry.go` | Per-user uninstall names matched only to the current installation |
| `taskbar.go` | Window taskbar properties, stable grouping, and destruction cleanup |
| `sync.go` | `PrepareTaskbar`; migrate existing shortcut identities before the first window |
| `rename.go` | Pinned-item renames, conflict rejection, and Shell notifications |
| `icon.go` | Restore the system default application icon when no icon is embedded |
| `window.go` | `NewWindow`; set taskbar identity before display |

The parent `appearance` package provides fixed identity, PE processing, and file replacement transactions. This package owns system calls, retaining call order and rollback. Apply and PrepareTaskbar resolve the executable through windowsIconExecutable before calling private helpers; tests must preserve this canonical-path precondition, including Windows 8.3 aliases. `InternalName` never follows the user's display name; EXE, data-directory, browser-profile, and single-instance identities remain fixed.

The cache starts empty on every process launch. Only a successful synchronization plus save records the name/icon pair; changes or failed synchronization require the original native transaction. Ordinary saves still persist config and propagate save failures without reading/parsing the EXE. Failed appearance transactions invalidate the cache so rollback failures cannot be mistaken for synchronized state.

Tests in `tests/internal/appearance/windows/` modify temporary EXEs, links, and registry entries only. Portable PE/file tests live in the parent test directory. Running-EXE and system-name cases require Windows; compiling fixtures requires Go. Compilation, resource checks, and property checks cannot replace visible Explorer, notification, or pinned-item validation. Update headers, directory maps, and validation records.
