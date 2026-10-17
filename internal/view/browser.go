// [INPUT]: The current MyGo locale, embedded WebView loading state, top background color, and caption button area.
// [OUTPUT]: A matching title bar without shadows, a Windows logo/name menu, and a thin loading indicator; controls follow the background.
// [POS]: Website container; background sampling is read-only, and view fields belong to the UI main thread.
// [PROTOCOL]: Update this header and internal/view/AGENTS.md; update the root AGENTS.md when directory boundaries change.
package view

import (
	"context"
	"encoding/hex"
	"log"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	l10n "github.com/ohxxx/PageDock/internal/locale"
)

const (
	browserColorInterval   = 500 * time.Millisecond
	browserLoadingInterval = 100 * time.Millisecond
	browserLoadingHeight   = 2
)

type Browser struct {
	Menu       *Menu
	Web        *mygo.WebView
	background ui.Color
	loading    bool
}

func (v *Browser) Render(c *ui.Context) {
	applyTheme(c)
	background := v.background
	if background.A == 0 {
		background = c.Theme().Background
	}
	ui.Column(c).Fill().Background(background).Children(func() {
		if bar := c.TitleBar(); bar.Height > 0 {
			ui.Box(c).Height(bar.Height).Shrink(0).DragWindow().Children(func() { v.Menu.Render(c, background) })
		}
		if v.Web != nil {
			ui.WebView(c, v.Web).Grow(1)
		}
		if v.loading {
			v.loadingIndicator(c, background)
		}
	})
}

func (v *Browser) loadingIndicator(c *ui.Context, background ui.Color) {
	previous := c.Theme()
	theme := *previous
	// Loading indicator contrast follows the page background, independent of the system theme.
	brightness := 299*int(background.R) + 587*int(background.G) + 114*int(background.B)
	theme.Accent = ui.RGB(36, 36, 36)
	if brightness < 128000 {
		theme.Accent = ui.RGB(237, 237, 237)
	}
	c.SetTheme(&theme)
	var indicator ui.Element
	if c.Preferences().ReduceMotion {
		indicator = ui.Box(c).Background(theme.Accent).Role(ui.RoleProgress)
	} else {
		indicator = ui.Progress(c, -1).Background(ui.Transparent)
	}
	indicator.Absolute().Top(max(0, c.TitleBar().Height-browserLoadingHeight)).
		Left(0).Right(0).Height(browserLoadingHeight).Radius(0).
		Label(l10n.Text("pageLoading")).PassThrough()
	c.SetTheme(previous)
}

func (v *Browser) FollowLoading(win *mygo.Window, page *mygo.Page) {
	stop := make(chan struct{})
	win.OnClosed(func() { close(stop) })
	go func() {
		ticker := time.NewTicker(browserLoadingInterval)
		defer ticker.Stop()
		var previous bool
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
			}
			// Native state also covers pending responses, reloads, stops, and load failures.
			loading := page.IsLoading()
			if loading != previous {
				previous = loading
				win.Update(func() { v.loading = loading })
			}
		}
	}()
}

func (v *Browser) FollowBackground(win *mygo.Window, page *mygo.Page) {
	ctx, cancel := context.WithCancel(context.Background())
	win.OnClosed(cancel)
	ready := make(chan struct{}, 1)
	page.OnDOMReady(func() {
		select {
		case ready <- struct{}{}:
		default:
		}
	})
	go func() {
		ticker := time.NewTicker(browserColorInterval)
		defer ticker.Stop()
		var previous string
		for {
			select {
			case <-ctx.Done():
				return
			case <-ready:
			case <-ticker.C:
			}
			if !win.IsVisible() || win.IsMinimized() {
				continue
			}
			readCtx, stop := context.WithTimeout(ctx, 2*time.Second)
			value, err := page.EvalContext(readCtx, browserBackgroundScript)
			stop()
			color, ok := value.(string)
			if err != nil || !ok || len(color) != 6 || color == previous {
				continue
			}
			rgb, err := hex.DecodeString(color)
			if err != nil {
				continue
			}
			// The runtime draws Windows caption buttons, so the native window background must stay synchronized.
			if err := win.SetBackgroundColor("#" + color); err != nil {
				log.Printf(l10n.Text("backgroundFailed"), err)
				continue
			}
			previous = color
			win.Update(func() { v.background = ui.RGB(rgb[0], rgb[1], rgb[2]) })
		}
	}()
}

// Read only the top background; do not change page layout or expose another Go service to websites.
// shortcut: Use the base color for gradients and images; add pixel sampling only if continuous patterns are required.
const browserBackgroundScript = `(() => {
  if (!document.body) return null;
  const canvas = document.createElement('canvas');
  canvas.width = canvas.height = 1;
  const context = canvas.getContext('2d');
  context.fillStyle = 'Canvas';
  context.fillRect(0, 0, 1, 1);
  const layers = new Set([document.documentElement, document.body,
    ...document.elementsFromPoint(innerWidth / 2, 1).reverse()]);
  for (const element of layers) {
    context.fillStyle = getComputedStyle(element).backgroundColor;
    context.fillRect(0, 0, 1, 1);
  }
  return Array.from(context.getImageData(0, 0, 1, 1).data).slice(0, 3)
    .map(value => value.toString(16).padStart(2, '0')).join('');
})()`
