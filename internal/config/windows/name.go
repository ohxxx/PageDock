//go:build windows

// [INPUT]: An application name normalized by shared validation.
// [OUTPUT]: Reject invalid Windows filenames, reserved names, and trailing periods.
// [POS]: Platform input validation shared by settings and Shell links.
// [PROTOCOL]: Update internal/config/windows/AGENTS.md and behavior tests under tests/.
package windows

import (
	"errors"
	"path/filepath"
	"strings"

	l10n "github.com/ohxxx/PageDock/internal/locale"
)

func ValidateName(name string) error {
	if name != "" && (strings.ContainsAny(name, `<>:"/\|?*`) || strings.HasSuffix(name, ".") || !filepath.IsLocal(name)) {
		return errors.New(l10n.Text("nameWindowsFilename"))
	}
	return nil
}
