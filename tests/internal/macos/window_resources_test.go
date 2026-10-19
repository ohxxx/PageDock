//go:build desktop && darwin

// [INPUT]: Repeated real native window show/hide/close and process thread measurements.
// [OUTPUT]: Window ordering leaves bounded native workers after animations have settled.
// [POS]: Regression for AppKit animation workers starving subsequent WebKit navigation.
// [PROTOCOL]: Update tests/internal/AGENTS.md and patches/mygo/AGENTS.md.
package pagedock

import (
	"os"
	"testing"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func TestDesktopWindowOrderingReleasesWorkers(t *testing.T) {
	library, err := purego.Dlopen("/usr/lib/libproc.dylib", purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		t.Fatal(err)
	}
	defer purego.Dlclose(library)
	var info func(int32, int32, uint64, unsafe.Pointer, int32) int32
	purego.RegisterLibFunc(&info, library, "proc_pidinfo")
	threads := func() int32 {
		// proc_taskinfo from the platform SDK: six uint64 fields, then twelve int32 fields.
		var task struct {
			SizesAndTimes [6]uint64
			Counters      [9]int32
			Threads       int32
			Running       int32
			Priority      int32
		}
		const procPIDTaskInfo = 4
		if got := info(int32(os.Getpid()), procPIDTaskInfo, 0, unsafe.Pointer(&task), int32(unsafe.Sizeof(task))); got != int32(unsafe.Sizeof(task)) {
			t.Fatalf("proc_pidinfo returned %d bytes", got)
		}
		return task.Threads
	}
	cycle := func() {
		win := mygo.NewWindow(mygo.WindowOptions{Hidden: true, Width: 300, Height: 200,
			Content: ui.View(func(c *ui.Context) {})})
		if win == nil {
			t.Fatal("create window")
		}
		win.ShowInactive()
		time.Sleep(40 * time.Millisecond)
		win.Hide()
		win.ShowInactive()
		win.Destroy()
	}
	for range 3 {
		cycle()
	}
	time.Sleep(time.Second)
	baseline := threads()
	for range 40 {
		cycle()
	}
	// A few runtime/render workers may be created lazily; per-window growth is not allowed.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		mygo.RunOnMain(func() {})
		if current := threads(); current <= baseline+12 {
			t.Logf("native threads: baseline=%d settled=%d", baseline, current)
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("native workers remain after window teardown: baseline=%d current=%d", baseline, threads())
}
