// [INPUT]: The current MyGo locale, default logo, shared settings draft, platform capabilities, and native control events.
// [OUTPUT]: App name, website, icon, permissions, and data actions with essential hints and shared save/confirmation flows.
// [POS]: Native settings content using system inputs, switches, images, and buttons.
// [PROTOCOL]: Update this header, internal/view/AGENTS.md, and settings behavior tests.
package view

import (
	"fmt"
	"runtime"
	"time"

	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
	"github.com/ohxxx/PageDock/internal/brand"
	prefs "github.com/ohxxx/PageDock/internal/config"
	l10n "github.com/ohxxx/PageDock/internal/locale"
	website "github.com/ohxxx/PageDock/internal/site"
)

func (v *Settings) generalSection(c *ui.Context) {
	t := c.Theme()
	nameHelp := l10n.Text("nameHelp")
	if runtime.GOOS == "darwin" {
		nameHelp = l10n.Text("nameHelpMac")
	}
	ui.Column(c).Gap(24).Children(func() {
		ui.Field(c, l10n.Text("appNameLabel"), func() {
			if ui.TextInput(c.Key("appName"), &v.Draft.AppName).
				Placeholder(l10n.Text("appName")).Padding(12).Background(t.Background).Disabled(v.Busy).Submitted() {
				v.Begin(Save)
			}
		}).Description(nameHelp)
		ui.Field(c, l10n.Text("websiteLabel"), func() {
			input := ui.TextInput(c.Key("url"), &v.Draft.URL).
				Placeholder("https://example.com").Padding(12).Background(t.Background).Disabled(v.Busy).AutoFocus()
			if input.Submitted() {
				v.Begin(Save)
			}
		})
		ui.Divider(c)
		settingsToggle(c.Key("http"), l10n.Text("allowHTTP"), l10n.Text("httpHelp"), &v.Draft.AllowHTTP, v.Busy)
		ui.Divider(c)
		ui.Field(c, l10n.Text("language"), func() {
			ui.RadioGroup(c, func() {
				ui.Radio(c, &v.Draft.Language, "", l10n.Text("languageSystem")).Disabled(v.Busy)
				for _, tag := range l10n.Catalog.Languages() {
					ui.Radio(c, &v.Draft.Language, tag, i18n.Autonym(tag)).Disabled(v.Busy)
				}
			}).Row().Wrap().Gap(12)
		}).Description(l10n.Text("languageHelp"))
	})
}

func (v *Settings) iconSection(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Gap(20).Children(func() {
		ui.Text(c, l10n.Text("appIcon")).FontWeight(600)
		ui.Row(c).Gap(20).AlignItems(ui.Center).Children(func() {
			icon := v.Icon
			if icon == nil {
				icon = brand.Icon
			}
			ui.Image(c, icon).Size(80, 80).Fit(ui.Contain).Radius(16).Shrink(0)
			ui.Column(c).Gap(8).Grow(1).MinWidth(0).Children(func() {
				label := l10n.Text("iconDefault")
				if v.Icon != nil {
					label = l10n.Text("iconCustom")
				}
				ui.Text(c, label).FontWeight(600)
				ui.Text(c, l10n.Text("iconHint")).TextColor(t.TextMuted)
			})
		})
		ui.Row(c).Gap(10).Wrap().Children(func() {
			if ui.Button(c, l10n.Text("pickIcon")).Padding(9, 14).Disabled(v.Busy || !v.IconSupported).Clicked() {
				v.Begin(PickIcon)
			}
			if ui.Button(c, l10n.Text("restoreIcon")).Padding(9, 14).Disabled(v.Busy || v.Draft.IconPNG == "").Clicked() {
				v.Draft.IconPNG, v.Icon = "", nil
				v.Status, v.Failed = l10n.Text("restoreIconPending"), false
			}
		})
		ui.Divider(c)
		if v.IconSupported {
			help := l10n.Text("iconHelpMac")
			if runtime.GOOS == "windows" {
				help = l10n.Text("iconHelpWindows")
			}
			ui.Text(c, help).TextColor(t.TextMuted)
		} else {
			ui.Text(c, l10n.Text("iconUnsupported")).TextColor(t.TextMuted)
		}
	})
}

