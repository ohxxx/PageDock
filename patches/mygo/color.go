// [INPUT]: sRGB components of an opaque window background.
// [OUTPUT]: Whether Windows caption buttons should use white glyphs.
// [POS]: MyGo Windows patch copied into the isolated dependency during builds; excluded on other platforms.
// [PROTOCOL]: Update patches/mygo/AGENTS.md and run color behavior tests.
package windows

import "math"

func captionBackgroundIsDark(r, g, b uint8) bool {
	linear := func(v uint8) float64 {
		s := float64(v) / 255
		if s <= 0.04045 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	luminance := 0.2126*linear(r) + 0.7152*linear(g) + 0.0722*linear(b)
	// Equal black/white text contrast occurs when (L + .05)^2 = 1.05 * .05.
	return luminance < math.Sqrt(1.05*0.05)-0.05
}
