//go:build windows

// [INPUT]: The current MyGo locale, current EXE, exactly matched Shell links, name, and config save callback.
// [OUTPUT]: Visible file/shortcut and installed-app names, with directory metadata and registry rollback on failure.
// [POS]: Native Windows naming boundary; preserve paths, pinned items, uninstall entries, and notification identity.
// [PROTOCOL]: Update internal/appearance/windows/AGENTS.md, native Windows behavior tests, and validation boundaries.
package windows

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"github.com/ohxxx/PageDock/internal/appearance"
	l10n "github.com/ohxxx/PageDock/internal/locale"
)

var (
	nameSHSetLocalizedName        = iconShell32.NewProc("SHSetLocalizedName")
	nameSHRemoveLocalizedName     = iconShell32.NewProc("SHRemoveLocalizedName")
	nameWritePrivateProfileString = syscall.NewLazyDLL("kernel32.dll").NewProc("WritePrivateProfileStringW")
	nameGetPrivateProfileString   = syscall.NewLazyDLL("kernel32.dll").NewProc("GetPrivateProfileStringW")
)

const (
	localizedNamesSection       = "LocalizedFileNames"
	nameCodePageUTF8            = 65001
	nameRejectInvalidCharacters = 8 // MB_ERR_INVALID_CHARS
)

type nameFolderSnapshot struct {
	path          string
	attributes    uint32
	ini           []byte
	iniAttributes uint32
	existed       bool
}

func captureNameFolder(path string) (nameFolderSnapshot, error) {
	state := nameFolderSnapshot{path: path}
	folder, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return state, err
	}
	state.attributes, err = syscall.GetFileAttributes(folder)
	if err != nil {
		return state, err
	}
	iniPath := filepath.Join(path, "desktop.ini")
	state.ini, err = os.ReadFile(iniPath)
	if errors.Is(err, os.ErrNotExist) {
		return state, nil
	}
	if err != nil {
		return state, err
	}
	state.existed = true
	ini, err := syscall.UTF16PtrFromString(iniPath)
	if err != nil {
		return state, err
	}
	state.iniAttributes, err = syscall.GetFileAttributes(ini)
	return state, err
}

func (s nameFolderSnapshot) restore() error {
	path := filepath.Join(s.path, "desktop.ini")
	ini, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	if attrs, statErr := syscall.GetFileAttributes(ini); statErr == nil {
		if err = syscall.SetFileAttributes(ini, attrs&^syscall.FILE_ATTRIBUTE_READONLY); err != nil {
			return err
		}
	} else if !errors.Is(statErr, syscall.ERROR_FILE_NOT_FOUND) {
		return statErr
	}
	if s.existed {
		err = os.WriteFile(path, s.ini, 0o600)
		if err == nil {
			err = syscall.SetFileAttributes(ini, s.iniAttributes)
		}
	} else {
		err = os.Remove(path)
		if errors.Is(err, os.ErrNotExist) {
			err = nil
		}
	}
	folder, pathErr := syscall.UTF16PtrFromString(s.path)
	if pathErr != nil {
		return errors.Join(err, pathErr)
	}
	// Flush the Profile API cache after restoring files so failed transaction values cannot remain visible.
	nameWritePrivateProfileString.Call(0, 0, 0, uintptr(unsafe.Pointer(ini)))
	return errors.Join(err, syscall.SetFileAttributes(folder, s.attributes))
}

func localizedNameEntry(path string) (string, error) {
	section, _ := syscall.UTF16PtrFromString(localizedNamesSection)
	key, err := syscall.UTF16PtrFromString(filepath.Base(path))
	if err != nil {
		return "", err
	}
	ini, err := syscall.UTF16PtrFromString(filepath.Join(filepath.Dir(path), "desktop.ini"))
	if err != nil {
		return "", err
	}
	buffer := make([]uint16, iconMaxPath)
	n, _, _ := nameGetPrivateProfileString.Call(uintptr(unsafe.Pointer(section)), uintptr(unsafe.Pointer(key)), 0, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)), uintptr(unsafe.Pointer(ini)))
	if n >= uintptr(len(buffer)-1) {
		return "", errors.New(l10n.Text("windowsNameMetadata"))
	}
	return syscall.UTF16ToString(buffer[:n]), nil
}

// Shell reliably reads Unicode desktop.ini; the Profile API otherwise preserves an existing file's ANSI encoding.
func (s nameFolderSnapshot) prepareUnicode() error {
	if len(s.ini) >= 2 && s.ini[0] == 0xff && s.ini[1] == 0xfe {
		return nil
	}
	data := []byte{0xff, 0xfe}
	if len(s.ini) > 0 {
		convert := syscall.NewLazyDLL("kernel32.dll").NewProc("MultiByteToWideChar")
		input := s.ini
		var codePage uintptr // CP_ACP preserves the old INI system code page.
		if len(input) >= 3 && string(input[:3]) == "\xef\xbb\xbf" {
			codePage, input = nameCodePageUTF8, input[3:]
		}
		if len(input) > 0 {
			n, _, err := convert.Call(codePage, nameRejectInvalidCharacters, uintptr(unsafe.Pointer(&input[0])), uintptr(len(input)), 0, 0)
			if n == 0 {
				return err
			}
			chars := make([]uint16, n)
			if ok, _, err := convert.Call(codePage, nameRejectInvalidCharacters, uintptr(unsafe.Pointer(&input[0])), uintptr(len(input)), uintptr(unsafe.Pointer(&chars[0])), n); ok == 0 {
				return err
			}
			for _, char := range chars {
				data = binary.LittleEndian.AppendUint16(data, char)
			}
		}
	}
	path := filepath.Join(s.path, "desktop.ini")
	ini, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	if s.existed {
		if err := syscall.SetFileAttributes(ini, s.iniAttributes&^syscall.FILE_ATTRIBUTE_READONLY); err != nil {
			return err
		}
	}
	err = os.WriteFile(path, data, 0o600)
	if s.existed {
		err = errors.Join(err, syscall.SetFileAttributes(ini, s.iniAttributes))
	}
	nameWritePrivateProfileString.Call(0, 0, 0, uintptr(unsafe.Pointer(ini)))
	return err
}

