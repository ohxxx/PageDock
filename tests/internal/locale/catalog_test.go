// [INPUT]: Bilingual catalogs and formatting arguments.
// [OUTPUT]: Language fallback and preservation of user content and error chains.
// [POS]: Catalog behavior tests without source-layout assertions.
// [PROTOCOL]: Update the owning AGENTS.md and behavior tests under tests/.
package locale

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestLanguageMessages(t *testing.T) {
	for _, tc := range []struct{ language, save string }{
		{English, "Save and apply"}, {Chinese, "保存并应用"},
		{"en-GB", "Save and apply"}, {"zh-CN", "保存并应用"}, {"fr", "Save and apply"},
	} {
		if got := Catalog.Format(tc.language, "saveApply", nil); got != tc.save {
			t.Errorf("%s: %q", tc.language, got)
		}
		cause := errors.New("disk unavailable")
		err := fmt.Errorf(Catalog.Format(tc.language, "configWrite", nil), cause)
		if !errors.Is(err, cause) || !strings.Contains(err.Error(), cause.Error()) || strings.Contains(err.Error(), "%!") {
			t.Fatalf("translation lost the cause: %v", err)
		}
		got := fmt.Sprintf(Catalog.Format(tc.language, "notificationTitle", nil), "Work %s 台")
		if !strings.Contains(got, "Work %s 台") || strings.Contains(got, "%!") {
			t.Fatalf("translation changed a user-provided name: %s", got)
		}
	}
}
