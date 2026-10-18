//go:build windows

// [INPUT]: The Go compiler and temporary directories.
// [OUTPUT]: Canonical temporary paths and unsigned dual-architecture EXE fixtures with original multilingual resources.
// [POS]: Native Windows transaction test inputs that do not touch installed applications.
// [PROTOCOL]: Update tests/internal/appearance/windows/AGENTS.md and shared PE fixtures.
package windows

import (
	"bytes"
	"crypto/rand"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/tc-hib/winres"
	"github.com/tc-hib/winres/version"
)

func windowsFixtureDir(t *testing.T) string {
	t.Helper()
	// Match windowsIconExecutable before calling private transaction helpers; TEMP may use an 8.3 path.
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

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
