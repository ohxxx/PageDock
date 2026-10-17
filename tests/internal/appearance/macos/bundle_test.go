//go:build darwin

// [INPUT]: Isolated temporary bundles, test icons, and failed config writes.
// [OUTPUT]: Atomic system name/icon updates, signing, cache/save failure rollback, idempotence, and default restoration.
// [POS]: macOS bundle-icon tests that never launch or register real user applications.
// [PROTOCOL]: Update this header, internal/appearance/macos/AGENTS.md, and tests/AGENTS.md.
package macos

import (
	"bytes"
	"errors"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/ohxxx/PageDock/internal/appearance"
	prefs "github.com/ohxxx/PageDock/internal/config"
)

func iconTestBundle(t *testing.T) (string, []byte) {
	t.Helper()
	bundle := filepath.Join(t.TempDir(), "Icon QA.app")
	contents := filepath.Join(bundle, "Contents")
	if err := os.MkdirAll(filepath.Join(contents, "MacOS"), 0o755); err != nil {
		t.Fatal(err)
	}
	executable, err := os.ReadFile("/usr/bin/true")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(contents, "MacOS", "IconQA"), executable, 0o755); err != nil {
		t.Fatal(err)
	}
	info := map[string]any{"CFBundleIdentifier": "com.ohxxx.pagedock.icon-test." + filepath.Base(filepath.Dir(filepath.Dir(bundle))), "CFBundleExecutable": "IconQA", "CFBundleName": "IconQA", "CFBundleDisplayName": "Icon QA", "CFBundlePackageType": "APPL", "CFBundleVersion": "1", "CFBundleIconFile": "Original.icns", "CFBundleIconName": "OriginalAsset"}
	if err = os.MkdirAll(filepath.Join(contents, "Resources"), 0o755); err != nil {
		t.Fatal(err)
	}
	originalPNG, err := prefs.DecodeIcon(testIcon(t, color.NRGBA{G: 255, A: 255}))
	if err != nil {
		t.Fatal(err)
	}
	originalICNS, err := appearance.EncodeICNS(originalPNG)
	if err != nil {
		t.Fatal(err)
	}
	originalPath := filepath.Join(contents, "Resources", "Original.icns")
	if err = os.WriteFile(originalPath, originalICNS, 0o644); err != nil {
		t.Fatal(err)
	}
	if err = writeBundleInfo(filepath.Join(contents, "Info.plist"), info); err != nil {
		t.Fatal(err)
	}
	if _, err = iconCommand(nil, "/usr/bin/codesign", "--force", "--sign", "-", bundle); err != nil {
		t.Fatal(err)
	}
	// Unregister only this test's temporary bundle during cleanup.
	t.Cleanup(func() { _, _ = iconCommand(nil, launchServicesRegister, "-u", bundle) })
	return bundle, originalICNS
}

func TestBundleIconUpdateRollbackAndRestore(t *testing.T) {
	bundle, originalICNS := iconTestBundle(t)
	contents := filepath.Join(bundle, "Contents")
	originalPath := filepath.Join(contents, "Resources", "Original.icns")
	refreshes := 0
	refresh := func() error { refreshes++; return nil }
	red := testIcon(t, color.NRGBA{R: 255, A: 255})
	saved := ""
	changed, err := updateBundleAppearance(bundle, prefs.Config{IconPNG: red}, func() error { saved = red; return nil }, refresh)
	if err != nil || !changed || saved != red {
		t.Fatalf("apply: %v %v", changed, err)
	}
	applied, err := readBundleInfo(filepath.Join(contents, "Info.plist"))
	if err != nil {
		t.Fatal(err)
	}
	name := applied["CFBundleIconFile"].(string)
	resource := filepath.Join(contents, "Resources", name)
	before, err := os.ReadFile(resource)
	if err != nil || !bytes.HasPrefix(before, []byte("icns")) {
		t.Fatalf("missing ICNS: %v", err)
	}
	if _, err = iconCommand(nil, "/usr/bin/codesign", "--verify", "--deep", "--strict", bundle); err != nil {
		t.Fatal(err)
	}
	changed, err = updateBundleAppearance(bundle, prefs.Config{IconPNG: red}, func() error { return nil }, refresh)
	if err != nil || changed || refreshes != 1 {
		t.Fatalf("unchanged icon rebuilt bundle or refreshed cache: %v %v refresh=%d", changed, err, refreshes)
	}

	cacheFailure := errors.New("icon cache unavailable")
	attempts, saveCalled := 0, false
	changed, err = updateBundleAppearance(bundle, prefs.Config{IconPNG: testIcon(t, color.NRGBA{B: 255, A: 255})}, func() error {
		saveCalled = true
		return nil
	}, func() error {
		attempts++
		if attempts == 1 {
			return cacheFailure
		}
		rolledBack, readErr := os.ReadFile(resource)
		if readErr != nil || !bytes.Equal(before, rolledBack) {
			t.Fatal("cache refreshed before original bundle restored")
		}
		return nil
	})
	if changed || !errors.Is(err, cacheFailure) || saveCalled || attempts != 2 {
		t.Fatalf("cache failure: changed=%v err=%v saved=%v attempts=%d", changed, err, saveCalled, attempts)
	}
	fail := errors.New("configuration disk full")
	changed, err = updateBundleAppearance(bundle, prefs.Config{IconPNG: testIcon(t, color.NRGBA{B: 255, A: 255})}, func() error { return fail }, refresh)
	if changed || !errors.Is(err, fail) {
		t.Fatalf("save failure not reported: %v %v", changed, err)
	}
	after, err := os.ReadFile(resource)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("old icon not restored: %v", err)
	}
	restoredInfo, err := readBundleInfo(filepath.Join(contents, "Info.plist"))
	if err != nil || restoredInfo["CFBundleIconFile"] != name {
		t.Fatalf("old info not restored: %v", err)
	}
	changed, err = updateBundleAppearance(bundle, prefs.Config{}, func() error { saved = ""; return nil }, refresh)
	if err != nil || !changed || saved != "" {
		t.Fatalf("restore default: %v %v", changed, err)
	}
	restoredInfo, err = readBundleInfo(filepath.Join(contents, "Info.plist"))
	if err != nil {
		t.Fatal(err)
	}
	if restoredInfo["CFBundleIconFile"] != "Original.icns" || restoredInfo["CFBundleIconName"] != "OriginalAsset" {
		t.Fatal("default icon metadata not restored")
	}
	if data, err := os.ReadFile(originalPath); err != nil || !bytes.Equal(data, originalICNS) {
		t.Fatal("default image changed")
	}
	if _, ok := restoredInfo[originalIconKey]; ok {
		t.Fatal("custom metadata remains")
	}
	if _, err := os.Stat(resource); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("custom icon remains: %v", err)
	}
	if _, err = iconCommand(nil, "/usr/bin/codesign", "--verify", "--deep", "--strict", bundle); err != nil {
		t.Fatal(err)
	}
}

