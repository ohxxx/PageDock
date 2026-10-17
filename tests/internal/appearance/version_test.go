// [INPUT]: Real x64/ARM64 EXEs, multilingual version resources, and name/icon config.
// [OUTPUT]: Combined name/icon changes, identity preservation, default restoration, signed-file rejection, and name-string resource tests.
// [POS]: Windows file behavior tests runnable on every host; not a substitute for system presentation validation.
// [PROTOCOL]: Update internal/appearance/AGENTS.md, tests/AGENTS.md, and platform validation records.
package appearance

import (
	"bytes"
	"crypto/rand"
	"debug/pe"
	"encoding/binary"
	"image/color"
	"os"
	"testing"
	"unicode/utf16"

	prefs "github.com/ohxxx/PageDock/internal/config"
	"github.com/tc-hib/winres"
	"github.com/tc-hib/winres/version"
)

func windowsNamedFixture(t *testing.T, arch string) []byte {
	t.Helper()
	original := testWindowsEXE(t, arch)
	var rs winres.ResourceSet
	identifier := "com.ohxxx.pagedock.name-test." + rand.Text()
	for _, lang := range []uint16{version.LangDefault, 0x804} {
		info := &version.Info{FileVersion: [4]uint16{2, 3, 4, 5}, ProductVersion: [4]uint16{6, 7, 8, 9}}
		for key, value := range map[string]string{version.ProductName: "Original app", version.FileDescription: "Original description", version.CompanyName: "Keep company", "MyGoIdentifier": identifier, version.OriginalFilename: "keep.exe", version.ProductVersion: "6.7.8.9"} {
			if err := info.Set(lang, key, value); err != nil {
				t.Fatal(err)
			}
		}
		if err := rs.Set(winres.RT_VERSION, winres.ID(1), lang, info.Bytes()); err != nil {
			t.Fatal(err)
		}
	}
	if err := rs.Set(winres.RT_RCDATA, winres.Name("KEEP"), 0, []byte("keep other data")); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := rs.WriteToEXE(&out, bytes.NewReader(original)); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func TestPEAppearanceRoundTrip(t *testing.T) {
	for _, arch := range []string{"amd64", "arm64"} {
		t.Run(arch, func(t *testing.T) { testPEAppearanceRoundTrip(t, windowsNamedFixture(t, arch)) })
	}
}

func testPEAppearanceRoundTrip(t *testing.T, original []byte) {
	t.Helper()
	before, err := winres.LoadFromEXE(bytes.NewReader(original))
	if err != nil {
		t.Fatal(err)
	}
	_, identifier, err := PEIdentity(original)
	if err != nil {
		t.Fatal(err)
	}
	current := original
	for _, config := range []prefs.Config{
		{AppName: "工作台 🖥 / <消息>"},
		{AppName: "工作台 🖥 / <消息>", IconPNG: testIcon(t, color.NRGBA{G: 255, A: 255})},
		{AppName: "第二个名称", IconPNG: testIcon(t, color.NRGBA{G: 255, A: 255})},
		{IconPNG: testIcon(t, color.NRGBA{B: 255, A: 255})},
		{},
	} {
		updated, changed, err := RewritePE(current, config)
		if err != nil || !changed {
			t.Fatalf("update %q: %v %v", config.AppName, changed, err)
		}
		if _, changed, err := RewritePE(updated, config); err != nil || changed {
			t.Fatalf("idempotence: %v %v", changed, err)
		}
		rs, err := winres.LoadFromEXE(bytes.NewReader(updated))
		if err != nil {
			t.Fatal(err)
		}
		gotName, gotID, err := PEIdentity(updated)
		if err != nil || gotID != identifier {
			t.Fatalf("identity changed: %q %v", gotID, err)
		}
		if config.AppName != "" {
			if gotName != config.AppName {
				t.Fatalf("system name=%q", gotName)
			}
			block := rs.Get(winres.RT_STRING, peDisplayNameBlock, winres.LCIDNeutral)
			length := int(binary.LittleEndian.Uint16(block))
			chars := make([]uint16, length)
			for i := range chars {
				chars[i] = binary.LittleEndian.Uint16(block[2+i*2:])
			}
			if string(utf16.Decode(chars)) != config.AppName {
				t.Fatal("shortcut display resource differs")
			}
			rs.Walk(func(kind, id winres.Identifier, lang uint16, data []byte) bool {
				if kind != winres.RT_VERSION {
					return true
				}
				info, err := version.FromBytes(data)
				if err != nil {
					t.Fatal(err)
				}
				old, err := version.FromBytes(before.Get(kind, id, lang))
				if err != nil {
					t.Fatal(err)
				}
				if info.FileVersion != old.FileVersion || info.ProductVersion != old.ProductVersion {
					t.Fatal("version numbers changed")
				}
				for language, table := range info.Table() {
					originalTable := old.Table()[language]
					for key, value := range *originalTable {
						if key == version.FileDescription || key == version.ProductName {
							value = config.AppName
						}
						if (*table)[key] != value {
							t.Fatalf("%s changed unexpectedly: %q", key, (*table)[key])
						}
					}
				}
				return true
			})
		} else if rs.Get(winres.RT_RCDATA, peNameStateName, 0) != nil || rs.Get(winres.RT_STRING, peDisplayNameBlock, 0) != nil {
			t.Fatal("name resources remain after reset")
		}
		current = updated
	}
	if err := verifyPEIconRewrite(original, current, before); err != nil {
		t.Fatalf("original resources not restored: %v", err)
	}
}

func TestPENameRejectsSignedAndCorruptState(t *testing.T) {
	original := windowsNamedFixture(t, "amd64")
	offset := int(binary.LittleEndian.Uint32(original[0x3c:])) + 4 + 20 + 112 + pe.IMAGE_DIRECTORY_ENTRY_SECURITY*8
	signed := bytes.Clone(original)
	binary.LittleEndian.PutUint32(signed[offset:], uint32(len(signed)))
	binary.LittleEndian.PutUint32(signed[offset+4:], 8)
	signed = append(signed, make([]byte, 8)...)
	if _, _, err := RewritePE(signed, prefs.Config{AppName: "改名"}); err == nil {
		t.Fatal("signed executable was rewritten")
	}
	for _, corrupt := range []bool{true, false} {
		rs, err := winres.LoadFromEXE(bytes.NewReader(original))
		if err != nil {
			t.Fatal(err)
		}
		if corrupt {
			err = rs.Set(winres.RT_RCDATA, peNameStateName, 0, []byte("broken"))
		} else {
			err = rs.Set(winres.RT_STRING, peDisplayNameBlock, 0, peNameString("unrelated resource"))
		}
		if err != nil {
			t.Fatal(err)
		}
		var out bytes.Buffer
		if err = rs.WriteToEXE(&out, bytes.NewReader(original)); err != nil {
			t.Fatal(err)
		}
		if _, _, err = RewritePE(out.Bytes(), prefs.Config{AppName: "改名"}); err == nil {
			t.Fatal("corrupt snapshot or occupied name resource accepted")
		}
	}
}

func TestPackagedPEAppearanceRoundTrip(t *testing.T) {
	path := os.Getenv("PAGEDOCK_TEST_WINDOWS_EXE")
	if path == "" {
		t.Skip("set PAGEDOCK_TEST_WINDOWS_EXE to verify a packaged binary")
	}
	data, err := ReadExecutable(path)
	if err != nil {
		t.Fatal(err)
	}
	testPEAppearanceRoundTrip(t, data)
}
