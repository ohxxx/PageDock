//go:build windows

// [INPUT]: The current EXE path, saved display name, and main-thread HWND.
// [OUTPUT]: Taskbar relaunch name, command, icon, and stable grouping; release window properties before destruction.
// [POS]: Native Windows Shell taskbar metadata; keep MyGo's internal name and WebView2 profile unchanged.
// [PROTOCOL]: Update internal/appearance/windows/AGENTS.md, native taskbar tests, and platform validation scope.
package windows

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	"github.com/ohxxx/PageDock/internal/appearance"
	l10n "github.com/ohxxx/PageDock/internal/locale"
)

const (
	taskbarPropertyStoreIID = "{886d8eeb-8cf2-4446-8d02-cdba1dbdcf99}"
	taskbarPropertyFormat   = "{9f4c2855-9f79-4b39-a8d0-e1d42de1d5f3}"
	taskbarCommand          = 2
	taskbarIcon             = 3
	taskbarDisplayName      = 4
	taskbarAppID            = 5
	taskbarGetValue         = 5
	taskbarSetValue         = 6
	taskbarString           = 31 // VT_LPWSTR
	taskbarIconResourceID   = 1  // EXE icon resource shared by MyGo windows and the tray.
	taskbarDestroyMessage   = 2  // WM_DESTROY
	taskbarSubclassID       = 1
)

var (
	taskbarGetWindowStore = iconShell32.NewProc("SHGetPropertyStoreForWindow")
	taskbarClearVariant   = iconOle32.NewProc("PropVariantClear")
	taskbarComctl32       = syscall.NewLazyDLL("comctl32.dll")
	taskbarSetSubclass    = taskbarComctl32.NewProc("SetWindowSubclass")
	taskbarRemoveSubclass = taskbarComctl32.NewProc("RemoveWindowSubclass")
	taskbarDefSubclass    = taskbarComctl32.NewProc("DefSubclassProc")
	taskbarSubclass       uintptr
)

func init() { taskbarSubclass = syscall.NewCallback(taskbarWindowProc) }

type taskbarPropertyKey struct {
	format syscall.GUID
	id     uint32
}

// PROPVARIANT string member; the union occupies 16 bytes on Windows x64/ARM64.
type taskbarVariant struct {
	kind uint16
	_    [3]uint16
	text *uint16
	_    uintptr
}

func taskbarIdentity(executable string) string {
	// Independent of the display name; portable copies at different paths do not share pinned items.
	digest := sha256.Sum256([]byte(strings.ToLower(filepath.Clean(executable))))
	return fmt.Sprintf("%s.%x", appearance.InternalName, digest[:16])
}

func windowTaskbarStore(hwnd uintptr) (*iconCOMObject, error) {
	iid, err := iconGUID(taskbarPropertyStoreIID)
	if err != nil {
		return nil, err
	}
	var store *iconCOMObject
	hr, _, _ := taskbarGetWindowStore.Call(hwnd, uintptr(unsafe.Pointer(&iid)), uintptr(unsafe.Pointer(&store)))
	return store, iconHRESULT(hr)
}

func linkTaskbarStore(link *iconShellLink) (*iconCOMObject, error) {
	iid, err := iconGUID(taskbarPropertyStoreIID)
	if err != nil {
		return nil, err
	}
	var store *iconCOMObject
	err = link.link.call(0, uintptr(unsafe.Pointer(&iid)), uintptr(unsafe.Pointer(&store)))
	return store, err
}

func setTaskbarString(store *iconCOMObject, id uint32, value string) error {
	variant := taskbarVariant{}
	if value != "" {
		text, err := syscall.UTF16PtrFromString(value)
		if err != nil {
			return err
		}
		variant.kind, variant.text = taskbarString, text
	}
	format, err := iconGUID(taskbarPropertyFormat)
	if err != nil {
		return err
	}
	key := taskbarPropertyKey{format, id}
	err = store.call(taskbarSetValue, uintptr(unsafe.Pointer(&key)), uintptr(unsafe.Pointer(&variant)))
	runtime.KeepAlive(variant)
	return err
}

func getTaskbarString(store *iconCOMObject, id uint32) (string, error) {
	format, err := iconGUID(taskbarPropertyFormat)
	if err != nil {
		return "", err
	}
	key := taskbarPropertyKey{format, id}
	var value taskbarVariant
	if err := store.call(taskbarGetValue, uintptr(unsafe.Pointer(&key)), uintptr(unsafe.Pointer(&value))); err != nil {
		return "", err
	}
	defer taskbarClearVariant.Call(uintptr(unsafe.Pointer(&value)))
	if value.kind == 0 {
		return "", nil
	}
	if value.kind != taskbarString || value.text == nil {
		return "", syscall.EINVAL
	}
	var chars []uint16
	for offset := uintptr(0); ; offset += 2 {
		char := *(*uint16)(unsafe.Add(unsafe.Pointer(value.text), offset))
		if char == 0 {
			break
		}
		chars = append(chars, char)
	}
	return syscall.UTF16ToString(chars), nil
}

func configureWindowTaskbar(hwnd uintptr, name string) error {
	executable, err := windowsIconExecutable()
	if err != nil {
		return err
	}
	iconPath, iconIndex, err := taskbarIconLocation(executable)
	if err != nil {
		return err
	}
	store, err := windowTaskbarStore(hwnd)
	if err != nil {
		return fmt.Errorf(l10n.Text("taskbarUpdate"), err)
	}
	defer store.call(iconCOMRelease)
	// MyGo Destroy does not trigger OnClose; remove properties in the native destruction message instead.
	if ok, _, callErr := taskbarSetSubclass.Call(hwnd, taskbarSubclass, taskbarSubclassID, 0); ok == 0 {
		return fmt.Errorf(l10n.Text("taskbarUpdate"), callErr)
	}
	values := []struct {
		id    uint32
		value string
	}{
		{taskbarCommand, syscall.EscapeArg(executable)},
		{taskbarIcon, fmt.Sprintf("%s,%d", iconPath, iconIndex)},
		{taskbarDisplayName, name},
		// Set the ID last so Shell updates the taskbar using the properties above.
		{taskbarAppID, taskbarIdentity(executable)},
	}
	for _, value := range values {
		if err := setTaskbarString(store, value.id, value.value); err != nil {
			err = errors.Join(err, clearTaskbarProperties(store))
			return fmt.Errorf(l10n.Text("taskbarUpdate"), err)
		}
	}
	return nil
}

func clearTaskbarProperties(store *iconCOMObject) error {
	var err error
	for _, id := range []uint32{taskbarAppID, taskbarDisplayName, taskbarIcon, taskbarCommand} {
		err = errors.Join(err, setTaskbarString(store, id, ""))
	}
	return err
}

func taskbarWindowProc(hwnd, message, wparam, lparam, subclass, _ uintptr) uintptr {
	if message == taskbarDestroyMessage {
		store, err := windowTaskbarStore(hwnd)
		if err == nil {
			err = clearTaskbarProperties(store)
			_ = store.call(iconCOMRelease)
		}
		if err != nil {
			log.Printf("%s", fmt.Errorf(l10n.Text("taskbarUpdate"), err))
		}
		taskbarRemoveSubclass.Call(hwnd, taskbarSubclass, subclass)
	}
	result, _, _ := taskbarDefSubclass.Call(hwnd, message, wparam, lparam)
	return result
}
