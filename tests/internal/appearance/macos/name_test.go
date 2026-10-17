//go:build darwin

// [INPUT]: Real temporary bundles, localized names, save failures, and separate Foundation reader processes.
// [OUTPUT]: Synchronized system file/Bundle names, with complete restoration of localized defaults and permission descriptions.
// [POS]: macOS Dock/Finder name-source tests without changes to user Dock preferences.
// [PROTOCOL]: Update internal/appearance/macos/AGENTS.md, tests/AGENTS.md, and actual Dock validation records.
package macos

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
	prefs "github.com/ohxxx/PageDock/internal/config"
)

func TestBundleSystemDisplayName(t *testing.T) {
	bundle, _ := iconTestBundle(t)
	resources := filepath.Join(bundle, "Contents", "Resources")
	original := make(map[string][]byte)
	for language, name := range map[string]string{"en": "Original name", "zh-Hans": "原始中文名称"} {
		path := filepath.Join(resources, language+".lproj", "InfoPlist.strings")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := writeBundleInfo(path, map[string]any{"CFBundleName": name, "CFBundleDisplayName": name, "NSCameraUsageDescription": "保留相机说明"}); err != nil {
			t.Fatal(err)
		}
		original[path], _ = os.ReadFile(path)
	}
	if _, err := iconCommand(nil, "/usr/bin/codesign", "--force", "--sign", "-", bundle); err != nil {
		t.Fatal(err)
	}
	beforeName := readSystemBundleName(t, bundle)
	refresh := func() error { return nil }
	for _, name := range []string{"新名称 & <消息>", "再次改名"} {
		changed, err := updateBundleAppearance(bundle, prefs.Config{AppName: name}, func() error { return nil }, refresh)
		if err != nil || !changed {
			t.Fatalf("rename: %v %v", changed, err)
		}
		checkBundleSystemName(t, bundle, name)
		for path := range original {
			values, err := readBundleInfo(path)
			if err != nil || values["CFBundleName"] != name || values["CFBundleDisplayName"] != name || values["NSCameraUsageDescription"] != "保留相机说明" {
				t.Fatalf("localized name or permission text changed incorrectly: %s %v", path, err)
			}
		}
		if changed, err = updateBundleAppearance(bundle, prefs.Config{AppName: name}, func() error { return nil }, refresh); err != nil || changed {
			t.Fatalf("unchanged name rewrote bundle: %v %v", changed, err)
		}
	}
	failure := errors.New("configuration denied")
	if _, err := updateBundleAppearance(bundle, prefs.Config{AppName: "未保存的名称"}, func() error { return failure }, refresh); !errors.Is(err, failure) {
		t.Fatal(err)
	}
	checkBundleSystemName(t, bundle, "再次改名")
	if _, err := updateBundleAppearance(bundle, prefs.Config{}, func() error { return nil }, refresh); err != nil {
		t.Fatal(err)
	}
	for path, before := range original {
		after, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(before, after) {
			t.Fatalf("original localization not restored: %s %v", path, err)
		}
	}
	if _, err := os.Stat(filepath.Join(resources, "Base.lproj")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("generated localization left after reset: %v", err)
	}
	if restored := readSystemBundleName(t, bundle); restored != beforeName {
		t.Fatalf("system name not restored: %+v want %+v", restored, beforeName)
	}
}

func checkBundleSystemName(t *testing.T, bundle, name string) {
	t.Helper()
	actual := readSystemBundleName(t, bundle)
	if strings.TrimSuffix(actual.File, ".app") != name || actual.Bundle != name {
		t.Fatalf("system names=%+v want=%q", actual, name)
	}
}

type systemBundleName struct{ File, Bundle string }

