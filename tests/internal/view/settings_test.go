// [INPUT]: The current locale, native view input/click/key events, and window size changes.
// [OUTPUT]: Seamless title bars, name/section drafts, permission saves, icon capability, keyboard access, and narrow light/dark layout tests.
// [POS]: MyGo ui.Tester drives real controls without source-string or file-layout assertions.
// [PROTOCOL]: Update this header, internal/view/AGENTS.md, and tests/AGENTS.md; update the root AGENTS.md when directory boundaries change.
package view

import (
	"image/color"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
	prefs "github.com/ohxxx/PageDock/internal/config"
	l10n "github.com/ohxxx/PageDock/internal/locale"
)

func clickNative(t *testing.T, tt *ui.Tester, label string) {
	t.Helper()
	if err := tt.Click(label); err != nil {
		t.Fatal(err)
	}
}

func TestNativeSettingsSave(t *testing.T) {
	var actions []Action
	var saved prefs.Config
	v := &Settings{OnAction: func(action Action, config prefs.Config) {
		actions = append(actions, action)
		saved = config
	}}
	tt := ui.NewTester(v.Render, 640, 900)
	clickNative(t, tt, "应用名称")
	tt.Type("团队工作台")
	clickNative(t, tt, "网站地址")
	tt.Type("http://localhost:8080")
	clickNative(t, tt, "保存并应用")
	if len(actions) != 0 || !v.Failed || !tt.HasText("HTTPS") || v.Busy {
		t.Fatalf("invalid URL was not rejected: %+v, %q", actions, v.Status)
	}
	clickNative(t, tt, "允许 HTTP")
	clickNative(t, tt, "权限")
	clickNative(t, tt, "启用网站通知")
	clickNative(t, tt, "允许相机")
	clickNative(t, tt, "允许麦克风")
	clickNative(t, tt, "允许定位")
	clickNative(t, tt, "保存并应用")
	want := prefs.Config{AppName: "团队工作台", URL: "http://localhost:8080/", AllowHTTP: true, AllowNotifications: true, AllowCamera: true, AllowMicrophone: true, AllowGeolocation: prefs.GeolocationSupported}
	if len(actions) != 1 || actions[0] != Save || saved != want || !v.Busy {
		t.Fatalf("native controls did not submit edited config: %v, %+v", actions, saved)
	}
	clickNative(t, tt, "保存并应用")
	clickNative(t, tt, "常规")
	clickNative(t, tt, "允许 HTTP")
	clickNative(t, tt, "权限")
	clickNative(t, tt, "允许相机")
	clickNative(t, tt, "允许麦克风")
	if len(actions) != 1 || !v.Draft.AllowHTTP || !v.Draft.AllowCamera || !v.Draft.AllowMicrophone {
		t.Fatal("busy controls accepted duplicate action or changed the draft")
	}
}

func TestNativeSettingsEnterAndDisabledActions(t *testing.T) {
	var actions []Action
	v := &Settings{OnAction: func(action Action, _ prefs.Config) { actions = append(actions, action) }}
	tt := ui.NewTester(v.Render, 640, 1000)
	clickNative(t, tt, "打开已保存网站")
	clickNative(t, tt, "权限")
	clickNative(t, tt, "发送测试通知")
	if len(actions) != 0 {
		t.Fatal("unavailable actions were invoked")
	}
	clickNative(t, tt, "常规")
	clickNative(t, tt, "网站地址")
	tt.Type("https://EXAMPLE.com")
	tt.Key(0, ui.KeyEnter)
	if len(actions) != 1 || actions[0] != Save {
		t.Fatalf("Enter did not save: %v", actions)
	}
	v.Busy, v.SavedURL, v.NotificationsSupported = false, "https://example.com/", true
	tt.Frame()
	clickNative(t, tt, "权限")
	clickNative(t, tt, "发送测试通知")
	if len(actions) != 2 || actions[1] != Notify {
		t.Fatalf("notification action: %v", actions)
	}
	if v.Draft.URL != "https://EXAMPLE.com" {
		t.Fatal("notification changed the unsaved draft")
	}
}

