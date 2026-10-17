//go:build windows

// [INPUT]: Pinned taskbar shortcuts matched to the current EXE, a valid name, and a file transaction.
// [OUTPUT]: Rename within the same directory and notify Shell; reject conflicts and preserve targets, arguments, and pinning.
// [POS]: Windows taskbar caches follow real link names; desktop and Start menu links only change display names.
// [PROTOCOL]: Update AGENTS.md, native shortcut tests, and Windows visual validation.
package windows

import (
	"errors"
	"path/filepath"
	"syscall"
	"unsafe"

	prefs "github.com/ohxxx/PageDock/internal/config"
	l10n "github.com/ohxxx/PageDock/internal/locale"
)

func taskbarShortcutPath(path, name string) (string, error) {
	if err := prefs.ValidatePlatformName(name); err != nil {
		return "", err
	}
	if name == "" {
		return "", errors.New(l10n.Text("nameWindowsFilename"))
	}
	return filepath.Join(filepath.Dir(path), name+".lnk"), nil
}

func moveTaskbarShortcut(from, to string) error {
	old, err := syscall.UTF16PtrFromString(from)
	if err != nil {
		return err
	}
	next, err := syscall.UTF16PtrFromString(to)
	if err != nil {
		return err
	}
	return syscall.MoveFile(old, next) // Never replace another pinned item in the same directory.
}

func notifyTaskbarRename(from, to string) {
	old, oldErr := syscall.UTF16PtrFromString(from)
	next, nextErr := syscall.UTF16PtrFromString(to)
	if oldErr == nil && nextErr == nil {
		iconSHChangeNotify.Call(iconSHCNERenameItem, iconSHCNFPathW|iconSHCNFFlush, uintptr(unsafe.Pointer(old)), uintptr(unsafe.Pointer(next)))
	}
}
