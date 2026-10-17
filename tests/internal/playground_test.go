//go:build desktop

// [INPUT]: The actual playground Node server, native windows, WKWebView, and a notification output observer.
// [OUTPUT]: Settings-controlled permissions and real playground button tests for independent sends, individual clicks, replacement, closing, and origin isolation.
// [POS]: Frontend-to-Go integration; packaged apps validate system banners separately, without treating substitutes as delivery evidence.
// [PROTOCOL]: Update internal/AGENTS.md, playground/AGENTS.md, and tests/AGENTS.md, including Validation records.
package pagedock

import (
	"bufio"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo"
	prefs "github.com/ohxxx/PageDock/internal/config"
)

func startPlaygroundLab(t *testing.T) string {
	t.Helper()
	command := exec.Command("node", filepath.Join("..", "playground", "server.mjs"))
	command.Env = append(os.Environ(), "PORT=0", "FOREIGN_PORT=0")
	command.Stderr = os.Stderr
	output, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = command.Process.Kill(); _ = command.Wait() })
	ready := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(output)
		if scanner.Scan() {
			ready <- scanner.Text()
		} else {
			ready <- ""
		}
	}()
	select {
	case line := <-ready:
		var server struct {
			URL string `json:"url"`
		}
		if err := json.Unmarshal([]byte(line), &server); err != nil || server.URL == "" {
			t.Fatalf("playground did not start: %q, %v", line, err)
		}
		return server.URL
	case <-time.After(10 * time.Second):
		t.Fatal("playground startup timed out")
		return ""
	}
}

