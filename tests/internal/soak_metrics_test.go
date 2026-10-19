//go:build desktop && pagedock_soak && darwin

// [INPUT]: Endurance phases, runtime memory statistics, notification records, and weak window/view references.
// [OUTPUT]: JSONL measurements, GC checkpoints, profiles, and explicit cleanup assertions.
// [POS]: Test-only evidence collection; the external runner measures native/WebKit RSS independently.
// [PROTOCOL]: Update tests/internal/AGENTS.md, tests/soak/AGENTS.md, and validation records.
package pagedock

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"runtime/pprof"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo"
)

type soakMetrics struct {
	file                                       *os.File
	encoder                                    *json.Encoder
	started                                    time.Time
	cycles, notifications, navigations, clicks int
	checkpoints                                int
}

type soakSample struct {
	Time            string  `json:"time"`
	Elapsed         float64 `json:"elapsed_s"`
	Phase           string  `json:"phase"`
	PostGC          bool    `json:"post_gc"`
	PID             int     `json:"pid"`
	Development     bool    `json:"development"`
	Cycles          int     `json:"cycles"`
	Notifications   int     `json:"notifications"`
	Navigations     int     `json:"navigations"`
	Clicks          int     `json:"clicks"`
	HeapAlloc       uint64  `json:"heap_alloc"`
	HeapInuse       uint64  `json:"heap_inuse"`
	HeapSys         uint64  `json:"heap_sys"`
	HeapReleased    uint64  `json:"heap_released"`
	HeapObjects     uint64  `json:"heap_objects"`
	TotalAlloc      uint64  `json:"total_alloc"`
	NumGC           uint32  `json:"num_gc"`
	Goroutines      int     `json:"goroutines"`
	Followers       int     `json:"followers"`
	NativeRecords   int     `json:"native_records"`
	QueueRecords    int     `json:"queue_records"`
	ActiveCallbacks int     `json:"active_callbacks"`
	Windows         int     `json:"windows"`
	RetainedWindows int     `json:"retained_windows"`
	RetainedPages   int     `json:"retained_pages"`
	RetainedViews   int     `json:"retained_views"`
}

func newSoakMetrics(t *testing.T) *soakMetrics {
	t.Helper()
	file, err := os.Create(filepath.Join(*soakOutput, "go-memory.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(*soakOutput, "pid"), []byte(strconv.Itoa(os.Getpid())), 0600); err != nil {
		file.Close()
		t.Fatal(err)
	}
	return &soakMetrics{file: file, encoder: json.NewEncoder(file), started: time.Now()}
}

func (s *soakMetrics) close() { s.file.Close() }

func (s *soakMetrics) sample(t *testing.T, phase string, refs *soakReferences, gc bool) soakSample {
	t.Helper()
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	row := soakSample{Time: time.Now().UTC().Format(time.RFC3339Nano), Elapsed: time.Since(s.started).Seconds(),
		Phase: phase, PostGC: gc, PID: os.Getpid(), Development: mygo.IsDev(), Cycles: s.cycles, Notifications: s.notifications, Navigations: s.navigations, Clicks: s.clicks,
		HeapAlloc: memory.HeapAlloc, HeapInuse: memory.HeapInuse, HeapSys: memory.HeapSys, HeapReleased: memory.HeapReleased,
		HeapObjects: memory.HeapObjects, TotalAlloc: memory.TotalAlloc, NumGC: memory.NumGC, Goroutines: runtime.NumGoroutine(),
		Windows: len(mygo.Windows()), ActiveCallbacks: soakActiveCallbacks()}
	appNotifications.mu.Lock()
	row.QueueRecords = len(appNotifications.records)
	row.NativeRecords = mygo.PageDockSoakNativeCount()
	appNotifications.mu.Unlock()
	if gc {
		var stacks bytes.Buffer
		if err := pprof.Lookup("goroutine").WriteTo(&stacks, 2); err != nil {
			t.Fatal(err)
		}
		for _, stack := range strings.Split(stacks.String(), "\n\n") {
			if strings.Contains(stack, "internal/view.followBrowser.func") {
				row.Followers++
			}
		}
		for _, ref := range refs.windows {
			if ref.Value() != nil {
				row.RetainedWindows++
			}
		}
		for _, ref := range refs.pages {
			if ref.Value() != nil {
				row.RetainedPages++
			}
		}
		for _, ref := range refs.views {
			if ref.Value() != nil {
				row.RetainedViews++
			}
		}
	}
	if err := s.encoder.Encode(row); err != nil {
		t.Fatal(err)
	}
	if err := s.file.Sync(); err != nil {
		t.Fatal(err)
	}
	return row
}

func (s *soakMetrics) checkpoint(t *testing.T, phase string, refs *soakReferences, profile bool) {
	t.Helper()
	soakWait(t, func() bool { return soakActiveCallbacks() == 0 }, "callback teardown at checkpoint")
	runtime.GC()
	row := s.sample(t, phase, refs, true)
	if row.NativeRecords != row.QueueRecords || row.NativeRecords > maxNativeNotifications || row.Windows != 0 || row.Followers != 0 {
		t.Fatalf("resources after closed windows: %+v", row)
	}
	if row.RetainedWindows > 1 || row.RetainedPages > 1 || row.RetainedViews > 1 {
		t.Fatalf("closed objects accumulated after GC: windows=%d pages=%d views=%d", row.RetainedWindows, row.RetainedPages, row.RetainedViews)
	}
	if profile && (s.checkpoints%10 == 0 || phase != "loaded") {
		for _, name := range []string{"heap", "goroutine"} {
			file, err := os.Create(filepath.Join(*soakOutput, fmtProfile(phase, s.checkpoints, name)))
			if err != nil {
				t.Fatal(err)
			}
			err = pprof.Lookup(name).WriteTo(file, 0)
			closeErr := file.Close()
			if err != nil {
				t.Fatal(err)
			}
			if closeErr != nil {
				t.Fatal(closeErr)
			}
		}
	}
	s.checkpoints++
	t.Logf("phase=%s elapsed=%.0fs cycles=%d notifications=%d heap=%.2fMiB goroutines=%d native=%d retained=%d/%d/%d",
		phase, row.Elapsed, row.Cycles, row.Notifications, float64(row.HeapAlloc)/(1<<20), row.Goroutines, row.NativeRecords,
		row.RetainedWindows, row.RetainedPages, row.RetainedViews)
}

func fmtProfile(phase string, index int, name string) string {
	return fmt.Sprintf("%03d-%s-%s.pprof", index, phase, name)
}

func (s *soakMetrics) finish(t *testing.T, refs *soakReferences) {
	t.Helper()
	s.checkpoint(t, "idle_final", refs, true)
	if mygo.PageDockSoakNativeCount() != 0 {
		t.Fatal("native notification records survived cleanup")
	}
	// Record explicit scavenging separately from natural idle recovery.
	debug.FreeOSMemory()
	s.sample(t, "forced_scavenge", refs, true)
	if err := os.WriteFile(filepath.Join(*soakOutput, "completed"), []byte("passed\n"), 0600); err != nil {
		t.Fatal(err)
	}
}
