//go:build desktop && darwin

// [INPUT]: Native application icons and default settings in an isolated desktop test process.
// [OUTPUT]: Real NSApplication icon synchronization on default restoration, preventing stale Dock icons after bundle-only updates.
// [POS]: Runtime system-icon behavior tests without changes to user applications or system preferences.
// [PROTOCOL]: Update internal/appearance/macos/AGENTS.md, tests/AGENTS.md, and validation records.
package macos

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo"
	prefs "github.com/ohxxx/PageDock/internal/config"
)

func TestDesktopDefaultIconReplacesRunningIcon(t *testing.T) {
	red, err := prefs.DecodeIcon(testIcon(t, color.NRGBA{R: 255, A: 255}))
	if err != nil {
		t.Fatal(err)
	}
	if err := mygo.App.Dock.SetIcon(red); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = mygo.App.Dock.SetIcon(nil) })
	if !redIcon(t, runningIcon(t)) {
		t.Fatal("native icon fixture was not applied")
	}
	if _, err := Apply(prefs.Config{}, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if redIcon(t, runningIcon(t)) {
		t.Fatal("default setting left the previous icon in NSApplication / Dock")
	}
}

func runningIcon(t *testing.T) image.Image {
	t.Helper()
	var encoded string
	mygo.RunOnMain(func() {
		app := objc.ID(objc.GetClass("NSApplication")).Send(objc.RegisterName("sharedApplication"))
		icon := app.Send(objc.RegisterName("applicationIconImage"))
		data := icon.Send(objc.RegisterName("TIFFRepresentation"))
		rep := objc.ID(objc.GetClass("NSBitmapImageRep")).Send(objc.RegisterName("imageRepWithData:"), data)
		properties := objc.ID(objc.GetClass("NSDictionary")).Send(objc.RegisterName("dictionary"))
		const pngFileType = 4
		data = rep.Send(objc.RegisterName("representationUsingType:properties:"), uintptr(pngFileType), properties)
		text := data.Send(objc.RegisterName("base64EncodedStringWithOptions:"), uintptr(0))
		encoded = objc.Send[string](text, objc.RegisterName("UTF8String"))
	})
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func redIcon(t *testing.T, img image.Image) bool {
	t.Helper()
	b := img.Bounds()
	r, g, blue, a := img.At(b.Min.X+b.Dx()/2, b.Min.Y+b.Dy()/2).RGBA()
	t.Logf("native icon center rgba=%x,%x,%x,%x", r, g, blue, a)
	return r > 0xc000 && r > g*2 && r > blue*2 && a > 0xf000
}
