// [INPUT]: Temporary Go modules, private functions, and real tests under tests/.
// [OUTPUT]: Test discovery, private access, failure propagation, and missing/conflicting input behavior.
// [POS]: Execution tests for test assembly; avoid source matching or JSON-shape-only assertions.
// [PROTOCOL]: Update tools/test-overlay/main.go and tests/AGENTS.md.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestOverlayRunsPrivatePackageTests(t *testing.T) {
	root := t.TempDir()
	write := func(name, contents string) {
		t.Helper()
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.com/overlayfixture\n\ngo 1.27.1\n")
	write("tests/go.mod", "module example.com/overlayfixture/tests\n")
	write("private.go", "package fixture\nfunc answer() int { return 42 }\n")
	if _, err := prepareOverlay(root); err == nil {
		t.Fatal("empty suite accepted")
	}
	write("tests/private_test.go", `package fixture
import "testing"
func TestPrivate(t *testing.T) { if answer() != 42 { t.Fatal("private answer changed") } }
`)
	path, err := prepareOverlay(root)
	if err != nil {
		t.Fatal(err)
	}
	run := func() ([]byte, error) {
		cmd := exec.Command("go", "test", "-overlay="+path, "-count=1", "-v", ".")
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "GOFLAGS=", "GOWORK=off")
		return cmd.CombinedOutput()
	}
	if output, err := run(); err != nil || !strings.Contains(string(output), "--- PASS: TestPrivate") {
		t.Fatalf("private test did not run: %v\n%s", err, output)
	}
	write("private.go", "package fixture\nfunc answer() int { return 0 }\n")
	if output, err := run(); err == nil || !strings.Contains(string(output), "private answer changed") {
		t.Fatalf("test failure lost: %v\n%s", err, output)
	}
	write("private_test.go", "package fixture\n")
	if _, err := prepareOverlay(root); err == nil {
		t.Fatal("duplicate test path accepted")
	}
	if err := os.Remove(filepath.Join(root, "private_test.go")); err != nil {
		t.Fatal(err)
	}
	write("tests/missing/private_test.go", "package fixture\n")
	if _, err := prepareOverlay(root); err == nil {
		t.Fatal("missing product package accepted")
	}
}

func TestPlatformGroupsKeepPrivateAccessAndTags(t *testing.T) {
	root := t.TempDir()
	write := func(name, contents string) {
		t.Helper()
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.com/groups\n\ngo 1.27.1\n")
	write("tests/go.mod", "module example.com/groups/tests\n")
	write("private.go", "package fixture\nfunc answer() int { return 42 }\n")
	write("tests/packages.json", `{"windows":"."}`)
	write("tests/windows/input_test.go", `package fixture
import "testing"
func TestGrouped(t *testing.T) { if answer() != 42 { t.Fatal("private access lost") } }
`)
	write("tests/windows/desktop_test.go", `//go:build desktop

package fixture
import "testing"
func TestDesktopFailure(t *testing.T) { t.Fatal("desktop failure preserved") }
`)
	path, err := prepareOverlay(root)
	if err != nil {
		t.Fatal(err)
	}
	run := func(tags string) ([]byte, error) {
		cmd := exec.Command("go", "test", "-overlay="+path, "-tags="+tags, "-count=1", "-v", ".")
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "GOFLAGS=", "GOWORK=off")
		return cmd.CombinedOutput()
	}
	if output, err := run(""); err != nil || !strings.Contains(string(output), "--- PASS: TestGrouped") {
		t.Fatalf("grouped private test did not run: %v\n%s", err, output)
	}
	if output, err := run("desktop"); err == nil || !strings.Contains(string(output), "desktop failure preserved") {
		t.Fatalf("grouped build tag or failure lost: %v\n%s", err, output)
	}
	write("tests/windows_input_test.go", "package fixture\n")
	if _, err := prepareOverlay(root); err == nil {
		t.Fatal("grouped test collision accepted")
	}
	for _, invalid := range []string{`{"windows":"../outside"}`, `{"../outside":"."}`} {
		write("tests/packages.json", invalid)
		if _, err := prepareOverlay(root); err == nil {
			t.Fatal("outside package path accepted")
		}
	}
}
