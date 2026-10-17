//go:build windows

// [INPUT]: Real Windows EXEs, Shell links, and save failures in isolated directories.
// [OUTPUT]: Win32 icon loading, preserved link targets/icons/arguments, rollback, and running-image replacement tests.
// [POS]: Windows-only execution without access to user shortcuts or installed applications.
// [PROTOCOL]: Update internal/appearance/windows/AGENTS.md, tests/AGENTS.md, validation commands, and platform boundaries.
package windows

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"image/color"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"unsafe"

	"github.com/ohxxx/PageDock/internal/appearance"
	prefs "github.com/ohxxx/PageDock/internal/config"
)

func TestWindowsIconAndShortcuts(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := initializeIconCOM(); err != nil {
		t.Fatal(err)
	}
	defer iconCoUninitialize.Call()
	dir := t.TempDir()
	path := filepath.Join(dir, "泊页 程序.exe")
	original := testWindowsEXE(t, runtime.GOARCH)
	if err := os.WriteFile(path, original, 0o700); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(dir, "泊页.lnk")
	link, err := openIconShellLink("", iconSTGMRead)
	if err != nil {
		t.Fatal(err)
	}
	args, _ := syscall.UTF16PtrFromString("--保留参数=test")
	const setArguments, getArguments, getIconLocation = 11, 10, 16
	if err := link.link.call(setArguments, uintptr(unsafe.Pointer(args))); err != nil {
		t.Fatal(err)
	}
	if err := link.save(linkPath, path, appearance.InternalName); err != nil {
		t.Fatal(err)
	}
	link.close()
	otherPath := filepath.Join(dir, "其他应用.lnk")
	other, err := openIconShellLink("", iconSTGMRead)
	if err != nil {
		t.Fatal(err)
	}
	if err := other.save(otherPath, path+".unrelated", appearance.InternalName); err != nil {
		t.Fatal(err)
	}
	other.close()
	beforeLink, err := os.ReadFile(linkPath)
	if err != nil {
		t.Fatal(err)
	}
	beforeOther, err := os.ReadFile(otherPath)
	if err != nil {
		t.Fatal(err)
	}
	updated, _, err := appearance.RewritePE(original, prefs.Config{IconPNG: testIcon(t, color.NRGBA{G: 255, A: 255})})
	if err != nil {
		t.Fatal(err)
	}
	wantErr := errors.New("save denied")
	if err := replaceWindowsAppearance(path, updated, []string{dir}, false, func() error { return wantErr }); !errors.Is(err, wantErr) {
		t.Fatalf("failure: %v", err)
	}
	for file, want := range map[string][]byte{path: original, linkPath: beforeLink, otherPath: beforeOther} {
		got, err := os.ReadFile(file)
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("rollback failed for %s: %v", file, err)
		}
	}
	if err := replaceWindowsAppearance(path, updated, []string{dir}, false, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	checkNativePEIcon(t, path)
	link, err = openIconShellLink(linkPath, iconSTGMRead)
	if err != nil {
		t.Fatal(err)
	}
	defer link.close()
	if target, err := link.target(); err != nil || target != path {
		t.Fatalf("target=%q err=%v", target, err)
	}
	buffer := make([]uint16, iconMaxPath)
	var index int32
	if err := link.link.call(getIconLocation, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)), uintptr(unsafe.Pointer(&index))); err != nil {
		t.Fatal(err)
	}
	if syscall.UTF16ToString(buffer) != path || index != -taskbarIconResourceID {
		t.Fatal("link icon not updated")
	}
	clear(buffer)
	if err := link.link.call(getArguments, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer))); err != nil {
		t.Fatal(err)
	}
	if syscall.UTF16ToString(buffer) != "--保留参数=test" {
		t.Fatal("link arguments lost")
	}
	gotOther, err := os.ReadFile(otherPath)
	if err != nil || !bytes.Equal(gotOther, beforeOther) {
		t.Fatal("changed another application's link")
	}
	reset, _, err := appearance.RewritePE(updated, prefs.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := replaceWindowsAppearance(path, reset, []string{dir}, false, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	restored, err := openIconShellLink(linkPath, iconSTGMRead)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.close()
	if err := restored.link.call(getIconLocation, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)), uintptr(unsafe.Pointer(&index))); err != nil {
		t.Fatal(err)
	}
	if syscall.UTF16ToString(buffer) == path {
		t.Fatal("default icon still points to the deleted custom resource")
	}
	var icon uintptr
	count, _, _ := iconShell32.NewProc("ExtractIconExW").Call(uintptr(unsafe.Pointer(&buffer[0])), uintptr(index), uintptr(unsafe.Pointer(&icon)), 0, 1)
	if count != 1 || icon == 0 {
		t.Fatal("restored default icon cannot be loaded")
	}
	syscall.NewLazyDLL("user32.dll").NewProc("DestroyIcon").Call(icon)
}

func checkNativePEIcon(t *testing.T, path string) {
	t.Helper()
	const loadAsDataFile, loadAsImageResource, imageIcon = 2, 0x20, 1
	kernel := syscall.NewLazyDLL("kernel32.dll")
	user := syscall.NewLazyDLL("user32.dll")
	text, _ := syscall.UTF16PtrFromString(path)
	module, _, err := kernel.NewProc("LoadLibraryExW").Call(uintptr(unsafe.Pointer(text)), 0, loadAsDataFile|loadAsImageResource)
	if module == 0 {
		t.Fatal(err)
	}
	defer kernel.NewProc("FreeLibrary").Call(module)
	icon, _, err := user.NewProc("LoadImageW").Call(module, 1, imageIcon, 32, 32, 0)
	if icon == 0 {
		t.Fatal(err)
	}
	defer user.NewProc("DestroyIcon").Call(icon)
}

func TestWindowsRunningIconReplacement(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	original, err := appearance.ReadExecutable(self)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "running.exe")
	if err := os.WriteFile(path, original, 0o700); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(path, "-test.run=^TestWindowsIconHelper$")
	cmd.Env = append(os.Environ(), "PAGEDOCK_ICON_HELPER=1")
	input, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = input.Close(); _ = cmd.Wait() }()
	if line, err := bufio.NewReader(output).ReadString('\n'); err != nil || line != "ready\n" {
		t.Fatalf("helper not ready: %q %v", line, err)
	}
	updated, _, err := appearance.RewritePE(original, prefs.Config{IconPNG: testIcon(t, color.NRGBA{R: 255, A: 255})})
	if err != nil {
		t.Fatal(err)
	}
	if err := replaceWindowsAppearance(path, updated, []string{dir}, false, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	checkNativePEIcon(t, path)
	// A new process must run the replaced executable while the old process remains alive until the test ends.
	restarted := exec.Command(path, "-test.run=^TestWindowsIconHelper$")
	restarted.Env = append(os.Environ(), "PAGEDOCK_ICON_HELPER=1")
	if out, err := restarted.CombinedOutput(); err != nil || !bytes.Contains(out, []byte("ready\n")) {
		t.Fatalf("relaunch failed: %s %v", out, err)
	}
}

func TestWindowsIconHelper(t *testing.T) {
	if os.Getenv("PAGEDOCK_ICON_HELPER") != "1" {
		return
	}
	fmt.Println("ready")
	_, _ = os.Stdin.Read(make([]byte, 1))
}
