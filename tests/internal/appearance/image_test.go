// [INPUT]: Solid-color pixels selected by tests.
// [OUTPUT]: Bounded PNG/base64 image fixtures.
// [POS]: Test input helpers that never read real user images.
// [PROTOCOL]: Update the owning AGENTS.md and behavior tests under tests/.
package appearance

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"testing"
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
