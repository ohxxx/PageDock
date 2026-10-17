//go:build darwin

// [INPUT]: The current MyGo locale, saved name/PNG, current macOS bundle, config save operation, and resolved data directory.
// [OUTPUT]: Synchronize localized name and bundle/Dock/notification icons; save after registration/cache refresh, restoring bundle and caches on failure.
// [POS]: Shared macOS name/icon transaction; modify only writable local ad-hoc bundles and preserve the config directory on restart.
// [PROTOCOL]: Update this header, internal/appearance/macos/AGENTS.md, and validation boundaries in tests/AGENTS.md.
package macos

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ebitengine/purego"
	"github.com/egoist/mygo"
	"github.com/ohxxx/PageDock/internal/appearance"
	prefs "github.com/ohxxx/PageDock/internal/config"
	l10n "github.com/ohxxx/PageDock/internal/locale"
)

const (
	customIconPrefix       = "PageDockIcon-"
	originalIconKey        = "PageDockOriginalIcon"
	originalNameKey        = "PageDockOriginalName"
	launchServicesRegister = "/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister"
)

func currentAppBundle() string {
	executable, err := os.Executable()
	if err != nil {
		return ""
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return ""
	}
	macOS := filepath.Dir(executable)
	contents := filepath.Dir(macOS)
	bundle := filepath.Dir(contents)
	if filepath.Base(macOS) != "MacOS" || filepath.Base(contents) != "Contents" || filepath.Ext(bundle) != ".app" {
		return ""
	}
	return bundle
}

func Apply(config prefs.Config, save func() error) (bool, error) {
	encoded := config.IconPNG
	pngData, err := prefs.DecodeIcon(encoded)
	if err != nil {
		return false, err
	}
	bundle := currentAppBundle()
	var changed bool
	if bundle == "" {
		if encoded != "" || config.AppName != "" {
			return false, errors.New(l10n.Text("macWritableBundle"))
		}
		err = save()
	} else {
		changed, err = updateBundleAppearance(bundle, config, save, refreshNotificationIcon)
	}
	if err != nil {
		return changed, err
	}
	// Bundle resources and the running Dock icon are independent; synchronize at startup and each save, with nil restoring the system default.
	return changed, mygo.App.Dock.SetIcon(pngData)
}

// Prepare and verify the new bundle before an atomic swap; swap back on save failure, never partially write the original bundle.
func updateBundleAppearance(bundle string, config prefs.Config, save, refresh func() error) (bool, error) {
	encoded := config.IconPNG
	pngData, err := prefs.DecodeIcon(encoded)
	if err != nil {
		return false, err
	}
	plistPath := filepath.Join(bundle, "Contents", "Info.plist")
	info, err := readBundleInfo(plistPath)
	if err != nil {
		return false, err
	}
	original, managed := info[originalIconKey].(map[string]any)
	if _, exists := info[originalIconKey]; exists && !managed {
		return false, errors.New(l10n.Text("macDefaultIconInvalid"))
	}
	oldIcon, _ := info["CFBundleIconFile"].(string)
	name := ""
	if len(pngData) > 0 {
		name = fmt.Sprintf("%s%x.icns", customIconPrefix, sha256.Sum256(pngData))
	}
	iconChanged := !((name == "" && !managed) || (name != "" && managed && oldIcon == name))
	nameChanged, err := bundleNameChanged(bundle, info, config.AppName)
	if err != nil {
		return false, err
	}
	if !iconChanged && !nameChanged {
		return false, save()
	}
	signature, err := iconCommand(nil, "/usr/bin/codesign", "--display", "--verbose=4", bundle)
	if err != nil {
		return false, err
	}
	if !strings.Contains(string(signature), "Signature=adhoc") {
		return false, errors.New(l10n.Text("macSigned"))
	}
	temp, err := os.MkdirTemp(filepath.Dir(bundle), ".pagedock-icon-*")
	if err != nil {
		return false, fmt.Errorf(l10n.Text("macDirectoryWritable"), err)
	}
	defer func() {
		if temp != "" {
			_ = os.RemoveAll(temp)
		}
	}()
	staged := filepath.Join(temp, filepath.Base(bundle))
	if _, err = iconCommand(nil, "/usr/bin/ditto", "--norsrc", bundle, staged); err != nil {
		return false, err
	}
	if iconChanged {
		resources := filepath.Join(staged, "Contents", "Resources")
		if err = os.MkdirAll(resources, 0o755); err != nil {
			return false, err
		}
		if managed {
			if !strings.HasPrefix(oldIcon, customIconPrefix) || filepath.Base(oldIcon) != oldIcon || filepath.Ext(oldIcon) != ".icns" {
				return false, errors.New(l10n.Text("macCustomIconInvalid"))
			}
			if err = os.Remove(filepath.Join(resources, oldIcon)); err != nil && !errors.Is(err, os.ErrNotExist) {
				return false, err
			}
		} else {
			original = make(map[string]any)
			for _, key := range []string{"CFBundleIconFile", "CFBundleIconName"} {
				if value, ok := info[key]; ok {
					original[key] = value
				}
			}
		}
		if name == "" {
			delete(info, originalIconKey)
			for _, key := range []string{"CFBundleIconFile", "CFBundleIconName"} {
				delete(info, key)
				if value, ok := original[key]; ok {
					info[key] = value
				}
			}
		} else {
			data, err := appearance.EncodeICNS(pngData)
			if err != nil {
				return false, err
			}
			if err = os.WriteFile(filepath.Join(resources, name), data, 0o644); err != nil {
				return false, err
			}
			info[originalIconKey] = original
			info["CFBundleIconFile"] = name
			delete(info, "CFBundleIconName")
		}
	}
	if nameChanged {
		if err = updateBundleName(staged, info, config.AppName); err != nil {
			return false, err
		}
	}
	if err = writeBundleInfo(filepath.Join(staged, "Contents", "Info.plist"), info); err != nil {
		return false, err
	}
	if _, err = iconCommand(nil, "/usr/bin/codesign", "--force", "--sign", "-", "--preserve-metadata=entitlements,requirements,flags,runtime", staged); err != nil {
		return false, err
	}
	if _, err = iconCommand(nil, "/usr/bin/codesign", "--verify", "--deep", "--strict", staged); err != nil {
		return false, err
	}
	if err = swapIconBundle(staged, bundle); err != nil {
		return false, fmt.Errorf(l10n.Text("appearanceReplace"), err)
	}
	// staged now holds the original bundle; defer removes it only after registration and config persistence succeed.
	_, err = iconCommand(nil, launchServicesRegister, "-f", bundle)
	if err == nil {
		err = refresh()
	}
	if err == nil {
		err = save()
	}
	if err != nil {
		if rollback := swapIconBundle(staged, bundle); rollback != nil {
			// Keep the only original bundle and include its recovery location in the error.
			preserved := temp
			temp = ""
			return false, fmt.Errorf(l10n.Text("macRollback"), err, rollback, preserved)
		}
		_, registerErr := iconCommand(nil, launchServicesRegister, "-f", bundle)
		return false, errors.Join(err, registerErr, refresh())
	}
	return true, nil
}

