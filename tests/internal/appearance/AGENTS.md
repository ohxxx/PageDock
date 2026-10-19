# Appearance transaction tests

Overlay maps tests to the corresponding product package. Shared resource tests stay here; platform behavior lives in subdirectories.

| Path | Coverage |
|---|---|
| `file_test.go` | Real file replacement, config-save rollback, and recovery-copy retention |
| `pe_test.go` / `version_test.go` | Dual-architecture PE icons/names, unrelated resource preservation, and default restoration |
| `image_test.go` / `testmain_test.go` | Bounded image fixtures, imported padding/corner pixels across every ICNS/PE size, and isolated locale; resource tests need no desktop event loop |
| `macos/` | Temporary signed bundles, system names, IconServices, and runtime Dock icons |
| `windows/` | Portable save-cache behavior plus native running EXEs, Shell links, registry, taskbar properties, and rollback |

Native platform tests retain explicit `darwin`/`windows` build tags; the Windows cache tests run on every host. macOS desktop cases also require `desktop` and run packages serially. Compiling PE fixtures requires Go; Windows environments without Go must run a documented subset that does not need it. Connect platform implementations through resource APIs needed by real callers; no product APIs solely for tests or static contract tests.

Run `make test` and `make test-desktop`. Record target-platform execution separately from cross-compilation. Update product packages, owning directories, and test maps.
