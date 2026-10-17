//go:build desktop && darwin

// [INPUT]: Unique temporary application identities, changing bundle icons, and real IconServices.
// [OUTPUT]: Images resolved by the notification bundle identifier follow saves and default restoration.
// [POS]: System icon cache tests without restarting Notification Center or modifying user apps/config.
// [PROTOCOL]: Update the directory AGENTS.md and real notification validation records.
package macos

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo"
	prefs "github.com/ohxxx/PageDock/internal/config"
)

func TestDesktopNotificationIconCacheFollowsBundle(t *testing.T) {
	fixture, _ := iconTestBundle(t)
	// LaunchServices does not register apps in system temporary directories; use isolated bundles inside the repository for real cache tests.
	root := filepath.Join("..", "..", "..", ".scratch")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	dir, err := os.MkdirTemp(root, "notification-icon-cache-*")
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := filepath.Abs(filepath.Join(dir, "Icon QA.app"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = iconCommand(nil, launchServicesRegister, "-u", bundle)
		_ = os.RemoveAll(dir)
	})
	if _, err := iconCommand(nil, "/usr/bin/ditto", fixture, bundle); err != nil {
		t.Fatal(err)
	}
	info, err := readBundleInfo(filepath.Join(bundle, "Contents", "Info.plist"))
	if err != nil {
		t.Fatal(err)
	}
	identifier := info["CFBundleIdentifier"].(string)
	// Metadata transaction tests use nonexistent asset names; this default image must be a loadable real ICNS.
	delete(info, "CFBundleIconName")
	if err := writeBundleInfo(filepath.Join(bundle, "Contents", "Info.plist"), info); err != nil {
		t.Fatal(err)
	}
	if _, err := iconCommand(nil, "/usr/bin/codesign", "--force", "--sign", "-", bundle); err != nil {
		t.Fatal(err)
	}
	for _, red := range []bool{true, false, true} {
		encoded := ""
		if red {
			encoded = testIcon(t, color.NRGBA{R: 255, A: 255})
		}
		changed, err := updateBundleAppearance(bundle, prefs.Config{IconPNG: encoded}, func() error { return nil }, invalidateSystemIcons)
		if err != nil || !changed {
			t.Fatalf("update notification icon: changed=%v err=%v", changed, err)
		}
		// Separate processes model reloaded Notification Center, avoiding the test process's own memory cache.
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestDesktopNotificationIconRead$", "-test.count=1")
		cmd.Env = append(os.Environ(), "PAGEDOCK_TEST_ICON_BUNDLE="+identifier, "PAGEDOCK_TEST_ICON_RED="+strconv.FormatBool(red))
		out, err := cmd.CombinedOutput()
		cancel()
		if err != nil {
			t.Fatalf("fresh notification icon reader: %v\n%s", err, out)
		}
	}
}

func TestDesktopNotificationIconRead(t *testing.T) {
	identifier := os.Getenv("PAGEDOCK_TEST_ICON_BUNDLE")
	if identifier == "" {
		t.Skip("only run by the icon cache regression")
	}
	if err := loadIconServices(); err != nil {
		t.Fatal(err)
	}
	img := notificationBundleIcon(t, identifier)
	red := os.Getenv("PAGEDOCK_TEST_ICON_RED") == "true"
	r, g, b, _ := img.At(img.Bounds().Dx()/2, img.Bounds().Dy()/2).RGBA()
	if (red && !(r > 0xc000 && r > g*2 && r > b*2)) || (!red && !(g > 0xc000 && g > r*2 && g > b*2)) {
		t.Fatalf("notification cache retained previous icon: red=%v rgba=%x,%x,%x", red, r, g, b)
	}
}

func notificationBundleIcon(t *testing.T, identifier string) image.Image {
	t.Helper()
	encoded := make(chan string, 1)
	mygo.RunOnMain(func() {
		text := objc.ID(objc.GetClass("NSString")).Send(objc.RegisterName("stringWithUTF8String:"), identifier)
		icon := objc.ID(objc.GetClass("ISBundleIdentifierIcon")).Send(objc.RegisterName("alloc")).Send(objc.RegisterName("initWithBundleIdentifier:"), text)
		type size struct{ Width, Height float64 }
		descriptor := objc.ID(objc.GetClass("ISImageDescriptor")).Send(objc.RegisterName("alloc")).Send(objc.RegisterName("initWithSize:scale:"), size{64, 64}, float64(2))
		onImage := objc.NewBlock(func(_ objc.Block, image uintptr) {
			defer icon.Send(objc.RegisterName("release"))
			defer descriptor.Send(objc.RegisterName("release"))
			if image == 0 {
				encoded <- ""
				return
			}
			rep := objc.ID(objc.GetClass("NSBitmapImageRep")).Send(objc.RegisterName("alloc")).Send(objc.RegisterName("initWithCGImage:"), image)
			defer rep.Send(objc.RegisterName("release"))
			properties := objc.ID(objc.GetClass("NSDictionary")).Send(objc.RegisterName("dictionary"))
			const pngFileType = 4
			data := rep.Send(objc.RegisterName("representationUsingType:properties:"), uintptr(pngFileType), properties)
			text := data.Send(objc.RegisterName("base64EncodedStringWithOptions:"), uintptr(0))
			encoded <- objc.Send[string](text, objc.RegisterName("UTF8String"))
		})
		defer onImage.Release()
		icon.Send(objc.RegisterName("getCGImageForImageDescriptor:completion:"), descriptor, onImage)
	})
	select {
	case value := <-encoded:
		data, err := base64.StdEncoding.DecodeString(value)
		if err != nil {
			t.Fatal(err)
		}
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		return img
	case <-time.After(10 * time.Second):
		t.Fatal("system notification icon lookup timed out")
		return nil
	}
}
