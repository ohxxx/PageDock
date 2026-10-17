//go:build !darwin && !windows

// [INPUT]: The icon saved in native settings and the config write operation.
// [OUTPUT]: Retain the configured icon without applying it; preserve normal saves, startup, and default restoration.
// [POS]: Icon capability boundary on other platforms; settings explicitly report it as unsupported.
// [PROTOCOL]: Update this header, internal/appearance/AGENTS.md, and platform notes.
package appearance

import (
	"github.com/egoist/mygo"
	prefs "github.com/ohxxx/PageDock/internal/config"
)

func Apply(_ prefs.Config, save func() error) (bool, error) {
	return false, save()
}

func Relaunch(_ string) { mygo.App.Relaunch() }
