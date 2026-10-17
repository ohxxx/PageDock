// [INPUT]: Page backgrounds opposite to the system theme and colors near the black/white contrast threshold.
// [OUTPUT]: Windows caption buttons choose the higher-contrast black or white glyphs.
// [POS]: Patch color algorithm tests using the same implementation as production builds.
// [PROTOCOL]: Update patches/mygo/AGENTS.md.
package windows

import "testing"

func TestCaptionBackgroundContrast(t *testing.T) {
	for _, tc := range []struct {
		name    string
		r, g, b uint8
		dark    bool
	}{
		{"dark page", 18, 18, 18, true},
		{"light page", 248, 248, 248, false},
		{"dark settings", 33, 33, 33, true},
		{"light settings", 250, 250, 250, false},
		{"gray below crossover", 117, 117, 117, true},
		{"gray above crossover", 118, 118, 118, false},
		{"blue", 0, 0, 255, true},
		{"green", 0, 255, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := captionBackgroundIsDark(tc.r, tc.g, tc.b); got != tc.dark {
				t.Fatalf("background (%d,%d,%d): white glyphs = %v, want %v", tc.r, tc.g, tc.b, got, tc.dark)
			}
		})
	}
}
