# Centralized tests

All Go behavior tests, Node tests, and platform validation scripts live here. Product implementations remain in internal/, patches/, and playground/, without duplicate tests or product source.

## Directory map

| Path | Responsibility |
|---|---|
| `internal/` and module subdirectories | Cross-module integration and package behavior tests; see its AGENTS.md |
| `packages.json` | Original package ownership for platform groups; unlisted directories mirror product paths |
| `patches/mygo/color_test.go` | Windows caption color selection, runnable on every host |
| `patches/mygo/updater/` | Shared old-EXE cleanup scope/retries/deadlines; windows/ holds real process-lock tests |
| `tools/test-overlay/main_test.go` | Temporary modules executing real Go tests for private access, failure propagation, and invalid inputs |
| `playground/server.test.mjs` | Real loopback HTTP, dual origins, logo resources, and resource isolation |
| `playground/go.mjs` | Repository discovery and shared MyGo patch/test argument preparation for both platform scripts |
| `playground/macos/notifications.mjs` | Real macOS notification banners from a packaged app |
| `playground/windows/media.mjs` | Windows file-device inputs, real authorization changes, and local WebRTC |
| `go.mod` | Isolate package discovery only; tests use root dependencies without separate installation or tidy |

## Go test interface

Go defines packages by directory; existing tests access private package state. tools/test-overlay maps tests/<product path>/*_test.go back to <product path>/*_test.go for native -overlay compilation. Platform groups needing shared private state declare their original package in packages.json. Group prefixes prevent virtual filename collisions. Do not copy product source or export test-only state. Tests retain their original package working directory, cases, package declarations, and build tags.

```sh
make test
make test TEST_ARGS='-run TestTargetURL -count=1'
make test-desktop
make test-desktop TEST_ARGS='-run TestDesktopBrowser -count=1'
make test-playground
make test-notifications
make test-media-windows # Run on a Windows desktop
make check
```

For direct Go or IDE use, prepare both files at the repository root and pass them to every test, test compilation, and vet command:

```sh
patch_mod=$(go run ./tools/mygo-patch)
test_overlay=$(go run ./tools/test-overlay)
go test -modfile="$patch_mod" -overlay="$test_overlay" -race ./...
go test -modfile="$patch_mod" -overlay="$test_overlay" -tags desktop -c -o .scratch/pagedock.test ./internal
go vet -modfile="$patch_mod" -overlay="$test_overlay" ./...
```

**Bare go test ./... and IDE test buttons without overlay do not run these tests.** tests/ is not an independent application module; use Make by default. Regenerate overlays after adding/removing tests. For cross-compilation, prepare on the host before setting GOOS/GOARCH on go test -c; do not run preparation tools as target binaries.

## Maintenance rules

- Shared tests mirror product package paths. Put platform cases in macos/, windows/, or other/ and update packages.json when original private state is needed. Filenames end in _test.go without repeating directory responsibilities. Reject empty sets, missing product directories, duplicate targets, path escapes, and linked test files.
- Test sources live here, but working directories remain product package directories. Resolve app integration ../playground, macOS appearance ../../../.scratch, and test subprocess directories accordingly.
- Retain platform filters and desktop tags. Native tests require explicit darwin/windows constraints; directory names do not select compilation. Portable Windows menu tests still run on all hosts. Packages needing MyGo event loops use mutually exclusive TestMain entries; serialize desktop packages to isolate focus. Use temporary files, local sites, and isolated config only. Never access remote databases.
- Do not write static contracts matching source, directories, or declarations. Tool tests execute real discovery/failure behavior; product tests assert results, state, and error paths.
- Both platform scripts can start from any working directory and prepare identical MyGo patches/overlay. Windows scripts still accept precompiled EXEs.
- Preserve notification test-bundle registration/restoration and Windows media temporary-profile cleanup. Ordinary desktop tests do not automatically invoke physical devices or real-banner suites.
- Separate cross-compilation from actual platform behavior. See Validation records below; raw logs stay in .scratch/. Update root/owning AGENTS.md files and source headers together.

## Validation records

### Platform directory organization

2026-10-17: Unified platform groups and short filenames for config platform entries, Windows menus, MyGo patch definitions, centralized tests, and validation scripts. Rules live in AGENTS files; original logs are in local .scratch/platform-layout/.

| Check | Result for that change |
|---|---|
| macOS ARM64 | Full make test and make test-desktop race suites passed, along with make test-playground and make check |
| Windows 11 ARM64 VM | Five groups covering app/config/views/caption colors/old-EXE cleanup: 52 cases passed on ARM64 and 52 under x64 emulation, including real WebViews, shortcuts, caption pixels, and file locks |
| Builds | make build and strict codesign verification passed; make build-windows produced x64/ARM64 EXEs |
| Cross-compilation | Product packages and all desktop test packages compiled for macOS amd64, Windows amd64/arm64, and Linux amd64/arm64 |
| Migration audit | Test discovery sets remained identical across three platforms; 10 generated patch files were byte-identical; dependencies, package metadata, catalogs, and permission resources were unchanged |
| /ponytail-review | Reviewed callers, platform filters, UI threads, cleanup/rollback, script paths, and test assembly for a single-user desktop app; no blocking issues found |

Two symlink cases per Windows architecture were skipped due to standard-user privileges. That Windows pass did not run Node-dependent Playground/media suites; macOS did not repeat real banner validation. Previously documented system/device behavior is historical acceptance, not a retest by this change.

Linux had compilation only, with no desktop execution. x64 ran under Windows ARM64 emulation, not physical x64 hardware. Windows EXEs were built, but the environment lacked NSIS, so no installers were generated.

### Minimal Playground and default branding

2026-10-17: Simplified both Playground pages and README; installed one shared default logo for packaging, native previews, Windows title bars, README, and Playground. Evidence is in .scratch/minimal-design/.

- make test-playground, make test, make test-desktop, and make check passed on macOS ARM64.
- Browser inspection covered 1280-pixel light layouts and 390-pixel dark layouts, logo decoding, no horizontal overflow, folded details, keyboard expansion/focus, and no automatic media capture. Both pages had no console errors after reload.
- make build and strict codesign verification passed. The packaged ICNS contained valid PNG entries through 1024 pixels.
- make build-windows produced ARM64/x64 EXEs. The existing TestPackagedPEIconRoundTrip passed against both, covering custom icons followed by complete default-resource restoration.
- Windows 11 ARM64 VM ran 11 native settings/menu cases on ARM64 and 11 under x64 emulation, including icon draft/default restoration and saved-branding behavior.
- View desktop test packages cross-compiled for Windows ARM64/amd64, Linux ARM64/amd64, and macOS amd64.

This pass did not repeat actual system notification-banner/icon presentation or physical microphone/camera acceptance. Linux had compilation only; x64 used emulation. Missing NSIS prevented Windows installer generation. Build success and control tests do not establish those untested system-visible results.

### English documentation and comments

2026-10-17: Converted 29 Markdown documents and comments in 125 source/template files to English. User-facing localized strings, test inputs, configuration, permission resources, and logo bytes remained unchanged during this language-only pass. Evidence is in .scratch/english/.

- An ad hoc Go scanner audit checked 113 Go/template files: executable tokens were unchanged after excluding comments, including comments in embedded scripts. Non-Go source comparisons also confirmed comment-only changes.
- Repository Markdown and source-comment scans found no remaining Chinese prose. Chinese UI strings and language-sensitive test fixtures remain intentional.
- make test, make test-playground, make check, and git diff --check passed. Go reused cached test results where executable inputs were unchanged.
- /ponytail-review covered the shared logo callers, Playground controls/events, resource boundaries, custom/default restoration, and translation scope; no blocking issues were found. The local review record is .scratch/english/review.md.

Desktop and platform suites listed above were completed before the comment/documentation conversion and were not repeated afterward. This pass adds no new system-notification, physical-device, or Linux desktop acceptance claims.

### Larger default logo

2026-10-17: Enlarged the existing browser/tray mark within the same 1254-pixel square canvas. The visible mark increased from 602 to 1126 pixels wide (about 1.87 times), reducing side margins from 26% to 5%. The three dots and uniform tray stroke remain. Native views, packaging, README, and Playground continue using the single internal/brand/icon.png source. Evidence and the built-in imagegen prompt are in .scratch/minimal-design/scale/.

- make test-playground passed. Focused desktop race tests passed for native icon drafts/default restoration, unsupported-platform settings preservation, title-menu layout/saved branding, and macOS runtime default-icon restoration.
- make build and strict codesign verification passed. The packaged ICNS contains valid PNG entries through 1024 pixels; its largest image preserves the enlarged mark.
- make build-windows produced ARM64/x64 EXEs. TestPackagedPEIconRoundTrip passed against each, verifying custom-icon replacement and restoration of the packaged default resources.
- Linux ARM64/amd64 product builds passed. git diff --check passed. /ponytail-review found no blocking issues in the asset change and its existing consumers; application logic and stored configuration were unchanged.

Windows/Linux desktop execution and actual Dock/notification-center presentation of the new artwork were not repeated. NSIS remains unavailable, so Windows installers were not generated.

### English packaging metadata

2026-10-17: Changed the default camera/microphone descriptions in mygo.json to match the existing English permission resource. The Simplified Chinese resource remains localized. Updated the root language guidelines to keep packaging defaults in English.

make check, make build, and strict codesign verification passed. Inspection of the generated macOS bundle confirmed English base descriptions, intact en/zh-Hans permission resources, and unchanged language metadata. A configuration-file scan found only the intentional Simplified Chinese language label in the English catalog outside the Chinese catalog itself. /ponytail-review found no blocking issues. Evidence is in .scratch/english/metadata/.

This change affects macOS default permission text only. Windows/Linux builds and real camera/microphone permission dialogs were not repeated for this correction.
