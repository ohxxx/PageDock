// [INPUT]: The current MyGo locale, saved app name, native menus, and platform shortcuts.
// [OUTPUT]: Settings, About, website navigation, and browsing-data actions shared by system menus and the Windows title bar.
// [POS]: Application controls that remain available outside website content.
// [PROTOCOL]: Update this header and internal/AGENTS.md; update the root AGENTS.md when directory boundaries change.
package pagedock

import (
	"fmt"
	"runtime"

	"github.com/egoist/mygo"
	l10n "github.com/ohxxx/PageDock/internal/locale"
	views "github.com/ohxxx/PageDock/internal/view"
	"github.com/ohxxx/PageDock/internal/view/windows"
)

const browserZoomStep = 1.2

func (a *application) menu() *mygo.Menu {
	return mygo.NewMenu(a.menuItems())
}

func (a *application) menuItems() []*mygo.MenuItem {
	name := a.config.DisplayName()
	about := &mygo.MenuItem{Role: mygo.RoleAbout, Label: fmt.Sprintf(l10n.Text("aboutApp"), name)}
	if runtime.GOOS == "windows" {
		windows.ConfigureAbout(about, name, a.report)
	}
	return []*mygo.MenuItem{
		{Label: name, Submenu: []*mygo.MenuItem{
			about,
			{Label: l10n.Text("settingsMenu"), Accelerator: "CmdOrCtrl+,", Click: func(*mygo.MenuItem, *mygo.Window) { go a.showSettings() }},
			mygo.Separator(),
			{Role: mygo.RoleHide, Label: fmt.Sprintf(l10n.Text("hideApp"), name)},
			{Role: mygo.RoleQuit, Label: fmt.Sprintf(l10n.Text("quitApp"), name)},
		}},
		{Role: mygo.RoleEditMenu, Label: l10n.Text("editMenu")},
		{Label: l10n.Text("websiteMenu"), Submenu: []*mygo.MenuItem{
			{Label: l10n.Text("homeMenu"), Accelerator: "CmdOrCtrl+Shift+H", Click: func(*mygo.MenuItem, *mygo.Window) { go a.home() }},
			{Label: l10n.Text("reload"), Accelerator: "CmdOrCtrl+R", Click: a.pageAction(func(p *mygo.Page) { p.Reload() })},
			{Label: l10n.Text("back"), Accelerator: "Alt+Left", Click: func(*mygo.MenuItem, *mygo.Window) { go a.history(false) }},
			{Label: l10n.Text("forward"), Accelerator: "Alt+Right", Click: func(*mygo.MenuItem, *mygo.Window) { go a.history(true) }},
			mygo.Separator(),
			{Label: l10n.Text("clearMenu"), Click: func(*mygo.MenuItem, *mygo.Window) {
				go func() {
					a.mu.Lock()
					defer a.mu.Unlock()
					if !a.started || a.restarting {
						return
					}
					a.showSettingsLocked()
					v := a.settingsUI
					a.settings.Update(func() { v.Begin(views.Clear) })
				}()
			}},
		}},
		{Label: l10n.Text("viewMenu"), Submenu: []*mygo.MenuItem{
			{Label: l10n.Text("forceReload"), Accelerator: "CmdOrCtrl+Shift+R", Click: a.pageAction(func(p *mygo.Page) { p.ReloadIgnoringCache() })},
			{Label: l10n.Text("devTools"), Accelerator: "CmdOrCtrl+Alt+I", Click: a.pageAction(func(p *mygo.Page) { p.ToggleDevTools() })},
			mygo.Separator(),
			{Label: l10n.Text("actualSize"), Accelerator: "CmdOrCtrl+0", Click: a.pageAction(func(p *mygo.Page) { p.SetZoomFactor(1) })},
			{Label: l10n.Text("zoomIn"), Accelerator: "CmdOrCtrl+Plus", Click: a.pageAction(func(p *mygo.Page) { p.SetZoomFactor(p.ZoomFactor() * browserZoomStep) })},
			{Label: l10n.Text("zoomOut"), Accelerator: "CmdOrCtrl+-", Click: a.pageAction(func(p *mygo.Page) { p.SetZoomFactor(p.ZoomFactor() / browserZoomStep) })},
			mygo.Separator(),
			{Role: mygo.RoleToggleFullScreen, Label: l10n.Text("fullScreen")},
		}},
		{Role: mygo.RoleWindowMenu, Label: l10n.Text("windowMenu")},
	}
}

func (a *application) home() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.openSiteLocked()
}

func (a *application) history(forward bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.main == nil || a.main.IsDestroyed() || a.mainWeb == nil {
		return
	}
	if forward {
		a.mainWeb.Page().GoForward()
	} else {
		a.mainWeb.Page().GoBack()
	}
}

// An embedded WebView has no Window.Page; page menu actions must target the current website explicitly.
func (a *application) pageAction(action func(*mygo.Page)) func(*mygo.MenuItem, *mygo.Window) {
	return func(*mygo.MenuItem, *mygo.Window) {
		go func() {
			a.mu.Lock()
			defer a.mu.Unlock()
			if a.main != nil && !a.main.IsDestroyed() && a.mainWeb != nil {
				action(a.mainWeb.Page())
			}
		}()
	}
}

func (a *application) popupTitleMenu(win *mygo.Window, x, y int) {
	a.mu.Lock()
	if !a.started || a.restarting || win.IsDestroyed() {
		a.mu.Unlock()
		return
	}
	items := a.menuItems()
	items = append(append(items[0].Submenu, mygo.Separator()), items[1:]...)
	menu := mygo.NewMenu(items)
	a.mu.Unlock()
	menu.PopupAt(win, x, y)
}
