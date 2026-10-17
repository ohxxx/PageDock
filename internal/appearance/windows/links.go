//go:build windows

// [INPUT]: The current MyGo locale, known shortcut folders, original executable path, and EXE/config transaction.
// [OUTPUT]: Synchronize matching links' icons/taskbar identities, retaining arguments and other properties; update pinned paths/name mappings with rollback.
// [POS]: Windows desktop, Start menu, and pinned taskbar branding; do not modify other applications' shortcuts.
// [PROTOCOL]: Update internal/appearance/windows/AGENTS.md, native Windows tests, and platform boundaries.
package windows

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	l10n "github.com/ohxxx/PageDock/internal/locale"
)

const (
	iconShellLinkCLSID       = "{00021401-0000-0000-C000-000000000046}"
	iconShellLinkIID         = "{000214F9-0000-0000-C000-000000000046}"
	iconPersistFileIID       = "{0000010b-0000-0000-C000-000000000046}"
	iconCOMApartmentThreaded = 2
	iconCOMInprocServer      = 1
	iconCOMRelease           = 2
	iconLinkGetPath          = 3
	iconLinkGetArguments     = 10
	iconLinkSetIconLocation  = 17
	iconLinkSetPath          = 20
	iconPersistLoad          = 5
	iconPersistSave          = 6
	iconSTGMRead             = 0
	iconSTGMReadWrite        = 2
	iconMaxPath              = 32768
	iconSHCNEUpdateItem      = 0x2000
	iconSHCNERenameItem      = 1
	iconPinnedFolderID       = "{9E3995AB-1F9C-4F13-B827-48B24B6C7174}"
	iconSHCNEAssocChanged    = 0x8000000
	iconSHCNFPathW           = 5
	iconSHCNFFlush           = 0x1000
	iconKFDontVerify         = 0x4000
)

var (
	iconOle32                = syscall.NewLazyDLL("ole32.dll")
	iconShell32              = syscall.NewLazyDLL("shell32.dll")
	iconCoInitializeEx       = iconOle32.NewProc("CoInitializeEx")
	iconCoUninitialize       = iconOle32.NewProc("CoUninitialize")
	iconCoCreateInstance     = iconOle32.NewProc("CoCreateInstance")
	iconCLSIDFromString      = iconOle32.NewProc("CLSIDFromString")
	iconCoTaskMemFree        = iconOle32.NewProc("CoTaskMemFree")
	iconSHGetKnownFolderPath = iconShell32.NewProc("SHGetKnownFolderPath")
	iconSHChangeNotify       = iconShell32.NewProc("SHChangeNotify")
)

func iconHRESULT(result uintptr) error {
	if int32(result) < 0 {
		return fmt.Errorf(l10n.Text("windowsShell"), uint32(result))
	}
	return nil
}

func initializeIconCOM() error {
	hr, _, _ := iconCoInitializeEx.Call(0, iconCOMApartmentThreaded)
	return iconHRESULT(hr)
}

func iconGUID(value string) (guid syscall.GUID, err error) {
	text, err := syscall.UTF16PtrFromString(value)
	if err != nil {
		return guid, err
	}
	hr, _, _ := iconCLSIDFromString.Call(uintptr(unsafe.Pointer(text)), uintptr(unsafe.Pointer(&guid)))
	return guid, iconHRESULT(hr)
}

func iconShortcutRoots() ([]string, error) {
	// KNOWNFOLDERID supports redirected folders, OneDrive desktops, and non-default user directories.
	folders := []string{
		"{B4BFCC3A-DB2C-424C-B029-7FE99A87C641}", // Desktop
		"{A77F5D77-2E2B-44C3-A6A2-ABA601054A51}", // Programs
		"{C4AA340D-F20F-4863-AFEF-F87EF2E6BA25}", // PublicDesktop
		"{0139D44E-6AFE-49F2-8690-3DAFCAE6FFB8}", // CommonPrograms
		iconPinnedFolderID,                       // UserPinned
	}
	var roots []string
	for _, name := range folders {
		path, err := iconKnownFolderPath(name)
		if err != nil {
			return nil, err
		}
		roots = append(roots, path)
	}
	return roots, nil
}

func iconKnownFolderPath(name string) (string, error) {
	guid, err := iconGUID(name)
	if err != nil {
		return "", err
	}
	var path *uint16
	hr, _, _ := iconSHGetKnownFolderPath.Call(uintptr(unsafe.Pointer(&guid)), iconKFDontVerify, 0, uintptr(unsafe.Pointer(&path)))
	if err := iconHRESULT(hr); err != nil {
		return "", err
	}
	defer iconCoTaskMemFree.Call(uintptr(unsafe.Pointer(path)))
	var chars []uint16
	for offset := uintptr(0); ; offset += 2 {
		char := *(*uint16)(unsafe.Add(unsafe.Pointer(path), offset))
		if char == 0 {
			break
		}
		chars = append(chars, char)
	}
	return syscall.UTF16ToString(chars), nil
}

