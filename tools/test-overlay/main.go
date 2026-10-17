// [INPUT]: Repository root, Go behavior tests under tests/, and package ownership in packages.json.
// [OUTPUT]: A native Go overlay path mapping tests only, without copying product source or changing the module cache.
// [POS]: Test assembly for Makefile and standalone validation scripts, retaining private package access and platform filtering.
// [PROTOCOL]: Update tests/AGENTS.md, Makefile, and this tool's behavior tests.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	root, err := os.Getwd()
	var path string
	if err == nil {
		path, err = prepareOverlay(root)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(path)
}

func prepareOverlay(root string) (string, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	// Go discovers packages from the real working directory; canonicalize macOS temp paths and linked checkouts consistently.
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	for _, name := range []string{"go.mod", filepath.Join("tests", "go.mod")} {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			return "", fmt.Errorf("run from the repository root: %w", err)
		}
	}
	tests := filepath.Join(root, "tests")
	packages := make(map[string]string)
	data, err := os.ReadFile(filepath.Join(tests, "packages.json"))
	if err == nil {
		if err := json.Unmarshal(data, &packages); err != nil {
			return "", fmt.Errorf("read test packages: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return "", err
	}
	for source, target := range packages {
		if !filepath.IsLocal(filepath.FromSlash(source)) || !filepath.IsLocal(filepath.FromSlash(target)) {
			return "", fmt.Errorf("test package paths must stay inside the repository: %s -> %s", source, target)
		}
	}
	overlay := struct{ Replace map[string]string }{Replace: make(map[string]string)}
	err = filepath.WalkDir(tests, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), "_test.go") {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("test must be a regular file: %s", path)
		}
		relative, err := filepath.Rel(tests, path)
		if err != nil {
			return err
		}
		target := filepath.Join(root, relative)
		group := filepath.ToSlash(filepath.Dir(relative))
		if owner, ok := packages[group]; ok {
			// Platform directories organize tests only; private access and the working directory remain in the original product package.
			name := filepath.Base(filepath.Dir(relative)) + "_" + filepath.Base(relative)
			target = filepath.Join(root, filepath.FromSlash(owner), name)
		}
		info, err := os.Stat(filepath.Dir(target))
		if err != nil {
			return fmt.Errorf("test package missing for %s: %w", relative, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("test package is not a directory: %s", relative)
		}
		if _, err := os.Lstat(target); !os.IsNotExist(err) {
			return fmt.Errorf("test target must not already exist: %s (%v)", target, err)
		}
		if previous, ok := overlay.Replace[target]; ok {
			return fmt.Errorf("test target collision: %s and %s", previous, path)
		}
		overlay.Replace[target] = path
		return nil
	})
	if err != nil {
		return "", err
	}
	if len(overlay.Replace) == 0 {
		return "", fmt.Errorf("no Go tests found in %s", tests)
	}
	data, err = json.MarshalIndent(overlay, "", "  ")
	if err != nil {
		return "", err
	}
	dir := filepath.Join(root, ".cache", "tests")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "overlay.json")
	if previous, err := os.ReadFile(path); err == nil && bytes.Equal(previous, data) {
		return path, nil
	}
	f, err := os.CreateTemp(dir, ".overlay-*.json")
	if err != nil {
		return "", err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(data); err != nil {
		f.Close()
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(f.Name(), path); err != nil {
		return "", err
	}
	return path, nil
}
