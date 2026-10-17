// [INPUT]: Real Go Windows executables, PNGs, and original multilingual resources.
// [OUTPUT]: Icon application, repeated saves, changes, default restoration, and non-icon resource preservation tests.
// [POS]: PE file behavior tests runnable on macOS/Linux; not a substitute for Windows Shell validation.
// [PROTOCOL]: Update internal/appearance/AGENTS.md, tests/AGENTS.md, and platform validation records.
package appearance

import (
	"bytes"
	"debug/pe"
	"encoding/binary"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/tc-hib/winres"
)

func testWindowsEXE(t *testing.T, arch string) []byte {
	t.Helper()
	dir := t.TempDir()
	source, target := filepath.Join(dir, "main.go"), filepath.Join(dir, "app.exe")
	if err := os.WriteFile(source, []byte("package main\nfunc main() { println(\"icon test\") }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "build", "-ldflags=-s -w", "-o", target, source)
	cmd.Env = append(os.Environ(), "GOOS=windows", "GOARCH="+arch, "CGO_ENABLED=0")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build fixture: %v\n%s", err, output)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestPEIconRoundTrip(t *testing.T) {
	for _, arch := range []string{"amd64", "arm64"} {
		t.Run(arch, func(t *testing.T) {
			original := testWindowsEXE(t, arch)
			testPEIconRoundTrip(t, original)
			var rs winres.ResourceSet
			for _, lang := range []uint16{winres.LCIDNeutral, 0x409} {
				if err := rs.Set(winres.RT_RCDATA, winres.Name("KEEP"), lang, []byte("other resource")); err != nil {
					t.Fatal(err)
				}
			}
			// Fully restore real default icons, named resources, and multilingual resources after repeated changes.
			withIcon, _, err := rewritePEIcon(original, testIcon(t, color.NRGBA{B: 255, A: 255}))
			if err != nil {
				t.Fatal(err)
			}
			icons, err := winres.LoadFromEXE(bytes.NewReader(withIcon))
			if err != nil {
				t.Fatal(err)
			}
			icons.Walk(func(kind, id winres.Identifier, lang uint16, data []byte) bool {
				if kind == winres.RT_ICON || kind == winres.RT_GROUP_ICON {
					if err := rs.Set(kind, id, lang, data); err != nil {
						t.Fatal(err)
					}
				}
				return true
			})
			if err := rs.Set(winres.RT_GROUP_ICON, winres.Name("DEFAULT"), 0x409, icons.Get(winres.RT_GROUP_ICON, winres.ID(1), winres.LCIDNeutral)); err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			if err := rs.WriteToEXE(&out, bytes.NewReader(original)); err != nil {
				t.Fatal(err)
			}
			testPEIconRoundTrip(t, out.Bytes())
		})
	}
}

func testPEIconRoundTrip(t *testing.T, original []byte) {
	t.Helper()
	current := original
	for _, c := range []color.NRGBA{{R: 255, A: 255}, {G: 255, A: 255}} {
		encoded := testIcon(t, c)
		updated, changed, err := rewritePEIcon(current, encoded)
		if err != nil || !changed {
			t.Fatalf("apply: changed=%v err=%v", changed, err)
		}
		if _, changed, err := rewritePEIcon(updated, encoded); err != nil || changed {
			t.Fatalf("same icon: changed=%v err=%v", changed, err)
		}
		rs, err := winres.LoadFromEXE(bytes.NewReader(updated))
		if err != nil {
			t.Fatal(err)
		}
		group := rs.Get(winres.RT_GROUP_ICON, winres.ID(1), winres.LCIDNeutral)
		if len(group) < 6 || binary.LittleEndian.Uint16(group[4:]) != 9 {
			t.Fatal("native icon ID 1 does not contain all sizes")
		}
		for offset := 6; offset < len(group); offset += 14 {
			id := winres.ID(binary.LittleEndian.Uint16(group[offset+12:]))
			img, err := png.Decode(bytes.NewReader(rs.Get(winres.RT_ICON, id, winres.LCIDNeutral)))
			if err != nil {
				t.Fatal(err)
			}
			if got := color.NRGBAModel.Convert(img.At(img.Bounds().Dx()/2, img.Bounds().Dy()/2)); got != c {
				t.Fatalf("embedded icon pixel = %v, want %v", got, c)
			}
		}
		current = updated
	}
	restored, changed, err := rewritePEIcon(current, "")
	if err != nil || !changed {
		t.Fatalf("restore: changed=%v err=%v", changed, err)
	}
	want, err := winres.LoadFromEXE(bytes.NewReader(original))
	if err != nil && err != winres.ErrNoResources {
		t.Fatal(err)
	}
	if err := verifyPEIconRewrite(original, restored, want); err != nil {
		t.Fatal(err)
	}
	if _, changed, err := rewritePEIcon(restored, ""); err != nil || changed {
		t.Fatalf("already default: changed=%v err=%v", changed, err)
	}
}

func TestPEIconRejectsSignedOrInvalidInput(t *testing.T) {
	original := testWindowsEXE(t, "amd64")
	encoded := testIcon(t, color.NRGBA{A: 255})
	for _, input := range [][]byte{nil, []byte("broken executable")} {
		if _, _, err := rewritePEIcon(input, encoded); err == nil {
			t.Fatal("invalid PE accepted")
		}
	}
	if _, _, err := rewritePEIcon(original, "not PNG"); err == nil {
		t.Fatal("invalid PNG accepted")
	}
	// Certificate directory is in PE32+ optional header, after 112 bytes.
	peOffset := int(binary.LittleEndian.Uint32(original[0x3c:]))
	certificateOffset := peOffset + 4 + 20 + 112 + pe.IMAGE_DIRECTORY_ENTRY_SECURITY*8
	signed := bytes.Clone(original)
	binary.LittleEndian.PutUint32(signed[certificateOffset:], uint32(len(signed)))
	binary.LittleEndian.PutUint32(signed[certificateOffset+4:], 8)
	signed = append(signed, make([]byte, 8)...)
	if _, _, err := rewritePEIcon(signed, encoded); err == nil {
		t.Fatal("signed PE accepted")
	}
}

func TestPackagedPEIconRoundTrip(t *testing.T) {
	path := os.Getenv("PAGEDOCK_TEST_WINDOWS_EXE")
	if path == "" {
		t.Skip("set PAGEDOCK_TEST_WINDOWS_EXE to verify a packaged binary")
	}
	data, err := ReadExecutable(path)
	if err != nil {
		t.Fatal(err)
	}
	testPEIconRoundTrip(t, data)
}
