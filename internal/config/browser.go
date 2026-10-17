// [INPUT]: The current MyGo locale, an app-specific profile with no active browser session, and a cleanup deadline.
// [OUTPUT]: Delete the profile with bounded retries for busy files, propagate errors, and never follow directory links.
// [POS]: Browser-data cleanup after restart, called only by the Windows entry point and isolated behavior tests.
// [PROTOCOL]: Update internal/config/AGENTS.md, the root AGENTS.md, and validation records.
package config

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	l10n "github.com/ohxxx/PageDock/internal/locale"
)

const browserCleanupRetry = 100 * time.Millisecond

func removeBrowsingProfile(ctx context.Context, path string) error {
	if !filepath.IsAbs(path) {
		return errors.New(l10n.Text("profileAbsolute"))
	}
	parent, name := filepath.Dir(path), filepath.Base(path)
	info, err := os.Lstat(parent)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New(l10n.Text("profileParent"))
	}
	root, err := os.OpenRoot(parent)
	if err != nil {
		return err
	}
	defer root.Close()
	for {
		info, err := root.Lstat(name)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New(l10n.Text("profileDirectory"))
		}
		if err = root.RemoveAll(name); err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf(l10n.Text("profileBusy"), errors.Join(err, ctx.Err()))
		case <-time.After(browserCleanupRetry):
		}
	}
}