func readSystemBundleName(t *testing.T, bundle string) systemBundleName {
	t.Helper()
	result := filepath.Join(t.TempDir(), "names.json")
	cmd := exec.Command(os.Args[0], "-test.run=^TestBundleSystemNameReader$", "-test.count=1")
	cmd.Env = append(os.Environ(), "PAGEDOCK_NAME_BUNDLE="+bundle, "PAGEDOCK_NAME_RESULT="+result)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("system name: %v\n%s", err, out)
	}
	data, err := os.ReadFile(result)
	if err != nil {
		t.Fatal(err)
	}
	var actual systemBundleName
	if err = json.Unmarshal(data, &actual); err != nil {
		t.Fatal(err)
	}
	return actual
}

func TestBundleNameMigratesPreviousFormat(t *testing.T) {
	bundle, _ := iconTestBundle(t)
	path := filepath.Join(bundle, "Contents", "Info.plist")
	info, err := readBundleInfo(path)
	if err != nil {
		t.Fatal(err)
	}
	info[originalNameKey] = map[string]any{"CFBundleName": info["CFBundleName"], "CFBundleDisplayName": info["CFBundleDisplayName"]}
	name := "之前保存的名称"
	info["CFBundleName"], info["CFBundleDisplayName"] = name, name
	if err = writeBundleInfo(path, info); err != nil {
		t.Fatal(err)
	}
	if _, err = iconCommand(nil, "/usr/bin/codesign", "--force", "--sign", "-", bundle); err != nil {
		t.Fatal(err)
	}
	if changed, err := updateBundleAppearance(bundle, prefs.Config{AppName: name}, func() error { return nil }, func() error { return nil }); err != nil || !changed {
		t.Fatalf("previous format not repaired: %v %v", changed, err)
	}
	checkBundleSystemName(t, bundle, name)
}

func TestBundleNameRejectsLinkedResources(t *testing.T) {
	bundle, _ := iconTestBundle(t)
	apply := func(name string) error {
		_, err := updateBundleAppearance(bundle, prefs.Config{AppName: name}, func() error { return nil }, func() error { return nil })
		return err
	}
	if err := apply("第一个名称"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(bundle, "Contents", "Resources", "Base.lproj", "InfoPlist.strings")
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	external := filepath.Join(t.TempDir(), "external.strings")
	if err = os.Rename(path, external); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink(external, path); err != nil {
		t.Fatal(err)
	}
	if err = apply("不应写入外部文件"); err == nil {
		t.Fatal("linked resource accepted")
	}
	after, err := os.ReadFile(external)
	if err != nil || !bytes.Equal(after, original) {
		t.Fatalf("external resource modified: %v", err)
	}
}

func TestBundleSystemNameReader(t *testing.T) {
	bundle := os.Getenv("PAGEDOCK_NAME_BUNDLE")
	if bundle == "" {
		t.Skip("only run as a fresh system-name reader")
	}
	library, err := purego.Dlopen("/System/Library/Frameworks/Foundation.framework/Foundation", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		t.Fatal(err)
	}
	defer purego.Dlclose(library)
	pool := objc.ID(objc.GetClass("NSAutoreleasePool")).Send(objc.RegisterName("new"))
	defer pool.Send(objc.RegisterName("drain"))
	text := func(value string) objc.ID {
		return objc.ID(objc.GetClass("NSString")).Send(objc.RegisterName("stringWithUTF8String:"), value)
	}
	manager := objc.ID(objc.GetClass("NSFileManager")).Send(objc.RegisterName("defaultManager"))
	visible := manager.Send(objc.RegisterName("displayNameAtPath:"), text(bundle))
	actual := systemBundleName{File: objc.Send[string](visible, objc.RegisterName("UTF8String"))}
	b := objc.ID(objc.GetClass("NSBundle")).Send(objc.RegisterName("bundleWithPath:"), text(bundle))
	visible = b.Send(objc.RegisterName("objectForInfoDictionaryKey:"), text("CFBundleDisplayName"))
	actual.Bundle = objc.Send[string](visible, objc.RegisterName("UTF8String"))
	data, err := json.Marshal(actual)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(os.Getenv("PAGEDOCK_NAME_RESULT"), data, 0o600); err != nil {
		t.Fatal(err)
	}
}
