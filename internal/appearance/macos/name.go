//go:build darwin

// [INPUT]: The current MyGo locale, staged app bundle, name draft, and original bundle metadata.
// [OUTPUT]: Localized display names shared by Dock/Finder and Bundle, with byte-for-byte resource restoration.
// [POS]: macOS name resources within the existing signed bundle-swap transaction; preserve paths and Dock preferences.
// [PROTOCOL]: Update internal/appearance/macos/AGENTS.md, system name tests, and platform validation records.
package macos

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	prefs "github.com/ohxxx/PageDock/internal/config"
	l10n "github.com/ohxxx/PageDock/internal/locale"
)

const (
	localizedNameKey         = "PageDockDisplayName"
	originalLocalizationsKey = "Localizations"
	maxNameResourceBytes     = 1 << 20
)

type bundleNameResource struct {
	Data            []byte `json:",omitempty"`
	FileExists      bool
	DirectoryExists bool
}

func bundleNameChanged(bundle string, info map[string]any, name string) (bool, error) {
	name, err := prefs.ValidateName(name)
	if err != nil {
		return false, err
	}
	_, managed := info[originalNameKey].(map[string]any)
	if _, exists := info[originalNameKey]; exists && !managed {
		return false, errors.New(l10n.Text("macDefaultNameInvalid"))
	}
	if name == "" {
		return managed, nil
	}
	base := strings.TrimSuffix(filepath.Base(bundle), ".app")
	return !managed || info[localizedNameKey] != name || info["CFBundleName"] != base || info["CFBundleDisplayName"] != base, nil
}

func updateBundleName(bundle string, info map[string]any, name string) error {
	original, managed := info[originalNameKey].(map[string]any)
	keys := []string{"CFBundleName", "CFBundleDisplayName"}
	if !managed {
		original = make(map[string]any)
		for _, key := range keys {
			if value, ok := info[key]; ok {
				original[key] = value
			}
		}
	}
	resources := filepath.Join(bundle, "Contents", "Resources")
	if stat, err := os.Lstat(resources); err == nil {
		if !stat.IsDir() {
			return errors.New(l10n.Text("macResourcesInvalid"))
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	var saved map[string]bundleNameResource
	if raw, ok := original[originalLocalizationsKey]; ok {
		data, err := json.Marshal(raw)
		if err != nil {
			return err
		}
		if err = json.Unmarshal(data, &saved); err != nil {
			return err
		}
	} else if name != "" {
		var err error
		saved, err = captureBundleNameResources(resources)
		if err != nil {
			return err
		}
		original[originalLocalizationsKey] = saved
	}
	for language, resource := range saved {
		if filepath.Base(language) != language || !strings.HasSuffix(language, ".lproj") || language == ".lproj" || len(resource.Data) > maxNameResourceBytes {
			return errors.New(l10n.Text("macOriginalNameInvalid"))
		}
		dir := filepath.Join(resources, language)
		path := filepath.Join(dir, "InfoPlist.strings")
		// Recheck every save so a symlink replaced after an earlier save cannot write outside the staged bundle.
		for i, target := range []string{dir, path} {
			stat, err := os.Lstat(target)
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil {
				return err
			}
			if (i == 0 && !stat.IsDir()) || (i == 1 && (!stat.Mode().IsRegular() || stat.Size() > maxNameResourceBytes)) {
				return errors.New(l10n.Text("macNameFileInvalid"))
			}
		}
		if name == "" {
			if resource.FileExists {
				if err := os.WriteFile(path, resource.Data, 0o644); err != nil {
					return err
				}
			} else if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
			if !resource.DirectoryExists {
				entries, err := os.ReadDir(dir)
				if err != nil && !errors.Is(err, os.ErrNotExist) {
					return err
				}
				if err == nil && len(entries) == 0 {
					if err = os.Remove(dir); err != nil {
						return err
					}
				}
			}
			continue
		}
		values := make(map[string]any)
		if _, err := os.Stat(path); err == nil {
			values, err = readBundleInfo(path)
			if err != nil {
				return err
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		for _, key := range keys {
			values[key] = name
		}
		if err := writeBundleInfo(path, values); err != nil {
			return err
		}
	}
	for _, key := range keys {
		delete(info, key)
		if name != "" {
			// The system uses InfoPlist.strings only after the unlocalized name matches the bundle filename.
			info[key] = strings.TrimSuffix(filepath.Base(bundle), ".app")
		} else if value, ok := original[key]; ok {
			info[key] = value
		}
	}
	if name == "" {
		delete(info, originalNameKey)
		delete(info, localizedNameKey)
	} else {
		info[originalNameKey] = original
		info[localizedNameKey] = name
	}
	return nil
}

func captureBundleNameResources(resources string) (map[string]bundleNameResource, error) {
	entries, err := os.ReadDir(resources)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	saved := map[string]bundleNameResource{"Base.lproj": {}}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".lproj") {
			continue
		}
		if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return nil, errors.New(l10n.Text("macNameDirectoryInvalid"))
		}
		saved[entry.Name()] = bundleNameResource{DirectoryExists: true}
	}
	for language, resource := range saved {
		path := filepath.Join(resources, language, "InfoPlist.strings")
		stat, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if !stat.Mode().IsRegular() || stat.Size() > maxNameResourceBytes {
			return nil, errors.New(l10n.Text("macNameResourceInvalid"))
		}
		resource.Data, err = os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		resource.FileExists = true
		saved[language] = resource
	}
	return saved, nil
}
