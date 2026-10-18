//go:build desktop && darwin

// [INPUT]: Native application icons and default settings in an isolated desktop test process.
// [OUTPUT]: Real NSApplication icon padding/corners after import and default restoration without stale Dock images.
// [POS]: Runtime system-icon behavior tests without changes to user applications or system preferences.
// [PROTOCOL]: Update internal/appearance/macos/AGENTS.md, tests/AGENTS.md, and validation records.
package macos

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
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

func TestDesktopImportedIconRetainsShape(t *testing.T) {
	raw, err := prefs.DecodeIcon(testIcon(t, color.NRGBA{R: 255, A: 255}))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "square.png")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	encoded, err := prefs.ReadIcon(path)
	if err != nil {
		t.Fatal(err)
	}
	raw, err = prefs.DecodeIcon(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if err := mygo.App.Dock.SetIcon(raw); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = mygo.App.Dock.SetIcon(nil) })
	got := runningIcon(t)
	if !redIcon(t, got) {
		t.Fatal("native Dock icon lost the imported artwork")
	}
	size := got.Bounds().Dx()
	for _, point := range []image.Point{{0, size / 2}, {size / 2, 0}, {size / 10, size / 10}, {size - 1, size / 2}, {size / 2, size - 1}} {
		if _, _, _, alpha := got.At(point.X, point.Y).RGBA(); alpha != 0 {
			t.Fatalf("native icon lost padding/corners at %v: %x", point, alpha)
		}
	}
	first := 0
	for ; first < size; first++ {
		if _, _, _, alpha := got.At(size/2, first).RGBA(); alpha >= 0x8000 {
			break
		}
	}
	if first < size/10-1 || first > size/10+1 {
		t.Fatalf("native icon has its upper edge at %d of %d", first, size)
	}
	t.Logf("native icon canvas=%dx%d top margin=%d", size, got.Bounds().Dy(), first)
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