func TestDesktopPlaygroundNotificationLab(t *testing.T) {
	url := startPlaygroundLab(t)
	type delivery struct {
		options mygo.NotificationOptions
		click   func()
	}
	deliveries := make(chan delivery, 16)
	closed := make(chan string, 16)
	present, dismiss := showPWANotification, closePWANotification
	showPWANotification = func(options mygo.NotificationOptions, click func()) error {
		if options.Title == "拒绝路径" {
			return mygo.ErrNotificationsDenied
		}
		deliveries <- delivery{options, click}
		return nil
	}
	closePWANotification = func(id string) { closed <- id }
	defer func() { showPWANotification, closePWANotification = present, dismiss }()
	config := prefs.Config{URL: url + "/", AllowHTTP: true, AllowNotifications: true}
	if err := desktopApp.store.Save(config); err != nil {
		t.Fatal(err)
	}
	desktopApp.start()
	defer func() {
		_, window := desktopWindows()
		if window != nil {
			window.Destroy()
		}
	}()
	ready := func() {
		eventually(t, func() bool {
			value, _ := mygo.EvalAs[string](desktopPage(), `document.getElementById('worker')?.textContent`)
			return value == "已激活"
		})
	}
	ready()
	click := func(id string) {
		t.Helper()
		pageValue(t, desktopPage(), `document.getElementById(`+strconv.Quote(id)+`).click(); return "clicked";`)
	}
	feedback := func(want string) {
		t.Helper()
		eventually(t, func() bool {
			value, _ := mygo.EvalAs[string](desktopPage(), `document.getElementById('feedback').textContent`)
			return strings.Contains(value, want)
		})
	}
	next := func() delivery {
		t.Helper()
		select {
		case got := <-deliveries:
			return got
		case <-time.After(5 * time.Second):
			t.Fatal("no native delivery from playground button")
			return delivery{}
		}
	}
	click("permission-request")
	feedback("权限结果")
	if value := pageValue(t, desktopPage(), `return document.getElementById('permission').textContent;`); value != "granted" {
		t.Fatal(value)
	}
	click("direct")
	direct := next()
	feedback("普通通知 API 已接收")
	if direct.options.Title != "泊页 · 一条测试消息" || !strings.Contains(direct.options.Body, "你好") {
		t.Fatalf("wrong payload: %+v", direct.options)
	}
	direct.click()
	feedback("点击回传")
	if got := pageValue(t, desktopPage(), `return document.getElementById('last-click').textContent;`); !strings.Contains(got, "conversation-01") {
		t.Fatal(got)
	}
	click("close")
	select {
	case id := <-closed:
		if id != direct.options.ID {
			t.Fatal("closed wrong notification")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("close missing")
	}
	// Keep every default consecutive message; send all first, then check clicks individually to catch accidental merging.
	seen := map[string]bool{direct.options.ID: true}
	var burst []delivery
	for i := range 6 {
		room := "burst-" + strconv.Itoa(i)
		pageValue(t, desktopPage(), `document.getElementById('room').value=`+strconv.Quote(room)+`; return 'set';`)
		button, receipt := "direct", "普通通知 API 已接收"
		if i%2 != 0 {
			button, receipt = "pwa", "PWA API 已接收"
		}
		click(button)
		d := next()
		feedback(receipt)
		if seen[d.options.ID] {
			t.Fatalf("independent notification %d replaced an earlier message: %s", i, d.options.ID)
		}
		seen[d.options.ID] = true
		burst = append(burst, d)
	}
	for i, d := range burst {
		d.click()
		want := "burst-" + strconv.Itoa(i)
		eventually(t, func() bool {
			got, _ := mygo.EvalAs[string](desktopPage(), `document.getElementById('last-click').textContent`)
			return strings.Contains(got, want)
		})
	}
	pageValue(t, desktopPage(), `document.getElementById('room').value='conversation-01'; document.getElementById('tag').value='replacement'; return 'set';`)
	click("replace")
	old, replacement := next(), next()
	feedback("旧对象已请求关闭")
	if old.options.ID != replacement.options.ID {
		t.Fatal("same tag failed to replace")
	}
	if got := pageValue(t, desktopPage(), `
		const requests = Array.from(document.querySelectorAll('#events li'))
			.filter(row => row.querySelector('strong').textContent === '普通通知请求')
			.map(row => JSON.parse(row.querySelector('pre').textContent))
			.filter(message => message.options.tag === 'replacement');
		return String(new Set(requests.map(message => message.options.data.messageId)).size);
	`); got != "2" {
		t.Fatal("replacement requests reused a diagnostic messageId:", got)
	}
	select {
	case <-closed:
		t.Fatal("old object closed replacement")
	default:
	}
	click("pwa")
	pwa := next()
	feedback("PWA API 已接收")
	pwa.click()
	feedback("点击回传")
	if got := pageValue(t, desktopPage(), `return document.getElementById('last-click').textContent;`); !strings.HasPrefix(got, "PWA →") {
		t.Fatal(got)
	}
	// Report current PWA API coverage diagnostically; an empty result does not prove support.
	click("pwa")
	next()
	feedback("PWA API 已接收")
	click("list-pwa")
	feedback("PWA 查询结果")
	t.Log("PWA enumeration diagnostic:", pageValue(t, desktopPage(), `return document.getElementById('events').firstElementChild.textContent;`))
	pageValue(t, desktopPage(), `document.querySelector('details').open=true; document.getElementById('delay').value='1'; document.getElementById('background-only').checked=true; return 'set';`)
	visibility := pageValue(t, desktopPage(), `return document.visibilityState;`)
	click("schedule")
	if visibility == "visible" {
		feedback("网站规则跳过通知")
		select {
		case <-deliveries:
			t.Fatal("visible-page filter sent notification")
		default:
		}
	} else {
		next()
		feedback("PWA API 已接收")
	}
	pageValue(t, desktopPage(), `document.getElementById('background-only').checked=false; return 'set';`)
	click("schedule")
	next()
	feedback("PWA API 已接收")
	click("schedule")
	click("cancel")
	feedback("延迟已取消")
	select {
	case <-deliveries:
		t.Fatal("cancelled timer sent notification")
	case <-time.After(1200 * time.Millisecond):
	}
	foreignURL := pageValue(t, desktopPage(), `return document.getElementById('foreign').href;`)
	t.Log("playground secondary origin:", foreignURL)
	foreignJSON, _ := json.Marshal(foreignURL)
	_, _ = desktopPage().Eval("location.href = " + string(foreignJSON))
	foreignOrigin := strings.TrimSuffix(foreignURL, "/")
	eventually(t, func() bool { return desktopPage().URL() == foreignOrigin+"/" })
	got := pageValue(t, desktopPage(), `try { await mygo.call('PWA.Receive', {title:'非法来源'}); return 'allowed'; } catch { return 'denied'; }`)
	if got != "denied" {
		t.Fatal("foreign origin reached native bridge")
	}
	pageValue(t, desktopPage(), `location.href=`+strconv.Quote(url+"/")+`; return 'returning';`)
	eventually(t, func() bool { return desktopPage().URL() == url+"/" })
	ready()
	pageValue(t, desktopPage(), `document.getElementById('title').value='拒绝路径'; return 'set';`)
	click("pwa")
	feedback("操作失败")
	if got := pageValue(t, desktopPage(), `return document.getElementById('permission').textContent;`); got != "denied" {
		t.Fatal(got)
	}
	desktopApp.mu.Lock()
	updated := desktopApp.config
	updated.AllowNotifications = false
	err := desktopApp.saveSettingsLocked(updated)
	desktopApp.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	ready()
	click("direct")
	feedback("操作失败")
	select {
	case <-deliveries:
		t.Fatal("disabled notifications sent a message")
	default:
	}
}
