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
| `soak/run.py` | Explicit macOS endurance run with a signed temporary app, native/WebKit RSS and footprint, and notification transport isolation |
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
python3 tests/soak/run.py --duration 1800 --recovery 300 --output .scratch/resource-soak/run-name # macOS desktop, new output directory
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
- Endurance tests require desktop,pagedock_soak,darwin and the runner's isolated dependency overlay. They retain real notification registry/bridge/window behavior but replace macOS banner delivery. Use --race only for short correctness checks; collect normal-build RSS/footprint separately. See soak/AGENTS.md for measurement and process cleanup boundaries.
- Separate cross-compilation from actual platform behavior. See Validation records below; raw logs stay in .scratch/. Update root/owning AGENTS.md files and source headers together.

## GitHub Actions

`.github/workflows/test.yml` executes make check, make test, and make test-playground on macOS, Windows, and Ubuntu 24.04. It downloads dependencies before preparing the MyGo patch and test overlay, and runs actionlint on Linux. Release builds reuse this workflow as a required gate. Hosted jobs do not execute desktop, real notification, or physical-media suites.

`.github/workflows/release.yml` packages macOS DMGs, Windows NSIS installers, and Linux Debian/tar packages for amd64 and arm64. `make build BUILD_ARGS='-platform <targets> -o build'` enables DMGs while retaining dependency patches. Manual runs retain packages as Actions artifacts; only matching version-tag pushes publish. Validate version mismatch rejection, missing-package failure, and draft-only retry behavior by executing the workflow shell steps with isolated fixtures. Remote publishing must be verified separately from local syntax/build checks.

## Validation records

### Extended notification/navigation/window validation

2026-10-19: Added an explicit macOS endurance runner and actual process-memory collection. Evidence is in .scratch/resource-soak-20261019/. A unique ad-hoc test app uses temporary config, two loopback origins, real WKWebViews and the actual MyGo notification registry/bridge. Only system notification delivery is replaced, so this does not measure Notification Center or banners.

- A 60-second load plus 30-second recovery run passed with the race detector: 20 windows, 80 navigations, 8,160 notification sends, and 120 verified clicks. Notification records returned to zero, goroutines returned to seven, and all attributable WebKit processes exited with the test app.
- With the default development build options, the full three-trusted-page workload did not pass: four runs stopped at the first navigation of window 33, after 32 completed windows and 128 navigations. Reproduction held both at three-second pacing and at half-second pacing, and with notifications entirely disabled. Native navigation policy had allowed the load, but the actual document remained about:blank without commit/DOM/finish events. This is an unresolved loading failure in that build configuration; it must not be presented as a successful full-workload endurance result or attributed to notification retention.
- Controls with one trusted page followed by one foreign-origin page passed: 88 windows/176 navigations with notifications disabled, and 150 windows/300 navigations/20,400 notification sends with notifications enabled. These controls distinguish the navigation combination from notification volume and raw window count.
- The one-trusted-page development-build endurance run completed 30 minutes of load and five minutes of recovery: 599 windows, 1,198 navigations, 81,464 notification sends, and 1,198 verified clicks. The warmed-up Go heap remained near 8 MiB and ended at 7.29 MiB after clearing notifications and idling. Goroutines returned to seven; notification registries, callbacks, followers, and registered windows returned to zero. One closed Window/Page/Browser reference remained throughout without accumulation. All attributable WebKit processes exited with the test app.
- Native heap snapshots from that completed run found 432, then 501, then 599 WKBackForwardListItem objects as window cycles increased. At idle start and after more than four minutes of recovery, both WKBackForwardListItem and _NSWorkspacePowerNotifier remained at 599 objects. Stable overall RSS and Go-heap assertions therefore do not establish complete native-resource reclamation. The exact retaining path and behavior on other macOS releases remain unverified.
- A second complete 30-minute load plus five-minute recovery run used MyGo's production linker setting and mygo_noinspector tag, with runtime development=false. The full three-trusted-page workload completed 598 windows, 2,392 navigations, 243,984 notification sends, and 3,588 verified clicks without the development-build loading failure. The warmed-up post-GC heap ranged from 8.00 to 8.12 MiB and ended at 7.32 MiB. Goroutines returned to seven; notifications, callbacks, followers, and registered windows returned to zero; one closed Window/Page/Browser remained. All attributable WebKit processes exited.
- Production native snapshots grew from 303 history items/101 power observers at five minutes to 612/204 at ten minutes and 1,230/410 at twenty minutes. At idle start and 4 minutes 15 seconds into recovery, the counts remained 1,794/598 (334,880 bytes for these objects alone, excluding allocations they reference). The application's warmed-up RSS increased slowly, from 146.28 to 150.17 MiB; the fitted slope was 0.12 MiB/min. Physical-footprint median was 63.16 MiB under load and 62.60 MiB during recovery. Idle CPU median was 0%. These results establish small persistent native-object accumulation, not complete memory recovery; allocation stacks and a safe fix remain outstanding.
- Native reference-tree probes attributed history objects to WebKit-owned memory and observers to system block captures, but do not identify the allocating code. A separate leaks scan reported one 32-byte xpc_date_t object; it did not diagnose the retained-object growth. The measurements used macOS 27.2 (26B5112a), ARM64, Go 1.27.1, and MyGo 0.5.1. No Windows/Linux execution, real banner delivery, or GPU allocation measurement was performed in this endurance task.
- Disposable process fixtures passed exact-bundle cleanup, preservation of another same-named executable in a different path, repeated cleanup, and SIGTERM-to-SIGKILL escalation. make test and make check passed. Ordinary tests exclude the endurance tags.
- The final production-options 60-second load plus 30-second recovery run passed with the race detector: 20 windows, 80 navigations, 8,160 sends, and 120 clicks, with no attributable WebKit process after exit. Offline analysis checks passed clock alignment and slope units. /ponytail-review found no blocking issue in the new test tooling, but retained the native-object accumulation and development navigation failure as unresolved runtime findings. The task changed no product implementation and made no commit; all 54 product/build baseline hashes and HEAD remained unchanged.

