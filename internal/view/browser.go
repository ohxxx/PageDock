// [INPUT]: The current MyGo locale, embedded WebView loading state, top background color, and caption button area.
// [OUTPUT]: A matching title bar and thin loading indicator, with adaptive visible-only sampling and document-scoped canvas reuse.
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
	browserColorInterval       = 500 * time.Millisecond
	browserColorMaxInterval    = 2 * time.Second
	browserLoadingInterval     = 100 * time.Millisecond
	browserLoadingIdleInterval = time.Second
	browserReadTimeout         = 2 * time.Second
	browserLoadingHeight       = 2
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
	var previous bool
	followBrowser(win, page, func(_ context.Context, wake bool) time.Duration {
		// Keep an idle fallback: MyGo's navigation events do not cover native LoadURL/Reload starts.
		loading := page.IsLoading()
		if loading != previous {
			previous = loading
			win.Update(func() { v.loading = loading })
		}
		if loading || wake {
			return browserLoadingInterval
		}
		return browserLoadingIdleInterval
	})
}

func (v *Browser) FollowBackground(win *mygo.Window, page *mygo.Page) {
	var previous string
	interval := browserColorInterval
	followBrowser(win, page, func(ctx context.Context, wake bool) time.Duration {
		if wake {
			interval = browserColorInterval
		}
		readCtx, stop := context.WithTimeout(ctx, browserReadTimeout)
		value, err := page.EvalContext(readCtx, browserBackgroundScript)
		stop()
		color, ok := value.(string)
		if err == nil && ok && len(color) == 6 && color != previous {
			rgb, err := hex.DecodeString(color)
			if err == nil {
				// The runtime draws Windows caption buttons; keep the native background synchronized.
				if err := win.SetBackgroundColor("#" + color); err != nil {
					log.Printf(l10n.Text("backgroundFailed"), err)
				} else {
					previous = color
					win.Update(func() { v.background = ui.RGB(rgb[0], rgb[1], rgb[2]) })
					interval = browserColorInterval
					return interval
				}
			}
		}
		interval = min(interval*2, browserColorMaxInterval)
		return interval
	})
}

// Both followers sleep without a timer while hidden/minimized and release their event subscriptions on close.
func followBrowser(win *mygo.Window, page *mygo.Page, sample func(context.Context, bool) time.Duration) {
	ctx, cancel := context.WithCancel(context.Background())
	wake := make(chan struct{}, 1)
	signal := func() {
		select {
		case wake <- struct{}{}:
		default:
		}
	}
	off := []func(){
		win.OnClosed(cancel), win.OnShow(signal), win.OnHide(signal),
		win.OnMinimize(signal), win.OnRestore(signal), win.OnFocus(signal),
		// App.Hide/Show does not emit window visibility events for unfocused macOS windows.
		mygo.App.OnDidBecomeActive(signal), mygo.App.OnDidResignActive(signal),
		mygo.App.OnActivate(func(bool) { signal() }),
		page.OnWillNavigate(func(*mygo.NavigateEvent) { signal() }),
		page.OnDidNavigate(func(string) { signal() }), page.OnDOMReady(signal),
		page.OnDidFinishLoad(signal), page.OnDidFailLoad(func(*mygo.LoadError) { signal() }),
	}
	if win.IsDestroyed() {
		cancel()
	}
	go func() {
		timer := time.NewTimer(0)
		defer timer.Stop()
		defer cancel()
		defer func() {
			for _, unsubscribe := range off {
				unsubscribe()
			}
		}()
		for {
			var event bool
			select {
			case <-ctx.Done():
				return
			case <-wake:
				event = true
			case <-timer.C:
			}
			timer.Stop()
			if ctx.Err() != nil {
				return
			}
			if !win.IsVisible() || win.IsMinimized() {
				continue
			}
			timer.Reset(sample(ctx, event))
		}
	}()
}

// Read only the top background; do not change page layout or expose another Go service to websites.
// shortcut: Use the base color for gradients and images; add pixel sampling only if continuous patterns are required.
const browserBackgroundScript = `(() => {
  if (!document.body) return null;
  const key = Symbol.for('pagedock.background-sampler');
  let context = globalThis[key];
  if (!context) {
    const canvas = document.createElement('canvas');
    canvas.width = canvas.height = 1;
    context = canvas.getContext('2d', {willReadFrequently: true});
    if (!context) return null;
    globalThis[key] = context;
  }
  context.clearRect(0, 0, 1, 1);
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