func readBundleInfo(path string) (map[string]any, error) {
	data, err := iconCommand(nil, "/usr/bin/plutil", "-convert", "json", "-o", "-", path)
	if err != nil {
		return nil, err
	}
	var info map[string]any
	if err = json.Unmarshal(data, &info); err != nil {
		return nil, err
	}
	return info, nil
}

func writeBundleInfo(path string, info map[string]any) error {
	data, err := json.Marshal(info)
	if err != nil {
		return err
	}
	_, err = iconCommand(data, "/usr/bin/plutil", "-convert", "xml1", "-o", path, "-")
	return err
}

func iconCommand(input []byte, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = bytes.NewReader(input)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf(l10n.Text("commandFailed"), filepath.Base(name), err, strings.TrimSpace(string(out)))
	}
	return out, nil
}

// Darwin renamex_np with RENAME_SWAP atomically exchanges complete directories on the same filesystem.
var loadRenameSwap = sync.OnceValues(func() (func(string, string, uint32) int32, error) {
	library, err := purego.Dlopen("/usr/lib/libSystem.B.dylib", purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		return nil, err
	}
	var rename func(string, string, uint32) int32
	purego.RegisterLibFunc(&rename, library, "renamex_np")
	return rename, nil
})

func swapIconBundle(from, to string) error {
	rename, err := loadRenameSwap()
	if err != nil {
		return err
	}
	const renameSwap = 0x2
	if rename(from, to, renameSwap) != 0 {
		return errors.New(l10n.Text("macExchange"))
	}
	return nil
}

// Appearance updates replace the whole bundle; relaunch packaged apps through LaunchServices to refresh system identity.
// MyGo's supervisor handles development restarts to retain hot reload.
func Relaunch(dataDir string) {
	bundle := currentAppBundle()
	if mygo.IsDev() || bundle == "" {
		mygo.App.Relaunch()
		return
	}
	mygo.App.OnQuit(func() {
		// LaunchServices does not inherit the caller's environment; explicitly retain the resolved config directory.
		args := []string{"-n", "--env", "PAGEDOCK_DATA_DIR=" + dataDir, bundle, "--args"}
		args = append(args, os.Args[1:]...)
		if _, err := iconCommand(nil, "/usr/bin/open", args...); err != nil {
			log.Printf(l10n.Text("reopenApp"), err)
		}
	})
	mygo.App.Quit()
}
