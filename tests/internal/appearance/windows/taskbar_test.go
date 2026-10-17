//go:build windows

// [INPUT]: Isolated real Win32 windows, COM properties, and temporary Shell shortcuts.
// [OUTPUT]: Taskbar name, relaunch command, stable identity, destruction cleanup, pinned-link migration, and rollback behavior.
// [POS]: Native Windows tests without real user shortcuts; these do not replace Explorer visual validation.
// [PROTOCOL]: Update internal/appearance/windows/AGENTS.md and tests/AGENTS.md, including Validation records.
package windows

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"unsafe"

	"github.com/ohxxx/PageDock/internal/appearance"
)

func TestWindowsTaskbarWindowNameAndLifetime(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := initializeIconCOM(); err != nil {
		t.Fatal(err)
	}
	defer iconCoUninitialize.Call()
	user32 := syscall.NewLazyDLL("user32.dll")
	class, _ := syscall.UTF16PtrFromString("STATIC")
	hwnd, _, err := user32.NewProc("CreateWindowExW").Call(0, uintptr(unsafe.Pointer(class)), 0, 0, 0, 0, 100, 100, 0, 0, 0, 0)
	if hwnd == 0 {
		t.Fatal(err)
	}
	defer func() {
		if hwnd != 0 {
			user32.NewProc("DestroyWindow").Call(hwnd)
		}
	}()
	store, err := windowTaskbarStore(hwnd)
	if err != nil {
		t.Fatal(err)
	}
	defer store.call(iconCOMRelease)
	executable, err := windowsIconExecutable()
	if err != nil {
		t.Fatal(err)
	}
	id := taskbarIdentity(executable)
	iconPath, iconIndex, err := taskbarIconLocation(executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"哈哈哈222", "工作台 🖥", appearance.InternalName} {
		if err := configureWindowTaskbar(hwnd, name); err != nil {
			t.Fatal(err)
		}
		for property, want := range map[uint32]string{
			taskbarDisplayName: name, taskbarAppID: id,
			taskbarCommand: syscall.EscapeArg(executable),
			taskbarIcon:    fmt.Sprintf("%s,%d", iconPath, iconIndex),
		} {
			if got, err := getTaskbarString(store, property); err != nil || got != want {
				t.Fatalf("property %d = %q, want %q: %v", property, got, want, err)
			}
		}
	}
	if err := configureWindowTaskbar(hwnd, "invalid\x00name"); err == nil {
		t.Fatal("invalid name must fail instead of keeping partial relaunch metadata")
	}
	for _, property := range []uint32{taskbarDisplayName, taskbarAppID, taskbarCommand, taskbarIcon} {
		if got, err := getTaskbarString(store, property); err != nil || got != "" {
			t.Fatalf("property %d retained after failure: %q %v", property, got, err)
		}
	}
	if err := configureWindowTaskbar(hwnd, appearance.InternalName); err != nil {
		t.Fatal(err)
	}
	// DestroyWindow bypasses MyGo OnClose; allocated properties must still be cleared before destruction.
	if ok, _, err := user32.NewProc("DestroyWindow").Call(hwnd); ok == 0 {
		t.Fatal(err)
	}
	hwnd = 0
	for _, property := range []uint32{taskbarDisplayName, taskbarAppID, taskbarCommand, taskbarIcon} {
		if got, err := getTaskbarString(store, property); err != nil || got != "" {
			t.Fatalf("property %d retained after destroy: %q %v", property, got, err)
		}
	}
}

