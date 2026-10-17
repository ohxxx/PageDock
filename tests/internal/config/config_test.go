// [INPUT]: Old/new permission config, temporary user directories, and storage failures.
// [OUTPUT]: Name/permission retention after restart, default denial of new permissions in old config, and original-data retention on write failure.
// [POS]: Actual configuration file read/write tests.
// [PROTOCOL]: Update this header, internal/config/AGENTS.md, and tests/AGENTS.md; update the root AGENTS.md when directory boundaries change.
package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	l10n "github.com/ohxxx/PageDock/internal/locale"
)

func TestConfigSurvivesRestartAndReplacement(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data", Filename)
	store := Store{Path: path}
	initial, err := store.Load()
	if err != nil || initial.URL != "" {
		t.Fatalf("first launch: %+v, %v", initial, err)
	}
	first := Config{AppName: "  工作台 🖥  ", URL: "https://EXAMPLE.com:443/inbox", AllowNotifications: true, AllowCamera: true, AllowMicrophone: true, AllowGeolocation: true}
	if err := store.Save(first); err != nil {
		t.Fatal(err)
	}
	restarted := Store{Path: path}
	got, err := restarted.Load()
	want := first
	want.URL = "https://example.com/inbox"
	want.AppName = "工作台 🖥"
	if err != nil || got != want {
		t.Fatalf("restored: %+v, %v", got, err)
	}
	next := Config{URL: "http://localhost:8081/", AllowHTTP: true}
	if err := restarted.Save(next); err != nil {
		t.Fatal(err)
	}
	got, err = store.Load()
	if err != nil || got != next {
		t.Fatalf("replacement: %+v, %v", got, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("config permissions: %v", info.Mode())
	}
	files, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(files) != 1 {
		t.Fatalf("temporary files leaked: %v, %v", files, err)
	}
}

func TestLegacyConfigKeepsNewPermissionsDisabled(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), Filename)}
	if err := os.WriteFile(store.Path, []byte(`{"url":"https://example.com/","allowNotifications":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load()
	want := Config{URL: "https://example.com/", AllowNotifications: true}
	if err != nil || got != want {
		t.Fatalf("legacy permissions: %+v, %v", got, err)
	}
}

func TestInvalidSavePreservesExistingConfig(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), Filename)}
	existing := Config{URL: "https://example.com/"}
	if err := store.Save(existing); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(Config{URL: "https://example.com/?token=secret"}); err == nil {
		t.Fatal("expected validation error")
	}
	got, err := store.Load()
	if err != nil || got != existing {
		t.Fatalf("previous config was changed: %+v, %v", got, err)
	}
}

func TestCorruptConfigIsReportedWithoutOverwriting(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), Filename)}
	for _, content := range []string{"{broken", `{"url":"file:///tmp/page"}`, `{"url":"https://example.com/?token=secret"}`} {
		if err := os.WriteFile(store.Path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := store.Load(); err == nil {
			t.Fatal("expected corrupt or unsafe config error")
		}
		got, err := os.ReadFile(store.Path)
		if err != nil || string(got) != content {
			t.Fatalf("read changed the file: %s, %v", got, err)
		}
	}
}

func TestSaveFailureDoesNotLeaveTemporaryFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, Filename)
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := (Store{Path: path}).Save(Config{URL: "https://example.com/"}); err == nil {
		t.Fatal("expected replacement error")
	}
	files, err := os.ReadDir(dir)
	if err != nil || len(files) != 1 || !files[0].IsDir() {
		t.Fatalf("failed write leaked files or replaced target: %v, %v", files, err)
	}
}

func TestInvalidNamePreservesExistingConfig(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), Filename)}
	existing := Config{AppName: "原名称", URL: "https://example.com/"}
	if err := store.Save(existing); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"坏\n名称", "坏\x00名称", "坏\u202e名称", strings.Repeat("名", maxAppNameLength+1), string([]byte{0xff})} {
		next := existing
		next.AppName = name
		if err := store.Save(next); err == nil {
			t.Fatalf("invalid name accepted: %q", name)
		}
		if got, err := store.Load(); err != nil || got != existing {
			t.Fatalf("old config changed: %+v %v", got, err)
		}
	}
	existing.AppName = "  "
	if err := store.Save(existing); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load()
	if err != nil || got.AppName != "" || got.DisplayName() != l10n.Text("appName") {
		t.Fatalf("default name: %+v %v", got, err)
	}
}