### Notification lifetime and idle resource use

2026-10-19: Bounded website/settings-test notifications, skipped unchanged Windows appearance processing after successful synchronization, and replaced fixed browser tickers with visible-only adaptive followers. Evidence, failing regressions, measurements, and the /ponytail-review record are in .scratch/resource-fixes-20261019/.

- Notification regressions first retained 1,024 records and a cancelled page closure. The completed lifecycle keeps at most 256 native records, expires them after 24 hours (virtual time), and releases page closures after navigation (weak reference plus GC). Tests also cover repeat clicks, same-ID replacement, stale timers/cancellation, failed delivery, and full cleanup. Native output is an observable test boundary; these tests do not request system notification permission.
- Windows save tests first entered EXE synchronization again for ordinary saves. They now verify one initial synchronization, independent config persistence, name/icon changes, retry after save/native/rollback failures, and 32 concurrent saves. The cache-only benchmark on macOS ARM64 measured 15.99 ns/op, 0 B/op, and 0 allocations/op; it excludes config disk writes and is not a Windows latency measurement.
- A real local WKWebView previously created 18 canvases during the sampling test and read 8 pixels in its 4-second measurement window. It now reuses 1 canvas and reads 2 pixels over 4 seconds. Tests cover hidden/minimized suspension, restoration, callback reclamation after destruction, actual caption colors, dynamic themes, reload, stop, and network failure.
- A new real-window regression exposed missing resumption after macOS app unhide when another window held focus. Adding app activation subscriptions fixed it; subscriptions are removed on close.
- make test and the full make test-desktop suite passed with the race detector on macOS ARM64; strengthened cache and sampling assertions passed in focused follow-ups. make check passed. PWA/Playground integration covered actual local page-to-Go sends, individual clicks, same-tag replacement, permission denial/recovery, and origin isolation. Real-banner and physical-media tests remain explicit opt-ins and were not run.
- Make built darwin/arm64, darwin/amd64, windows/amd64, windows/arm64, linux/amd64, and linux/arm64. Both macOS bundles passed strict codesign verification. The application, view, and Windows appearance desktop test packages compiled for Windows/Linux on both architectures and macOS amd64. Windows EXEs built; NSIS was unavailable, so installers were skipped.
- /ponytail-review traced all notification entry points/reset paths, queue/main-thread locking, timer identity, original-page authorization, startup/save/rollback caching, follower subscriptions, and native navigation fallbacks. No blocking findings remained. No commits, pushes, or releases were made.

The earlier fix-validation run did not include Windows/Linux native execution, real system notification banners, or long-duration native/RSS/GPU profiling. Its evidence establishes the bounded Go lifetimes and reduced local sampling work above, not a blanket claim that a remote website or system WebView cannot leak memory. See the extended validation record for later endurance measurements.

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