func ownsWindowsLocalizedName(path, executable string) (bool, error) {
	entry, err := localizedNameEntry(path)
	if err != nil {
		return false, err
	}
	if version := strings.LastIndex(entry, ";v"); version >= 0 {
		if _, err := strconv.ParseUint(entry[version+2:], 10, 32); err == nil {
			entry = entry[:version]
		}
	}
	suffix := fmt.Sprintf(",-%d", appearance.DisplayNameID)
	if !strings.HasPrefix(entry, "@") || !strings.HasSuffix(entry, suffix) {
		return false, nil
	}
	module := strings.TrimSuffix(entry[1:], suffix)
	if strings.EqualFold(module, executable) {
		return true, nil
	}
	// SHSetLocalizedName may shorten a module path to its 8.3 name.
	resource, err := os.Stat(module)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	current, err := os.Stat(executable)
	return err == nil && os.SameFile(resource, current), err
}

func setWindowsLocalizedName(path, executable, name string, custom bool) error {
	target, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	var result uintptr
	if custom {
		module, err := syscall.UTF16PtrFromString(executable)
		if err != nil {
			return err
		}
		result, _, _ = nameSHSetLocalizedName.Call(uintptr(unsafe.Pointer(target)), uintptr(unsafe.Pointer(module)), appearance.DisplayNameID)
		if err := iconHRESULT(result); err != nil {
			return err
		}
		entry, err := localizedNameEntry(path)
		if err != nil {
			return err
		}
		// Version changes to fixed resource IDs; otherwise the MUI cache keeps returning the first name.
		value, _ := syscall.UTF16PtrFromString(fmt.Sprintf("%s;v%d", entry, crc32.ChecksumIEEE([]byte(name))))
		section, _ := syscall.UTF16PtrFromString(localizedNamesSection)
		key, _ := syscall.UTF16PtrFromString(filepath.Base(path))
		ini, _ := syscall.UTF16PtrFromString(filepath.Join(filepath.Dir(path), "desktop.ini"))
		if ok, _, err := nameWritePrivateProfileString.Call(uintptr(unsafe.Pointer(section)), uintptr(unsafe.Pointer(key)), uintptr(unsafe.Pointer(value)), uintptr(unsafe.Pointer(ini))); ok == 0 {
			return err
		}
	} else {
		// Restore defaults by removing only this app's mapping, preserving localized names from other sources.
		owned, err := ownsWindowsLocalizedName(path, executable)
		if err != nil {
			return err
		}
		if !owned {
			return nil
		}
		result, _, _ = nameSHRemoveLocalizedName.Call(uintptr(unsafe.Pointer(target)))
	}
	return iconHRESULT(result)
}

func saveWindowsNames(executable string, shortcuts []iconShortcut, name string, custom bool, save func() error) (err error) {
	paths := []string{executable}
	for _, shortcut := range shortcuts {
		paths = append(paths, shortcut.path)
	}
	if !custom {
		owned := paths[:0]
		for _, path := range paths {
			matched, readErr := ownsWindowsLocalizedName(path, executable)
			if readErr != nil {
				return readErr
			}
			if matched {
				owned = append(owned, path)
			}
		}
		paths = owned
	}
	if len(paths) == 0 {
		return save()
	}
	var folders []nameFolderSnapshot
	seen := make(map[string]bool)
	for _, path := range paths {
		dir := filepath.Dir(path)
		key := strings.ToLower(dir)
		if seen[key] {
			continue
		}
		snapshot, readErr := captureNameFolder(dir)
		if readErr != nil {
			return readErr
		}
		seen[key] = true
		folders = append(folders, snapshot)
	}
	defer func() {
		if err != nil {
			for _, folder := range folders {
				if restoreErr := folder.restore(); restoreErr != nil {
					err = errors.Join(err, fmt.Errorf(l10n.Text("windowsNameRestore"), folder.path, restoreErr))
				}
			}
		}
	}()
	if custom {
		for _, folder := range folders {
			if err = folder.prepareUnicode(); err != nil {
				return err
			}
		}
	}
	for _, path := range paths {
		if err = setWindowsLocalizedName(path, executable, name, custom); err != nil {
			return fmt.Errorf(l10n.Text("windowsNameUpdate"), path, err)
		}
	}
	return save()
}
