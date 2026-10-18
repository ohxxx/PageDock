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
- Test sources live here, but working directories remain product package directories. Resolve app integration ../playground, macOS appearance ../../../.scratch, and test subprocess directories accordingly. Canonicalize temporary module roots before spawning Go; Windows Shell transaction fixtures use the same canonical paths as the public executable entry, including when TEMP contains 8.3 names.
- Retain platform filters and desktop tags. Native tests require explicit darwin/windows constraints; directory names do not select compilation. Portable Windows menu tests still run on all hosts. Packages needing MyGo event loops use mutually exclusive TestMain entries; serialize desktop packages to isolate focus. Use temporary files, local sites, and isolated config only. Never access remote databases.
- Do not write static contracts matching source, directories, or declarations. Tool tests execute real discovery/failure behavior; product tests assert results, state, and error paths.
- Both platform scripts can start from any working directory and prepare identical MyGo patches/overlay. Windows scripts still accept precompiled EXEs.
- Preserve notification test-bundle registration/restoration and Windows media temporary-profile cleanup. Ordinary desktop tests do not automatically invoke physical devices or real-banner suites.
- Separate cross-compilation from actual platform behavior. See Validation records below; raw logs stay in .scratch/. Update root/owning AGENTS.md files and source headers together.

## GitHub Actions

`.github/workflows/test.yml` executes make check, make test, and make test-playground on macOS, Windows, and Ubuntu 24.04. It downloads dependencies before preparing the MyGo patch and test overlay, and runs actionlint on Linux. Release builds reuse this workflow as a required gate. Hosted jobs do not execute desktop, real notification, or physical-media suites.

`.github/workflows/release.yml` packages macOS DMGs, Windows NSIS installers, and Linux Debian/tar packages for amd64 and arm64. `make build BUILD_ARGS='-platform <targets> -o build'` enables DMGs while retaining dependency patches. Manual runs retain packages as Actions artifacts; only matching version-tag pushes publish. Validate version mismatch rejection, missing-package failure, and draft-only retry behavior by executing the workflow shell steps with isolated fixtures. Remote publishing must be verified separately from local syntax/build checks.

## Validation records

### Imported icon sizing and corners

2026-10-18: Icon import now centers visible artwork in an approximately 80% area of the 512-pixel canvas, replaces existing transparent margins, and rounds fully opaque rectangles with antialiased edges. Existing alpha silhouettes stay intact; saved icons decode unchanged and older icons adopt the sizing when reselected. Evidence and the /ponytail-review record are in .scratch/icon-padding/.

- The new PNG/JPEG sizing tests failed against the old importer with visible bounds (0,0)-(512,512). The completed importer passed padding, corners, antialiasing, translucent cutouts, empty transparency, asymmetric input margins, odd/narrow aspect ratios, exact repeat-import pixels, and storage checks.
- make test with the race detector and make check passed on macOS ARM64. Import/resource tests also passed as darwin/amd64 binaries under Rosetta, exercising the vector rasterizer's Intel path. Its existing x/image dependency requires the newly recorded x/sys v0.48.0 indirect dependency for SIMD detection on amd64.
- Decoded all eight ICNS representations and nine PE icon sizes after real import; margins, transparent corners, artwork color, and relative size passed. Existing dual-architecture PE transaction/default-restoration tests also passed.
- Targeted desktop tests passed for imported images and default restoration. The real NSApplication image read back as 1024x1024 with a 102-pixel top margin and transparent corners. This validates the native image, not visible Dock or notification presentation.
- Make built macOS, Windows, and Linux ARM64/amd64 targets. Both macOS bundles passed strict codesign verification. Windows EXEs built; NSIS was unavailable, so installers were skipped. Windows/Linux native execution and real notification presentation were not performed. Linux icon selection remains unavailable.
- comparison.png shows the actual importer applied to the repository logo, before and after; the temporary preview probe was removed. No installed app, user config, or system privacy setting was modified.
- /ponytail-review traced selection, preview, persistence, ICNS/PE encoding, Dock updates, restored defaults, resource limits, and dependency selection. No blocking findings remained.

### GitHub release automation

2026-10-18: Added native CI and gated tag releases, using Mayoi's release flow while retaining PageDock's Makefile patches, centralized tests, and ad-hoc/unsigned packaging. Evidence and the /ponytail-review record are in .scratch/github-release/.

- make check, make test with the race detector, and make test-playground passed on macOS ARM64. actionlint 1.7.12 and Bash syntax checks passed for both workflows.
- Executed the workflow shell steps with isolated fixtures: valid versions/manual branches passed; tag mismatches and invalid stable versions failed. All eight expected package files were selected, source files were excluded, and each missing package failed collection. A simulated upload failure left a draft; retry uploaded before publishing; published versions were refused.
- MyGo built all six macOS/Windows/Linux ARM64/amd64 targets through the updated Makefile. Both macOS bundles passed strict codesign checks and both DMGs passed hdiutil verification. Both Windows EXEs passed the existing packaged icon/name resource round-trip tests.
- Both Linux tar archives retained executable permissions and matched the built binaries. Debian control metadata contained the correct version, architecture, and WebKit dependency. Actual macOS/Linux collection and SHA256SUMS generation/verification passed for all six distribution files.
- /ponytail-review covered callers, permissions, versions, artifact selection, draft retry behavior, and documentation. No blocking issues were found.

Local NSIS is unavailable, so Windows installers were not generated; actual collection correctly rejected their absence. The workflow installs NSIS on its Windows packaging runner. GitHub-hosted execution, live release publication, Windows/Linux native execution, and device/notification acceptance were not performed in this pass. No commits, pushes, tags, or releases were created.

2026-10-18 hosted follow-up: Test run 39091361861 and manual Release run 39091416810 passed macOS/Linux tests and reproduced six Windows failures. The runner's TEMP used RUNNER~1 while Shell returned runneradmin. Four private Shell transaction tests bypassed the public entry's canonicalization; two overlay tests launched Go from an unnormalized path. Fixtures now resolve paths before invoking those existing helpers. Package collection also replaces spaces with hyphens before upload so SHA256SUMS uses the exact public asset names.

- Local make check/test/test-playground, actionlint, shell syntax, Windows amd64/arm64 test compilation, eight normalized filename/checksum checks, and each missing-package rejection passed.
- [Test run 39091833802](https://github.com/ohxxx/PageDock/actions/runs/39091833802) passed all three hosted systems at 7e3bf6f. Windows still used RUNNER~1 and passed the native Shell and real overlay-discovery suites.
- [Manual Release run 39091839363](https://github.com/ohxxx/PageDock/actions/runs/39091839363) passed validation, all three test jobs, and all six build targets at the same commit. Downloaded all eight packages: both DMGs passed hdiutil verification, both Windows packages were NSIS installers, and Linux archives retained executable permissions and Debian members. This manual run did not publish.
- /ponytail-review traced the unchanged public path normalization, private helper callers, fixture assertions, package names, checksum generation, and draft publication sequence. No blocking issues remained. Evidence is in .scratch/github-release/.

This hosted pass validates headless tests and packaging. It does not establish installer execution, GUI behavior, physical media, or notification delivery. Tag-triggered publication remains a separate workflow gate.

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
