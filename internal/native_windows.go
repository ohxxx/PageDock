//go:build windows

// [INPUT]: Name, icon, taskbar, and restart operations for the build target.
// [OUTPUT]: Platform implementations used by application orchestration.
// [POS]: Select native operations at compile time; platform packages depend on shared appearance resources without cycles.
// [PROTOCOL]: Update internal/AGENTS.md, appearance/AGENTS.md, and platform behavior tests.
package pagedock

import (
	"github.com/ohxxx/PageDock/internal/appearance/windows"
)

var (
	applyAppearance = windows.Apply
	relaunch        = windows.Relaunch
	prepareTaskbar  = windows.PrepareTaskbar
	newWindow       = windows.NewWindow
)
