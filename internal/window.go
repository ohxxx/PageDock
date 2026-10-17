// [INPUT]: Saved settings, application error state, and UI main-thread callbacks.
// [OUTPUT]: Native settings/website windows, navigation authorization, loading feedback, and activation.
// [POS]: Window orchestration under the application lock; view/ owns controls.
// [PROTOCOL]: Update this header, internal/AGENTS.md, tests/AGENTS.md, and related behavior tests.
package pagedock

import (
	"fmt"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	prefs "github.com/ohxxx/PageDock/internal/config"
	l10n "github.com/ohxxx/PageDock/internal/locale"
	website "github.com/ohxxx/PageDock/internal/site"
	views "github.com/ohxxx/PageDock/internal/view"
)

func (a *application) showSettings() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.showSettingsLocked()
}

func (a *application) showSettingsLocked() {
	if !a.started || a.restarting {
		return
	}
	if a.settings != nil && !a.settings.IsDestroyed() {
		a.refreshSettingsLocked()
		focus(a.settings)
		return
	}
	v := &views.Settings{
		Menu:  views.NewMenu(a.config),
		Draft: a.config, Icon: views.IconPreview(a.config.IconPNG), SavedURL: a.config.URL, SavedName: a.config.AppName, Problem: a.problem,
		IconSupported:          prefs.IconSupported,
		NotificationsSupported: mygo.NotificationsSupported(),
	}
	v.OnAction = func(action views.Action, config prefs.Config) { go a.runSettings(v, action, config) }
	win, err := newWindow(mygo.WindowOptions{
		Title: fmt.Sprintf(l10n.Text("settingsTitle"), a.config.DisplayName()), Width: views.SettingsWidth, Height: views.SettingsHeight,
		MinWidth: 420, MinHeight: 480, StateKey: "settings",
		TitleBarStyle:   mygo.TitleBarHidden,
		BackgroundColor: views.WindowBackground,
		Content:         ui.View(v.Render),
	}, a.config.DisplayName())
	a.settings, a.settingsUI = win, v
	v.Menu.Attach(win, func(x, y int) { go a.popupTitleMenu(win, x, y) })
	if err != nil {
		a.problem = err.Error()
		a.refreshSettingsLocked()
	}
	focus(win)
}

func (a *application) refreshSettingsLocked() {
	if a.settings == nil || a.settings.IsDestroyed() {
		return
	}
	v, problem, savedURL := a.settingsUI, a.problem, a.config.URL
	a.settings.Update(func() { v.Problem, v.SavedURL = problem, savedURL })
}

func (a *application) openSiteLocked() {
	if !a.started || a.restarting {
		return
	}
	if err := a.openSiteAtLocked(a.config.URL); err != nil {
		a.problem = err.Error()
		a.showSettingsLocked()
	}
}

func (a *application) openSiteAtLocked(raw string) error {
	u, err := website.TargetURL(a.config.URL, a.config.AllowHTTP)
	if err != nil {
		return err
	}
	if _, err := website.NavigationURL(raw, a.config.AllowHTTP); err != nil {
		return err
	}
	configuredOrigin := website.Origin(u)
	allowHTTP := a.config.AllowHTTP
	permissions := a.config
	previous := a.main
	v := &views.Browser{Menu: views.NewMenu(a.config)}
	win, err := newWindow(mygo.WindowOptions{
		Title: a.config.DisplayName() + " · " + u.Hostname(), Width: 1180, Height: 800,
		MinWidth: 420, MinHeight: 400, StateKey: "main",
		TitleBarStyle:   mygo.TitleBarHidden,
		BackgroundColor: views.WindowBackground,
		Content:         ui.View(v.Render),
	}, a.config.DisplayName())
	if err != nil {
		win.Destroy()
		return err
	}
	v.Menu.Attach(win, func(x, y int) { go a.popupTitleMenu(win, x, y) })
	web, err := win.NewWebView(mygo.WebViewOptions{
		BackgroundColor: views.WindowBackground,
		Page: mygo.PageOptions{
			TrustedOrigins: []string{configuredOrigin},
			PreloadScript:  website.Preload(configuredOrigin, permissions.AllowNotifications),
		},
	})
	if err != nil {
		win.Destroy()
		return err
	}
	a.main, a.mainWeb = win, web
	win.Update(func() { v.Web = web })
	page := web.Page()
	v.FollowLoading(win, page)
	v.FollowBackground(win, page)
	page.SetPermissionHandler(func(req mygo.PermissionRequest) bool {
		return prefs.AllowsPermissions(req.Origin, configuredOrigin, permissions, req.Permissions)
	})
	page.OnWillNavigate(func(e *mygo.NavigateEvent) {
		// Navigation grants no Go/device access; allow login redirects and callbacks containing code parameters.
		if _, err := website.NavigationURL(e.URL, allowHTTP); err != nil {
			e.PreventDefault()
			go a.navigationBlocked(win, err.Error())
			return
		}
	})
	page.SetWindowOpenHandler(func(req mygo.WindowOpenRequest) *mygo.WindowOptions {
		// Same-site popups reuse the main window; cross-site popups open in the browser without inheriting Go permissions.
		if website.SameOrigin(req.URL, configuredOrigin) {
			go func() {
				if err := page.LoadURL(req.URL); err != nil {
					a.report(l10n.Text("linkOpenFailed"))
				}
			}()
		} else {
			go a.openExternal(req.URL)
		}
		return nil
	})
	page.OnDidFailLoad(func(e *mygo.LoadError) {
		// WKWebView error -999 means cancelled navigation, such as a user reload, rather than a load failure.
		if e.Code != -999 {
			go a.loadFailed(win)
		}
	})
	if previous != nil && !previous.IsDestroyed() {
		previous.Destroy()
	}
	a.problem = ""
	if err := page.LoadURL(raw); err != nil {
		return err
	}
	focus(win)
	return nil
}

func (a *application) loadFailed(win *mygo.Window) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.main != win || win.IsDestroyed() {
		return
	}
	a.problem = l10n.Text("pageLoadFailed")
	a.showSettingsLocked()
}

func (a *application) navigationBlocked(win *mygo.Window, reason string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.main != win || win.IsDestroyed() {
		return
	}
	a.problem = fmt.Sprintf(l10n.Text("navigationBlocked"), reason)
	a.showSettingsLocked()
}

func (a *application) openExternal(raw string) {
	u, err := website.ParseURL(raw)
	if err != nil {
		a.report(l10n.Text("externalBlocked"))
		return
	}
	if err := mygo.Shell.OpenExternal(u.String()); err != nil {
		a.report(l10n.Text("externalFailed"))
	}
}