func TestWindowsTaskbarShortcutMigrationAndRollback(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := initializeIconCOM(); err != nil {
		t.Fatal(err)
	}
	defer iconCoUninitialize.Call()
	dir := filepath.Join(t.TempDir(), "应用 程序")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(dir, "Original.exe")
	other := filepath.Join(dir, "Other.exe")
	for _, path := range []string{executable, other} {
		if err := os.WriteFile(path, []byte("shortcut target"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	shortcut, unrelated := filepath.Join(dir, "固定项.lnk"), filepath.Join(dir, "其他.lnk")
	for path, target := range map[string]string{shortcut: executable, unrelated: other} {
		writeLegacyTaskbarLink(t, path, target)
	}
	read := func(path string) []byte {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	before, beforeOther := read(shortcut), read(unrelated)
	failure := errors.New("save denied")
	if err := syncTaskbarShortcuts([]string{dir}, executable, "工作台 🖥", func() error { return failure }); !errors.Is(err, failure) {
		t.Fatalf("rollback error: %v", err)
	}
	if !bytes.Equal(read(shortcut), before) {
		t.Fatal("failed migration changed pinned shortcut")
	}
	if err := syncTaskbarShortcuts([]string{dir}, executable, "工作台 🖥", func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	link, err := openIconShellLink(shortcut, iconSTGMRead)
	if err != nil {
		t.Fatal(err)
	}
	defer link.close()
	store, err := linkTaskbarStore(link)
	if err != nil {
		t.Fatal(err)
	}
	defer store.call(iconCOMRelease)
	if got, err := getTaskbarString(store, taskbarAppID); err != nil || got != taskbarIdentity(executable) {
		t.Fatalf("shortcut identity = %q: %v", got, err)
	}
	for property, want := range map[uint32]string{
		taskbarDisplayName: "工作台 🖥",
		taskbarCommand:     syscall.EscapeArg(executable) + " --profile=保留参数",
	} {
		if got, err := getTaskbarString(store, property); err != nil || got != want {
			t.Fatalf("shortcut property %d = %q, want %q: %v", property, got, want, err)
		}
	}
	if got, err := link.target(); err != nil || got != executable {
		t.Fatalf("shortcut target = %q: %v", got, err)
	}
	buffer := make([]uint16, iconMaxPath)
	const getArguments = 10
	if err := link.link.call(getArguments, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer))); err != nil {
		t.Fatal(err)
	}
	if got := syscall.UTF16ToString(buffer); got != "--profile=保留参数" {
		t.Fatalf("arguments changed: %q", got)
	}
	if !bytes.Equal(read(unrelated), beforeOther) {
		t.Fatal("migration changed another application's shortcut")
	}
	after, err := os.Stat(shortcut)
	if err != nil {
		t.Fatal(err)
	}
	if err := syncTaskbarShortcuts([]string{dir}, executable, "工作台 🖥", func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	again, err := os.Stat(shortcut)
	if err != nil || !again.ModTime().Equal(after.ModTime()) {
		t.Fatalf("unchanged identity rewrote shortcut: %v", err)
	}
	if taskbarIdentity(strings.ToUpper(executable)) != taskbarIdentity(executable) || taskbarIdentity(other) == taskbarIdentity(executable) {
		t.Fatal("identity must ignore path casing but isolate portable copies")
	}
	// Even with an unchanged identity, synchronize repeated renames and defaults rather than reusing the first implicit-link name.
	for _, name := range []string{"Second name", appearance.InternalName} {
		if err := syncTaskbarShortcuts([]string{dir}, executable, name, func() error { return nil }); err != nil {
			t.Fatal(err)
		}
		updated, err := openIconShellLink(shortcut, iconSTGMRead)
		if err != nil {
			t.Fatal(err)
		}
		properties, err := linkTaskbarStore(updated)
		if err != nil {
			updated.close()
			t.Fatal(err)
		}
		got, err := getTaskbarString(properties, taskbarDisplayName)
		_ = properties.call(iconCOMRelease)
		updated.close()
		if err != nil || got != name {
			t.Fatalf("renamed shortcut = %q, want %q: %v", got, name, err)
		}
	}
}

func writeLegacyTaskbarLink(t *testing.T, path, executable string) {
	t.Helper()
	link, err := openIconShellLink("", iconSTGMRead)
	if err != nil {
		t.Fatal(err)
	}
	defer link.close()
	arguments, _ := syscall.UTF16PtrFromString("--profile=保留参数")
	const setArguments = 11
	if err := link.link.call(setArguments, uintptr(unsafe.Pointer(arguments))); err != nil {
		t.Fatal(err)
	}
	if err := link.save(path, executable, appearance.InternalName); err != nil {
		t.Fatal(err)
	}
	store, err := linkTaskbarStore(link)
	if err != nil {
		t.Fatal(err)
	}
	defer store.call(iconCOMRelease)
	if err := setTaskbarString(store, taskbarAppID, ""); err != nil {
		t.Fatal(err)
	}
	text, _ := syscall.UTF16PtrFromString(path)
	if err := link.file.call(iconPersistSave, uintptr(unsafe.Pointer(text)), 1); err != nil {
		t.Fatal(err)
	}
}
