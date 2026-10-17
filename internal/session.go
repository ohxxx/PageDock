// [INPUT]: Saved config, app activation events, and platform appearance preparation.
// [OUTPUT]: Single-process state, startup recovery, window activation, and shared error presentation.
// [POS]: Desktop lifecycle/state owner; background work holds the app lock, window.go orchestrates windows, and view/ renders them.
// [PROTOCOL]: Update this header, internal/AGENTS.md, tests/AGENTS.md, and related behavior tests.
package pagedock

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/egoist/mygo"
	prefs "github.com/ohxxx/PageDock/internal/config"
	views "github.com/ohxxx/PageDock/internal/view"
)

type application struct {
	mu         sync.Mutex
	store      prefs.Store
	config     prefs.Config
	problem    string
	started    bool
	restarting bool
	main       *mygo.Window
	mainWeb    *mygo.WebView
	settings   *mygo.Window
	settingsUI *views.Settings
}

func (a *application) start() {
	a.mu.Lock()
	defer a.mu.Unlock()
	config, err := a.store.Load()
	if err == nil && os.Getenv("MYGO_LOCALE") == "" {
		applyLanguage(config.Language)
	}
	if err == nil {
		var changed bool
		changed, err = applyAppearance(config, func() error { return nil })
		if changed {
			a.restarting = true
			relaunch(filepath.Dir(a.store.Path))
			return
		}
	}
	if err == nil {
		err = prepareTaskbar(config.DisplayName())
	}
	a.config, a.started = config, true
	mygo.App.SetMenu(a.menu())
	if err != nil {
		a.problem = err.Error()
	}
	if err != nil || a.config.URL == "" {
		a.showSettingsLocked()
		return
	}
	a.openSiteLocked()
}

func (a *application) activate() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.started || a.restarting {
		return
	}
	if a.main != nil && !a.main.IsDestroyed() {
		focus(a.main)
	} else if a.config.URL != "" {
		a.openSiteLocked()
	} else {
		a.showSettingsLocked()
	}
}

func focus(win *mygo.Window) {
	win.Show()
	win.Restore()
	// Windows app activation may select any visible window; focus the requested window last.
	mygo.App.Focus()
	win.Focus()
}

func (a *application) report(message string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.problem = message
	a.showSettingsLocked()
}
