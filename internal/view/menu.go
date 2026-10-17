// [INPUT]: Saved app name and icon, caption button area, current background, and native menu.
// [OUTPUT]: Shared saved branding, image previews, and menu callbacks; windows/ draws the Windows variant.
// [POS]: Shared settings/website title bar state; UI-thread access only, with menu actions acquiring the app lock in the background.
// [PROTOCOL]: Update internal/view/AGENTS.md, title bar behavior tests, and platform validation records.
package view

import (
	"runtime"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	prefs "github.com/ohxxx/PageDock/internal/config"
	"github.com/ohxxx/PageDock/internal/view/windows"
)

type Menu struct {
	name   string
	icon   *ui.Bitmap
	onOpen func(int, int)
}

func NewMenu(config prefs.Config) *Menu {
	if runtime.GOOS != "windows" {
		return nil
	}
	return &Menu{name: config.AppName, icon: IconPreview(config.IconPNG)}
}

func (v *Menu) Attach(win *mygo.Window, onOpen func(int, int)) {
	if v != nil {
		win.Update(func() { v.onOpen = onOpen })
	}
}

func (v *Menu) Render(c *ui.Context, background ui.Color) {
	if v != nil {
		windows.RenderMenu(c, background, v.name, v.icon, v.onOpen)
	}
}

func IconPreview(encoded string) *ui.Bitmap {
	data, err := prefs.DecodeIcon(encoded)
	if err != nil || len(data) == 0 {
		return nil
	}
	bitmap, err := ui.DecodeBitmap(data)
	if err != nil {
		return nil
	}
	return bitmap
}
