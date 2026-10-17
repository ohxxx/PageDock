// [INPUT]: The current MyGo locale, native control events, config draft, caption area, window size, and background results.
// [OUTPUT]: A title bar blending into the sidebar/content, Windows branding menu, adaptive sections, draft, and fixed save bar.
// [POS]: View fields belong to the UI main thread; background operations receive config by value.
// [PROTOCOL]: Update this header, internal/view/AGENTS.md, and settings behavior tests.
package view

import (
	"github.com/egoist/mygo/ui"
	prefs "github.com/ohxxx/PageDock/internal/config"
	l10n "github.com/ohxxx/PageDock/internal/locale"
)

type Action uint8

const (
	Save Action = iota
	Open
	Notify
	Clear
	Reset
	PickIcon
)

const (
	settingsGeneral = iota
	settingsAppearance
	settingsPermissions
	settingsData
)

var settingsSections = [...]struct{ label, title string }{
	{"general", "generalTitle"}, {"appearance", "appearanceTitle"},
	{"permissions", "permissionsTitle"}, {"data", "dataTitle"},
}

type Settings struct {
	Menu                   *Menu
	Draft                  prefs.Config
	Icon                   *ui.Bitmap
	SavedURL               string
	SavedName              string
	Problem                string
	Status                 string
	Failed                 bool
	Busy                   bool
	section                int
	IconSupported          bool
	NotificationsSupported bool
	OnAction               func(Action, prefs.Config)
}

func (v *Settings) Begin(action Action) {
	if v.Busy {
		return
	}
	if action == PickIcon && !v.IconSupported {
		v.Status, v.Failed = l10n.Text("iconUnsupported"), true
		return
	}
	config := v.Draft
	if action == Save {
		var err error
		config, err = config.Validate()
		if err != nil {
			v.section = settingsGeneral
			v.Status, v.Failed = err.Error(), true
			return
		}
	}
	v.Busy, v.Failed, v.Status = true, false, l10n.Text("processing")
	v.OnAction(action, config)
}

func (v *Settings) Render(c *ui.Context) {
	applyTheme(c)
	t := c.Theme()
	width, _ := c.Size()
	compact := width < settingsCompactWidth*c.Preferences().TextScale
	ui.Column(c).Fill().Background(t.Background).Children(func() {
		if bar := c.TitleBar(); bar.Height > 0 {
			ui.Row(c).Height(bar.Height).Shrink(0).Background(t.Surface).DragWindow().Children(func() {
				if !compact {
					ui.Box(c).Width(settingsSidebarWidth)
					ui.Box(c).Grow(1).FillHeight().Background(t.Background)
				}
				v.Menu.Render(c, t.Surface)
			})
		}
		layout := ui.Row(c).Grow(1).MinHeight(0).AlignItems(ui.Stretch)
		if compact {
			layout.Column()
		}
		layout.Children(func() {
			v.navigation(c, compact)
			ui.Column(c.Key("content")).Grow(1).MinWidth(0).MinHeight(0).Children(func() {
				padding := float32(settingsPadding)
				if compact {
					padding = 20
				}
				ui.Scroll(c.Key(v.section)).Grow(1).Padding(padding).Gap(24).Children(func() {
					section := settingsSections[v.section]
					ui.Text(c, l10n.Text(section.title)).FontSize(t.Rem(1.7)).FontWeight(600)
					if v.Problem != "" {
						ui.Text(c.Key("problem"), v.Problem).TextColor(t.Danger).Selectable()
					}
					switch v.section {
					case settingsGeneral:
						v.generalSection(c)
					case settingsAppearance:
						v.iconSection(c)
					case settingsPermissions:
						v.permissionsSection(c)
					case settingsData:
						v.dataSection(c)
					}
				})
				v.footer(c, padding)
			})
		})
	})
}

func (v *Settings) navigation(c *ui.Context, compact bool) {
	t := c.Theme()
	nav := ui.Column(c).Background(t.Surface).Padding(16).Gap(28).Shrink(0)
	if !compact {
		nav.Width(settingsSidebarWidth)
	}
	nav.Children(func() {
		if !compact {
			ui.Column(c).Padding(10, 12).Gap(5).Children(func() {
				ui.Text(c, (prefs.Config{AppName: v.SavedName}).DisplayName()).FontSize(t.Rem(1.4)).FontWeight(600)
			})
		}
		tabs := ui.TabsBase(c, &v.section, len(settingsSections))
		if !compact {
			tabs.List.Column().AlignItems(ui.Stretch).FocusGroup(ui.Vertical)
		} else {
			tabs.List.Wrap()
		}
		tabs.List.Gap(4).Children(func() {
			for i, section := range settingsSections {
				tab := tabs.Tab(i).Padding(10, 14).Radius(t.Radius).TextColor(t.TextMuted)
				if compact {
					tab.Grow(1).Justify(ui.Center)
				}
				if i == v.section {
					tab.Background(t.SurfacePressed).TextColor(t.Text).FontWeight(600)
				} else if tab.Hovered() {
					tab.Background(t.SurfaceHover)
				}
				tab.Children(func() { ui.Text(c, l10n.Text(section.label)) })
			}
		})
	})
}

func (v *Settings) footer(c *ui.Context, padding float32) {
	t := c.Theme()
	ui.Column(c).Shrink(0).Padding(16, padding).Gap(12).
		Border(1, t.Border).BorderWidth(1, 0, 0, 0).Children(func() {
		if v.Status != "" {
			color := t.TextMuted
			if v.Failed {
				color = t.Danger
			}
			ui.Text(c.Key("status"), v.Status).TextColor(color).Selectable()
		}
		ui.Row(c).Gap(10).Wrap().Justify(ui.End).Children(func() {
			if ui.Button(c, l10n.Text("openSaved")).Padding(9, 14).Disabled(v.Busy || v.SavedURL == "").Clicked() {
				v.Begin(Open)
			}
			if ui.PrimaryButton(c, l10n.Text("saveApply")).Padding(9, 18).Disabled(v.Busy).Clicked() {
				v.Begin(Save)
			}
		})
	})
}
