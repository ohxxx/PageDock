# Opt-in resource endurance tests

`run.py` builds and launches a temporary ad-hoc signed macOS app with a unique bundle identity. `tests/internal/soak_test.go` drives the workload; `soak_metrics_test.go` records application lifetimes. Ordinary Make tests do not run this workload.

`--windows-build arm64` or `--windows-build amd64` produces an isolated Windows test EXE without running it. Copy that EXE and `windows.ps1` to a Windows desktop. The PowerShell runner creates a fresh LOCALAPPDATA under its output folder, clears process-local WebView2 overrides, and measures only its own executable and WebView2 processes using that unique profile. It restores inherited environment values afterward. Evidence/profile directories remain for inspection; process cleanup does not access other applications.

`macos_history.m` is a standalone ARC/AppKit/WebKit diagnostic, without Go or MyGo. It loads three data URLs per WebView, drops each view, and checks weak history references after five seconds. It also measures retain-count growth across ten reads of the same item. A nonzero result identifies native ownership failure; it is not a PageDock regression test or an instruction to over-release system objects. Compare with WebKit bug 325374 / commit fa42178 when the system WebKit changes.

`macos_navigation.m` independently repeats three same-origin loads and one foreign-origin load in real WKWebViews. Supply two loopback HTTP origins and a cycle count. It uses a nonpersistent data store and reports the exact URL on timeout. The optional `development`, `separate`, and `no-animations` arguments distinguish inspector settings, shared/per-window process pools, and AppKit automatic window ordering. `macos_navigation.py` owns two observable HTTP servers, records every request, and collects process stacks/descriptors during the probe's ten-second timeout drain. Neither program contains application code or notification work.

`windows_http.go` is a standalone diagnostic HTTP server with two ephemeral loopback origins and its own native handle measurements. Build the named file explicitly for Windows. `windows-external.ps1` starts it beside a separate soak host, passes the two validated origins through `windows.ps1 -Origins`, and stops only its created server. Compare the two processes before assigning handle growth to WebView2 or the application. This changes measurement ownership only; the workload still visits both real HTTP origins and executes notification bridge assertions. The standalone fixture mirrors the embedded soak fixture's payload and DOM load.

`windows-device.ps1` creates and releases standalone D3D11 devices and immediate contexts without MyGo, WebView2, or a native window. Compare hardware with `-Software` (WARP) in separate processes. It records returned COM reference counts and handles after ten warm-up devices plus an idle drain. Completion is not a handle-recovery assertion. MyGo's existing process-local `MYGO_GPU=0` is a separate renderer diagnostic, recorded in windows.ps1's manifest; never treat a software-only run as the normal product baseline.

`windows-swapchain.ps1` adds real native windows and DXGI flip swap chains to that independent comparison. It clears/presents a frame, hides/shows and destroys each window, releases every owned COM reference, and drains ten seconds after ten warm-up cycles plus the requested measured cycles. Run hardware and WARP in separate STA PowerShell processes. This isolates presented graphics from device creation alone; it does not run application code or change graphics settings.

`windows-handles.ps1` counts kernel handle types for an explicitly selected test PID. `-ProcessTargets` adds the target PID and exit state of process handles. `-ObjectNames` additionally reads Event names and available process image paths; other object names are excluded because some name queries can block. Duplicated handles are closed immediately; it does not close remote handles or read their contents. `windows-runtime.ps1` independently runs the official WebView2 .NET SDK with one shared environment, local HTML, repeated controller creation/Close, browser-process exit waits, and an idle interval. `-Composition` selects a composition controller; `-VisualTree` attaches an owned DirectComposition device/target/visual, and `-NavigateOrigins` serves three trusted-page visits plus a foreign-origin visit from two disposable loopback servers. `-Continuous` starts the next controller without waiting for browser-process exit. These modes distinguish controller type, rendering, navigation, and overlapping browser lifetimes. Run it in an STA PowerShell process with matching SDK/loader architecture. It creates a new profile, clears only its own WebView2 overrides, and restores them afterward. Its exit code describes API/workload completion; handle-growth conclusions require reading the measurements. No SDK, runtime, or system setting is installed or changed.

`runner_test.py` checks process-cleanup behavior with disposable executables. `analyze.py` (macOS) and `analyze_windows.py` read completed evidence and generate interval statistics plus time-series PNGs; both need Matplotlib. The Windows analysis keeps private bytes separate from shared working-set mappings and reports handle growth even when Go behavior assertions pass. `analyze_test.py` checks clock alignment, trend units, and private-memory interpretation with synthetic measurements.