func TestNativeSettingsSmallWindowScroll(t *testing.T) {
	var actions []Action
	v := &Settings{OnAction: func(action Action, _ prefs.Config) { actions = append(actions, action) }}
	tt := ui.NewTester(v.Render, 420, 480)
	tt.SetDark(true)
	for _, label := range []string{"网站地址", "保存并应用", "打开已保存网站"} {
		r, ok := tt.Find(label)
		if !ok || r.W <= 0 || r.X < 0 || r.X+r.W > 420 {
			t.Fatalf("%s overflows: %+v", label, r)
		}
	}
	clickNative(t, tt, "数据")
	tt.Scroll(200, 240, 0, 1000)
	r, ok := tt.Find("清除登录数据…")
	if !ok || r.Y < 0 || r.Y+r.H > 480 {
		t.Fatalf("clear action did not scroll into view: %+v", r)
	}
	clickNative(t, tt, "清除登录数据…")
	if len(actions) != 1 || actions[0] != Clear {
		t.Fatalf("clear action: %v", actions)
	}
	v.Busy = false
	tt.Frame()
	r, ok = tt.Find("清空全部数据…")
	if !ok || r.Y < 0 || r.Y+r.H > 480 {
		t.Fatalf("reset action did not scroll into view: %+v", r)
	}
	clickNative(t, tt, "清空全部数据…")
	clickNative(t, tt, "清空全部数据…")
	if len(actions) != 2 || actions[1] != Reset || !v.Busy {
		t.Fatalf("reset action or duplicate submission: %v", actions)
	}
}

func TestNativeIconDraftAndReset(t *testing.T) {
	original := testIcon(t, color.NRGBA{G: 255, A: 255})
	var action Action
	var saved prefs.Config
	v := &Settings{Draft: prefs.Config{URL: "https://example.com/", IconPNG: original}, Icon: IconPreview(original), IconSupported: true, OnAction: func(a Action, c prefs.Config) { action, saved = a, c }}
	tt := ui.NewTester(v.Render, 420, 680)
	clickNative(t, tt, "外观")
	clickNative(t, tt, "选择图标…")
	if action != PickIcon || saved.IconPNG != original || !v.Busy {
		t.Fatal("picker did not preserve draft")
	}
	clickNative(t, tt, "恢复默认图标")
	if v.Draft.IconPNG != original {
		t.Fatal("busy draft changed")
	}
	v.Busy = false
	tt.Frame()
	clickNative(t, tt, "外观")
	clickNative(t, tt, "恢复默认图标")
	if v.Draft.IconPNG != "" || v.Icon != nil || !tt.HasText("保存后恢复默认图标") {
		t.Fatal("default icon was not staged")
	}
	clickNative(t, tt, "保存并应用")
	if action != Save || saved.IconPNG != "" || saved.URL != "https://example.com/" {
		t.Fatal("default icon was not submitted with website")
	}
}

func TestNativeUnsupportedIconPreservesSettings(t *testing.T) {
	original := testIcon(t, color.NRGBA{B: 255, A: 255})
	store := prefs.Store{Path: filepath.Join(t.TempDir(), prefs.Filename)}
	var actions []Action
	v := &Settings{
		Draft: prefs.Config{URL: "https://example.com/", IconPNG: original}, Icon: IconPreview(original),
		OnAction: func(action Action, config prefs.Config) {
			actions = append(actions, action)
			if action == Save {
				if err := store.Save(config); err != nil {
					t.Fatal(err)
				}
			}
		},
	}
	tt := ui.NewTester(v.Render, 640, 1100)
	clickNative(t, tt, "外观")
	clickNative(t, tt, "选择图标…")
	if len(actions) != 0 || v.Busy {
		t.Fatal("unsupported icon picker was invoked")
	}
	if !tt.HasText("当前平台不支持自定义应用与通知图标") {
		t.Fatal("unsupported icon capability was not explained")
	}
	v.Begin(PickIcon)
	if len(actions) != 0 || v.Busy || !v.Failed {
		t.Fatal("unsupported icon action bypassed the UI guard")
	}
	clickNative(t, tt, "权限")
	clickNative(t, tt, "允许相机")
	clickNative(t, tt, "保存并应用")
	want := prefs.Config{URL: "https://example.com/", IconPNG: original, AllowCamera: true}
	if got, err := store.Load(); err != nil || got != want || len(actions) != 1 || actions[0] != Save {
		t.Fatalf("normal save lost imported icon or permission: %+v, %v, %v", got, err, actions)
	}
	v.Busy = false
	tt.Frame()
	clickNative(t, tt, "外观")
	clickNative(t, tt, "恢复默认图标")
	clickNative(t, tt, "保存并应用")
	want.IconPNG = ""
	if got, err := store.Load(); err != nil || got != want || len(actions) != 2 {
		t.Fatalf("unsupported platform could not remove stored icon: %+v, %v, %v", got, err, actions)
	}
}

