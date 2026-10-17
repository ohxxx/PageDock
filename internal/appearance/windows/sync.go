//go:build windows

// [INPUT]: The current EXE, known shortcut folders, and existing links' taskbar identities.
// [OUTPUT]: Migrate matching links' identity, name, icon, and internal path, restoring original link contents on failure.
// [POS]: Migrate pinned items before the first window to prevent explicit taskbar identity from creating a second button group.
// [PROTOCOL]: Update internal/appearance/windows/AGENTS.md and Windows shortcut tests.
package windows

import (
	"fmt"
	"path/filepath"
	"runtime"
)

func PrepareTaskbar(name string) error {
	executable, err := windowsIconExecutable()
	if err != nil {
		return err
	}
	roots, err := iconShortcutRoots()
	if err != nil {
		return err
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := initializeIconCOM(); err != nil {
		return err
	}
	defer iconCoUninitialize.Call()
	return syncTaskbarShortcuts(roots, executable, name, func() error { return nil })
}

func syncTaskbarShortcuts(roots []string, executable, name string, save func() error) error {
	shortcuts, err := captureIconShortcuts(roots, executable)
	if err != nil {
		return err
	}
	var pending []iconShortcut
	iconPath, iconIndex, err := taskbarIconLocation(executable)
	if err != nil {
		return err
	}
	for _, shortcut := range shortcuts {
		link, err := openIconShellLink(shortcut.path, iconSTGMRead)
		if err != nil {
			return err
		}
		store, err := linkTaskbarStore(link)
		var id, displayName, icon string
		if err == nil {
			id, err = getTaskbarString(store, taskbarAppID)
			if err == nil {
				displayName, err = getTaskbarString(store, taskbarDisplayName)
			}
			if err == nil {
				icon, err = getTaskbarString(store, taskbarIcon)
			}
			_ = store.call(iconCOMRelease)
		}
		link.close()
		if err != nil {
			return err
		}
		if id != taskbarIdentity(executable) || displayName != name || icon != fmt.Sprintf("%s,%d", iconPath, iconIndex) || shortcut.taskbar && filepath.Base(shortcut.path) != name+".lnk" {
			pending = append(pending, shortcut)
		}
	}
	if len(pending) == 0 {
		return save()
	}
	err = saveIconShortcuts(pending, executable, name, save)
	refreshIconShell(executable, pending)
	return err
}
