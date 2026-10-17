//go:build desktop

// [INPUT]: A local desktop session with isolated config and locale.
// [OUTPUT]: The MyGo event loop and results for this package's desktop tests.
// [POS]: Explicit desktop entry; run packages serially without loading real user config.
// [PROTOCOL]: Update the owning AGENTS.md and behavior tests under tests/.
package view

import (
	"os"
	"testing"
	"time"

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
	mygo.App.SetName("PageDockTests.view")
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
func eventually(t *testing.T, check func() bool) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("timed out waiting for desktop state")
}

func pageValue(t *testing.T, page *mygo.Page, script string) string {
	t.Helper()
	value, err := mygo.EvalAs[string](page, script)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func focus(win *mygo.Window) {
	win.Show()
	win.Restore()
	// Windows app activation may select any visible window; focus the requested window last.
	mygo.App.Focus()
	win.Focus()
}
