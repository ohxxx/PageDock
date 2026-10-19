# PageDock

> Global project rules and directory map. Update this document whenever the folder structure changes.

## Synchronization protocol

1. Code changes require updates to key file headers and the owning directory's `AGENTS.md`; update this file for global changes.
2. Directories with more than 10 files need their own `AGENTS.md`. Desktop code is organized by responsibility under `internal/`; see its map.
3. Documented entries, responsibilities, dependencies, and validation commands must match the code.
4. Keep no obsolete implementations, unused compatibility branches, `_old`, `_backup`, or pending-deletion placeholders.

## Technology and scope

| Layer | Implementation |
|---|---|
| Desktop runtime | Go 1.27.1, MyGo 0.5.1, system WebView |
| Application UI | Native `github.com/egoist/mygo/ui` controls; only the target website uses a WebView |
| Settings | `App.Path(PathUserData)/config.json`, replaced after writing a temporary file in the same directory |
| Notifications | Website Notification / page-side PWA notifications to MyGo native notifications, using the website's existing message source |

A single-user, single-process desktop shell with no backend or database. Set a URL on first use; subsequent launches load the saved site.

## Structure and change entry points

| File/directory | Responsibility |
|---|---|
| `main.go` | The sole root executable entry, calling `pagedock.Run()` on the main thread |
| `internal/` | Startup in run.go, state/lock in session.go, window orchestration in window.go, settings transactions in settings.go, menus in menu.go, locale application in language.go, the narrow PWA bridge in notification.go, and bounded native notification lifetime in notifications.go |
| `internal/config/` | Shared config, validation, permissions, and safe file cleanup; windows/ owns name validation and WebView2 profile discovery |
| `internal/appearance/` | Shared resource encoding/file transactions; macos/ owns bundles/names/caches, windows/ owns EXE/Shell/registry/taskbar; internal/native_*.go selects the platform |
| `internal/brand/` | One default-logo source and native bitmap shared by packaging, settings, title bars, README, and Playground |
| `internal/site/` | Saved/session URL validation, exact origins, and PWA preload scripts |
| `internal/view/` | Shared settings, website container, branding state, and theme; windows/menu.go renders the title bar menu and windows/about.go configures About |
| `internal/locale/` | catalog.go registers native catalogs; en.json and zh-Hans.json provide English and Simplified Chinese |
| `tests/internal/` | Cross-module desktop/notification/language/reset integration and mirrored module behavior tests; see tests/AGENTS.md |
| `tests/playground/` | Shared server tests and Go argument preparation; macos/ checks banners, windows/ validates media |
| `tests/soak/` | Opt-in macOS notification/navigation/window endurance runner with isolated process memory evidence |
| `tests/patches/` / `tests/tools/` | Dependency patch and test-assembly behavior tests |
| `tools/test-overlay/` | Map centralized tests to product packages, preserving platform-group ownership through tests/packages.json without copying product code |
| `playground/` | Standalone local dual-origin notification/media test site, excluded from app packaging |
| `resources/darwin/` | English/Chinese system camera and microphone usage descriptions |
| `patches/mygo/` / `tools/mygo-patch/` | Shared lifecycle, macos/ process resources and visibility events, windows/ integration patches, and immutable dependency copies keyed by patch contents without module-cache changes |
| Directory `AGENTS.md` files / `tests/AGENTS.md` | Module maps, interface constraints, and platform validation scope |
| `go.mod` / `go.sum` / `mygo.json` | Pinned dependencies and package metadata; the app entry remains root `.` |
| `Makefile` / `README.md` | Development, test, check, and build commands; concise product and usage information |
| `.github/` | Native CI checks and tag-triggered six-target GitHub releases; see its AGENTS.md |

## Global constraints

