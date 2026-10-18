// [INPUT]: Temporary PNG/JPEG files, damaged images, and config storage.
// [OUTPUT]: Icon visual sizing, antialiased corners, alpha/aspect preservation, repeat import, storage, and invalid-image rejection tests.
// [POS]: Icon input boundary tests without real desktops or user application bundles.
// [PROTOCOL]: Update this header, internal/AGENTS.md, and tests/AGENTS.md.
package pagedock

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	prefs "github.com/ohxxx/PageDock/internal/config"
	views "github.com/ohxxx/PageDock/internal/view"
)

func testIcon(t *testing.T, value color.NRGBA) string {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, iconSize, iconSize))
	for y := range iconSize {
		for x := range iconSize {
			img.SetNRGBA(x, y, value)
		}
	}
	var data bytes.Buffer
	if err := png.Encode(&data, img); err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(data.Bytes())
}

func TestIconImportAndStorage(t *testing.T) {
	for _, format := range []string{"png", "jpeg"} {
		t.Run(format, func(t *testing.T) {
			src := image.NewNRGBA(image.Rect(0, 0, 120, 60))
			for y := range 60 {
				for x := range 120 {
					src.SetNRGBA(x, y, color.NRGBA{R: 255, A: 255})
				}
			}
			var data bytes.Buffer
			if format == "png" {
				if err := png.Encode(&data, src); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := jpeg.Encode(&data, src, nil); err != nil {
					t.Fatal(err)
				}
			}
			path := filepath.Join(t.TempDir(), "icon."+format)
			if err := os.WriteFile(path, data.Bytes(), 0o600); err != nil {
				t.Fatal(err)
			}
			encoded, err := prefs.ReadIcon(path)
			if err != nil {
				t.Fatal(err)
			}
			pngData, err := prefs.DecodeIcon(encoded)
			if err != nil {
				t.Fatal(err)
			}
			img, err := png.Decode(bytes.NewReader(pngData))
			if err != nil {
				t.Fatal(err)
			}
			r, _, _, a := img.At(iconSize/2, iconSize/2).RGBA()
			_, _, _, edgeAlpha := img.At(0, 0).RGBA()
			if r < 0xf000 || a != 0xffff || edgeAlpha != 0 {
				t.Fatalf("wrong pixels: center=%x/%x edge=%x", r, a, edgeAlpha)
			}
			store := prefs.Store{Path: filepath.Join(t.TempDir(), prefs.Filename)}
			want := prefs.Config{URL: "https://example.com/", IconPNG: encoded}
			if err := store.Save(want); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			got, err := store.Load()
			if err != nil || got != want {
				t.Fatalf("image depends on source path: %v", err)
			}
			if views.IconPreview(got.IconPNG) == nil {
				t.Fatal("saved image cannot be previewed")
			}
		})
	}
}

func TestIconRejectsInvalidInput(t *testing.T) {
	var huge bytes.Buffer
	if err := png.Encode(&huge, image.NewNRGBA(image.Rect(0, 0, maxIconDimension+1, 1))); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{"invalid": []byte("broken"), "oversized": bytes.Repeat([]byte{0}, maxIconBytes+1), "dimension": huge.Bytes()} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), name)
			if err := os.WriteFile(path, data, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := prefs.ReadIcon(path); err == nil {
				t.Fatal("accepted invalid image")
			}
		})
	}
	valid := testIcon(t, color.NRGBA{G: 255, A: 255})
	raw, _ := base64.StdEncoding.DecodeString(valid)
	for _, encoded := range []string{"not base64", base64.StdEncoding.EncodeToString(raw[:len(raw)/2])} {
		if _, err := prefs.DecodeIcon(encoded); err == nil {
			t.Fatal("accepted corrupt stored PNG")
		}
	}
	store := prefs.Store{Path: filepath.Join(t.TempDir(), prefs.Filename)}
	want := prefs.Config{URL: "https://example.com/", IconPNG: valid}
	if err := store.Save(want); err != nil {
		t.Fatal(err)
	}
	bad := want
	bad.IconPNG = "bad"
	if err := store.Save(bad); err == nil {
		t.Fatal("bad icon overwrote config")
	}
	if got, err := store.Load(); err != nil || got != want {
		t.Fatal("previous icon lost")
	}
}

func TestIconImportAddsPaddingAndRoundedCorners(t *testing.T) {
	for _, format := range []string{"png", "jpeg"} {
		t.Run(format, func(t *testing.T) {
			src := image.NewNRGBA(image.Rect(0, 0, 96, 96))
			draw.Draw(src, src.Bounds(), image.NewUniform(color.NRGBA{R: 40, G: 100, B: 220, A: 255}), image.Point{}, draw.Src)
			got := importIconFixture(t, src, format)
			if bounds := importedIconBounds(got); bounds != image.Rect(51, 51, 461, 461) {
				t.Fatalf("visible bounds = %v; want centered artwork occupying about 80%% of the canvas", bounds)
			}
			for _, point := range []image.Point{{51, 51}, {460, 51}, {51, 460}, {460, 460}} {
				if _, _, _, a := got.At(point.X, point.Y).RGBA(); a != 0 {
					t.Fatalf("corner %v is opaque: %x", point, a)
				}
			}
			if _, _, blue, a := got.At(256, 256).RGBA(); blue < 0xc000 || a != 0xffff {
				t.Fatal("rounding changed the center artwork")
			}
			antialiased := false
			for y := 51; y < 140; y++ {
				for x := 51; x < 140; x++ {
					_, _, _, a := got.At(x, y).RGBA()
					antialiased = antialiased || (a > 0 && a < 0xffff)
				}
			}
			if !antialiased {
				t.Fatal("rounded edge has no antialiasing")
			}
		})
	}
}

