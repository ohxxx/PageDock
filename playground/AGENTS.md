# Playground: notifications and media

Tests and validation scripts live under tests/playground/; this directory contains only the runnable local site.

This standalone local notification/media site is not PageDock's settings UI, is not embedded in the app, and is excluded from MyGo builds. Native settings manage websites and browser permissions.

| File | Responsibility |
|---|---|
| `index.html` / `style.css` | Neutral monochrome notification form, folded advanced options/event details, light/dark appearance, and narrow layouts |
| `app.js` | Real Notification/Service Worker APIs, messageId request/receipt correlation, errors, clicks, and page visibility |
| `media.html` / `media.js` | Microphone, camera, and mixed-permission tests; stop real capture after 3 seconds and verify decoding, volume, and frames without uploading media |
| `sw.js` | Local Worker activation, Worker-send diagnostics, and browser-native click callbacks; no caching or remote Push |
| `server.mjs` | Standard-library Node dual-loopback service, resource allowlist, shared logo, same-origin diagnostics, and bounded memory logs |
| `tests/playground/server.test.mjs` | Real HTTP input/output, origin isolation, resource boundaries, and failure paths |
| `tests/playground/windows/media.mjs` | Generate non-silent WAV/dynamic Y4M sources, isolate profiles, run real permissions/WebRTC, and clean up; accepts a precompiled test EXE |
| `tests/playground/macos/notifications.mjs` | Copy Dev identity/resources, compile/sign an isolated notification test bundle, unregister it afterward, and re-register Dev |
| `package.json` | Dependency-free development, syntax checks, and native Node tests |

Start with root `make playground` or `npm run dev` here. The primary port defaults to 4173; the other origin uses an assigned port. PORT/FOREIGN_PORT override these, with 0 requesting system allocation. Listen only on 127.0.0.1. The first output line is JSON containing both actual URLs.

The logo and favicon use /icon.png, reading only internal/brand/icon.png without exposing directories or arbitrary files. Show primary actions by default and keep details in keyboard-accessible native details elements. Exports retain full events; folding changes neither notifications, timers, permissions, nor capture. Device actions still require explicit clicks.

/media.html uses real getUserMedia, MediaRecorder, and video-frame callbacks. All-zero microphone levels produce media-warning, not media-pass; a file/sample count alone does not prove sound capture. Recordings remain in page memory for playback and are released on the next test/navigation. Diagnostics contain statistics only, never audio/video or device names/IDs. Do not automatically request permissions; start on click, allow immediate stop, and exclude time waiting for system authorization from the 3-second capture limit. For release validation, launch the make build artifact through LaunchServices with explicit isolated PAGEDOCK_DATA_DIR; development authorization does not prove release identity. Check denial when app permission is off, actual capture when on, mixed-request rejection, and device release. Retain same-origin /api/events statistics and TCC identity evidence.

Run `make test-playground`. `make test-desktop` starts an independent random-port service; tests/internal/playground_test.go loads these same resources in a real WebView. Desktop integration substitutes native notification output; actual banners/clicks require packaged-app validation.

`make test-notifications` compiles packaged race tests and checks each real system presentation. Run make dev once to generate the Dev bundle. Keep evidence in .scratch/native-notifications/run-* without changing user config/system preferences, and clear only this run's notifications. PAGEDOCK_TEST_NOTIFICATION_INTERVAL=0s tests bursts; PAGEDOCK_TEST_NOTIFICATION_BACKGROUND=1 tests background behavior; PAGEDOCK_TEST_NOTIFICATION_OBSERVE=30s retains banners for observation, up to one minute. Screen sharing affects results, so observe after all sends.

Keep Dev Resources in the test bundle and use the current icon as its default, preventing empty test config from restoring a missing icon. Always clean registration on normal completion or subprocess failure, then rename the test bundle suffix to .fixture to retain evidence. Never leave an iconless executable bundle registered under the same identity. Successful presentation logs do not prove notification-card icon correctness; inspect icons separately.

Do not mock MyGo in pages, bypass origin restrictions, or write user config. Record API acceptance, system presentation, and actual clicks separately. getNotifications and Worker-side sends expose capability gaps; empty arrays or API presence do not prove support. Render logs with textContent; server records stay in bounded memory. Update this map, root AGENTS.md, and tests/AGENTS.md including Validation records after behavior changes.

Keep the default tag empty so consecutive messages remain independent. Test replacement only with an explicitly entered shared tag. Never default to a session tag, which merges different messageIds into one system notification. Desktop tests send all ordinary/PWA messages before checking individual click mappings; this does not prove every banner appeared. Record system suppression such as screen sharing and Focus separately during device validation.

On Windows, run `node tests/playground/windows/media.mjs` from the project root; it prepares patches/overlay automatically. Only device inputs use deterministic sources. Do not hide authorization bugs with auto-grant flags. Validate physical camera/microphone devices separately through /media.html.
