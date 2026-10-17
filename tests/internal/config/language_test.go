// [INPUT]: Bilingual catalogs, config storage, desktop-free native controls, and formatted errors.
// [OUTPUT]: Language persistence, old-config retention after invalid input, translation fallback, and error-chain tests.
// [POS]: Behavior tests without source matching or fixed file-layout requirements.
// [PROTOCOL]: Update this header, internal/config/AGENTS.md, and tests/AGENTS.md.
package config

import (
	"os"
	"path/filepath"
	"testing"

	l10n "github.com/ohxxx/PageDock/internal/locale"
)

func TestLanguageConfig(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), Filename)}
	for _, language := range []string{l10n.English, l10n.Chinese, ""} {
		want := Config{URL: "https://example.com/", Language: language, AllowCamera: true}
		if err := store.Save(want); err != nil {
			t.Fatal(err)
		}
		got, err := store.Load()
		if err != nil || got != want {
			t.Fatalf("language round trip: %+v, %v", got, err)
		}
		before, _ := os.ReadFile(store.Path)
		want.Language = "unsupported"
		if err := store.Save(want); err == nil {
			t.Fatal("unsupported language was accepted")
		}
		after, _ := os.ReadFile(store.Path)
		if string(before) != string(after) {
			t.Fatal("invalid language overwrote settings")
		}
	}
	if err := os.WriteFile(store.Path, []byte(`{"url":"https://example.com/"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if got, err := store.Load(); err != nil || got.Language != "" {
		t.Fatalf("old configuration must follow the system: %+v, %v", got, err)
	}
	if err := store.PrepareReset(func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CompleteReset(func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if got, err := store.Load(); err != nil || got != (Config{}) {
		t.Fatalf("reset retained settings: %+v, %v", got, err)
	}
}
