//go:build windows

// [INPUT]: EXEs at canonical isolated paths, real Shell shortcuts, and unique test uninstall registry keys.
// [OUTPUT]: Native Shell names, version resources, installed-app names, preserved arguments, and complete rollback tests.
// [POS]: Windows-only tests that preserve user shortcuts and real installation records.
// [PROTOCOL]: Update internal/appearance/windows/AGENTS.md, tests/AGENTS.md, and cross-platform validation records.
package windows

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"unsafe"

	"github.com/ohxxx/PageDock/internal/appearance"
	prefs "github.com/ohxxx/PageDock/internal/config"
)

func TestWindowsAppearanceNamesAndRollback(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := initializeIconCOM(); err != nil {
		t.Fatal(err)
	}
	defer iconCoUninitialize.Call()
	dir := filepath.Join(windowsFixtureDir(t), "应用 程序")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, "original.exe")
	original := windowsNamedFixture(t, runtime.GOARCH)
	if err := os.WriteFile(exe, original, 0o700); err != nil {
		t.Fatal(err)
	}
	_, identifier, err := appearance.PEIdentity(original)
	if err != nil {
		t.Fatal(err)
	}
	key := createNameTestInstall(t, identifier, dir)
	ini := filepath.Join(dir, "desktop.ini")
	beforeINI := []byte("[Unrelated]\r\nKeep=other settings\r\n")
	if err := os.WriteFile(ini, beforeINI, 0o600); err != nil {
		t.Fatal(err)
	}
	shortcut := filepath.Join(dir, "原快捷方式.lnk")
	link, err := openIconShellLink("", iconSTGMRead)
	if err != nil {
		t.Fatal(err)
	}
	arguments, _ := syscall.UTF16PtrFromString("--profile=保留参数")
	const setArguments = 11
	if err = link.link.call(setArguments, uintptr(unsafe.Pointer(arguments))); err != nil {
		t.Fatal(err)
	}
	if err = link.save(shortcut, exe, appearance.InternalName); err != nil {
		t.Fatal(err)
	}
	link.close()
	beforeLink, err := os.ReadFile(shortcut)
	if err != nil {
		t.Fatal(err)
	}
	config := prefs.Config{AppName: "工作台 🖥"}
	updated, changed, err := appearance.RewritePE(original, config)
	if err != nil || !changed {
		t.Fatalf("rewrite: %v %v", changed, err)
	}
	failure := errors.New("configuration denied")
	err = replaceWindowsAppearance(exe, updated, []string{dir}, true, func() error {
		return failure
	})
	if !errors.Is(err, failure) {
		t.Fatalf("rollback: %v", err)
	}
	for path, want := range map[string][]byte{exe: original, shortcut: beforeLink, ini: beforeINI} {
		got, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("not rolled back %s: %v", path, err)
		}
	}
	if name, err := registryString(key, "DisplayName"); err != nil || name != "Original app" {
		t.Fatalf("registry rollback: %q %v", name, err)
	}
	if err = replaceWindowsAppearance(exe, updated, []string{dir}, true, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	checkWindowsName(t, exe, shortcut, key, config.AppName)
	// Renaming again must make Shell resolve the new string instead of returning the first rename's cached value.
	config.AppName = "第二个名字"
	second, _, err := appearance.RewritePE(updated, config)
	if err != nil {
		t.Fatal(err)
	}
	if err = replaceWindowsAppearance(exe, second, []string{dir}, true, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	checkWindowsName(t, exe, shortcut, key, config.AppName)
	link, err = openIconShellLink(shortcut, iconSTGMRead)
	if err != nil {
		t.Fatal(err)
	}
	if target, err := link.target(); err != nil || target != exe {
		t.Fatalf("shortcut target: %q %v", target, err)
	}
	buffer := make([]uint16, iconMaxPath)
	const getArguments = 10
	if err = link.link.call(getArguments, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer))); err != nil {
		t.Fatal(err)
	}
	link.close()
	if syscall.UTF16ToString(buffer) != "--profile=保留参数" {
		t.Fatal("shortcut arguments changed")
	}
	restored, _, err := appearance.RewritePE(second, prefs.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = replaceWindowsAppearance(exe, restored, []string{dir}, false, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if entry, err := localizedNameEntry(shortcut); err != nil || entry != "" {
		t.Fatalf("default shortcut name: %q %v", entry, err)
	}
	if got := windowsShellDisplayName(t, shortcut); got != "原快捷方式" {
		t.Fatalf("default visible name: %q", got)
	}
	if name, err := registryString(key, "DisplayName"); err != nil || name != "Original app" {
		t.Fatalf("default installed name: %q %v", name, err)
	}
}

func checkWindowsName(t *testing.T, exe, shortcut string, key syscall.Handle, want string) {
	t.Helper()
	if got := windowsShellDisplayName(t, shortcut); got != want {
		t.Fatalf("shortcut display=%q want=%q", got, want)
	}
	if got := windowsShellDisplayName(t, exe); got != want {
		t.Fatalf("EXE display=%q want=%q", got, want)
	}
	if name, err := registryString(key, "DisplayName"); err != nil || name != want {
		t.Fatalf("installed name: %q %v", name, err)
	}
	// Windows uses actual VERSIONINFO to resolve the product name and sending process's file description.
	module := syscall.NewLazyDLL("version.dll")
	path, _ := syscall.UTF16PtrFromString(exe)
	size, _, _ := module.NewProc("GetFileVersionInfoSizeW").Call(uintptr(unsafe.Pointer(path)), 0)
	if size == 0 {
		t.Fatal("missing native version resource")
	}
	data := make([]byte, size)
	if ok, _, err := module.NewProc("GetFileVersionInfoW").Call(uintptr(unsafe.Pointer(path)), 0, size, uintptr(unsafe.Pointer(&data[0]))); ok == 0 {
		t.Fatal(err)
	}
	translation, _ := syscall.UTF16PtrFromString(`\VarFileInfo\Translation`)
	var languages *uint16
	var translationBytes uint32
	if ok, _, err := module.NewProc("VerQueryValueW").Call(uintptr(unsafe.Pointer(&data[0])), uintptr(unsafe.Pointer(translation)), uintptr(unsafe.Pointer(&languages)), uintptr(unsafe.Pointer(&translationBytes))); ok == 0 || translationBytes < 4 {
		t.Fatalf("version translation: %v", err)
	}
	language := unsafe.Slice(languages, 2)
	for _, field := range []string{"ProductName", "FileDescription"} {
		query, _ := syscall.UTF16PtrFromString(fmt.Sprintf(`\StringFileInfo\%04x%04x\%s`, language[0], language[1], field))
		var value *uint16
		var length uint32
		if ok, _, err := module.NewProc("VerQueryValueW").Call(uintptr(unsafe.Pointer(&data[0])), uintptr(unsafe.Pointer(query)), uintptr(unsafe.Pointer(&value)), uintptr(unsafe.Pointer(&length))); ok == 0 {
			t.Fatal(err)
		}
		if got := syscall.UTF16ToString(unsafe.Slice(value, length)); got != want {
			t.Fatalf("%s=%q want=%q", field, got, want)
		}
	}
}

func windowsShellDisplayName(t *testing.T, path string) string {
	t.Helper()
	iid, err := iconGUID("{43826d1e-e718-42ee-bc55-a1e261c37bfe}")
	if err != nil {
		t.Fatal(err)
	}
	text, _ := syscall.UTF16PtrFromString(path)
	var item *iconCOMObject
	hr, _, _ := iconShell32.NewProc("SHCreateItemFromParsingName").Call(uintptr(unsafe.Pointer(text)), 0, uintptr(unsafe.Pointer(&iid)), uintptr(unsafe.Pointer(&item)))
	if err := iconHRESULT(hr); err != nil {
		t.Fatal(err)
	}
	defer item.call(iconCOMRelease)
	var display *uint16
	const getDisplayName = 5
	if err := item.call(getDisplayName, 0, uintptr(unsafe.Pointer(&display))); err != nil {
		t.Fatal(err)
	}
	defer iconCoTaskMemFree.Call(uintptr(unsafe.Pointer(display)))
	var chars []uint16
	for offset := uintptr(0); ; offset += 2 {
		char := *(*uint16)(unsafe.Add(unsafe.Pointer(display), offset))
		if char == 0 {
			break
		}
		chars = append(chars, char)
	}
	return syscall.UTF16ToString(chars)
}

func createNameTestInstall(t *testing.T, identifier, dir string) syscall.Handle {
	t.Helper()
	path, _ := syscall.UTF16PtrFromString(windowsUninstallRoot + `\` + identifier)
	var key syscall.Handle
	var disposition uint32
	dll := syscall.NewLazyDLL("advapi32.dll")
	result, _, _ := dll.NewProc("RegCreateKeyExW").Call(syscall.HKEY_CURRENT_USER, uintptr(unsafe.Pointer(path)), 0, 0, 0, syscall.KEY_ALL_ACCESS, 0, uintptr(unsafe.Pointer(&key)), uintptr(unsafe.Pointer(&disposition)))
	if result != 0 {
		t.Fatal(syscall.Errno(result))
	}
	if disposition != 1 {
		syscall.RegCloseKey(key)
		t.Fatal("test registry key already exists")
	}
	t.Cleanup(func() {
		syscall.RegCloseKey(key)
		if result, _, _ := dll.NewProc("RegDeleteKeyW").Call(syscall.HKEY_CURRENT_USER, uintptr(unsafe.Pointer(path))); result != 0 {
			t.Error(syscall.Errno(result))
		}
	})
	for name, value := range map[string]string{"InstallLocation": dir, "DisplayName": "Original app"} {
		if err := setRegistryString(key, name, value); err != nil {
			t.Fatal(err)
		}
	}
	return key
}

func TestWindowsNameKeepsOtherInstall(t *testing.T) {
	original := windowsNamedFixture(t, runtime.GOARCH)
	_, identifier, err := appearance.PEIdentity(original)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	key := createNameTestInstall(t, identifier, filepath.Join(dir, "other-install"))
	saved := false
	if err := saveWindowsInstallName(filepath.Join(dir, "app.exe"), identifier, "新名称", func() error { saved = true; return nil }); err != nil || !saved {
		t.Fatalf("unrelated install blocked save: %v", err)
	}
	if name, err := registryString(key, "DisplayName"); err != nil || name != "Original app" {
		t.Fatalf("modified another install: %q %v", name, err)
	}
}
