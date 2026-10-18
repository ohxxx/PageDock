// [INPUT]: Synthetic PNGs imported through config and real temporary Windows executables.
// [OUTPUT]: Icon fixtures and decoded ICNS/PE pixels proving transparent margins and corners survive resource encoding.
// [POS]: Portable resource behavior tests; never read real user images or modify installed applications.
// [PROTOCOL]: Update the owning AGENTS.md and behavior tests under tests/.
package appearance

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	prefs "github.com/ohxxx/PageDock/internal/config"
	"github.com/tc-hib/winres"
)

const iconSize = 512

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

func TestImportedIconResourcesPreserveShape(t *testing.T) {
	raw, err := base64.StdEncoding.DecodeString(testIcon(t, color.NRGBA{B: 255, A: 255}))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "icon.png")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
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
	t.Run("icns", func(t *testing.T) {
		data, err := EncodeICNS(pngData)
		if err != nil {
			t.Fatal(err)
		}
		entries := 0
		for offset := 8; offset < len(data); entries++ {
			if len(data)-offset < 8 {
				t.Fatal("truncated ICNS entry")
			}
			size := int(binary.BigEndian.Uint32(data[offset+4:]))
			if size < 8 || size > len(data)-offset {
				t.Fatal("invalid ICNS entry size")
			}
			assertResourceIconShape(t, data[offset+8:offset+size])
			offset += size
		}
		if entries != 8 {
			t.Fatalf("ICNS has %d images; want 8", entries)
		}
	})
	t.Run("pe", func(t *testing.T) {
		data, _, err := rewritePEIcon(testWindowsEXE(t, "amd64"), encoded)
		if err != nil {
			t.Fatal(err)
		}
		resources, err := winres.LoadFromEXE(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		images := 0
		resources.Walk(func(kind, id winres.Identifier, lang uint16, data []byte) bool {
			if kind == winres.RT_ICON {
				assertResourceIconShape(t, data)
				images++
			}
			return true
		})
		if images != 9 {
			t.Fatalf("PE has %d images; want 9", images)
		}
	})
}

func assertResourceIconShape(t *testing.T, data []byte) {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	size := img.Bounds().Dx()
	for _, point := range []image.Point{{0, size / 2}, {size / 2, 0}, {size / 10, size / 10}, {size - 1, size / 2}, {size / 2, size - 1}} {
		if _, _, _, alpha := img.At(point.X, point.Y).RGBA(); alpha != 0 {
			t.Fatalf("%d-pixel resource lost transparency at %v: %x", size, point, alpha)
		}
	}
	if _, _, blue, alpha := img.At(size/2, size/2).RGBA(); blue != 0xffff || alpha != 0xffff {
		t.Fatal("resource changed the center artwork")
	}
	// The flat upper edge must remain near 10% of the canvas at every resolution.
	first := 0
	for ; first < size; first++ {
		if _, _, _, alpha := img.At(size/2, first).RGBA(); alpha >= 0x8000 {
			break
		}
	}
	if first < size/10-1 || first > size/10+1 {
		t.Fatalf("%d-pixel resource has its upper edge at %d", size, first)
	}
}
