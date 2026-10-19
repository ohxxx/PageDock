//go:build windows

// [INPUT]: Validated name and icon, the current Windows EXE, and the settings save callback.
// [OUTPUT]: Cached successful appearance synchronization and transactional EXE, shortcut, installed-app name, and config updates.
// [POS]: Windows appearance lifecycle using MyGo's running-EXE rename/replacement approach.
// [PROTOCOL]: Update internal/appearance/windows/AGENTS.md, platform tests, and README.
package windows

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/egoist/mygo"
	"github.com/ohxxx/PageDock/internal/appearance"
	prefs "github.com/ohxxx/PageDock/internal/config"
)

// Cache before the first rename; os.Executable may return the renamed running image's new path afterward.
var windowsIconExecutable = sync.OnceValues(func() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(path)
})

func Apply(config prefs.Config, save func() error) (bool, error) {
	return synchronizedAppearance.apply(config, save, applyAppearance)
}

var synchronizedAppearance appearanceCache

func applyAppearance(config prefs.Config, save func() error) (bool, error) {
	path, err := windowsIconExecutable()
	if err != nil {
		return false, err
	}
	original, err := appearance.ReadExecutable(path)
	if err != nil {
		return false, err
	}
	updated, changed, err := appearance.RewritePE(original, config)
	if err != nil {
		return false, err
	}
	if !changed {
		return false, save()
	}
	roots, err := iconShortcutRoots()
	if err != nil {
		return false, err
	}
	if err := replaceWindowsAppearance(path, updated, roots, config.AppName != "", save); err != nil {
		return false, err
	}
	return changed, nil
}

func replaceWindowsAppearance(path string, data []byte, roots []string, customName bool, save func() error) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := initializeIconCOM(); err != nil {
		return err
	}
	defer iconCoUninitialize.Call()
	// Read before renaming so Shell link tracking cannot retarget links to the old image's backup path.
	shortcuts, err := captureIconShortcuts(roots, path)
	if err != nil {
		return err
	}
	apply := func() error {
		current, err := appearance.ReadExecutable(path)
		if err != nil {
			return err
		}
		name, identifier, err := appearance.PEIdentity(current)
		if err != nil {
			return err
		}
		return saveIconShortcuts(shortcuts, path, name, func() error {
			return saveWindowsNames(path, shortcuts, name, customName, func() error {
				return saveWindowsInstallName(path, identifier, name, save)
			})
		})
	}
	err = appearance.ReplaceExecutable(path, data, renameIconExecutable, apply)
	refreshIconShell(path, shortcuts)
	return err
}

func renameIconExecutable(from, to string) error {
	const attempts = 10
	for attempt := 1; ; attempt++ {
		err := os.Rename(from, to)
		if err == nil || attempt == attempts || (!errors.Is(err, syscall.Errno(32)) && !errors.Is(err, syscall.Errno(5))) {
			return err
		}
		time.Sleep(time.Duration(attempt) * 100 * time.Millisecond)
	}
}

func Relaunch(_ string) { mygo.App.Relaunch() }