type iconCOMObject struct{ vtable *[21]uintptr }

//go:uintptrescapes
func (object *iconCOMObject) call(method int, args ...uintptr) error {
	hr, _, _ := syscall.SyscallN(object.vtable[method], append([]uintptr{uintptr(unsafe.Pointer(object))}, args...)...)
	return iconHRESULT(hr)
}

type iconShellLink struct{ link, file *iconCOMObject }

func openIconShellLink(path string, mode uint32) (*iconShellLink, error) {
	clsid, err := iconGUID(iconShellLinkCLSID)
	if err != nil {
		return nil, err
	}
	iid, err := iconGUID(iconShellLinkIID)
	if err != nil {
		return nil, err
	}
	s := &iconShellLink{}
	hr, _, _ := iconCoCreateInstance.Call(uintptr(unsafe.Pointer(&clsid)), 0, iconCOMInprocServer, uintptr(unsafe.Pointer(&iid)), uintptr(unsafe.Pointer(&s.link)))
	if err := iconHRESULT(hr); err != nil {
		return nil, err
	}
	iid, err = iconGUID(iconPersistFileIID)
	if err == nil {
		err = s.link.call(0, uintptr(unsafe.Pointer(&iid)), uintptr(unsafe.Pointer(&s.file)))
	}
	if err == nil && path != "" {
		var text *uint16
		text, err = syscall.UTF16PtrFromString(path)
		if err == nil {
			err = s.file.call(iconPersistLoad, uintptr(unsafe.Pointer(text)), uintptr(mode))
		}
	}
	if err != nil {
		s.close()
		return nil, err
	}
	return s, nil
}

func (s *iconShellLink) close() {
	if s.file != nil {
		_ = s.file.call(iconCOMRelease)
	}
	if s.link != nil {
		_ = s.link.call(iconCOMRelease)
	}
}

func (s *iconShellLink) target() (string, error) {
	buffer := make([]uint16, iconMaxPath)
	if err := s.link.call(iconLinkGetPath, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)), 0, 0); err != nil {
		return "", err
	}
	return syscall.UTF16ToString(buffer), nil
}

func (s *iconShellLink) save(path, executable, displayName string) error {
	target, err := syscall.UTF16PtrFromString(executable)
	if err != nil {
		return err
	}
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	if err := s.link.call(iconLinkSetPath, uintptr(unsafe.Pointer(target))); err != nil {
		return err
	}
	iconPath, iconIndex, err := taskbarIconLocation(executable)
	if err != nil {
		return err
	}
	iconText, err := syscall.UTF16PtrFromString(iconPath)
	if err != nil {
		return err
	}
	if err := s.link.call(iconLinkSetIconLocation, uintptr(unsafe.Pointer(iconText)), uintptr(iconIndex)); err != nil {
		return err
	}
	store, err := linkTaskbarStore(s)
	if err != nil {
		return err
	}
	defer store.call(iconCOMRelease)
	arguments := make([]uint16, iconMaxPath)
	if err := s.link.call(iconLinkGetArguments, uintptr(unsafe.Pointer(&arguments[0])), uintptr(len(arguments))); err != nil {
		return err
	}
	command := syscall.EscapeArg(executable)
	if args := syscall.UTF16ToString(arguments); args != "" {
		command += " " + args
	}
	// Explorer retains the first launch's implicit link, so its relaunch name must also be synchronized.
	for _, value := range []struct {
		id   uint32
		text string
	}{
		{taskbarCommand, command}, {taskbarDisplayName, displayName},
		{taskbarIcon, fmt.Sprintf("%s,%d", iconPath, iconIndex)},
		{taskbarAppID, taskbarIdentity(executable)},
	} {
		if err := setTaskbarString(store, value.id, value.text); err != nil {
			return err
		}
	}
	// IPersistFile.Save persists ShellLink properties together with the target and icon.
	return s.file.call(iconPersistSave, uintptr(unsafe.Pointer(name)), 1)
}

type iconShortcut struct {
	previousPath string
	taskbar      bool
	path         string
	original     []byte
	mode         fs.FileMode
}

