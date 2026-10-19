# Opt-in resource endurance tests

`run.py` builds and launches a temporary ad-hoc signed macOS app with a unique bundle identity. `tests/internal/soak_test.go` drives the workload; `soak_metrics_test.go` records application lifetimes. Ordinary Make tests do not run this workload.

`runner_test.py` checks process-cleanup behavior with disposable executables. `analyze.py` reads completed evidence and generates interval statistics plus a time-series PNG; it needs Matplotlib in the selected Python environment. `analyze_test.py` checks clock alignment and trend calculations with synthetic measurements.

```sh
python3 tests/soak/run.py --duration 60 --recovery 30 --race --output .scratch/resource-soak/smoke
python3 tests/soak/run.py --duration 1800 --recovery 300 --pace 3 --output .scratch/resource-soak/long
python3 tests/soak/runner_test.py # Exact-process cleanup and stuck-process escalation
python3 tests/soak/analyze_test.py # Shared time base and slope calculations
python3 tests/soak/analyze.py .scratch/resource-soak/long
```

Every output directory must be new and inside the repository's `.scratch/`. Flags are seconds. Use a local macOS desktop and Python 3.9 or newer. Run one desktop workload at a time. No remote services, physical devices, installed app modifications, or real system notifications are required.

The runner defaults to the same `mygo.production=1` linker setting and `mygo_noinspector` build tag as MyGo's release builder. `--development` explicitly retains developer tools for comparison. Record the build mode: a non-race test build alone is not equivalent to a production build. Runtime samples also record `mygo.IsDev()`.

`--batch` defaults to 128 notifications per trusted page; zero disables notification work to isolate navigation. Nonzero batches must be at least four so individual close and two click assertions remain meaningful. `--visits` defaults to three trusted pages per window; one isolates a trusted-to-foreign transition. Counts in the evidence, rather than the default workload below, describe a particular run.

## Workload and isolation

- Each cycle creates a real native window and embedded WebView, starts both production browser followers, visits three trusted pages and one foreign-origin page, hides/shows the window, and destroys it through application cleanup.
- Each trusted page sends 128 notifications in batches of 16, mixing unique IDs and same-tag replacement. It closes one notification and verifies two real bridge click callbacks. It also sends eight notifications through the native settings-test path. Each cycle totals 408 sends, four navigations, and six clicks.
- Each local page owns a 2 MiB JavaScript buffer and 500 DOM rows. Responses disable caching. TestMain isolates configuration and locale; no browser-profile cleanup is performed.
- MyGo 0.5.1 is pinned. A separate Go overlay changes only the macOS notification support/presentation transport and adds registry observation/click hooks. Real `Notification.Show`, `Close`, `notifications.byID`, queue eviction, context cancellation, and bridge events still execute. Neither the module cache nor product sources acquire test hooks.
- The test does not cover system Notification Center, operating-system permission prompts, notification banners, actual remote websites, settings rendering, or appearance transactions. Windows/Linux native lifetimes need separate target execution.

## Evidence and interpretation

- `go-memory.jsonl` records each cycle and a post-GC checkpoint about every 30 seconds. Only `post_gc: true` rows measure weak-reference retention and follower stacks. Every checkpoint requires zero registered windows, page callbacks, and follower goroutines; notification registries must agree and remain at most 256. The queue is not cleared between cycles.
- The weak-reference probe allows at most one recently closed window/page/view, as observed in the short baseline with MyGo 0.5.1. It fails if old windows accumulate. Test-owned weak-reference slices and output/profile work create a small amount of measurement overhead.
- After the active period, notifications are explicitly cleared and the app idles for the requested recovery period. GC checkpoints continue. The final `forced_scavenge` row follows `debug.FreeOSMemory`; report it separately from idle recovery. Go GC results alone do not establish native-memory recovery.
- `rss.jsonl` independently records RSS, CPU, and physical footprint every five seconds. WebKit children are selected by the macOS responsibility PID, not by all WebKit processes on the machine. Failed footprint reads are null, not zero. Compare warmed-up intervals and peaks, not only cold start with the final sample.
- Offline analysis aligns process samples with the Go test clock using their UTC timestamps. Samples after the final idle checkpoint are excluded from the memory plot and interval statistics so forced scavenging cannot be counted as idle recovery.
- Heap/goroutine profiles are saved at startup, periodic checkpoints, final load, and recovery. `manifest.json` records the overlay boundary and transport hashes; `result.json` requires behavior assertions and zero attributable WebKit processes after app exit.
- The runner enforces an independent deadline and terminates only processes whose executable path exactly matches its isolated bundle. LaunchServices registration is removed afterward. Evidence and the test bundle remain in `.scratch/` for inspection.
- Navigation timeouts report the target, native URL/loading state, actual document URL/readiness, last evaluation error, and navigation events. Do not retry or discard a failed cycle to make the endurance run pass. Record diagnostic workloads separately from the full three-visit workload.

Update this file, tests/AGENTS.md, tests/internal/AGENTS.md, and the relevant source headers when workload or measurement boundaries change. Record actual duration, workload counts, warmed-up memory trends, idle recovery, and remaining limitations in tests/AGENTS.md.
