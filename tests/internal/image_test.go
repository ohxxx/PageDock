// [INPUT]: Temporary PNG/JPEG files, damaged images, and config storage.
// [OUTPUT]: Icon scaling, transparent padding, independent persistence, and invalid-image rejection tests.
// [POS]: Icon input boundary tests without real desktops or user application bundles.
// [PROTOCOL]: Update this header, internal/AGENTS.md, and tests/AGENTS.md.
package pagedock

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
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

const (
	iconSize         = 512
	maxIconBytes     = 8 << 20
	maxIconDimension = 4096
)
