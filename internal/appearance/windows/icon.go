//go:build windows

// [INPUT]: The current EXE and Windows Shell system icons.
// [OUTPUT]: A valid taskbar/shortcut icon location; explicitly restore the system default when no icon is embedded.
// [POS]: Avoid stale custom-icon cache hits after default restoration, without creating extra icon files.
// [PROTOCOL]: Update internal/appearance/windows/AGENTS.md, native Windows tests, and screen validation.
package windows

import (
	"syscall"
	"unsafe"
)

func taskbarIconLocation(executable string) (string, int32, error) {
	path, err := syscall.UTF16PtrFromString(executable)
	if err != nil {
		return "", 0, err
	}
	const countIcons = ^uintptr(0) // ExtractIconExW uses nIconIndex = -1 to count icons.
	count, _, _ := iconShell32.NewProc("ExtractIconExW").Call(uintptr(unsafe.Pointer(path)), countIcons, 0, 0, 0)
	if count != 0 && uint32(count) != ^uint32(0) {
		return executable, -taskbarIconResourceID, nil
	}
	// Pointing to a nonexistent EXE icon resource keeps the last cached Shell image.
	const stockApplication, stockIconLocation, stockPathCapacity = 2, 0, 260 // SIID_APPLICATION / SHGSI_ICONLOCATION
	var stock struct {
		size               uint32
		icon               uintptr
		systemIndex, index int32
		path               [stockPathCapacity]uint16 // SHSTOCKICONINFO.szPath[MAX_PATH]
	}
	stock.size = uint32(unsafe.Sizeof(stock))
	hr, _, _ := iconShell32.NewProc("SHGetStockIconInfo").Call(stockApplication, stockIconLocation, uintptr(unsafe.Pointer(&stock)))
	if err := iconHRESULT(hr); err != nil {
		return "", 0, err
	}
	return syscall.UTF16ToString(stock.path[:]), stock.index, nil
}
