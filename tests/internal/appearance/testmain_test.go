// [INPUT]: An isolated subprocess marker for bilingual control tests; normal tests use Simplified Chinese.
// [OUTPUT]: Native control/error-message tests independent of the developer machine's locale.
// [POS]: Portable resource test entry without native windows or desktop event loops.
// [PROTOCOL]: Update this header, internal/appearance/AGENTS.md, and tests/AGENTS.md.
package appearance

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
