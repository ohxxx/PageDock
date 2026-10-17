//go:build darwin

// [INPUT]: The current MyGo locale, re-registered macOS bundle icon, and the current user's notification process.
// [OUTPUT]: Wait for IconServices invalidation, then reload Notification Center; bundle transactions handle failure rollback.
// [POS]: System cache boundary used only for icon changes/restoration; preserve notification identity, authorization, and history.
// [PROTOCOL]: Update bundle.go, the directory AGENTS.md, and real notification validation records in tests/AGENTS.md.
package macos

import (
	"errors"
	"fmt"
	"runtime"
	"slices"
	"structs"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
	l10n "github.com/ohxxx/PageDock/internal/locale"
)

const iconServicesPath = "/System/Library/PrivateFrameworks/IconServices.framework/IconServices"

var loadIconServices = sync.OnceValue(func() error {
	_, err := purego.Dlopen(iconServicesPath, purego.RTLD_NOW|purego.RTLD_LOCAL)
	return err
})

func refreshNotificationIcon() error {
	if err := invalidateSystemIcons(); err != nil {
		return fmt.Errorf(l10n.Text("iconCacheRefresh"), err)
	}
	// Notification Center keeps rendered images in memory; IconServices invalidation does not replace them.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	pool := objc.ID(objc.GetClass("NSAutoreleasePool")).Send(objc.RegisterName("new"))
	defer pool.Send(objc.RegisterName("drain"))
	identifier := objc.ID(objc.GetClass("NSString")).Send(objc.RegisterName("stringWithUTF8String:"), "com.apple.notificationcenterui")
	apps := objc.ID(objc.GetClass("NSRunningApplication")).Send(objc.RegisterName("runningApplicationsWithBundleIdentifier:"), identifier)
	for i, count := uintptr(0), uintptr(apps.Send(objc.RegisterName("count"))); i < count; i++ {
		app := apps.Send(objc.RegisterName("objectAtIndex:"), i)
		pid := int(objc.Send[int32](app, objc.RegisterName("processIdentifier")))
		if pid <= 0 {
			return errors.New(l10n.Text("notificationCenterProcess"))
		}
		if err := syscall.Kill(pid, syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
			return fmt.Errorf(l10n.Text("notificationCenterReload"), err)
		}
		deadline := time.Now().Add(3 * time.Second)
		for !objc.Send[bool](app, objc.RegisterName("isTerminated")) {
			if time.Now().After(deadline) {
				return errors.New(l10n.Text("notificationCenterTimeout"))
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
	return nil
}

func invalidateSystemIcons() error {
	if err := loadIconServices(); err != nil {
		return err
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	pool := objc.ID(objc.GetClass("NSAutoreleasePool")).Send(objc.RegisterName("new"))
	defer pool.Send(objc.RegisterName("drain"))
	managerClass := objc.ID(objc.GetClass("ISIconManager"))
	shared := objc.RegisterName("sharedInstance")
	connectionSelector := objc.RegisterName("connection")
	responds := objc.RegisterName("respondsToSelector:")
	// shortcut: macOS has no public notification icon cache refresh API; reject updates if private APIs change until the adapter is updated.
	if managerClass == 0 || !objc.Send[bool](managerClass, responds, shared) {
		return errors.New(l10n.Text("iconCacheUnsupported"))
	}
	manager := managerClass.Send(shared)
	if !objc.Send[bool](manager, responds, connectionSelector) {
		return errors.New(l10n.Text("iconCacheUnsupported"))
	}
	connection := manager.Send(connectionSelector)
	if connection == 0 {
		return errors.New(l10n.Text("iconServiceUnavailable"))
	}
	protocol := objc.GetProtocol("ISIconCacheServiceProtocol")
	if protocol == nil || !slices.ContainsFunc(protocol.CopyMethodDescriptionList(true, true), func(m objc.MethodDescription) bool {
		return m.Name() == "clearAllCachedItemsWithReply:"
	}) {
		return errors.New(l10n.Text("iconCacheUnsupported"))
	}
	done := make(chan error, 1)
	finish := func(err error) {
		select {
		case done <- err:
		default:
		}
	}
	onError := objc.NewBlock(func(_ objc.Block, err objc.ID) { finish(iconCacheError(err)) })
	defer onError.Release()
	onReply := iconCacheReply(func(_ objc.Block, success bool, err objc.ID) {
		if err != 0 || !success {
			finish(iconCacheError(err))
		} else {
			finish(nil)
		}
	})
	defer onReply.Release()
	proxy := connection.Send(objc.RegisterName("remoteObjectProxyWithErrorHandler:"), onError)
	clear := objc.RegisterName("clearAllCachedItemsWithReply:")
	if proxy == 0 {
		return errors.New(l10n.Text("iconCacheUnsupported"))
	}
	proxy.Send(clear, onReply)
	select {
	case err := <-done:
		return err
	case <-time.After(10 * time.Second):
		return errors.New(l10n.Text("iconCacheTimeout"))
	}
}

// shortcut: purego 0.11.1 encodes the block itself as @, but NSXPC requires @?; remove this ABI adapter when the dependency is fixed.
// https://clang.llvm.org/docs/Block-ABI-Apple.html#high-level
type iconBlockDescriptor struct {
	_                          structs.HostLayout
	Reserved, Size, Copy, Drop uintptr
	Signature                  *byte
}

type iconBlockLiteral struct {
	_          structs.HostLayout
	Class      uintptr
	Flags      uint32
	Reserved   uint32
	Invoke     uintptr
	Descriptor *iconBlockDescriptor
}

var (
	iconReplySignature  = []byte("v@?B@\x00")
	iconReplyDescriptor iconBlockDescriptor
	iconReplyOnce       sync.Once
)

func iconCacheReply(fn func(objc.Block, bool, objc.ID)) objc.Block {
	block := objc.NewBlock(fn)
	literal := *(**iconBlockLiteral)(unsafe.Pointer(&block))
	iconReplyOnce.Do(func() {
		iconReplyDescriptor = *literal.Descriptor
		if runtime.GOARCH == "amd64" {
			iconReplySignature[3] = 'c' // Intel macOS encodes BOOL as signed char.
		}
		iconReplyDescriptor.Signature = &iconReplySignature[0]
	})
	literal.Descriptor = &iconReplyDescriptor
	return block
}

func iconCacheError(err objc.ID) error {
	if err == 0 {
		return errors.New(l10n.Text("iconCacheIncomplete"))
	}
	description := err.Send(objc.RegisterName("description"))
	return errors.New(objc.Send[string](description, objc.RegisterName("UTF8String")))
}
