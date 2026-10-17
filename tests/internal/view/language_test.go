//go:build !desktop

// [INPUT]: Isolated English/Chinese test processes, real native input, language options, and window scaling.
// [OUTPUT]: Bilingual rendering, draft saves, busy-state disabling, and narrow-window accessibility tests.
// [POS]: ui.Tester drives user behavior without launching system apps or changing system locale preferences.
// [PROTOCOL]: Update this header, internal/view/AGENTS.md, and tests/AGENTS.md.
package view

import (
	"os"
	"os/exec"
	"testing"

	"github.com/egoist/mygo/ui"
	prefs "github.com/ohxxx/PageDock/internal/config"
	l10n "github.com/ohxxx/PageDock/internal/locale"
)

func TestLanguageInterface(t *testing.T) {
	if os.Getenv("PAGEDOCK_LANGUAGE_TEST") == "" {
		for _, language := range []string{l10n.English, l10n.Chinese} {
			t.Run(language, func(t *testing.T) {
				cmd := exec.Command(os.Args[0], "-test.run=^TestLanguageInterface$", "-test.timeout=45s")
				cmd.Env = append(os.Environ(), "PAGEDOCK_LANGUAGE_TEST=1", "MYGO_LOCALE="+language)
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("%v\n%s", err, out)
				}
			})
		}
		return
	}
	var saved prefs.Config
	v := &Settings{Draft: prefs.Config{URL: "https://example.com/"}, OnAction: func(_ Action, c prefs.Config) { saved = c }}
	tt := ui.NewTester(v.Render, 860, 900)
	tt.SetLocale(os.Getenv("MYGO_LOCALE"))
	clickNative(t, tt, "English")
	if v.Draft.Language != l10n.English || saved.Language != "" {
		t.Fatal("language did not remain a draft")
	}
	clickNative(t, tt, l10n.Text("saveApply"))
	if saved.Language != l10n.English || saved.URL != "https://example.com/" {
		t.Fatalf("language was not submitted: %+v", saved)
	}
	clickNative(t, tt, "简体中文")
	if v.Draft.Language != l10n.English {
		t.Fatal("busy form accepted a language change")
	}
	v.Busy = false
	tt.Frame()
	clickNative(t, tt, l10n.Text("languageSystem"))
	clickNative(t, tt, l10n.Text("saveApply"))
	if saved.Language != "" {
		t.Fatal("follow system was not submitted")
	}
	for _, scale := range []float32{1, 1.5} {
		for _, dark := range []bool{false, true} {
			view := &Settings{OnAction: func(Action, prefs.Config) {}}
			layout := ui.NewTester(view.Render, 420, 480)
			layout.SetLocale(os.Getenv("MYGO_LOCALE"))
			layout.SetDark(dark)
			layout.SetPreferences(ui.Preferences{TextScale: scale, HighContrast: true})
			for _, section := range settingsSections {
				clickNative(t, layout, l10n.Text(section.label))
				for _, key := range []string{"general", "appearance", "permissions", "data", "saveApply", "openSaved"} {
					r, ok := layout.Find(l10n.Text(key))
					if !ok || r.W <= 0 || r.X < 0 || r.X+r.W > 420 || r.Y < 0 || r.Y+r.H > 480 {
						t.Fatalf("%s overflows at scale %v: %+v", key, scale, r)
					}
				}
			}
		}
	}
}