func captureIconShortcuts(roots []string, executable string) ([]iconShortcut, error) {
	pinned, err := iconKnownFolderPath(iconPinnedFolderID)
	if err != nil {
		return nil, err
	}
	var shortcuts []iconShortcut
	seen := make(map[string]bool)
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			if err != nil {
				return err
			}
			if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !strings.EqualFold(filepath.Ext(path), ".lnk") {
				return nil
			}
			key := strings.ToLower(filepath.Clean(path))
			if seen[key] {
				return nil
			}
			seen[key] = true
			link, err := openIconShellLink(path, iconSTGMRead)
			if err != nil {
				return nil
			} // Leave damaged or non-file links belonging to other applications alone.
			defer link.close()
			target, err := link.target()
			if err != nil || target == "" {
				return nil
			}
			if resolved, err := filepath.EvalSymlinks(target); err == nil {
				target = resolved
			}
			if !strings.EqualFold(filepath.Clean(target), filepath.Clean(executable)) {
				return nil
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			original, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			relative, relErr := filepath.Rel(strings.ToLower(pinned), strings.ToLower(path))
			shortcuts = append(shortcuts, iconShortcut{path: path, original: original, mode: info.Mode(), taskbar: relErr == nil && filepath.IsLocal(relative)})
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf(l10n.Text("shortcutRead"), err)
		}
	}
	return shortcuts, nil
}

func saveIconShortcuts(shortcuts []iconShortcut, executable, displayName string, save func() error) (err error) {
	modified := 0
	var folders []nameFolderSnapshot
	defer func() {
		if err == nil {
			return
		}
		for i := len(folders) - 1; i >= 0; i-- {
			if restoreErr := folders[i].restore(); restoreErr != nil {
				err = errors.Join(err, fmt.Errorf(l10n.Text("windowsNameRestore"), folders[i].path, restoreErr))
			}
		}
		for i := modified - 1; i >= 0; i-- {
			shortcut := &shortcuts[i]
			from := shortcut.path
			if shortcut.previousPath != "" {
				if restoreErr := moveTaskbarShortcut(shortcut.path, shortcut.previousPath); restoreErr != nil {
					err = errors.Join(err, fmt.Errorf(l10n.Text("shortcutRestore"), shortcut.path, restoreErr))
					continue
				}
				shortcut.path, shortcut.previousPath = shortcut.previousPath, ""
			}
			if restoreErr := os.WriteFile(shortcut.path, shortcut.original, shortcut.mode.Perm()); restoreErr != nil {
				err = errors.Join(err, fmt.Errorf(l10n.Text("shortcutRestore"), shortcut.path, restoreErr))
			}
			if from != shortcut.path {
				notifyTaskbarRename(from, shortcut.path)
			}
		}
	}()
	for i := range shortcuts {
		shortcut := &shortcuts[i]
		link, openErr := openIconShellLink(shortcut.path, iconSTGMReadWrite)
		if openErr != nil {
			return openErr
		}
		modified++ // A failed Save may have partially written this link; restore it too.
		err = link.save(shortcut.path, executable, displayName)
		link.close()
		if err == nil && shortcut.taskbar {
			var next string
			next, err = taskbarShortcutPath(shortcut.path, displayName)
			if err == nil && next != shortcut.path {
				var owned bool
				owned, err = ownsWindowsLocalizedName(shortcut.path, executable)
				if err == nil && owned {
					var folder nameFolderSnapshot
					folder, err = captureNameFolder(filepath.Dir(shortcut.path))
					if err == nil {
						folders = append(folders, folder)
						err = setWindowsLocalizedName(shortcut.path, executable, "", false)
					}
				}
				if err == nil {
					err = moveTaskbarShortcut(shortcut.path, next)
				}
				if err == nil {
					shortcut.previousPath, shortcut.path = shortcut.path, next
				}
			}
		}
		if err != nil {
			return fmt.Errorf(l10n.Text("shortcutUpdate"), shortcut.path, err)
		}
	}
	return save()
}

func refreshIconShell(executable string, shortcuts []iconShortcut) {
	paths := []string{executable}
	for _, shortcut := range shortcuts {
		if shortcut.previousPath != "" {
			notifyTaskbarRename(shortcut.previousPath, shortcut.path)
		}
		paths = append(paths, shortcut.path)
	}
	for _, path := range paths {
		text, err := syscall.UTF16PtrFromString(path)
		if err == nil {
			iconSHChangeNotify.Call(iconSHCNEUpdateItem, iconSHCNFPathW|iconSHCNFFlush, uintptr(unsafe.Pointer(text)), 0)
		}
	}
	iconSHChangeNotify.Call(iconSHCNEAssocChanged, iconSHCNFFlush, 0, 0)
}
