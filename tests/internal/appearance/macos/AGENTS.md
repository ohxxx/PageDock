# macOS appearance tests

`bundle_test.go` verifies temporary ad-hoc bundle transactions and rollback. `name_test.go` uses separate processes for system names, legacy migration, and symlink rejection. `cache_test.go` and `icon_test.go` require `desktop`, covering notification icon caches, runtime Dock default restoration, and real NSApplication image padding/corners after importing an opaque icon. These pixel checks do not prove visible Dock or notification presentation. Every file retains a `darwin` build tag.

`desktop_test.go` provides an isolated MyGo event loop and excludes ordinary `testmain_test.go`; `image_test.go` only creates input images. Tests execute in `internal/appearance/macos/`, resolving evidence through `../../../.scratch`. Modify only this run's temporary bundles, then unregister and clean up. System-image subprocesses retain explicit environment switches.

Run `make test` and `make test-desktop`. Update the product macos map, parent test map, and validation records.
