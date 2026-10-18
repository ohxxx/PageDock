//go:build windows

// [INPUT]: Real Shell links in canonical temporary directories, consecutive names, conflicting files, and save failures.
// [OUTPUT]: Taskbar renames, preserved arguments/targets, default restoration, and complete file/directory metadata rollback.
// [POS]: Native Windows behavior tests that never modify real pinned items.
// [PROTOCOL]: Update internal/appearance/windows/AGENTS.md, tests/AGENTS.md, and Windows validation records.
package windows

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"unsafe"

	"github.com/ohxxx/PageDock/internal/appearance"
	prefs "github.com/ohxxx/PageDock/internal/config"
)

func TestWindowsTaskbarRenameAndRollback(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := initializeIconCOM(); err != nil {
		t.Fatal(err)
	}
	defer iconCoUninitialize.Call()
	dir := windowsFixtureDir(t)
	exe := filepath.Join(dir, "app.exe")
	if err := os.WriteFile(exe, testWindowsEXE(t, runtime.GOARCH), 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "Original.lnk")
	writeLegacyTaskbarLink(t, path, exe)
	for _, name := range []string{"工作台 甲", "Second", "SECOND", appearance.InternalName} {
		links, err := captureIconShortcuts([]string{dir}, exe)
		if err != nil || len(links) != 1 {
			t.Fatalf("capture: %d %v", len(links), err)
		}
		links[0].taskbar = true
		before, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		folder, err := captureNameFolder(dir)
		if err != nil {
			t.Fatal(err)
		}
		failure := errors.New("configuration denied")
		save := func(result error) error {
			return saveIconShortcuts(links, exe, name, func() error {
				return saveWindowsNames(exe, links, name, name != appearance.InternalName, func() error { return result })
			})
		}
		if err := save(failure); !errors.Is(err, failure) {
			t.Fatalf("rollback: %v", err)
		}
		if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, before) {
			t.Fatalf("original link lost: %v", err)
		}
		after, err := captureNameFolder(dir)
		if err != nil || !bytes.Equal(after.ini, folder.ini) || after.attributes != folder.attributes {
			t.Fatalf("metadata rollback: %v", err)
		}
		if err := save(nil); err != nil {
			t.Fatal(err)
		}
		if !strings.EqualFold(path, links[0].path) {
			if entry, err := localizedNameEntry(path); err != nil || entry != "" {
				t.Fatalf("obsolete localized name retained: %q %v", entry, err)
			}
		}
		path = filepath.Join(dir, name+".lnk")
		if links[0].path != path {
			t.Fatalf("renamed path = %q", links[0].path)
		}
		link, err := openIconShellLink(path, iconSTGMRead)
		if err != nil {
			t.Fatal(err)
		}
		target, err := link.target()
		buffer := make([]uint16, iconMaxPath)
		if err := link.link.call(iconLinkGetArguments, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer))); err != nil {
			t.Fatal(err)
		}
		if syscall.UTF16ToString(buffer) != "--profile=保留参数" {
			t.Fatal("rename lost launch arguments")
		}
		link.close()
		if err != nil || target != exe {
			t.Fatalf("target = %q: %v", target, err)
		}
	}
	links, err := captureIconShortcuts([]string{dir}, exe)
	if err != nil {
		t.Fatal(err)
	}
	links[0].taskbar = true
	collision := filepath.Join(dir, "Existing.lnk")
	if err := os.WriteFile(collision, []byte("unrelated"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := saveIconShortcuts(links, exe, "Existing", func() error { t.Fatal("collision saved configuration"); return nil }); err == nil {
		t.Fatal("replaced existing shortcut")
	}
	if got, _ := os.ReadFile(collision); string(got) != "unrelated" {
		t.Fatal("collision file changed")
	}
	for _, name := range []string{"../escape", `a\b`, "a:b", "NUL", "COM1", "LPT¹", "trailing.", "a?b"} {
		if _, err := (prefs.Config{AppName: name, URL: "https://example.com"}).Validate(); err == nil {
			t.Fatalf("unsafe Windows name accepted: %q", name)
		}
	}
}
