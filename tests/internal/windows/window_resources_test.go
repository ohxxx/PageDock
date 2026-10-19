//go:build desktop && pagedock_soak && windows

// [INPUT]: Repeated native UI window creation/destruction and current-process kernel handle counts.
// [OUTPUT]: Window-only teardown does not accumulate kernel handles after queued UI work settles.
// [POS]: Opt-in rendering diagnostic; failures can originate in the graphics driver without WebView2.
// [PROTOCOL]: Update tests/internal/windows/AGENTS.md and native validation records.
package pagedock

import (
	"runtime"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func TestDesktopWindowsNativeWindowHandles(t *testing.T) {
	count := syscall.NewLazyDLL("kernel32.dll").NewProc("GetProcessHandleCount")
	current, err := syscall.GetCurrentProcess()
	if err != nil {
		t.Fatal(err)
	}
	handles := func() uint32 {
		var n uint32
		if ok, _, err := count.Call(uintptr(current), uintptr(unsafe.Pointer(&n))); ok == 0 {
			t.Fatal(err)
		}
		return n
	}
	cycle := func() {
		win := mygo.NewWindow(mygo.WindowOptions{Hidden: true, Width: 400, Height: 300,
			TitleBarStyle: mygo.TitleBarHidden,
			Content:       ui.View(func(c *ui.Context) { ui.Text(c, "Isolated native window lifetime") })})
		if win == nil {
			t.Fatal("create native window")
		}
		win.ShowInactive()
		time.Sleep(50 * time.Millisecond)
		win.Hide()
		win.ShowInactive()
		win.Destroy()
	}
	for range 10 {
		cycle()
	}
	// Warm the same runtime/GC machinery used during the recovery check.
	mygo.RunOnMain(func() {})
	runtime.GC()
	time.Sleep(time.Second)
	baseline := handles()
	for range 40 {
		cycle()
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		mygo.RunOnMain(func() {})
		runtime.GC()
		if n := handles(); n <= baseline+8 {
			t.Logf("native handles: baseline=%d settled=%d", baseline, n)
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("native handles survived closed windows: baseline=%d current=%d", baseline, handles())
}