func (v *Settings) permissionsSection(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Gap(18).Children(func() {
		ui.Text(c, l10n.Text("browserPermissions")).FontWeight(600)
		if u, err := website.ParseURL(v.Draft.URL); err == nil {
			ui.Text(c, fmt.Sprintf(l10n.Text("originLabel"), website.Origin(u))).TextColor(t.TextMuted).Selectable()
		} else {
			ui.Text(c, l10n.Text("websiteFirst")).TextColor(t.TextMuted)
		}
		settingsToggle(c.Key("notifications"), l10n.Text("allowNotifications"), "", &v.Draft.AllowNotifications, v.Busy)
		ui.Divider(c)
		settingsToggle(c.Key("camera"), l10n.Text("allowCamera"), "", &v.Draft.AllowCamera, v.Busy)
		ui.Divider(c)
		settingsToggle(c.Key("microphone"), l10n.Text("allowMicrophone"), "", &v.Draft.AllowMicrophone, v.Busy)
		ui.Divider(c)
		location := ""
		if !prefs.GeolocationSupported {
			location = l10n.Text("platformUnsupported")
		}
		settingsToggle(c.Key("geolocation"), l10n.Text("allowGeolocation"), location, &v.Draft.AllowGeolocation, v.Busy || !prefs.GeolocationSupported)
		ui.Divider(c)
		ui.Text(c, l10n.Text("systemNotifications")).FontWeight(600)
		if !v.NotificationsSupported {
			ui.Text(c, l10n.Text("notificationsHelp")).TextColor(t.TextMuted)
		}
		if ui.Button(c, l10n.Text("sendNotification")).Padding(9, 14).AlignSelf(ui.Start).Disabled(v.Busy || !v.NotificationsSupported).Clicked() {
			v.Begin(Notify)
		}
	})
}

func settingsToggle(c *ui.Context, label, description string, value *bool, disabled bool) {
	t := c.Theme()
	row := ui.SwitchBase(c, value).Label(label).Description(description).
		Disabled(disabled).FillWidth().Gap(20).AlignItems(ui.Center).Radius(t.Radius)
	target := float32(0)
	if *value {
		target = 1
	}
	position := row.Animate("knob", target, 140*time.Millisecond)
	row.Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(5).Children(func() {
			ui.Text(c, label).FontWeight(500)
			if description != "" {
				ui.Text(c, description).TextColor(t.TextMuted).FontSize(t.Rem(0.92))
			}
		})
		ui.Box(c).Size(36, 20).Shrink(0).Draw(func(p *ui.Painter, r ui.Rect) {
			p.Fill(r, t.Border.Mix(t.Accent, position), 10)
			p.Fill(ui.Rect{X: r.X + 2 + 16*position, Y: r.Y + 2, W: 16, H: 16}, t.Background.Mix(t.AccentText, position), 8)
		})
	})
}

func (v *Settings) dataSection(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Gap(16).Children(func() {
		ui.Text(c, l10n.Text("signInCache")).FontWeight(600)
		ui.Text(c, l10n.Text("clearHelp")).TextColor(t.TextMuted)
		if ui.Button(c, l10n.Text("clearMenu")).Padding(9, 14).AlignSelf(ui.Start).Disabled(v.Busy).Clicked() {
			v.Begin(Clear)
		}
		ui.Divider(c)
		ui.Text(c, l10n.Text("resetApp")).FontWeight(600)
		ui.Text(c, l10n.Text("resetHelp")).TextColor(t.TextMuted)
		if ui.Button(c, l10n.Text("resetMenu")).Padding(9, 14).TextColor(t.Danger).AlignSelf(ui.Start).Disabled(v.Busy).Clicked() {
			v.Begin(Reset)
		}
	})
}
