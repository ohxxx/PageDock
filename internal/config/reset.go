// [INPUT]: The current MyGo locale, confirmed browser cleanup callback, private config directory, and pending reset marker.
// [OUTPUT]: Persist immediate/deferred cleanup; reset URL, permissions, appearance, and window state only after browsing-data cleanup succeeds.
// [POS]: Reset persistence boundary; delete only app-owned files and retain failed requests for retry at next launch.
// [PROTOCOL]: Update this header and internal/config/AGENTS.md; update the root AGENTS.md when directory boundaries change.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	l10n "github.com/ohxxx/PageDock/internal/locale"
)

const (
	resetFilename       = "reset-pending"
	windowStateFilename = "window-state.json" // Persistence file for MyGo default WindowStateStore.
	resetAllPending     = "all"
	resetSitePending    = "site"
)

func (s Store) PrepareReset(clearBrowsingData func() error) error {
	if err := clearBrowsingData(); err != nil {
		return fmt.Errorf(l10n.Text("resetBrowsingFailed"), err)
	}
	if err := s.writeResetRequest(""); err != nil {
		return fmt.Errorf(l10n.Text("resetSettingsFailed"), err)
	}
	return nil
}

// On Windows, exit all WebView2 instances before a new process cleans up, avoiding hidden WebView cleanup callbacks.
func (s Store) DeferReset(all bool) error {
	mode := resetSitePending
	if all {
		mode = resetAllPending
	}
	return s.writeResetRequest(mode)
}

func (s Store) writeResetRequest(mode string) error {
	dir := filepath.Dir(s.Path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf(l10n.Text("resetWrite"), err)
	}
	f, err := os.CreateTemp(dir, ".reset-*")
	if err != nil {
		return fmt.Errorf(l10n.Text("resetWrite"), err)
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if _, err := f.WriteString(mode); err != nil {
		return fmt.Errorf(l10n.Text("resetWrite"), err)
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf(l10n.Text("resetWrite"), err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf(l10n.Text("resetWrite"), err)
	}
	if err := os.Rename(f.Name(), filepath.Join(dir, resetFilename)); err != nil {
		return fmt.Errorf(l10n.Text("resetWrite"), err)
	}
	return nil
}

func (s Store) CompleteBrowsingReset(clear func() error) error {
	marker := filepath.Join(filepath.Dir(s.Path), resetFilename)
	mode, err := os.ReadFile(marker)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf(l10n.Text("resetRead"), err)
	}
	switch string(mode) {
	case "": // Older versions and other platforms completed browser cleanup before exit.
		return nil
	case resetAllPending, resetSitePending:
	default:
		return errors.New(l10n.Text("resetUnknown"))
	}
	if err := clear(); err != nil {
		return fmt.Errorf(l10n.Text("resetBrowsingRetry"), err)
	}
	if string(mode) == resetAllPending {
		return s.writeResetRequest("") // Retain the full-reset marker until name, icon, and settings cleanup also finishes.
	}
	if err := os.Remove(marker); err != nil {
		return fmt.Errorf(l10n.Text("resetBrowsingFinish"), err)
	}
	return nil
}

func (s Store) CompleteReset(resetIcon func() error) (bool, error) {
	dir := filepath.Dir(s.Path)
	marker := filepath.Join(dir, resetFilename)
	if info, err := os.Stat(marker); errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf(l10n.Text("resetRead"), err)
	} else if !info.Mode().IsRegular() {
		return false, errors.New(l10n.Text("resetNotFile"))
	}
	if mode, err := os.ReadFile(marker); err != nil {
		return false, fmt.Errorf(l10n.Text("resetRead"), err)
	} else if len(mode) != 0 {
		return false, errors.New(l10n.Text("resetBrowsingPending"))
	}
	if err := resetIcon(); err != nil {
		return false, fmt.Errorf(l10n.Text("resetAppearance"), err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, fmt.Errorf(l10n.Text("resetFindTemporary"), err)
	}
	paths := []string{s.Path, filepath.Join(dir, windowStateFilename), filepath.Join(dir, windowStateFilename+".tmp")}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".config-") && strings.HasSuffix(entry.Name(), ".json") {
			paths = append(paths, filepath.Join(dir, entry.Name()))
		}
	}
	for _, path := range paths {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return false, fmt.Errorf(l10n.Text("resetRemove"), filepath.Base(path), err)
		}
	}
	if err := os.Remove(marker); err != nil {
		return false, fmt.Errorf(l10n.Text("resetFinish"), err)
	}
	return true, nil
}
