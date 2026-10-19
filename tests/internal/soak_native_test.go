//go:build desktop && pagedock_soak && (darwin || windows)

// [INPUT]: The same isolated HTTP origins and real MyGo windows as the full endurance workload.
// [OUTPUT]: Native navigation lifetimes with independent application integrations and shutdown timing.
// [POS]: Differential diagnostic; completion comes from navigation events, not script polling.
// [PROTOCOL]: Update tests/internal/AGENTS.md and tests/soak/AGENTS.md with scope changes.
package pagedock

import (
	"fmt"
	"testing"
	"time"
	"weak"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	prefs "github.com/ohxxx/PageDock/internal/config"
	website "github.com/ohxxx/PageDock/internal/site"
	views "github.com/ohxxx/PageDock/internal/view"
)

func soakBareCycle(t *testing.T, source, foreign string, stats *soakMetrics, refs *soakReferences) {
	t.Helper()
	var content *mygo.WebView // Accessed only by the UI thread.
	view := &views.Browser{Menu: views.NewMenu(prefs.Config{})}
	options := mygo.WindowOptions{Title: "PageDock isolated MyGo diagnostic", Hidden: true,
		Width: 620, Height: 420, TitleBarStyle: mygo.TitleBarHidden, Content: ui.View(func(c *ui.Context) {
			if content != nil {
				ui.WebView(c, content).Fill()
			}
		})}
	if *soakBareRender {
		options.Content = ui.View(view.Render)
	}
	var win *mygo.Window
	if *soakBareTaskbar {
		var err error
		win, err = newWindow(options, options.Title)
		if err != nil {
			if win != nil {
				win.Destroy()
			}
			t.Fatal(err)
		}
	} else {
		win = mygo.NewWindow(options)
	}
	if win == nil {
		t.Fatal("create native diagnostic window")
	}
	defer win.Destroy()
	webOptions := mygo.WebViewOptions{}
	if *soakBareBridge {
		webOptions.Page = mygo.PageOptions{TrustedOrigins: []string{source}, PreloadScript: website.Preload(source, true)}
	}
	web, err := win.NewWebView(webOptions)
	if err != nil {
		t.Fatal(err)
	}
	win.Update(func() { content = web; view.Web = web })
	page := web.Page()
	if *soakBareBackground {
		view.FollowBackground(win, page)
	}
	if *soakBareLoading {
		view.FollowLoading(win, page)
	}
	refs.views = append(refs.views, weak.Make(view))
	refs.windows = append(refs.windows, weak.Make(win))
	refs.pages = append(refs.pages, weak.Make(page))
	win.ShowInactive()
	for visit := 0; visit <= *soakVisits; visit++ {
		origin := source
		if visit == *soakVisits {
			origin = foreign
		}
		target := fmt.Sprintf("%s/cycle-%d/page-%d", origin, stats.cycles, visit)
		if *soakBareEval {
			soakNavigate(t, page, target)
		} else {
			soakNativeNavigate(t, page, target)
		}
		stats.navigations++
	}
	if *soakBareBridge {
		if got := soakEval(t, page, `(async()=>{try{await mygo.call('PWA.Receive',{title:'foreign'});return 'allowed'}catch{return 'denied'}})()`); got != "denied" {
			t.Fatalf("foreign notification authorization: %v", got)
		}
	}
	time.Sleep(*soakBareDwell)
	win.Hide()
	time.Sleep(50 * time.Millisecond)
	win.ShowInactive()
	stats.cycles++
}

func soakNativeNavigate(t *testing.T, page *mygo.Page, target string) {
	t.Helper()
	completed := make(chan error, 1)
	finish := func(err error) {
		select {
		case completed <- err:
		default:
		}
	}
	off := page.OnDidFinishLoad(func() {
		if page.URL() == target {
			finish(nil)
		}
	})
	defer off()
	off = page.OnDidFailLoad(func(err *mygo.LoadError) { finish(fmt.Errorf("navigation failed: %+v", err)) })
	defer off()
	if err := page.LoadURL(target); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-completed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(20 * time.Second):
		t.Fatalf("native navigation timed out: %s", target)
	}
}
