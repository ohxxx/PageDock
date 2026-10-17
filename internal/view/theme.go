// [INPUT]: System appearance, text scaling, and increased contrast preferences.
// [OUTPUT]: Neutral theme, spacing, and window background shared by settings and the website container.
// [POS]: Native UI design tokens; preserve platform fonts, keyboard, focus, and animation behavior.
// [PROTOCOL]: Update this header, internal/view/AGENTS.md, and Validation records in tests/AGENTS.md.
package view

import (
	"github.com/egoist/mygo/ui"
)

const (
	WindowBackground     = "light-dark(#fafafa, #212121)"
	SettingsWidth        = 860
	SettingsHeight       = 700
	settingsCompactWidth = 680
	settingsSidebarWidth = 184
	settingsPadding      = 32
)

func applyTheme(c *ui.Context) {
	t := *c.Theme()
	t.Background, t.Surface = ui.Hex("#fafafa"), ui.Hex("#f2f2f2")
	t.SurfaceHover, t.SurfacePressed = ui.Hex("#eaeaea"), ui.Hex("#e2e2e2")
	t.Text, t.TextMuted, t.Border = ui.Hex("#202020"), ui.Hex("#666666"), ui.Hex("#dedede")
	t.Accent, t.AccentHover, t.AccentPressed = ui.Hex("#242424"), ui.Hex("#3a3a3a"), ui.Hex("#484848")
	t.AccentText, t.Danger = ui.Hex("#fafafa"), ui.Hex("#b42318")
	if t.Dark {
		t.Background, t.Surface = ui.Hex("#212121"), ui.Hex("#292929")
		t.SurfaceHover, t.SurfacePressed = ui.Hex("#333333"), ui.Hex("#3d3d3d")
		t.Text, t.TextMuted, t.Border = ui.Hex("#ededed"), ui.Hex("#adadad"), ui.Hex("#424242")
		t.Accent, t.AccentHover, t.AccentPressed = ui.Hex("#ededed"), ui.Hex("#dcdcdc"), ui.Hex("#cccccc")
		t.AccentText, t.Danger = ui.Hex("#212121"), ui.Hex("#ff938a")
	}
	t.Radius = 8
	t.Selection, t.Focus = t.Text.Alpha(0.16), t.TextMuted
	if c.Preferences().HighContrast {
		t.Border = t.Border.Mix(t.Text, 0.45)
		t.TextMuted = t.TextMuted.Mix(t.Text, 0.4)
		t.Focus = t.Text
	}
	c.SetTheme(&t)
}
