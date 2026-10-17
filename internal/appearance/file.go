// [INPUT]: The current MyGo locale, validated Windows EXE bytes, a same-directory target, and the settings save callback.
// [OUTPUT]: Complete staging, replacement, and rollback; the MyGo patch retries committed old-image cleanup on the next launch.
// [POS]: Shared Windows name/icon file transaction, using MyGo's running-executable rename approach.
// [PROTOCOL]: Update internal/appearance/AGENTS.md and real file behavior tests.
package appearance

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	l10n "github.com/ohxxx/PageDock/internal/locale"
)

func ReadExecutable(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > maxIconEXEBytes {
		return nil, errors.New(l10n.Text("executableInvalid"))
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxIconEXEBytes+1))
	if len(data) > maxIconEXEBytes {
		return nil, errors.New(l10n.Text("executableLarge"))
	}
	return data, err
}

func ReplaceExecutable(path string, data []byte, rename func(string, string) error, save func() error) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New(l10n.Text("executableNotFile"))
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".pagedock-icon-*.exe")
	if err != nil {
		return fmt.Errorf(l10n.Text("executableDirectory"), err)
	}
	staged := f.Name()
	defer os.Remove(staged)
	defer f.Close()
	if err = f.Chmod(info.Mode().Perm()); err != nil {
		return err
	}
	if _, err = f.Write(data); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	// Do not use MyGo's auto-cleaned .old-* name before commit; retain the original executable if rollback fails.
	suffix := rand.Text()
	previous := filepath.Join(filepath.Dir(path), ".pagedock-icon-recovery-"+suffix+".exe")
	if err = rename(path, previous); err != nil {
		return fmt.Errorf(l10n.Text("executableStage"), err)
	}
	if err = rename(staged, path); err != nil {
		if restoreErr := rename(previous, path); restoreErr != nil {
			return errors.Join(err, fmt.Errorf(l10n.Text("executablePreserved"), previous, restoreErr))
		}
		return err
	}
	if err = save(); err != nil {
		// Move the new executable aside before restoring the running old image; retain recovery copies and report rollback failure.
		if moveErr := rename(path, staged); moveErr != nil {
			return errors.Join(err, fmt.Errorf(l10n.Text("executableMoveFailed"), previous, moveErr))
		}
		if restoreErr := rename(previous, path); restoreErr != nil {
			_ = rename(staged, path)
			return errors.Join(err, fmt.Errorf(l10n.Text("executableRestore"), previous, restoreErr))
		}
		return err
	}
	if err := os.Remove(previous); err != nil {
		old := filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+".old-"+suffix)
		if err := rename(previous, old); err != nil {
			log.Printf(l10n.Text("executableCleanup"), previous, err)
		}
	}
	return nil
}
