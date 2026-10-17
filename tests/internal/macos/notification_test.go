//go:build desktop && darwin

// [INPUT]: Explicit PAGEDOCK_TEST_NATIVE_NOTIFICATIONS=1, a packaged test process, and local notification authorization.
// [OUTPUT]: System banner records for consecutive real WebView notifications; suppressed or missing banners fail validation.
// [POS]: Opt-in native notification validation; Show success or a substitute cannot prove banner presentation.
// [PROTOCOL]: Update internal/AGENTS.md and tests/AGENTS.md, including Validation records.
package pagedock

import (
	"crypto/rand"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/ohxxx/PageDock/internal/appearance"
	prefs "github.com/ohxxx/PageDock/internal/config"
)

func TestDesktopNativeNotificationBanners(t *testing.T) {
	if os.Getenv("PAGEDOCK_TEST_NATIVE_NOTIFICATIONS") != "1" {
		t.Skip("需要显式启用真实系统通知测试")
	}
	if !mygo.NotificationsSupported() {
		t.Fatal("必须从 .app 内运行测试二进制")
	}
	count, interval := 12, 800*time.Millisecond
	observe := time.Duration(0)
	if raw := os.Getenv("PAGEDOCK_TEST_NOTIFICATION_OBSERVE"); raw != "" {
		var err error
		observe, err = time.ParseDuration(raw)
		if err != nil || observe < 0 || observe > time.Minute {
			t.Fatalf("invalid observation duration %q (maximum 1m)", raw)
		}
	}
	if raw := os.Getenv("PAGEDOCK_TEST_NOTIFICATION_INTERVAL"); raw != "" {
		var err error
		interval, err = time.ParseDuration(raw)
		if err != nil || interval < 0 {
			t.Fatalf("invalid interval %q", raw)
		}
	}
	url := startPlaygroundLab(t)
	if err := desktopApp.store.Save(prefs.Config{URL: url + "/", AllowHTTP: true, AllowNotifications: true}); err != nil {
		t.Fatal(err)
	}
	desktopApp.start()
	_, window := desktopWindows()
	defer window.Destroy()
	window.Show()
	window.Focus()
	eventually(t, func() bool {
		value, _ := mygo.EvalAs[string](desktopPage(), `document.getElementById('worker')?.textContent`)
		return value == "已激活"
	})
	if os.Getenv("PAGEDOCK_TEST_NOTIFICATION_BACKGROUND") == "1" {
		mygo.App.Hide()
	}
	// Observe the real output while still calling native Show; never substitute a successful response.
	original := showPWANotification
	ids := make(chan string, count)
	showPWANotification = func(options mygo.NotificationOptions, click func()) error {
		t.Cleanup(func() { mygo.NewNotification(mygo.NotificationOptions{ID: options.ID}).Close() })
		ids <- options.ID
		return original(options, click)
	}
	defer func() { showPWANotification = original }()
	run := rand.Text()
	start := time.Now().Add(-time.Second)
	for i := range count {
		title := fmt.Sprintf("泊页连续通知验收 %s · %02d", run, i+1)
		pageValue(t, desktopPage(), `
			const options = {body:'本机横幅验收', tag:'', silent:true, data:{sequence:`+strconv.Itoa(i)+`}};
			if (`+strconv.Itoa(i)+` % 2) {
				await (await navigator.serviceWorker.ready).showNotification(`+strconv.Quote(title)+`, options);
			} else {
				await new Promise((resolve,reject) => {
					const n = new Notification(`+strconv.Quote(title)+`, options);
					n.onshow = resolve; n.onerror = reject;
				});
			}
			return 'accepted';
		`)
		time.Sleep(interval)
	}
	if observe > 0 {
		t.Logf("通知已全部发送，保留 %s 供现场观察，run %s", observe, run)
		time.Sleep(observe)
	}
	// System logs are asynchronous; allow the last banner to finish presenting and logging.
	time.Sleep(3 * time.Second)
	output, err := exec.Command("/usr/bin/log", "show", "--start", start.Format("2006-01-02 15:04:05"),
		"--style", "compact", "--info", "--debug", "--predicate",
		`(process == "NotificationCenter" OR process == "usernoted") AND eventMessage CONTAINS[c] `+strconv.Quote(appearance.InternalName)).CombinedOutput()
	if err != nil {
		t.Fatalf("system log: %v: %s", err, output)
	}
	if dir := os.Getenv("PAGEDOCK_TEST_EVIDENCE_DIR"); dir != "" {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "native-"+run+".log"), output, 0600); err != nil {
			t.Fatal(err)
		}
	}
	shown := 0
	for range count {
		id := <-ids
		if nativeNotificationDisplayed(string(output), id) {
			shown++
		} else {
			t.Logf("missing banner: %s", id)
		}
	}
	t.Logf("native banners %d/%d, interval %s, run %s", shown, count, interval, run)
	if shown != count {
		t.Fatalf("系统横幅缺失：%d/%d", shown, count)
	}
}

// Persistent-alert presentation lines contain only the record UUID; mutedAlert or system acceptance does not prove visibility.
func nativeNotificationDisplayed(log, id string) bool {
	var records []string
	uuid := regexp.MustCompile(`uuid:"([^"]+)"`)
	for line := range strings.SplitSeq(log, "\n") {
		if !strings.Contains(line, `req:"`+id+`"`) {
			continue
		}
		if match := uuid.FindStringSubmatch(line); match != nil {
			records = append(records, ":"+match[1]+":")
		}
	}
	for line := range strings.SplitSeq(log, "\n") {
		if !strings.Contains(line, "displaying as banner") && !strings.Contains(line, "displaying as alert") {
			continue
		}
		if strings.Contains(line, "mutedAlert") {
			continue
		}
		for _, record := range records {
			if strings.Contains(line, record) {
				return true
			}
		}
	}
	return false
}
