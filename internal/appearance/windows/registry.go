//go:build windows

// [INPUT]: The current MyGo locale, Windows package identity, installation path, display name, and save callback.
// [OUTPUT]: Update only the matching installation's per-user uninstall DisplayName; restore the old value on failure.
// [POS]: MyGo NSIS metadata adapter; portable copies do not create fake installation records.
// [PROTOCOL]: Update internal/appearance/windows/AGENTS.md and Windows registry tests.
package windows

import (
	"errors"
	"path/filepath"
	"strings"
	"syscall"
	"unicode/utf16"
	"unsafe"

	"github.com/ohxxx/PageDock/internal/appearance"
	l10n "github.com/ohxxx/PageDock/internal/locale"
)

const windowsUninstallRoot = `Software\Microsoft\Windows\CurrentVersion\Uninstall`

var nameRegSetValue = syscall.NewLazyDLL("advapi32.dll").NewProc("RegSetValueExW")

func registryString(key syscall.Handle, name string) (string, error) {
	text, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return "", err
	}
	var kind, size uint32
	if err = syscall.RegQueryValueEx(key, text, nil, &kind, nil, &size); err != nil {
		return "", err
	}
	if kind != syscall.REG_SZ || size > appearance.MaxNameStateBytes || size%2 != 0 {
		return "", errors.New(l10n.Text("windowsRegistryString"))
	}
	data := make([]uint16, size/2+1)
	if err = syscall.RegQueryValueEx(key, text, nil, &kind, (*byte)(unsafe.Pointer(&data[0])), &size); err != nil {
		return "", err
	}
	return syscall.UTF16ToString(data), nil
}

func setRegistryString(key syscall.Handle, name, value string) error {
	text, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return err
	}
	data, err := syscall.UTF16PtrFromString(value)
	if err != nil {
		return err
	}
	size := (len(utf16.Encode([]rune(value))) + 1) * 2
	result, _, _ := nameRegSetValue.Call(uintptr(key), uintptr(unsafe.Pointer(text)), 0, syscall.REG_SZ, uintptr(unsafe.Pointer(data)), uintptr(size))
	if result != 0 {
		return syscall.Errno(result)
	}
	return nil
}

func saveWindowsInstallName(executable, identifier, name string, save func() error) (err error) {
	if identifier == "" {
		return save()
	} // Unpackaged test executables have no installation identity.
	if strings.ContainsAny(identifier, `\/`) {
		return errors.New(l10n.Text("windowsIdentityPath"))
	}
	path, err := syscall.UTF16PtrFromString(windowsUninstallRoot + `\` + identifier)
	if err != nil {
		return err
	}
	var key syscall.Handle
	err = syscall.RegOpenKeyEx(syscall.HKEY_CURRENT_USER, path, 0, syscall.KEY_QUERY_VALUE, &key)
	if errors.Is(err, syscall.ERROR_FILE_NOT_FOUND) {
		return save()
	} // Portable copies have no uninstall entry.
	if err != nil {
		return err
	}
	location, err := registryString(key, "InstallLocation")
	if err != nil {
		syscall.RegCloseKey(key)
		return err
	}
	if resolved, resolveErr := filepath.EvalSymlinks(location); resolveErr == nil {
		location = resolved
	}
	if !strings.EqualFold(filepath.Clean(location), filepath.Dir(executable)) {
		syscall.RegCloseKey(key)
		return save()
	}
	previous, err := registryString(key, "DisplayName")
	syscall.RegCloseKey(key)
	if err != nil {
		return err
	}
	if previous == name {
		return save()
	}
	if err = syscall.RegOpenKeyEx(syscall.HKEY_CURRENT_USER, path, 0, syscall.KEY_SET_VALUE, &key); err != nil {
		return err
	}
	defer syscall.RegCloseKey(key)
	if err = setRegistryString(key, "DisplayName", name); err != nil {
		return err
	}
	if err = save(); err != nil {
		return errors.Join(err, setRegistryString(key, "DisplayName", previous))
	}
	return nil
}
