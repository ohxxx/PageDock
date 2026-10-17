//go:build desktop && windows

// [INPUT]: Real native windows, application menus, and Windows keyboard messages.
// [OUTPUT]: Shortcuts execute once, disabled items do not execute, and activation preserves the requested window focus.
// [POS]: Windows native settings/website focus tests without touching user windows.
// [PROTOCOL]: Update internal/AGENTS.md, tests/AGENTS.md, and MyGo patch notes.
package pagedock

import (
	"sync/atomic"
	"syscall"
	"testing"
	"unsafe"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func TestDesktopWindowsNativeMenuShortcut(t *testing.T) {
	var invoked atomic.Int32
	var value string
	win := mygo.NewWindow(mygo.WindowOptions{Width: 400, Height: 300, Content: ui.View(func(c *ui.Context) { ui.TextInput(c, &value).AutoFocus() })})
	defer win.Destroy()
	item := &mygo.MenuItem{Label: "Settings", Accelerator: "Ctrl+,", Click: func(*mygo.MenuItem, *mygo.Window) { invoked.Add(1) }}
	win.SetMenu(mygo.NewMenu([]*mygo.MenuItem{{Label: "App", Submenu: []*mygo.MenuItem{item}}, {Role: mygo.RoleEditMenu}}))
	user := syscall.NewLazyDLL("user32.dll")
	const childWindow, nextWindow = 5, 2
	var surface uintptr
	child, _, _ := user.NewProc("GetWindow").Call(win.NativeHandle(), childWindow)
	for child != 0 {
		var class [64]uint16
		user.NewProc("GetClassNameW").Call(child, uintptr(unsafe.Pointer(&class[0])), uintptr(len(class)))
		if syscall.UTF16ToString(class[:]) == "MyGoSurface" {
			surface = child
			break
		}
		child, _, _ = user.NewProc("GetWindow").Call(child, nextWindow)
	}
	if surface == 0 {
		t.Fatal("native surface missing")
	}
	press := func(key uintptr) {
		done := make(chan struct{})
		win.Update(func() {
			var before [256]byte
			user.NewProc("GetKeyboardState").Call(uintptr(unsafe.Pointer(&before[0])))
			keys := before
			const control, keyDown = 0x11, 0x100
			keys[control] = 0x80
			user.NewProc("SetKeyboardState").Call(uintptr(unsafe.Pointer(&keys[0])))
			user.NewProc("SendMessageW").Call(surface, keyDown, key, 1)
			user.NewProc("SetKeyboardState").Call(uintptr(unsafe.Pointer(&before[0])))
			close(done)
		})
		<-done
	}
	const comma = 0xbc
	press(comma)
	if invoked.Load() != 1 {
		t.Fatalf("native shortcut invoked %d times", invoked.Load())
	}
	item.SetEnabled(false)
	press(comma)
	if invoked.Load() != 1 {
		t.Fatal("disabled shortcut executed")
	}
	clipboard, err := mygo.Clipboard.Read()
	if err != nil {
		t.Fatal(err)
	}
	defer mygo.Clipboard.Write(clipboard)
	mygo.Clipboard.WriteText("只粘贴一次")
	press('V')
	eventually(t, func() bool {
		done := make(chan struct{})
		var got string
		win.Update(func() { got = value; close(done) })
		<-done
		if got != "" && got != "只粘贴一次" {
			t.Fatalf("paste repeated: %q", got)
		}
		return got == "只粘贴一次"
	})
}

func TestDesktopWindowsFocusKeepsRequestedWindow(t *testing.T) {
	options := mygo.WindowOptions{Width: 400, Height: 300, Content: ui.View(func(c *ui.Context) { ui.Text(c, "Focus test") })}
	first, second := mygo.NewWindow(options), mygo.NewWindow(options)
	defer first.Destroy()
	defer second.Destroy()
	for i := 0; i < 8; i++ {
		for _, target := range []*mygo.Window{first, second} {
			focus(target)
			if !target.IsFocused() {
				t.Fatal("application activation replaced the requested foreground window")
			}
		}
	}
}