- Communicate with the user in Chinese. Write project documentation and source comments in English. Never automatically commit, push, merge, or publish.
- Check macOS, Windows, and Linux compatibility for all code changes: shared callers, persistence, system integration, rollback, builds, and tests. Implement equivalent user behavior on target platforms; an in-app display change does not prove notification or shortcut support.
- Record system-feature implementation and validation by platform. At minimum compile and run relevant behavior tests. State platform limitations explicitly; do not silently degrade or claim untested device acceptance.
- `playground/` is a disposable local notification/media test site, not the settings UI. Settings remain native MyGo controls. Notifications default to an empty tag for independent sends; same-tag replacement requires explicit input. API acceptance does not prove banner presentation. Media capture starts only on a click, is time-limited, and never uploads media. Validate packaged and development process authorization separately.
- Keep one root main.go, product information, global rules, and build config. Organize desktop code under internal/, all tests/validation scripts under tests/, validation scope in tests/AGENTS.md, and raw evidence in .scratch/.
- Group platform implementations under macos/ and windows/, retaining shared logic outside. Do not repeat directory responsibilities in filenames. Keep Go platform suffixes only where needed for compile-time entry selection; native APIs require explicit build tags. Portable control rendering, file algorithms, and target definitions remain testable on every host. MyGo resources/darwin/ is a fixed protocol path; do not rename it to macos.
- Keep docs/ removed. Maintain rules and validation in AGENTS files; README contains product information and usage entry points only. Use tests/packages.json for tests needing shared private state, without test-only product interfaces.
- Do not create frontend/ or a root package.json: MyGo would infer an app frontend and generate unused TypeScript bindings.
- Never access remote databases, even read-only or indirectly through tests.
- Do not hardcode target websites, credentials, or user directories. Name stable protocol constants.
- Review responsibilities as a file approaches 500 lines; split files over 800 lines. Markdown is exempt.
- Do not write static contract tests. Test input/output, actual storage, failure paths, and permission behavior. Do not create empty, unused __init__.py files.
- Key headers declare inputs, outputs, location, and synchronization triggers.
- Use Refactoring and Design Patterns as references without adding frameworks/interfaces for hypothetical extensions.
- Run `/ponytail-review` after completing requirements and resolve confirmed in-scope issues.
- Follow the command-output rules in `/Users/xxx/.codex/RTK.md`.

## Runtime boundaries

