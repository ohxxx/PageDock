// [INPUT]: The current MyGo locale, package metadata, and optional PAGEDOCK_DATA_DIR.
// [OUTPUT]: A single-instance app, browser/settings and notification-record reset, default appearance restoration, and notification click activation.
// [POS]: internal startup orchestration called by root main.go; MyGo runs on the main thread and settings actions serialize in the background.
// [PROTOCOL]: Update this header and internal/AGENTS.md; update the root AGENTS.md when directory boundaries change.
package pagedock

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
	"github.com/ohxxx/PageDock/internal/appearance"
	prefs "github.com/ohxxx/PageDock/internal/config"
	l10n "github.com/ohxxx/PageDock/internal/locale"
)

// Run initializes and runs the desktop app on the process main thread.
func Run() {
	// A fixed internal name keeps user-data paths and the single-instance lock stable across display-name changes.
	mygo.App.SetName(appearance.InternalName)
	if dir := os.Getenv("PAGEDOCK_DATA_DIR"); dir != "" {
		abs, err := filepath.Abs(dir)
		if err != nil {
			log.Fatal(err)
		}
		mygo.App.SetPath(mygo.PathUserData, abs)
	}
	if !mygo.App.RequestSingleInstanceLock() {
		return
	}
	a := &application{}
	mygo.App.OnLocaleChanged(func(string) { go a.refreshLanguage() })
	mygo.Bind(&PWA{app: a})
	mygo.App.SetMenu(a.menu())
	mygo.App.OnSecondInstance(func(_ []string, _ string) { go a.activate() })
	mygo.App.OnActivate(func(_ bool) { go a.activate() })
	mygo.App.OnNotificationClick(func(id string) {
		if strings.HasPrefix(id, testNotificationID+":") || strings.HasPrefix(id, pwaNotificationIDPrefix) {
			go a.activate()
		}
	})
	mygo.App.WhenReady(func() {
		go func() {
			dir, err := mygo.App.Path(mygo.PathUserData)
			if err != nil {
				log.Fatal(err)
			}
			a.mu.Lock()
			a.store = prefs.Store{Path: filepath.Join(dir, prefs.Filename)}
			a.mu.Unlock()
			// Finish before the first window reads MyGo window state; the old process rewrites that file when exiting.
			appearanceChanged := false
			reset := false
			err = a.store.CompleteBrowsingReset(prefs.ClearBrowsingData)
			if err == nil {
				reset, err = a.store.CompleteReset(func() error {
					var appearanceErr error
					appearanceChanged, appearanceErr = applyAppearance(prefs.Config{}, func() error { return nil })
					if appearanceErr == nil {
						applyLanguage("")
					}
					return appearanceErr
				})
			}
			if err != nil {
				_, _ = mygo.Dialog.Message(mygo.MessageOptions{
					Type: mygo.MessageError, Title: l10n.Text("resetFailedTitle"), Message: err.Error(),
					Detail:  l10n.Text("resetFailedDetail"),
					Buttons: []string{l10n.Text("quit")}, DefaultButton: 0, CancelButton: 0,
				})
				mygo.App.Quit()
				return
			}
			if reset {
				appNotifications.clear()
				mygo.ClearNotifications()
				if appearanceChanged {
					relaunch(dir)
					return
				}
			}
			a.start()
		}()
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
