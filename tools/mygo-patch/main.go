// [INPUT]: Pinned MyGo sources, shared lifecycle, macOS pools/visibility, and Windows integration patches.
// [OUTPUT]: Complete composed MyGo sources and a modfile; preserve the module cache and fail on version/context drift.
// [POS]: Makefile build preparation; packaged apps, development, and tests use the same patches.
// [PROTOCOL]: Update the root AGENTS.md, patches/mygo/AGENTS.md, and Makefile.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"go/format"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/ohxxx/PageDock/patches/mygo/macos"
	"github.com/ohxxx/PageDock/patches/mygo/shared"
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
		return fmt.Errorf("MyGo patches require unmodified %s %s", mygoModule, mygoVersion)
	}
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	cache := filepath.Join(root, ".cache", "mygo-patch")
	if err := os.MkdirAll(cache, 0755); err != nil {
		return err
	}
	patchedFiles := make(map[string][]byte)
	replacements := append(append(shared.Replacements, macos.Replacements...), patches.Replacements...)
	for _, patch := range replacements {
		source := filepath.Join(module.Dir, patch.File)
		data, err := os.ReadFile(source)
		if err != nil {
			return err
		}
		if strings.Count(string(data), patch.Before) != 1 {
			return fmt.Errorf("MyGo %s changed; review the patches before building", patch.File)
		}
		// A file may have multiple patches; read upstream first, then layer this run's verified results.
		if previous, ok := patchedFiles[patch.File]; ok {
			data = previous
		}
		data, err = applyReplacement(data, patch.Before, patch.After)
		if err != nil {
			return fmt.Errorf("MyGo %s: %w", patch.File, err)
		}
		patchedFiles[patch.File] = data
	}
	// Format only after every replacement, preserving later source contexts.
	for name, data := range patchedFiles {
		data, err = format.Source(data)
		if err != nil {
			return fmt.Errorf("MyGo %s: %w", name, err)
		}
		patchedFiles[name] = data
	}
	for _, file := range patches.Files {
		data, err := os.ReadFile(filepath.Join(root, "patches", "mygo", file.Source))
		if err != nil {
			return err
		}
		patchedFiles[file.Target] = data
	}
	patched, err := publishDependency(cache, module.Dir, patchedFiles)
	if err != nil {
		return err
	}
	path := filepath.Join(cache, filepath.Base(patched)+".mod")
	for _, file := range []struct{ source, target string }{{"go.mod", path}, {"go.sum", strings.TrimSuffix(path, ".mod") + ".sum"}} {
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

// Each patch set owns a complete immutable copy. Removed patches cannot survive
// in a reused directory, and concurrent builds never observe partial publication.
func publishDependency(cache, upstream string, files map[string][]byte) (string, error) {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	slices.Sort(names)
	hash := sha256.New()
	for _, name := range names {
		fmt.Fprintf(hash, "%d:%s%d:", len(name), name, len(files[name]))
		hash.Write(files[name])
	}
	target := filepath.Join(cache, fmt.Sprintf("%s-%x", mygoVersion, hash.Sum(nil)[:16]))
	if info, err := os.Stat(target); err == nil {
		if !info.IsDir() {
			return "", fmt.Errorf("dependency cache is not a directory: %s", target)
		}
		return target, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}
	stage, err := os.MkdirTemp(cache, ".mygo-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(stage)
	if err := os.CopyFS(stage, os.DirFS(upstream)); err != nil {
		return "", err
	}
	for _, name := range names {
		if err := writeChanged(filepath.Join(stage, name), files[name]); err != nil {
			return "", err
		}
	}
	if err := os.Rename(stage, target); err != nil {
		if info, exists := os.Stat(target); exists != nil || !info.IsDir() {
			return "", err
		}
	}
	return target, nil
}

func applyReplacement(data []byte, before, after string) ([]byte, error) {
	if strings.Count(string(data), before) != 1 {
		return nil, fmt.Errorf("patch context changed after earlier replacements")
	}
	return []byte(strings.Replace(string(data), before, after, 1)), nil
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
