// [INPUT]: Embedded default PNG; [OUTPUT]: The default icon shared by native views.
// [POS]: Branding resources without config persistence; update this directory's AGENTS.md and icon consumers.
package brand

import (
	_ "embed"

	"github.com/egoist/mygo/ui"
)

//go:embed icon.png
var png []byte

var Icon = func() *ui.Bitmap {
	icon, err := ui.DecodeBitmap(png)
	if err != nil {
		panic("invalid embedded PageDock icon: " + err.Error())
	}
	return icon
}()