func TestIconImportDoesNotAccumulatePadding(t *testing.T) {
	plain := image.NewNRGBA(image.Rect(0, 0, 100, 100))
	ink := image.NewUniform(color.NRGBA{R: 80, G: 150, B: 240, A: 255})
	draw.Draw(plain, plain.Bounds(), ink, image.Point{}, draw.Src)
	want := importIconFixture(t, plain, "png")
	for _, bounds := range []image.Rectangle{image.Rect(10, 10, 110, 110), image.Rect(7, 15, 107, 115)} {
		padded := image.NewNRGBA(image.Rect(0, 0, 120, 140))
		draw.Draw(padded, bounds, ink, image.Point{}, draw.Src)
		assertIconPixelsEqual(t, importIconFixture(t, padded, "png"), want)
	}
	// Re-importing an already normalized PNG must not shrink or round it again.
	assertIconPixelsEqual(t, importIconFixture(t, want, "png"), want)
}

func TestIconImportPreservesTransparentShape(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 100, 100))
	// Separate translucent corner marks must keep their silhouette and center hole.
	ink := image.NewUniform(color.NRGBA{R: 90, G: 40, B: 200, A: 128})
	for _, bounds := range []image.Rectangle{
		image.Rect(0, 0, 20, 20), image.Rect(80, 0, 100, 20),
		image.Rect(0, 80, 20, 100), image.Rect(80, 80, 100, 100),
	} {
		draw.Draw(src, bounds, ink, image.Point{}, draw.Src)
	}
	got := importIconFixture(t, src, "png")
	if bounds := importedIconBounds(got); bounds != image.Rect(51, 51, 461, 461) {
		t.Fatalf("transparent shape bounds = %v", bounds)
	}
	for _, point := range []image.Point{{65, 65}, {446, 65}, {65, 446}, {446, 446}} {
		if _, _, _, a := got.At(point.X, point.Y).RGBA(); a != 0x8080 {
			t.Fatalf("existing corner/alpha at %v was changed: %x", point, a)
		}
	}
	if _, _, _, a := got.At(256, 256).RGBA(); a != 0 {
		t.Fatal("transparent center was filled")
	}
}

func TestIconImportPreservesAspectRatio(t *testing.T) {
	for _, size := range []image.Point{{120, 60}, {57, 113}, {1, 100}} {
		src := image.NewNRGBA(image.Rectangle{Max: size})
		draw.Draw(src, src.Bounds(), image.NewUniform(color.NRGBA{G: 150, A: 255}), image.Point{}, draw.Src)
		got := importIconFixture(t, src, "png")
		bounds := importedIconBounds(got)
		if max(bounds.Dx(), bounds.Dy()) != 410 {
			t.Fatalf("%v: visible size = %v", size, bounds.Size())
		}
		if delta := bounds.Dx()*size.Y - bounds.Dy()*size.X; delta < -max(size.X, size.Y) || delta > max(size.X, size.Y) {
			t.Fatalf("%v was stretched to %v", size, bounds.Size())
		}
		for _, sum := range []int{bounds.Min.X + bounds.Max.X, bounds.Min.Y + bounds.Max.Y} {
			if sum < 511 || sum > 512 {
				t.Fatalf("%v is not centered: %v", size, bounds)
			}
		}
	}
}

func TestIconImportPreservesEmptyTransparency(t *testing.T) {
	got := importIconFixture(t, image.NewNRGBA(image.Rect(0, 0, 19, 23)), "png")
	if !importedIconBounds(got).Empty() || got.Bounds() != image.Rect(0, 0, 512, 512) {
		t.Fatal("empty transparent input gained content or lost its canvas")
	}
}

func importIconFixture(t *testing.T, src image.Image, format string) image.Image {
	t.Helper()
	var data bytes.Buffer
	var err error
	if format == "jpeg" {
		err = jpeg.Encode(&data, src, nil)
	} else {
		err = png.Encode(&data, src)
	}
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "input."+format)
	if err := os.WriteFile(path, data.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	encoded, err := prefs.ReadIcon(path)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := prefs.DecodeIcon(encoded)
	if err != nil {
		t.Fatal(err)
	}
	got, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func importedIconBounds(img image.Image) image.Rectangle {
	var bounds image.Rectangle
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a != 0 {
				bounds = bounds.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	return bounds
}

func assertIconPixelsEqual(t *testing.T, got, want image.Image) {
	t.Helper()
	if got.Bounds() != want.Bounds() {
		t.Fatalf("canvas = %v; want %v", got.Bounds(), want.Bounds())
	}
	for y := got.Bounds().Min.Y; y < got.Bounds().Max.Y; y++ {
		for x := got.Bounds().Min.X; x < got.Bounds().Max.X; x++ {
			if color.NRGBAModel.Convert(got.At(x, y)) != color.NRGBAModel.Convert(want.At(x, y)) {
				t.Fatalf("pixel (%d, %d) changed after import", x, y)
			}
		}
	}
}

const (
	iconSize         = 512
	maxIconBytes     = 8 << 20
	maxIconDimension = 4096
)
