// [INPUT]: The native About menu item, saved app name, and error presentation callback.
// [OUTPUT]: A Windows About dialog shown in the background, with failure reporting.
// [POS]: Windows menu presentation; shared orchestration retains application state and locking.
// [PROTOCOL]: Update internal/view/windows/AGENTS.md and menu behavior tests.
package windows

import (
	"fmt"

	"github.com/egoist/mygo"
	l10n "github.com/ohxxx/PageDock/internal/locale"
)

func ConfigureAbout(about *mygo.MenuItem, name string, report func(string)) {
	about.Role = ""
	about.Click = func(_ *mygo.MenuItem, win *mygo.Window) {
		go func() {
			_, err := mygo.Dialog.Message(mygo.MessageOptions{Parent: win, Type: mygo.MessageInfo,
				Title: fmt.Sprintf(l10n.Text("aboutApp"), name), Message: name,
				Detail: fmt.Sprintf(l10n.Text("appVersion"), mygo.App.Version())})
			if err != nil {
				report(err.Error())
			}
		}()
	}
}
