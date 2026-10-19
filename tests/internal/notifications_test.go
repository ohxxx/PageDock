// [INPUT]: Notification deliveries, cancellation, replacements, failures, and virtual time.
// [OUTPUT]: Bounded native resources, released page closures, and no stale clicks or timers closing replacements.
// [POS]: Lifecycle behavior tests with an observable native boundary; no system notifications.
// [PROTOCOL]: Update tests/internal/AGENTS.md and resource validation records.
package pagedock

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"testing/synctest"
	"time"
	"weak"

	"github.com/egoist/mygo"
)

type notificationSink struct {
	mu      sync.Mutex
	active  map[string]func()
	failure error
}

func mockNotifications(t *testing.T, queue *notificationQueue) *notificationSink {
	t.Helper()
	sink := &notificationSink{active: make(map[string]func())}
	show, close := showPWANotification, closePWANotification
	showPWANotification = func(opts mygo.NotificationOptions, click func()) error {
		sink.mu.Lock()
		defer sink.mu.Unlock()
		sink.active[opts.ID] = click
		return sink.failure
	}
	closePWANotification = func(id string) {
		sink.mu.Lock()
		defer sink.mu.Unlock()
		delete(sink.active, id)
	}
	t.Cleanup(func() { queue.clear(); showPWANotification, closePWANotification = show, close })
	return sink
}

func (s *notificationSink) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.active)
}

func (s *notificationSink) click(id string) func() {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active[id]
}

func TestNotificationResourcesBounded(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var queue notificationQueue
		sink := mockNotifications(t, &queue)
		for i := range maxNativeNotifications * 4 {
			if err := queue.show(context.Background(), mygo.NotificationOptions{ID: fmt.Sprint(i)}, nil); err != nil {
				t.Fatal(err)
			}
		}
		if got := sink.count(); got != maxNativeNotifications {
			t.Fatalf("native records=%d, want %d", got, maxNativeNotifications)
		}
		if sink.click("0") != nil {
			t.Fatal("oldest notification was not evicted")
		}
		time.Sleep(nativeNotificationLifetime)
		synctest.Wait()
		if got := sink.count(); got != 0 {
			t.Fatalf("expired native records=%d", got)
		}
	})
}

func TestNotificationReplacementAndFailure(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var queue notificationQueue
		sink := mockNotifications(t, &queue)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		calls := 0
		opts := mygo.NotificationOptions{ID: "same-tag"}
		if err := queue.show(ctx, opts, func() { t.Error("stale callback") }); err != nil {
			t.Fatal(err)
		}
		oldClick := sink.click(opts.ID)
		time.Sleep(nativeNotificationLifetime / 2)
		if err := queue.show(context.Background(), opts, func() { calls++ }); err != nil {
			t.Fatal(err)
		}
		cancel()
		oldClick()
		time.Sleep(nativeNotificationLifetime / 2)
		synctest.Wait()
		if got := sink.count(); got != 1 {
			t.Fatalf("replacement removed by old timer/click/cancellation: %d", got)
		}
		currentClick := sink.click(opts.ID)
		currentClick()
		synctest.Wait()
		currentClick()
		synctest.Wait()
		if calls != 1 || sink.count() != 0 {
			t.Fatalf("click calls=%d native records=%d", calls, sink.count())
		}
		if err := queue.show(context.Background(), opts, nil); err != nil {
			t.Fatal(err)
		}
		sink.failure = errors.New("native delivery failed")
		if err := queue.show(context.Background(), opts, nil); !errors.Is(err, sink.failure) {
			t.Fatal(err)
		}
		if got := sink.count(); got != 0 {
			t.Fatalf("failed replacement leaked %d records", got)
		}
		sink.failure = nil
		if err := queue.show(ctx, opts, nil); !errors.Is(err, context.Canceled) {
			t.Fatalf("cancelled page sent a notification: %v", err)
		}
	})
}

// A heap object stands in for the page/context retained by a click closure.
func queuePageReference(t *testing.T, queue *notificationQueue, ctx context.Context) weak.Pointer[[4096]byte] {
	t.Helper()
	page := new([4096]byte)
	if err := queue.show(ctx, mygo.NotificationOptions{ID: "page"}, func() { runtime.KeepAlive(page) }); err != nil {
		t.Fatal(err)
	}
	return weak.Make(page)
}

func TestNotificationNavigationReleasesPage(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var queue notificationQueue
		sink := mockNotifications(t, &queue)
		ctx, cancel := context.WithCancel(context.Background())
		page := queuePageReference(t, &queue, ctx)
		cancel()
		synctest.Wait()
		runtime.GC()
		if page.Value() != nil {
			t.Error("native callback still retains the cancelled page")
		}
		if sink.count() != 1 {
			t.Error("navigation removed the notification history")
		}
		queue.clear()
		time.Sleep(nativeNotificationLifetime)
		synctest.Wait()
		if sink.count() != 0 {
			t.Fatal("reset left native notifications")
		}
	})
}