- Support Simplified Chinese, English, and system language, falling back to English for unmatched locales. Empty Config.Language follows the system. Save the choice atomically with the shared draft; failures preserve the active language. Apply through MyGo i18n.Catalog, App.SetLocale, and OnLocaleChanged. Background work waits for main-thread application; event callbacks dispatch goroutines without waiting for the app lock. Older config defaults to system language; full reset restores it, while clearing sign-in data retains language.
- Catalogs live in internal/locale/ and use native MyGo APIs directly, without long generated accessors or new translation dependencies. Settings, menus, app messages, validation, and platform errors use catalogs. User names, URLs, and website notification contents are not translated. Explicitly link MyGo's Simplified Chinese controls; synchronize bilingual macOS permission descriptions and CFBundleLocalizations. Keep default permission descriptions in mygo.json in English, matching resources/darwin/en.lproj/InfoPlist.strings; Chinese descriptions belong in zh-Hans.lproj. System dialogs/WebView messages may require restart to follow a language change; Linux reads locale environment at startup.
- Settings have General, Appearance, Permissions, and Data sections. Wide windows use a sidebar; narrow windows use top tabs. All sections share one draft/save bar. internal/view/theme.go owns tokens, following system light/dark appearance while preserving text scaling, high contrast, and keyboard access.
- Create settings with Content: ui.View(...), with nil Page(). Hide system title text/decorations but retain native buttons. Extend sidebar/content colors into the top drag area without horizontal rules or shadows. Never restore HTML/CSS/JavaScript settings or SetFrontend.
- Website windows hide system title text/decorations and retain native buttons. Reserve an untitled drag area from Context.TitleBar() using the page-top color. Sample colors read-only without changing website layout or adding a Go service. Native Content hosts ui.WebView without a permissions toolbar. Window.Page() is nil; use mainWeb.Page(). Explicitly route page-menu actions to the embedded WebView.
- Read mainWeb.Page().IsLoading() for a thin top loading indicator; remove it on completion, stop, or failure. Do not move website content, add shadows, or leave a permanent divider. Website-internal API requests are not page loads.
- Browser follower timers stop while hidden/minimized and release event subscriptions on close. Native window/navigation events wake them; loading retains a 100 ms active/1 s idle fallback for unreported native navigation starts. Stable background reads back off to 2 s and reuse one offscreen canvas per document.
- Windows caption-button colors follow each window's opaque background; settings retain system appearance. Page color sampling also updates the native window background. Do not override global themes to fix buttons. Build entries automatically prepare patches/mygo/; direct Go builds must use the preparer's modfile. Recheck/remove superseded patches after upstream upgrades.
- Windows settings/website windows show the saved logo/name at top left. Mouse or keyboard activation opens existing native menus: About, Settings, Website, View, and related actions. Truncate long names, hide in fullscreen, and preserve blank drag space/right-side buttons. Empty images use the internal/brand browser-tray logo; default names follow locale. Unsaved drafts do not change title bars. About reads the packaged version. macOS/Linux retain system menu entries.
- Disable WebView2 permission persistence on Windows. Before the first navigation, revoke only legacy camera, microphone, geolocation, and notification decisions, both grants and denials, preserving other permissions, cookies, and site storage. Wait for native completion before navigation; fail visibly and block loading on error, cancelling on close. Require WebView2 Runtime 1.0.1661.34 or newer. See tests/internal/windows/media_test.go and validation records for capture/local WebRTC coverage.
- Manage notifications, camera, microphone, and supported geolocation permissions in native settings. Save config atomically before recreating the page. Permission-only changes retain the session URL; temporary sessions/callbacks never overwrite the home page. Cross-site pages inherit no permissions. Disabling HTTP must not reload an HTTP session.
- Register only the narrow PWA.Receive/PWA.Close notification service. Website windows trust the exact saved origin only; config operations remain native-control/menu actions.
- Native view fields belong to the UI main thread. Pass background arguments by value and return results through Window.Update. View/event callbacks must not wait for the app lock.
- Website and settings-test notifications share a maximum of 256 native records per process, expiring after 24 hours. Click, close, failure, eviction, expiry, and full reset release native records/timers. Navigation releases page-specific callbacks while preserving bounded native history. Same-tag replacement must not be closed by stale click, cancellation, or expiry callbacks; native callbacks dispatch before acquiring the notification queue lock.
- URLs default to HTTPS; HTTP requires explicit opt-in. Reject usernames/passwords and common token parameters in saved URLs. Scan complete queries/fragments; a question mark inside a value must not hide outer parameters.
- Website windows allow cross-site HTTPS links, script navigation, and HTTP redirects subject to the HTTP setting. Navigation grants no Go/device permissions, and login callback parameters are never persisted. Cross-site popups open in the external browser.
- Website notifications default off. Enabling grants notifications only to the exact saved origin and permits PWA.Receive to forward title/body to MyGo. Camera/microphone are independently configured; mixed requests require every permission and unknown requests are denied.
- MyGo 0.5.1 on macOS has no geolocation permission callback; settings must mark it unavailable. Do not pretend unsupported permissions are controllable. Retain camera/microphone Info.plist usage descriptions.
- Inject the bridge only into top-level documents at the saved origin. Notification.permission reflects native settings; MyGo Show requests/enforces system permission and reports denial. PWA clicks use the website's existing NOTIFICATION_CLICK event, keeping data only in the page.
- Initialize user directories after WhenReady, never during MyGo build/binding generation.
- Native events run on the UI main thread; dispatch to a goroutine before acquiring application.mu. Never synchronously wait under a lock for a callback needing the same lock. Serialize config, window switching, and cleanup.
- Show native confirmation before destroying the website window for cleanup. Clear sign-in data removes all PageDock sites' browsing data but retains config. Clear all data also removes URLs, app permissions, window state, and app-managed notifications, then returns to empty first-run settings after restart. Do not reset system privacy permissions; Windows/Linux notification history may need manual cleanup.
- On Windows, both cleanup actions persist a request and restart. Before any window/WebView, delete the WebView2 profile belonging to the fixed internal name. Wait up to 15 seconds for locked files; on failure retain the marker and exit without opening the old site. Reset appearance, config, and window state only after successful browsing cleanup. Clear sign-in data retains config/window state. macOS/Linux retain native cleanup before restart; an empty marker means browsing data is already cleared and remains compatible with older requests.
- Windows profile paths follow pinned MyGo 0.5.1: LOCALAPPDATA/<internal name>/WebView2, falling back to the system temporary directory. They never follow display names or PAGEDOCK_DATA_DIR. Recheck on dependency upgrades. Do not follow profile/parent links or remove system browser directories. Explicitly reject WEBVIEW2_USER_DATA_FOLDER overrides rather than guessing paths or reporting false success.

