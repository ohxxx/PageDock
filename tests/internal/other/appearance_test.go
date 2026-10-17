//go:build !darwin && !windows

// [INPUT]: A valid icon retained from macOS, config write callbacks, and write failures.
// [OUTPUT]: Platforms that cannot apply icons still save sites/permissions and restore defaults, retaining old config on failure.
// [POS]: Storage behavior at unsupported-platform icon boundaries, without system icon API calls.
// [PROTOCOL]: Update this header, internal/AGENTS.md, tests/AGENTS.md, and platform validation boundaries.
package pagedock

import (
	"errors"
	"image/color"
	"path/filepath"
	"testing"

	"github.com/ohxxx/PageDock/internal/appearance"
	prefs "github.com/ohxxx/PageDock/internal/config"
)

func TestUnsupportedIconKeepsConfigUsable(t *testing.T) {
	store := prefs.Store{Path: filepath.Join(t.TempDir(), prefs.Filename)}
	config := prefs.Config{URL: "https://example.com/", IconPNG: testIcon(t, color.NRGBA{R: 255, A: 255})}
	if err := store.Save(config); err != nil {
		t.Fatal(err)
	}
	if changed, err := appearance.Apply(config, func() error { return nil }); err != nil || changed {
		t.Fatalf("imported icon blocked startup: changed=%v, err=%v", changed, err)
	}
	config.AllowNotifications, config.AllowCamera = true, true
	if changed, err := appearance.Apply(config, func() error { return store.Save(config) }); err != nil || changed {
		t.Fatalf("icon blocked permission save: changed=%v, err=%v", changed, err)
	}
	if got, err := store.Load(); err != nil || got != config {
		t.Fatalf("configuration did not persist: %+v, %v", got, err)
	}
	wantErr := errors.New("write denied")
	if changed, err := appearance.Apply(config, func() error { return wantErr }); !errors.Is(err, wantErr) || changed {
		t.Fatalf("save error was hidden: changed=%v, err=%v", changed, err)
	}
	if got, err := store.Load(); err != nil || got != config {
		t.Fatalf("failed save lost previous configuration: %+v, %v", got, err)
	}
	config.IconPNG = ""
	if changed, err := appearance.Apply(prefs.Config{}, func() error { return store.Save(config) }); err != nil || changed {
		t.Fatalf("default icon reset failed: changed=%v, err=%v", changed, err)
	}
	if got, err := store.Load(); err != nil || got != config {
		t.Fatalf("default icon was not saved: %+v, %v", got, err)
	}
}
