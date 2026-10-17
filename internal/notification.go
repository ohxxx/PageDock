// [INPUT]: The current MyGo locale and notification title, body, tag, and request ID submitted by the exact saved origin.
// [OUTPUT]: MyGo native notifications, close operations, and click events routed only to their original page.
// [POS]: A narrow notification service with no file, command, or configuration access.
// [PROTOCOL]: Update this header and internal/AGENTS.md; update the root AGENTS.md when directory boundaries change.
package pagedock

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/egoist/mygo"
	l10n "github.com/ohxxx/PageDock/internal/locale"
	website "github.com/ohxxx/PageDock/internal/site"
)

const (
	pwaNotificationIDPrefix = "pwa:"
	maxNotificationTitle    = 180
	maxNotificationBody     = 4000
	maxNotificationTag      = 1024
	maxNotificationKey      = 128
	pwaPermissionDenied     = "pagedock:notifications-denied"
)

var pwaNotificationClick = mygo.NewEvent[string]("pagedock:notification-click")

type WebNotification struct {
	Title  string `json:"title"`
	Body   string `json:"body"`
	Tag    string `json:"tag"`
	Key    string `json:"key"`
	Silent bool   `json:"silent"`
}

// PWA only receives/closes saved-site notifications; data and onclick stay in the page and are never persisted.
type PWA struct{ app *application }

func (p *PWA) source(ctx context.Context) (*mygo.Page, string, error) {
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}
	caller, page := mygo.CallerWindow(ctx), mygo.CallerPage(ctx)
	if caller == nil || page == nil {
		return nil, "", errors.New(l10n.Text("notificationPageOnly"))
	}
	p.app.mu.Lock()
	defer p.app.mu.Unlock()
	if caller != p.app.main || caller.IsDestroyed() || p.app.mainWeb == nil || page != p.app.mainWeb.Page() || !p.app.config.AllowNotifications {
		return nil, "", errors.New(l10n.Text("notificationDisabled"))
	}
	u, err := website.NavigationURL(page.URL(), p.app.config.AllowHTTP)
	if err != nil {
		return nil, "", errors.New(l10n.Text("notificationOriginInvalid"))
	}
	saved, err := website.TargetURL(p.app.config.URL, p.app.config.AllowHTTP)
	if err != nil || website.Origin(u) != website.Origin(saved) {
		return nil, "", errors.New(l10n.Text("notificationOriginMismatch"))
	}
	return page, website.Origin(saved), nil
}

func (p *PWA) Receive(ctx context.Context, event WebNotification) (string, error) {
	page, source, err := p.source(ctx)
	if err != nil {
		return "", err
	}
	if len(event.Tag) > maxNotificationTag || len(event.Key) > maxNotificationKey {
		return "", errors.New(l10n.Text("notificationIDLong"))
	}
	title := cleanNotificationText(event.Title, maxNotificationTitle)
	if title == "" {
		return "", errors.New(l10n.Text("notificationTitleEmpty"))
	}
	id := pwaNotificationID(source, event.Tag)
	key := event.Key
	err = showPWANotification(mygo.NotificationOptions{
		ID: id, Group: pwaOriginPrefix(source), Title: title,
		Body: cleanNotificationText(event.Body, maxNotificationBody), Silent: event.Silent,
	}, func() {
		// MyGo cancels the call context on navigation/destruction; never deliver old-page clicks to a new page.
		if ctx.Err() == nil {
			_ = pwaNotificationClick.EmitPage(page, key)
		}
	})
	if errors.Is(err, mygo.ErrNotificationsDenied) {
		return "", errors.New(pwaPermissionDenied)
	}
	if err != nil {
		return "", err
	}
	return id, nil
}

func (p *PWA) Close(ctx context.Context, id string) error {
	_, source, err := p.source(ctx)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(id, pwaOriginPrefix(source)) || len(id) > 200 {
		return errors.New(l10n.Text("notificationCloseForeign"))
	}
	closePWANotification(id)
	return nil
}

func pwaOriginPrefix(source string) string {
	return fmt.Sprintf("%s%x:", pwaNotificationIDPrefix, sha256.Sum256([]byte(source)))
}

func pwaNotificationID(source, tag string) string {
	prefix := pwaOriginPrefix(source)
	if tag != "" {
		return fmt.Sprintf("%stag:%x", prefix, sha256.Sum256([]byte(tag)))
	}
	return prefix + "message:" + rand.Text()
}

func cleanNotificationText(raw string, limit int) string {
	var b strings.Builder
	count := 0
	for _, r := range raw {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			continue
		}
		if count >= limit {
			break
		}
		b.WriteRune(r)
		count++
	}
	return strings.TrimSpace(b.String())
}

var showPWANotification = func(opts mygo.NotificationOptions, onClick func()) error {
	n := mygo.NewNotification(opts)
	n.OnClick(func() { n.Close(); onClick() })
	return n.Show()
}

var closePWANotification = func(id string) {
	mygo.NewNotification(mygo.NotificationOptions{ID: id}).Close()
}