`windows-handle-trace.ps1` enables, reads, or disables process-local native handle tracing only after matching an explicit PID to its exact isolated executable path. It returns event history and available stacks for current process handles, and closes only its own duplicates/query handles. Numeric handles can be reused: a historical event must not be attributed to the current target without matching the lifetime. Only events created by the selected process are mapped against its modules. Empty events or stacks are reported explicitly and do not identify an owner; verify tracing with a known local OpenProcess call before interpreting an empty capture. Layouts follow [phnt ntpsapi.h](https://github.com/winsiderss/phnt/blob/master/ntpsapi.h). Tracing changes allocation/performance and is diagnostic evidence only; never use a traced run as a production memory baseline or apply it to the installed application. Always disable it in a finally block while the target is still alive.

The SDK control additionally accepts `-CyclePace`, `-Dwell`, `-RecreateWindow`, `-ReadPermissions`, `-EvaluateScript`, `-RequestFilter`, and `-TargetVersion`. These independently compare pacing, time spent with a live controller before closing, native host recreation, permission enumeration, DevTools Runtime.evaluate, an unused MyGo custom-scheme request filter, and runtime compatibility policy. The filter leaves responses untouched. Record each option in result.json and run controls serially with separate profiles; API completion alone is not handle-reclamation proof.

`-DetachVisualRoot` additionally disconnects a composition controller's RootVisualTarget and commits that public-API change before Close. Compare it with the same dwell/workload; it is a teardown experiment, not a production workaround or a promise of handle recovery.

`windows.ps1 -ClosePageFirst` closes the embedded page before its host on one UI dispatch, then performs ordinary application cleanup. `-BareWebView` runs the same HTTP payload and four navigations in real MyGo windows without PageDock views, taskbar property setup, PWA work, browser followers, or script evaluation. It waits for native navigation events and still checks closed-window/page weak references. `-BareEval`, `-BareTaskbar`, `-BareBackground`, and `-BareRender` independently restore the application readiness polling, taskbar properties, production background follower, or native browser container/title menu within that bare control. `-BareLoading` restores the loading follower, `-BareBridge` restores the PWA preload plus a real cross-origin denied bridge call, and `-BareDwell` holds a page before closing to distinguish shutdown timing from work performed. These are differential diagnostics, not replacements for the full notification workload; their actual send count is zero.

```sh
python3 tests/soak/run.py --duration 60 --recovery 30 --race --output .scratch/resource-soak/smoke
python3 tests/soak/run.py --duration 1800 --recovery 300 --pace 3 --output .scratch/resource-soak/long
python3 tests/soak/runner_test.py # Exact-process cleanup and stuck-process escalation
python3 tests/soak/analyze_test.py # Shared time base and slope calculations
python3 tests/soak/analyze.py .scratch/resource-soak/long
python3 tests/soak/run.py --windows-build arm64 --output .scratch/resource-soak/windows-build
# On Windows, after copying the compiled EXE and runner:
powershell -NoProfile -ExecutionPolicy Bypass -File windows.ps1 -Executable .\PageDockSoak.exe -Output .\.scratch\long -Duration 1800 -Recovery 300
python3 tests/soak/analyze_windows.py .scratch/resource-soak/windows-long
# The SDK directory must contain the official Core DLL and matching WebView2Loader.dll:
powershell -NoProfile -STA -ExecutionPolicy Bypass -File windows-runtime.ps1 -SdkDirectory .\sdk -Output .\native-runtime -Cycles 60 -Recovery 90
powershell -NoProfile -STA -ExecutionPolicy Bypass -File windows-runtime.ps1 -SdkDirectory .\sdk -Output .\native-composition -Cycles 100 -Recovery 90 -Composition -VisualTree -NavigateOrigins -Continuous
# On macOS:
xcrun clang -fobjc-arc -framework AppKit -framework WebKit tests/soak/macos_history.m -o .scratch/native-history-probe
.scratch/native-history-probe 30
xcrun clang -fobjc-arc -framework AppKit -framework WebKit tests/soak/macos_navigation.m -o .scratch/native-navigation-probe
# With two disposable local HTTP servers already running:
.scratch/native-navigation-probe http://127.0.0.1:8000 http://127.0.0.1:8001 120
.scratch/native-navigation-probe http://127.0.0.1:8000 http://127.0.0.1:8001 120 production separate
```

The PowerShell examples set execution policy for that invocation only; they do not change the machine policy.

Every output directory must be new and inside the repository's `.scratch/`. Flags are seconds. Use a local macOS desktop and Python 3.9 or newer. Run one desktop workload at a time. No remote services, physical devices, installed app modifications, or real system notifications are required.

The Windows runner accepts a new absolute or relative output directory on the target machine; copy results into the repository's `.scratch/` afterward. Its `process-memory.jsonl` includes private bytes, working set, process handles, threads, CPU time, GDI/USER object counts, and attributable WebView2 children. Require zero children after app exit. `-TestPattern` also runs focused lifecycle regressions using the same isolation. Do not present working set as private memory or sum shared GPU mappings as unique RAM.

`--malloc-stack-logging` is a macOS diagnostic only. It passes MallocStackLogging=1 and Malloc=1 to the isolated app, allowing WebKit owner inspection without changing global environment. This changes allocation behavior; debugger-injected runs and stack-logging runs must not serve as production memory baselines.

The runner defaults to the same `mygo.production=1` linker setting and `mygo_noinspector` build tag as MyGo's release builder. `--development` explicitly retains developer tools for comparison. Record the build mode: a non-race test build alone is not equivalent to a production build. Runtime samples also record `mygo.IsDev()`.

`--batch` defaults to 128 notifications per trusted page; zero disables notification work to isolate navigation. Nonzero batches must be at least four so individual close and two click assertions remain meaningful. `--visits` defaults to three trusted pages per window; one isolates a trusted-to-foreign transition. Counts in the evidence, rather than the default workload below, describe a particular run.

## Workload and isolation

- Each cycle creates a real native window and embedded WebView, starts both production browser followers, visits three trusted pages and one foreign-origin page, hides/shows the window, and destroys it through application cleanup.
- Each trusted page sends 128 notifications in batches of 16, mixing unique IDs and same-tag replacement. It closes one notification and verifies two real bridge click callbacks. It also sends eight notifications through the native settings-test path. Each cycle totals 408 sends, four navigations, and six clicks.
- Each local page owns a 2 MiB JavaScript buffer and 500 DOM rows. Responses disable caching. TestMain isolates configuration and locale; macOS uses the unique bundle identity and Windows uses a temporary LOCALAPPDATA profile.
- MyGo 0.5.1 is pinned. A separate Go overlay changes the selected platform's notification support/presentation transport and adds registry observation/click hooks. Real `Notification.Show`, `Close`, `notifications.byID`, queue eviction, context cancellation, and bridge events still execute. Neither the module cache nor product sources acquire test hooks.
- The test does not cover system Notification Center/Shell notification transport, operating-system permission prompts, notification banners, actual remote websites, settings rendering, or appearance transactions. Linux native lifetimes need separate target execution.

## Evidence and interpretation

- `go-memory.jsonl` records each cycle and a post-GC checkpoint about every 30 seconds. Only `post_gc: true` rows measure weak-reference retention and follower stacks. Every checkpoint requires zero registered windows, page callbacks, and follower goroutines; notification registries must agree and remain at most 256. The queue is not cleared between cycles.
- The weak-reference probe requires zero closed windows/pages/views after queued UI work and native completion handlers drain, with bounded GC retries. The original dependency retained the last window/page through slice backing arrays. Test-owned weak-reference slices and output/profile work create a small amount of measurement overhead.
- After the active period, notifications are explicitly cleared and the app idles for the requested recovery period. GC checkpoints continue. The final `forced_scavenge` row follows `debug.FreeOSMemory`; report it separately from idle recovery. Go GC results alone do not establish native-memory recovery.
- `rss.jsonl` independently records RSS, CPU, and physical footprint every five seconds. WebKit children are selected by the macOS responsibility PID, not by all WebKit processes on the machine. Failed footprint reads are null, not zero. Compare warmed-up intervals and peaks, not only cold start with the final sample.
- Offline analysis aligns process samples with the Go test clock using their UTC timestamps. Samples after the final idle checkpoint are excluded from the memory plot and interval statistics so forced scavenging cannot be counted as idle recovery.
- Heap/goroutine profiles are saved at startup, periodic checkpoints, final load, and recovery. `manifest.json` records the overlay boundary and transport hashes; `result.json` requires behavior assertions and zero attributable WebKit processes after app exit.
- The runner enforces an independent deadline and terminates only processes whose executable path exactly matches its isolated bundle. LaunchServices registration is removed afterward. Evidence and the test bundle remain in `.scratch/` for inspection.
- Navigation timeouts report the target, native URL/loading state, actual document URL/readiness, last evaluation error, and navigation events. Do not retry or discard a failed cycle to make the endurance run pass. Record diagnostic workloads separately from the full three-visit workload.

Update this file, tests/AGENTS.md, tests/internal/AGENTS.md, and the relevant source headers when workload or measurement boundaries change. Record actual duration, workload counts, warmed-up memory trends, idle recovery, and remaining limitations in tests/AGENTS.md.
