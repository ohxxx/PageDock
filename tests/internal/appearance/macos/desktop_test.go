//go:build desktop && darwin

// [INPUT]: A local desktop session with isolated config and locale.
// [OUTPUT]: The MyGo event loop and results for this package's desktop tests.
// [POS]: Explicit desktop entry; run packages serially without loading real user config.
// [PROTOCOL]: Update the owning AGENTS.md and behavior tests under tests/.
package macos

import (
	"os"
	"testing"

	"github.com/egoist/mygo"
	l10n "github.com/ohxxx/PageDock/internal/locale"
)

func TestMain(m *testing.M) {
	// Isolate MyGo locale persistence; existing interaction tests always use Simplified Chinese.
	os.Setenv("MYGO_LOCALE", l10n.Chinese)
	dir, err := os.MkdirTemp("", "pagedock-desktop-test-")
	if err != nil {
		panic(err)
	}
	mygo.App.SetName("PageDockTests.appearance")
	mygo.App.SetPath(mygo.PathUserData, dir)
	mygo.App.OnWindowAllClosed(func() {})
	exitCode := make(chan int, 1)
	mygo.App.WhenReady(func() {
		go func() {
			exitCode <- m.Run()
			mygo.App.Quit()
		}()
	})
	if err := mygo.App.Run(); err != nil {
		panic(err)
	}
	code := <-exitCode
	os.RemoveAll(dir)
	os.Exit(code)
}
