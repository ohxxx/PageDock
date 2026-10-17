// [INPUT]: Saved name and image, default logo, caption button area, background, and menu callback.
// [OUTPUT]: The Windows title bar menu button and mouse/keyboard interactions.
// [POS]: Windows presentation using portable ui controls, with rendering tests on every host.
// [PROTOCOL]: Update internal/view/windows/AGENTS.md, the parent Menu, and input/layout tests.
package windows

import (
	"fmt"

	"github.com/egoist/mygo/ui"
	"github.com/ohxxx/PageDock/internal/brand"
	l10n "github.com/ohxxx/PageDock/internal/locale"
)

const (
	titleMenuInset     = 6
	titleMenuMaxWidth  = 280
	titleMenuDragSpace = 48
	titleMenuIconSize  = 18
)

func RenderMenu(c *ui.Context, background ui.Color, name string, icon *ui.Bitmap, onOpen func(int, int)) {
	if c.TitleBar().Height == 0 {
		return
	}
	bar := c.TitleBar()
	width, _ := c.Size()

	if name == "" {
		name = l10n.Text("appName")
	}
	ink := ui.Hex("#202020")
	if 299*int(background.R)+587*int(background.G)+114*int(background.B) < 128000 {
		ink = ui.Hex("#f7f7f7")
	}
	previous := c.Theme()
	theme := *previous
	theme.Focus = ink
	c.SetTheme(&theme)
	defer c.SetTheme(previous)
	button := ui.ButtonBase(c.Key("appMenu")).Label(fmt.Sprintf(l10n.Text("appMenu"), name)).
		Absolute().Left(bar.Left+titleMenuInset).Top(3).Height(bar.Height-6).
		MaxWidth(min(titleMenuMaxWidth, width-bar.Left-bar.Right-titleMenuDragSpace)).MinWidth(0).
		Padding(0, 8).Gap(6).Radius(4).TextColor(ink).FontSize(min(theme.Rem(0.85), bar.Height-10))
	if button.Hovered() {
		button.Background(ink.Alpha(0.10))
	}
	button.Children(func() {
		if icon == nil {
			icon = brand.Icon
		}
		ui.Image(c, icon).Size(titleMenuIconSize, titleMenuIconSize).Fit(ui.Contain).Shrink(0)
		ui.Text(c, name).SingleLine().MinWidth(0).Shrink(1)
	})
	if button.Clicked() && onOpen != nil {
		r := button.Bounds()
		onOpen(int(r.X), int(r.Y+r.H))
	}
}
