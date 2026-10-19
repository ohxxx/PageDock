// [INPUT]: Native notification requests, page lifetimes, clicks, and explicit close/reset operations.
// [OUTPUT]: Bounded native records with expiring callbacks and safe same-ID replacement.
// [POS]: Process-local lifecycle shared by website and settings-test notifications.
// [PROTOCOL]: Update internal/AGENTS.md and notification lifecycle tests when ownership changes.
package pagedock

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/egoist/mygo"
)

const (
	maxNativeNotifications     = 256
	nativeNotificationLifetime = 24 * time.Hour
)

type notificationRecord struct {
	id          string
	click       func()
	timer       *time.Timer
	stopContext func() bool
}

type notificationQueue struct {
	mu      sync.Mutex
	records []*notificationRecord
}

var appNotifications notificationQueue

func (q *notificationQueue) show(ctx context.Context, opts mygo.NotificationOptions, click func()) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	for _, record := range q.records {
		if record.id == opts.ID {
			// Show replaces this ID natively; do not close the new delivery with an old callback.
			q.removeLocked(record, false)
			break
		}
	}
	if len(q.records) == maxNativeNotifications {
		q.removeLocked(q.records[0], true)
	}
	record := &notificationRecord{id: opts.ID, click: click}
	q.records = append(q.records, record)
	// Native events run on the main thread. Never make them wait for a lock held across Show/Close.
	if err := showPWANotification(opts, func() { go q.clicked(record) }); err != nil {
		q.removeLocked(record, true)
		return err
	}
	record.timer = time.AfterFunc(nativeNotificationLifetime, func() {
		q.mu.Lock()
		defer q.mu.Unlock()
		q.removeLocked(record, true)
	})
	record.stopContext = context.AfterFunc(ctx, func() {
		q.mu.Lock()
		defer q.mu.Unlock()
		// Keep the native history, but drop the page closure and the context subscription.
		record.click, record.stopContext = nil, nil
	})
	return nil
}

func (q *notificationQueue) clicked(record *notificationRecord) {
	q.mu.Lock()
	click := record.click
	removed := q.removeLocked(record, true)
	q.mu.Unlock()
	if removed && click != nil {
		click()
	}
}

func (q *notificationQueue) close(id string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, record := range q.records {
		if record.id == id {
			q.removeLocked(record, true)
			return
		}
	}
}

// Pointer identity prevents queued clicks, expiry, and cancellation from removing a replacement.
func (q *notificationQueue) removeLocked(record *notificationRecord, closeNative bool) bool {
	i := slices.Index(q.records, record)
	if i < 0 {
		return false
	}
	q.records = slices.Delete(q.records, i, i+1)
	if record.timer != nil {
		record.timer.Stop()
		record.timer = nil
	}
	if record.stopContext != nil {
		record.stopContext()
		record.stopContext = nil
	}
	record.click = nil
	if closeNative {
		closePWANotification(record.id)
	}
	return true
}

func (q *notificationQueue) clear() {
	q.mu.Lock()
	defer q.mu.Unlock()
	for len(q.records) != 0 {
		q.removeLocked(q.records[0], true)
	}
}
