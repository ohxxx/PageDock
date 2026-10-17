// [INPUT]: Pinned MyGo sources and Windows patches for colors, permission caches, dialogs, menus, and old EXE cleanup.
// [OUTPUT]: A separate MyGo build copy and modfile; preserve the module cache and fail on version/context drift.
// [POS]: Makefile build preparation; packaged apps, development, and tests use the same patches.
// [PROTOCOL]: Update the root AGENTS.md, patches/mygo/AGENTS.md, and Makefile.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/format"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	patches "github.com/ohxxx/PageDock/patches/mygo/windows"
)

const mygoModule = "github.com/egoist/mygo"
const mygoVersion = "v0.5.1"

func main() {
	if err := prepare(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func prepare() error {
	cmd := exec.Command("go", "list", "-modfile=go.mod", "-m", "-json", mygoModule)
	cmd.Stderr = os.Stderr
	data, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("locate MyGo: %w", err)
	}
	var module struct {
		Dir, Version string
		Replace      *json.RawMessage
	}
	if err := json.Unmarshal(data, &module); err != nil {
		return err
	}
	if module.Version != mygoVersion || module.Replace != nil || module.Dir == "" {
		return fmt.Errorf("Windows patches require unmodified %s %s", mygoModule, mygoVersion)
	}
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	cache := filepath.Join(root, ".cache", "mygo-patch")
	if err := os.MkdirAll(cache, 0755); err != nil {
		return err
	}
	patched := filepath.Join(cache, mygoVersion)
	if _, err := os.Stat(patched); os.IsNotExist(err) {
		stage, err := os.MkdirTemp(cache, ".mygo-*")
		if err != nil {
			return err
		}
		defer os.RemoveAll(stage)
		if err := os.CopyFS(stage, os.DirFS(module.Dir)); err != nil {
			return err
		}
		if err := os.Rename(stage, patched); err != nil {
			if _, exists := os.Stat(patched); exists != nil {
				return err
			}
		}
	} else if err != nil {
		return err
	}
	patchedFiles := make(map[string][]byte)
	for _, patch := range patches.Replacements {
		source := filepath.Join(module.Dir, patch.File)
		data, err := os.ReadFile(source)
		if err != nil {
			return err
		}
		if strings.Count(string(data), patch.Before) != 1 {
			return fmt.Errorf("MyGo %s changed; review the Windows patches before building", patch.File)
		}
		// A file may have multiple patches; read upstream first, then layer this run's verified results.
		if previous, ok := patchedFiles[patch.File]; ok {
			data = previous
		}
		data, err = format.Source([]byte(strings.Replace(string(data), patch.Before, patch.After, 1)))
		if err != nil {
			return err
		}
		target := filepath.Join(patched, patch.File)
		if err := writeChanged(target, data); err != nil {
			return err
		}
		patchedFiles[patch.File] = data
	}
	for _, file := range patches.Files {
		data, err := os.ReadFile(filepath.Join(root, "patches", "mygo", file.Source))
		if err != nil {
			return err
		}
		if err := writeChanged(filepath.Join(patched, file.Target), data); err != nil {
			return err
		}
	}
	path := filepath.Join(cache, "build.mod")
	for _, file := range []struct{ source, target string }{{"go.mod", path}, {"go.sum", filepath.Join(cache, "build.sum")}} {
		data, err := os.ReadFile(filepath.Join(root, file.source))
		if err != nil {
			return err
		}
		if file.source == "go.mod" {
			relative, err := filepath.Rel(root, patched)
			if err != nil {
				return err
			}
			data = append(data, []byte("\nreplace "+mygoModule+" => "+strconv.Quote("./"+filepath.ToSlash(relative))+"\n")...)
		}
		if err := writeChanged(file.target, data); err != nil {
			return err
		}
	}
	fmt.Println(path)
	return nil
}

func writeChanged(path string, data []byte) error {
	previous, err := os.ReadFile(path)
	if err == nil && bytes.Equal(previous, data) {
		return nil
	}
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".patch-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