func TestBundleNameAndIconTransaction(t *testing.T) {
	bundle, originalIcon := iconTestBundle(t)
	plist := filepath.Join(bundle, "Contents", "Info.plist")
	store := prefs.Store{Path: filepath.Join(t.TempDir(), prefs.Filename)}
	initial := prefs.Config{URL: "https://example.com/"}
	if err := store.Save(initial); err != nil {
		t.Fatal(err)
	}
	config := initial
	config.AppName = "工作台 & <消息>"
	config.IconPNG = testIcon(t, color.NRGBA{B: 255, A: 255})
	refresh := func() error { return nil }
	apply := func(c prefs.Config) (bool, error) {
		return updateBundleAppearance(bundle, c, func() error { return store.Save(c) }, refresh)
	}
	if changed, err := apply(config); err != nil || !changed {
		t.Fatalf("apply: %v %v", changed, err)
	}
	info, err := readBundleInfo(plist)
	if err != nil {
		t.Fatalf("name not applied: %v %v", info, err)
	}
	checkBundleSystemName(t, bundle, config.AppName)
	oldIcon := info["CFBundleIconFile"]
	restarted, err := (prefs.Store{Path: store.Path}).Load()
	if err != nil || restarted != config {
		t.Fatalf("restart: %+v %v", restarted, err)
	}
	if changed, err := apply(restarted); err != nil || changed {
		t.Fatalf("restart rebuilt bundle: %v %v", changed, err)
	}
	failed := config
	failed.AppName = "未保存名称"
	failed.IconPNG = ""
	failure := errors.New("configuration disk full")
	if changed, err := updateBundleAppearance(bundle, failed, func() error { return failure }, refresh); changed || !errors.Is(err, failure) {
		t.Fatalf("failed save: %v %v", changed, err)
	}
	info, err = readBundleInfo(plist)
	if err != nil || info["CFBundleIconFile"] != oldIcon {
		t.Fatalf("name/icon not rolled back: %v %v", info, err)
	}
	checkBundleSystemName(t, bundle, config.AppName)
	config.AppName = "第二个名称"
	if changed, err := apply(config); err != nil || !changed {
		t.Fatalf("rename only: %v %v", changed, err)
	}
	info, err = readBundleInfo(plist)
	if err != nil || info["CFBundleIconFile"] != oldIcon {
		t.Fatalf("rename lost icon: %v %v", info, err)
	}
	checkBundleSystemName(t, bundle, config.AppName)
	config.AppName = ""
	if changed, err := apply(config); err != nil || !changed {
		t.Fatalf("restore name: %v %v", changed, err)
	}
	info, err = readBundleInfo(plist)
	if err != nil || info["CFBundleName"] != "IconQA" || info["CFBundleDisplayName"] != "Icon QA" || info["CFBundleIconFile"] != oldIcon {
		t.Fatalf("default name not restored: %v %v", info, err)
	}
	if changed, err := apply(initial); err != nil || !changed {
		t.Fatalf("reset: %v %v", changed, err)
	}
	info, err = readBundleInfo(plist)
	if err != nil || info[originalNameKey] != nil || info[originalIconKey] != nil {
		t.Fatalf("reset left overrides: %v %v", info, err)
	}
	data, err := os.ReadFile(filepath.Join(bundle, "Contents", "Resources", "Original.icns"))
	if err != nil || !bytes.Equal(data, originalIcon) {
		t.Fatalf("original icon changed: %v", err)
	}
	if _, err := iconCommand(nil, "/usr/bin/codesign", "--verify", "--deep", "--strict", bundle); err != nil {
		t.Fatal(err)
	}
}