## Validation

Make assembles centralized tests. Bare go test ./... does not discover them; direct Go/IDE instructions are in tests/AGENTS.md.

```sh
make test
make test-playground
make test-desktop # Requires a local desktop session; opens isolated test windows
python3 tests/soak/run.py --duration 1800 --recovery 300 --output .scratch/resource-soak/run-name # Opt-in macOS endurance; see tests/soak/AGENTS.md
make test-media-windows # Windows device inputs and real permission/WebRTC tests
make test-notifications # Real macOS banners; run make dev once for the Dev notification identity
make check
make build
make build BUILD_ARGS='-platform darwin/arm64,darwin/amd64' # Include release DMGs on macOS
make build-windows # Windows x64/ARM64 with caption-button patches
make dev
make playground # Local notification/media site on 127.0.0.1
```

PAGEDOCK_DATA_DIR isolates config/window-state directories, not macOS WebKit or Windows WebView2 profiles. Windows file cleanup tests use temporary LOCALAPPDATA and file locks only. Record target execution separately from cross-compilation. State the actual scope for notification presentation, cookies across restarts, and platform Push capabilities; successful builds do not prove notification delivery.

GitHub Actions runs headless checks on macOS, Windows, and Linux. Stable vMAJOR.MINOR.PATCH tags matching mygo.json run those checks before packaging both architectures of each platform and publishing a release with SHA256SUMS. Manual Release runs produce downloadable artifacts only. Builds use Makefile patch preparation, ad-hoc macOS signing, and unsigned Windows installers; see .github/AGENTS.md for permissions and retry behavior.

## Navigation for agents

Start at main.go and internal/run.go. Application state/locking: internal/session.go. Window orchestration: internal/window.go. Settings actions: internal/settings.go. Storage/reset/browser data: internal/config/. URL/preload scripts: internal/site/. Notification receive/close: internal/notification.go. Platform appearance: internal/appearance/. Default branding: internal/brand/. Native views: internal/view/. Translation: internal/locale/. Tests mirror responsibilities under tests/; tests/packages.json preserves platform package ownership. Commands and Validation records are in tests/AGENTS.md.

### Custom application icons

