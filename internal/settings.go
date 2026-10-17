// [INPUT]: Native settings actions, drafts passed by value, and background operation results.
// [OUTPUT]: Serialized action dispatch, image selection, test notifications, and main-thread updates through Window.Update.
// [POS]: Settings and save/cleanup orchestration; config/ and appearance/ own storage and platform details.
// [PROTOCOL]: Update this header, internal/AGENTS.md, tests/AGENTS.md, and related behavior tests.
package pagedock

import (
	"crypto/rand"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"

	"github.com/egoist/mygo"
	prefs "github.com/ohxxx/PageDock/internal/config"
	l10n "github.com/ohxxx/PageDock/internal/locale"
	website "github.com/ohxxx/PageDock/internal/site"
	views "github.com/ohxxx/PageDock/internal/view"
)

const testNotificationID = "pagedock-notification-test"

// runSettings serializes background actions without reading or writing view fields from worker threads.
func (a *application) runSettings(v *views.Settings, action views.Action, config prefs.Config) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.restarting || a.settingsUI != v || a.settings == nil || a.settings.IsDestroyed() {
		return
	}
	var status string
	var err error
	var picked *string
	switch action {
	case views.Save:
		err = a.saveSettingsLocked(config)
		if a.restarting {
			return
		}
		status = l10n.Text("settingsSaved")
	case views.PickIcon:
		picked, err = a.chooseIconLocked()
		status = l10n.Text("pickerCancelled")
		if picked != nil {
			status = l10n.Text("iconPicked")
		}
	case views.Open:
		if a.config.URL == "" {
			err = errors.New(l10n.Text("saveWebsiteFirst"))
		} else {
			a.openSiteLocked()
		}
		status = l10n.Text("openingWebsite")
	case views.Notify:
		err = sendTestNotification(a.config.DisplayName())
		status = l10n.Text("notificationSubmitted")
	case views.Clear:
		var cleared bool
		cleared, err = a.clearDataLocked()
		if a.restarting {
			return
		}
		status = l10n.Text("clearCancelled")
		if cleared {
			status = l10n.Text("siteDataCleared")
		}
	case views.Reset:
		var reset bool
		reset, err = a.resetAllLocked()
		if reset {
			return // Keep controls disabled during restart and stop updating the closing window.
		}
		status = l10n.Text("resetCancelled")
	default:
		err = errors.New(l10n.Text("actionUnknown"))
	}
	if err != nil {
		status = err.Error()
	}
	saved, problem := a.config, a.problem
	a.settings.Update(func() {
		v.Busy, v.Status, v.Failed = false, status, err != nil
		v.SavedURL, v.Problem = saved.URL, problem
		if picked != nil {
			v.Draft.IconPNG = *picked
			v.Icon = views.IconPreview(*picked)
		}
		if action == views.Save && err == nil {
			v.Draft = saved
			v.Icon = views.IconPreview(saved.IconPNG)
		}
	})
}

func sendTestNotification(name string) error {
	if !mygo.NotificationsSupported() {
		return errors.New(l10n.Text("notificationsUnavailable"))
	}
	err := mygo.NewNotification(mygo.NotificationOptions{
		ID: testNotificationID + ":" + rand.Text(), Title: fmt.Sprintf(l10n.Text("notificationTitle"), name),
		Body: l10n.Text("notificationBody"),
	}).Show()
	if errors.Is(err, mygo.ErrNotificationsDenied) {
		return errors.New(l10n.Text("notificationsDenied"))
	}
	return err
}

func (a *application) chooseIconLocked() (*string, error) {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Parent: a.settings, Title: l10n.Text("pickIconTitle"), ButtonLabel: l10n.Text("choose"),
		Filters: []mygo.FileFilter{{Name: l10n.Text("images"), Extensions: []string{"png", "jpg", "jpeg"}}},
	})
	if err != nil || len(paths) == 0 {
		return nil, err
	}
	encoded, err := prefs.ReadIcon(paths[0])
	if err != nil {
		return nil, err
	}
	return &encoded, nil
}

func (a *application) saveSettingsLocked(config prefs.Config) error {
	next, err := config.Validate()
	if err != nil {
		return err
	}
	current := next.URL
	if next.URL == a.config.URL && a.main != nil && !a.main.IsDestroyed() && a.mainWeb != nil {
		// Keep the session URL for permission-only changes; return to an allowed home page when HTTP is disabled.
		if raw := a.mainWeb.Page().URL(); raw != "" {
			if _, err := website.NavigationURL(raw, next.AllowHTTP); err == nil {
				current = raw
			}
		}
	}
	changed, err := applyAppearance(next, func() error { return a.store.Save(next) })
	if err != nil {
		return err
	}
	nameChanged := next.AppName != a.config.AppName
	languageChanged := next.Language != a.config.Language
	a.config = next
	if languageChanged {
		applyLanguage(next.Language)
	}
	if changed || nameChanged {
		a.stopSiteLocked()
		a.restarting = true
		relaunch(filepath.Dir(a.store.Path))
		return nil
	}
	return a.openSiteAtLocked(current)
}

func (a *application) clearDataLocked() (bool, error) {
	detail := l10n.Text("clearDetail")
	if runtime.GOOS == "windows" {
		detail += l10n.Text("clearRestart")
	}
	result, err := mygo.Dialog.Message(mygo.MessageOptions{
		Parent: a.settings, Type: mygo.MessageWarning, Title: l10n.Text("clearTitle"),
		Message: l10n.Text("clearQuestion"),
		Detail:  detail,
		// MyGo's legacy Windows confirmation maps OK to the first choice and Cancel to the last.
		Buttons: []string{l10n.Text("clearButton"), l10n.Text("cancel")}, DefaultButton: 1, CancelButton: 1,
	})
	if err != nil || result.Button != 0 {
		return false, err
	}
	return a.clearConfirmedDataLocked(false)
}

// Stop the website before cleanup so scripts cannot keep writing cookies or local storage.
func (a *application) stopSiteLocked() {
	if a.main != nil && !a.main.IsDestroyed() {
		a.main.Destroy()
	}
	a.main, a.mainWeb = nil, nil
}

func (a *application) resetAllLocked() (bool, error) {
	result, err := mygo.Dialog.Message(mygo.MessageOptions{
		Parent: a.settings, Type: mygo.MessageWarning, Title: l10n.Text("resetTitle"),
		Message: l10n.Text("resetQuestion"),
		Detail:  l10n.Text("resetDetail"),
		Buttons: []string{l10n.Text("resetButton"), l10n.Text("cancel")}, DefaultButton: 1, CancelButton: 1,
	})
	if err != nil || result.Button != 0 {
		return false, err
	}
	return a.clearConfirmedDataLocked(true)
}

func (a *application) clearConfirmedDataLocked(all bool) (bool, error) {
	a.stopSiteLocked()
	var err error
	if runtime.GOOS == "windows" {
		err = a.store.DeferReset(all)
	} else if all {
		err = a.store.PrepareReset(mygo.App.ClearBrowsingData)
	} else {
		err = mygo.App.ClearBrowsingData()
	}
	if err != nil {
		return false, err
	}
	if all {
		mygo.ClearNotifications()
		a.config = prefs.Config{}
	}
	a.problem = ""
	if all || runtime.GOOS == "windows" {
		a.restarting = true
		mygo.App.Relaunch()
	}
	return true, nil
}
