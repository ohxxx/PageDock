//go:build windows

// [INPUT]: Window options and the saved application display name.
// [OUTPUT]: A hidden window with platform taskbar identity set before display; callers handle showing and errors.
// [POS]: Shared Windows settings/website window creation entry, called only from background work.
// [PROTOCOL]: Update internal/appearance/windows/AGENTS.md and platform window behavior tests.
package windows

import (
	"github.com/egoist/mygo"
)

func NewWindow(options mygo.WindowOptions, name string) (*mygo.Window, error) {
	var win *mygo.Window
	var err error
	done := make(chan struct{})
	mygo.App.WhenReady(func() {
		options.Hidden = true
		win = mygo.NewWindow(options)
		err = configureWindowTaskbar(win.NativeHandle(), name)
		close(done)
	})
	<-done
	return win, err
}
