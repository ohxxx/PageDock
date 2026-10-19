// [INPUT]: Composed source replacements and a disposable Go program.
// [OUTPUT]: Every replacement affects executed behavior; overlapping contexts fail visibly.
// [POS]: Regression for preparation silently skipping patches after intermediate formatting.
// [PROTOCOL]: Update tools/mygo-patch/main.go and tests/AGENTS.md.
package main

import (
	"go/format"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestComposedPatchesExecute(t *testing.T) {
	source := []byte(`package main
import "fmt"
type state struct {
	ready        bool
}

func main() { fmt.Print(state{}.ready) }
`)
	for _, patch := range [][2]string{
		{"ready        bool", "ready        bool\n\tpermissionCancel func()"},
		{"ready        bool", "ready        bool\n\tresourceCount int"},
		{"fmt.Print(state{}.ready)", "s := state{resourceCount: 21, permissionCancel: func(){}}; s.permissionCancel(); fmt.Print(s.resourceCount * 2)"},
	} {
		var err error
		source, err = applyReplacement(source, patch[0], patch[1])
		if err != nil {
			t.Fatal(err)
		}
	}
	source, err := format.Source(source)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "main.go")
	if err := os.WriteFile(path, source, 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "run", path)
	cmd.Env = append(os.Environ(), "GOFLAGS=", "GOWORK=off")
	if output, err := cmd.CombinedOutput(); err != nil || strings.TrimSpace(string(output)) != "42" {
		t.Fatalf("composed program: %v\n%s", err, output)
	}
	if _, err := applyReplacement(source, "ready        bool", "invalid"); err == nil {
		t.Fatal("changed context was silently accepted")
	}
}

func TestRemovedPatchesDoNotSurvivePublication(t *testing.T) {
	upstream, cache := t.TempDir(), t.TempDir()
	for name, contents := range map[string]string{
		"go.mod":    "module example.com/patchfixture\n\ngo 1.27.1\n",
		"main.go":   "package main\nimport \"fmt\"\nvar base = 1\nfunc main() { fmt.Print(answer()) }\n",
		"answer.go": "package main\nfunc answer() int { return base }\n",
	} {
		if err := os.WriteFile(filepath.Join(upstream, name), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string][]byte{
		"answer.go": []byte("package main\nfunc answer() int { return base + 1 }\n"),
		"extra.go":  []byte("package main\nfunc init() { base = 41 }\n"),
	}
	var group sync.WaitGroup
	paths, failures := make([]string, 3), make([]error, 3)
	for i := range paths {
		group.Go(func() { paths[i], failures[i] = publishDependency(cache, upstream, files) })
	}
	group.Wait()
	for i, err := range failures {
		if err != nil || paths[i] != paths[0] {
			t.Fatalf("concurrent publication %d: %q %v", i, paths[i], err)
		}
	}
	restored, err := publishDependency(cache, upstream, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, check := range []struct{ path, want string }{{paths[0], "42"}, {restored, "1"}, {paths[0], "42"}} {
		cmd := exec.Command("go", "run", ".")
		cmd.Dir = check.path
		cmd.Env = append(os.Environ(), "GOFLAGS=", "GOWORK=off")
		if output, err := cmd.CombinedOutput(); err != nil || strings.TrimSpace(string(output)) != check.want {
			t.Fatalf("published program: want %s, got %s (%v)", check.want, output, err)
		}
	}
}
