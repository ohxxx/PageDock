//go:build !desktop

// [INPUT]: An isolated subprocess marker for bilingual control tests; normal tests use Simplified Chinese.
// [OUTPUT]: Native control/error-message tests independent of the developer machine's locale.
// [POS]: Non-desktop test entry; desktop_test.go manages desktop sessions.
// [PROTOCOL]: Update this header, internal/view/AGENTS.md, and tests/AGENTS.md.
package view

import (
	"os"
	"testing"

	l10n "github.com/ohxxx/PageDock/internal/locale"
)

func TestMain(m *testing.M) {
	if os.Getenv("PAGEDOCK_LANGUAGE_TEST") == "" {
		os.Setenv("MYGO_LOCALE", l10n.Chinese)
	}
	os.Exit(m.Run())
}
