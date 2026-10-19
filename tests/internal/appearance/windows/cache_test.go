// [INPUT]: Appearance changes, ordinary settings saves, and transaction failures.
// [OUTPUT]: Successful synchronization is reused without reading the EXE; failed synchronization is retried.
// [POS]: Portable behavior tests for the Windows save gate, without native APIs.
// [PROTOCOL]: Update tests/internal/appearance/windows/AGENTS.md and the product directory map.
package windows

import (
	"errors"
	"sync"
	"testing"

	prefs "github.com/ohxxx/PageDock/internal/config"
)

func TestAppearanceCacheSkipsOrdinarySaves(t *testing.T) {
	var cache appearanceCache
	reads, writes := 0, 0
	save := func() error { writes++; return nil }
	synchronize := func(c prefs.Config, save func() error) (bool, error) { reads++; return false, save() }
	config := prefs.Config{URL: "https://example.com"}
	if _, err := cache.apply(config, save, synchronize); err != nil {
		t.Fatal(err)
	}
	config.URL, config.Language, config.AllowNotifications = "https://other.example", "en", true
	if _, err := cache.apply(config, save, synchronize); err != nil {
		t.Fatal(err)
	}
	if reads != 1 || writes != 2 {
		t.Fatalf("ordinary save: EXE reads=%d config writes=%d; want 1 and 2", reads, writes)
	}
	config.AppName = "New name"
	if _, err := cache.apply(config, save, synchronize); err != nil {
		t.Fatal(err)
	}
	config.IconPNG = "new image contents"
	if _, err := cache.apply(config, save, synchronize); err != nil {
		t.Fatal(err)
	}
	if reads != 3 || writes != 4 {
		t.Fatalf("appearance changes: reads=%d writes=%d", reads, writes)
	}
}

func TestAppearanceCacheSerializesSaves(t *testing.T) {
	var cache appearanceCache
	var tasks sync.WaitGroup
	reads, writes := 0, 0
	save := func() error { writes++; return nil }
	synchronize := func(_ prefs.Config, save func() error) (bool, error) { reads++; return false, save() }
	for range 32 {
		tasks.Go(func() {
			if _, err := cache.apply(prefs.Config{}, save, synchronize); err != nil {
				t.Error(err)
			}
		})
	}
	tasks.Wait()
	if reads != 1 || writes != 32 {
		t.Fatalf("concurrent saves: EXE reads=%d config writes=%d", reads, writes)
	}
}

func BenchmarkAppearanceCacheOrdinarySave(b *testing.B) {
	var cache appearanceCache
	config := prefs.Config{AppName: "Custom"}
	save := func() error { return nil }
	synchronize := func(_ prefs.Config, save func() error) (bool, error) { return false, save() }
	if _, err := cache.apply(config, save, synchronize); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := cache.apply(config, save, synchronize); err != nil {
			b.Fatal(err)
		}
	}
}

func TestAppearanceCacheRetriesFailures(t *testing.T) {
	var cache appearanceCache
	config := prefs.Config{AppName: "Custom"}
	failure := errors.New("save failed")
	reads := 0
	synchronize := func(c prefs.Config, save func() error) (bool, error) { reads++; return true, save() }
	if _, err := cache.apply(config, func() error { return failure }, synchronize); !errors.Is(err, failure) {
		t.Fatal(err)
	}
	if _, err := cache.apply(config, func() error { return nil }, synchronize); err != nil {
		t.Fatal(err)
	}
	if reads != 2 {
		t.Fatalf("failed initial synchronization was cached: reads=%d", reads)
	}
	if _, err := cache.apply(config, func() error { return failure }, synchronize); !errors.Is(err, failure) {
		t.Fatal(err)
	}
	if reads != 2 {
		t.Fatalf("ordinary failed save read EXE again: %d", reads)
	}
	config.AppName = "Changed"
	if _, err := cache.apply(config, func() error { return failure }, synchronize); !errors.Is(err, failure) {
		t.Fatal(err)
	}
	if _, err := cache.apply(config, func() error { return nil }, synchronize); err != nil {
		t.Fatal(err)
	}
	if reads != 4 {
		t.Fatalf("failed appearance transaction was not retried: %d", reads)
	}
}

func TestAppearanceCacheInvalidatesAfterNativeFailure(t *testing.T) {
	var cache appearanceCache
	reads, writes := 0, 0
	failure := errors.New("native update and rollback failed")
	save := func() error { writes++; return nil }
	synchronize := func(_ prefs.Config, save func() error) (bool, error) {
		reads++
		if reads == 2 {
			return false, failure
		}
		return true, save()
	}
	for i, name := range []string{"Original", "Changed", "Original"} {
		changed, err := cache.apply(prefs.Config{AppName: name}, save, synchronize)
		if i == 1 {
			if !errors.Is(err, failure) {
				t.Fatal(err)
			}
		} else if err != nil || !changed {
			t.Fatalf("synchronization result: changed=%v error=%v", changed, err)
		}
	}
	if reads != 3 || writes != 2 {
		t.Fatalf("rollback failure reused stale state: reads=%d writes=%d", reads, writes)
	}
}
