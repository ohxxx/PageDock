# Language catalogs

`catalog.go` embeds `en.json` and `zh-Hans.json` directly and registers them with MyGo `i18n.Catalog`. Explicitly link MyGo's Simplified Chinese control package. `Text(key)` uses the current native locale; `Catalog` provides language enumeration and formatting.

Unmatched languages fall back to English. Never translate user-entered names, URLs, or website notification contents. This package only registers catalogs and looks up text; `config` saves choices, while `internal/language.go` applies locales and refreshes windows on the main thread.

Catalog formatting tests live in `tests/internal/locale/`; persistence, UI, and desktop switching belong to their respective test packages. Update headers, this document, bilingual usage descriptions, and behavior tests. Run `make test` and `make test-desktop`.