func TestNativeSettingsNavigationKeepsDraft(t *testing.T) {
	var saved prefs.Config
	v := &Settings{OnAction: func(_ Action, c prefs.Config) { saved = c }}
	tt := ui.NewTester(v.Render, SettingsWidth, SettingsHeight)
	clickNative(t, tt, "网站地址")
	tt.Type("https://EXAMPLE.com")
	clickNative(t, tt, "外观")
	tt.Key(0, ui.KeyDown)
	if v.section != settingsPermissions || !tt.Focused("权限") {
		t.Fatal("keyboard did not move to permissions")
	}
	clickNative(t, tt, "允许相机")
	tt.Key(0, ui.KeySpace)
	if v.Draft.AllowCamera {
		t.Fatal("keyboard did not switch the permission off")
	}
	tt.Key(0, ui.KeySpace)
	tt.SetSize(420, 480)
	clickNative(t, tt, "常规")
	if v.Draft.URL != "https://EXAMPLE.com" || !v.Draft.AllowCamera {
		t.Fatal("navigation or compact layout lost the draft")
	}
	tt.SetSize(SettingsWidth, SettingsHeight)
	clickNative(t, tt, "保存并应用")
	if saved.URL != "https://example.com/" || !saved.AllowCamera {
		t.Fatalf("save lost a different section's edit: %+v", saved)
	}
}

func TestNativeSettingsInvalidDraftReturnsToGeneral(t *testing.T) {
	calls := 0
	v := &Settings{OnAction: func(Action, prefs.Config) { calls++ }}
	tt := ui.NewTester(v.Render, SettingsWidth, SettingsHeight)
	clickNative(t, tt, "权限")
	clickNative(t, tt, "允许相机")
	clickNative(t, tt, "保存并应用")
	if calls != 0 || v.Busy || !v.Failed || v.section != settingsGeneral || !tt.HasText("网站地址") || !v.Draft.AllowCamera {
		t.Fatalf("invalid draft did not return to the editable field: %+v", v)
	}
	clickNative(t, tt, "网站地址")
	tt.Type("https://example.com")
	tt.Key(0, ui.KeyEnter)
	if calls != 1 || !v.Busy {
		t.Fatal("corrected draft could not be submitted with Enter")
	}
}

func TestNativeSettingsAdaptiveLayout(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, size := range [][2]int{{420, 480}, {640, 680}, {860, 700}} {
			for _, scale := range []float32{1, 1.5} {
				v := &Settings{OnAction: func(Action, prefs.Config) {}}
				tt := ui.NewTester(v.Render, size[0], size[1])
				tt.SetTitleBar(ui.TitleBar{Height: 28, Left: 80})
				tt.SetDark(dark)
				tt.SetPreferences(ui.Preferences{TextScale: scale, HighContrast: true, ReduceMotion: true})
				for _, section := range settingsSections {
					clickNative(t, tt, l10n.Text(section.label))
					tt.Scroll(float32(size[0]-40), float32(size[1]/2), 0, 2000)
					for _, label := range []string{"常规", "外观", "权限", "数据", "保存并应用", "打开已保存网站"} {
						r, ok := tt.Find(label)
						if !ok || r.W <= 0 || r.H <= 0 || r.X < 0 || r.Y < 0 || r.X+r.W > float32(size[0]) || r.Y+r.H > float32(size[1]) {
							t.Fatalf("%s outside %v (dark=%v, scale=%v): %+v", label, size, dark, scale, r)
						}
					}
				}
			}
		}
	}
}

func TestNativeSettingsTitleBarBlendsWithContent(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, width := range []int{420, SettingsWidth} {
			v := &Settings{}
			tt := ui.NewTester(v.Render, width, SettingsHeight)
			tt.SetDark(dark)
			for _, height := range []float32{28, 40, 0} {
				tt.SetTitleBar(ui.TitleBar{Height: height, Left: 80})
				for _, label := range []string{"常规", "网站地址", "保存并应用"} {
					r, ok := tt.Find(label)
					if !ok || r.Y < height {
						t.Fatalf("%s overlaps window controls: %+v", label, r)
					}
				}
				if height == 0 {
					continue
				}
				img := tt.Image()
				for _, x := range []int{8, width - 8} {
					background := img.At(x, int(height)+1)
					for y := 8; y <= int(height); y++ {
						if img.At(x, y) != background {
							t.Fatalf("title bar has a seam at (%d,%d), width=%d dark=%v", x, y, width, dark)
						}
					}
				}
			}
		}
	}
}
