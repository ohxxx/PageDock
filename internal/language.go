// [INPUT]: The locale module, saved language choice, and MyGo system locale events.
// [OUTPUT]: Main-thread locale application and menu/window title refreshes.
// [POS]: Background language orchestration; locale/ registers catalogs and config/ owns persistence.
// [PROTOCOL]: Update this header, locale/, internal/AGENTS.md, and language behavior tests.
package pagedock

import (
	"fmt"

	"github.com/egoist/mygo"
	l10n "github.com/ohxxx/PageDock/internal/locale"
	website "github.com/ohxxx/PageDock/internal/site"
)

// Background callers only; main-thread callbacks must not take application.mu and deadlock save/startup with locale events.
func applyLanguage(language string) {
	done := make(chan struct{})
	mygo.App.WhenReady(func() {
		mygo.App.SetLocale(language)
		close(done)
	})
	<-done
}

func (a *application) refreshLanguage() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.started || a.restarting {
		return
	}
	mygo.App.SetMenu(a.menu())
	if a.settings != nil && !a.settings.IsDestroyed() {
		a.settings.SetTitle(fmt.Sprintf(l10n.Text("settingsTitle"), a.config.DisplayName()))
	}
	if a.main != nil && !a.main.IsDestroyed() {
		if u, err := website.ParseURL(a.config.URL); err == nil {
			a.main.SetTitle(a.config.DisplayName() + " · " + u.Hostname())
		}
	}
}