- The default logo comes only from internal/brand/icon.png. mygo.json selects it for packaging; native previews, Windows title bars, Playground, and README use the same image. Default config remains empty; never overwrite saved custom images.
- Native settings own selection, preview, and default restoration. Saving icon changes restarts the app; full reset also restores defaults. Packaged macOS apps relaunch through LaunchServices while explicitly retaining the config directory; MyGo's supervisor restarts development apps.
- Persist image contents, not external paths. Bound PNG/JPEG size/dimensions and preserve aspect ratio. On import, fit visible alpha bounds into the centered 80% of a 512-pixel transparent canvas; replace existing transparent margins instead of adding them repeatedly. Give fully opaque rectangular artwork antialiased rounded corners, while retaining existing rounded/transparent silhouettes and alpha. macOS and Windows consume the same normalized PNG. Decode saved icons unchanged; older icons adopt the new sizing when reselected.
- macOS supports writable local ad-hoc bundles. Stage complete bundles, verify signatures, exchange atomically, and roll back on failure. Preserve default resources and the developer-signing boundary. Add no website bridge service.
- Windows supports unsigned x64/ARM64 EXEs in writable directories. Preserve non-icon resources and embed original icons for repeated changes/default restoration. Verify staged bytes before replacing the EXE, updating desktop/Start menu/pinned links, and saving config. Roll back failures; never let next-launch cleanup delete failed-recovery copies. The MyGo patch retries successfully committed .<current EXE filename>.old-* regular files every 100ms for up to 15 seconds at next startup. Log timeouts and retain files for later launches; never delete directories, links, or old copies of other EXEs. Relaunch through MyGo; windows/new notifications use icon resource 1. Shell cache refresh does not prove visible device behavior.
- Windows caches only successfully synchronized name/icon pairs for the current process. Ordinary URL/language/permission saves bypass EXE reads/parsing, while startup, actual appearance changes, and failed appearance transactions use the full synchronization/rollback path.
- Linux disables icon selection with an explanation. Retain valid saved images without applying them or blocking startup/ordinary saves. Default restoration/full reset can still remove them.
- Notifications use bundle icons. On changes/default restoration, wait for IconServices invalidation and reload the current user's Notification Center before saving config. Failures restore the bundle, registration, and caches. Fail explicitly if private APIs change; ordinary settings saves do not refresh icons. Open notification panels close, while history/authorization remain. Finder/Dock correctness or valid bundle resources do not prove notification icon presentation; record actual scope in tests/AGENTS.md.
- On macOS, synchronize the runtime Dock icon after startup and successful saves; an empty icon restores the default. Bundle resource updates do not replace runtime synchronization. Settings test notifications use a unique ID each time.

### Custom application names

- General settings provide a name input; empty restores the default. Keep only necessary action hints, without sidebar slogans or section introductions.
- Save name/icon together and restart on changes. On macOS, unlocalized CFBundleName/CFBundleDisplayName match the .app filename; custom names go in Base.lproj and existing locale InfoPlist.strings so Dock/Finder and the app agree. Reuse full-bundle signing, exchange, registration, cache refresh, and rollback. Full reset restores original names/language resources. Migrate older saved names at startup without requiring manual renaming.
- Names allow up to 64 Unicode characters and reject control/format characters. Windows also rejects forbidden filename characters, device names, and trailing periods. Use names only for internal taskbar links matched to the current EXE, never EXE/config/browser paths. MyGo's internal name, bundle ID, app filename, lock, and user-data directory stay fixed.
- Windows updates all EXE languages' product/file descriptions, desktop/Start menu/pinned visible names, and the matching installation's per-user uninstall name. Notifications retain MyGo's system-tray output/process metadata. Desktop/Start menu SHSetLocalizedName updates preserve real link paths. Authorized UserPinned taskbar links are renamed with real Shell notifications, retaining targets, arguments, pinning, and uninstall entries. Reject same-name collisions; roll back EXE, links, paths, directory metadata, registry, and config on failure. Default restoration removes this app's name overrides and restores original version resources.
- Explicitly set RelaunchDisplayNameResource through each window's IPropertyStore, alongside relaunch command/icon and a stable EXE-path AppUserModel.ID. Before the first window, synchronize existing links' IDs, launch names, commands, and icons. Rename internal taskbar links only; desktop/Start menu paths remain fixed. Clean old mappings transactionally and restore link bytes/paths/metadata on failure. With no embedded icon, use the Shell default application icon to avoid stale custom images. Finish before settings/website windows show; remove properties on native destruction.
- Keep the Windows MyGo internal name fixed because WebView2 profile paths depend on it. Never call App.SetName to change notification names and lose sign-in state. State Linux limitations separately from Windows/macOS validation.
- Actual Windows notification source names, visible shortcut names, and pinned items require Windows device validation; compilation and PE checks are insufficient. See tests/AGENTS.md for scope.
