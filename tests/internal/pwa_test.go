//go:build desktop

// [INPUT]: Local HTTP websites, real WKWebView/Service Worker, and observable native notification output.
// [OUTPUT]: Permission, both notification entry points, click/close, native rejection, and origin isolation tests.
// [POS]: No remote accounts, business messages, or changes to real websites.
// [PROTOCOL]: Update this header, internal/AGENTS.md, and tests/AGENTS.md; update the root AGENTS.md when directory boundaries change.
package pagedock

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/egoist/mygo"
	prefs "github.com/ohxxx/PageDock/internal/config"
)

func TestDesktopPWANotifications(t *testing.T) {
	type delivery struct {
		options mygo.NotificationOptions
		click   func()
	}
	delivered := make(chan delivery, 8)
	closed := make(chan string, 8)
	present, closeNotification := showPWANotification, closePWANotification
	showPWANotification = func(opts mygo.NotificationOptions, click func()) error {
		if opts.Title == "系统拒绝" {
			return mygo.ErrNotificationsDenied
		}
		delivered <- delivery{opts, click}
		return nil
	}
	closePWANotification = func(id string) { closed <- id }
	defer func() { showPWANotification, closePWANotification = present, closeNotification }()
	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/sw.js" {
			w.Header().Set("Content-Type", "application/javascript")
			fmt.Fprint(w, `self.addEventListener("install", () => self.skipWaiting()); self.addEventListener("activate", e => e.waitUntil(self.clients.claim()));`)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><h1>Notifications</h1><script>window.ready = true;</script>`)
	}))
	defer site.Close()
	foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<!doctype html><h1>Other origin</h1><script>window.ready = true;</script>`)
	}))
	defer foreign.Close()
	config := prefs.Config{URL: site.URL + "/", AllowHTTP: true, AllowNotifications: true}
	if err := desktopApp.store.Save(config); err != nil {
		t.Fatal(err)
	}
	desktopApp.start()
	defer func() {
		_, current := desktopWindows()
		if current != nil {
			current.Destroy()
		}
	}()
	eventually(t, func() bool { v, _ := mygo.EvalAs[bool](desktopPage(), `window.ready === true`); return v })
	next := func() delivery {
		t.Helper()
		select {
		case d := <-delivered:
			return d
		case <-time.After(5 * time.Second):
			t.Fatal("no native notification")
			return delivery{}
		}
	}
	if got := pageValue(t, desktopPage(), `return Notification.permission + ":" + await Notification.requestPermission();`); got != "granted:granted" {
		t.Fatalf("permission = %s", got)
	}
	got := pageValue(t, desktopPage(), `
		window.notice = new Notification("新消息", {body:"来自网站", tag:"message-42", data:{roomId:"direct"}, silent:true});
		notice.onclick = () => { window.clicked = notice.data.roomId; };
		await new Promise((resolve, reject) => { notice.onshow = resolve; notice.onerror = reject; });
		return notice.title + ":" + notice.body;
	`)
	if got != "新消息:来自网站" {
		t.Fatalf("Notification fields: %s", got)
	}
	direct := next()
	if direct.options.ID != pwaNotificationID(site.URL, "message-42") || !direct.options.Silent {
		t.Fatalf("native options: %+v", direct.options)
	}
	direct.click()
	eventually(t, func() bool { v, _ := mygo.EvalAs[string](desktopPage(), `window.clicked || ''`); return v == "direct" })
	if got := pageValue(t, desktopPage(), `await new Promise(resolve => { notice.onclose = resolve; notice.close(); }); return "closed";`); got != "closed" {
		t.Fatal(got)
	}
	select {
	case id := <-closed:
		if id != direct.options.ID {
			t.Fatal("closed wrong notification")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("close did not reach MyGo")
	}

	// After a same-tag replacement, closing the old object must not close the new notification.
	got = pageValue(t, desktopPage(), `
		const show = title => new Promise((resolve, reject) => {
			const n = new Notification(title, {tag:"replacement"}); n.onshow=()=>resolve(n); n.onerror=reject;
		});
		window.older = await show("旧通知"); window.newer = await show("新通知");
		await new Promise(resolve => { older.onclose=resolve; older.close(); });
		return "replaced";
	`)
	if got != "replaced" {
		t.Fatal(got)
	}
	next()
	replacement := next()
	select {
	case <-closed:
		t.Fatal("old Notification.close removed its replacement")
	default:
	}
	pageValue(t, desktopPage(), `await new Promise(resolve => { newer.onclose=resolve; newer.close(); }); return "closed";`)
	select {
	case id := <-closed:
		if id != replacement.options.ID {
			t.Fatal("closed wrong replacement")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("replacement close missing")
	}

	// Reproduce wbdev's preferred path: read the active registration in the page, then call showNotification.
	got = pageValue(t, desktopPage(), `
		await navigator.serviceWorker.register("/sw.js");
		window.registration = await navigator.serviceWorker.ready;
		window.addEventListener("NOTIFICATION_CLICK", e => { window.workerClick = e.detail.roomId; });
		await registration.showNotification("会话新消息", {body:"PWA 消息", tag:"room-42", data:{roomId:"worker",url:"/conversation/worker"}});
		return "shown";
	`)
	if got != "shown" {
		t.Fatal(got)
	}
	worker := next()
	if worker.options.Title != "会话新消息" || worker.options.Body != "PWA 消息" {
		t.Fatalf("PWA options: %+v", worker.options)
	}
	worker.click()
	eventually(t, func() bool {
		v, _ := mygo.EvalAs[string](desktopPage(), `window.workerClick || ''`)
		return v == "worker"
	})
	got = pageValue(t, desktopPage(), `try { await registration.showNotification("系统拒绝"); return "incorrectly-resolved"; } catch { return Notification.permission; }`)
	if got != "denied" {
		t.Fatalf("system denial was swallowed: %s", got)
	}

	// A cross-site login page cannot call the PWA notification service or close target-site/test notifications.
	foreignURL, _ := json.Marshal(foreign.URL)
	_, _ = desktopPage().Eval("location.href = " + string(foreignURL))
	eventually(t, func() bool {
		v, _ := mygo.EvalAs[bool](desktopPage(), `window.ready === true`)
		return desktopPage().URL() == foreign.URL+"/" && v
	})
	got = pageValue(t, desktopPage(), `try { await mygo.call("PWA.Receive",{title:"非法来源"}); return "allowed"; } catch { return "denied"; }`)
	if got != "denied" {
		t.Fatal("foreign origin received notification access")
	}
	// An old notification click must not reach a new document after returning to the original site.
	returnURL, _ := json.Marshal(site.URL)
	_, _ = desktopPage().Eval("location.href = " + string(returnURL))
	eventually(t, func() bool {
		v, _ := mygo.EvalAs[bool](desktopPage(), `window.ready === true`)
		return desktopPage().URL() == site.URL+"/" && v
	})
	got = pageValue(t, desktopPage(), `try { await mygo.call("PWA.Close","pagedock-notification-test"); return "allowed"; } catch { return "denied"; }`)
	if got != "denied" {
		t.Fatal("website closed test notification")
	}

	pageValue(t, desktopPage(), `window.leakedClick=false; mygo.on("pagedock:notification-click",()=>{window.leakedClick=true;}); return "listening";`)
	direct.click()
	got = pageValue(t, desktopPage(), `await mygo.call("PWA.Receive",{title:"导航后的消息"}); return String(window.leakedClick);`)
	next()
	if got != "false" {
		t.Fatal("old page notification click leaked into the new document")
	}

	config.AllowNotifications = false
	desktopApp.mu.Lock()
	err := desktopApp.saveSettingsLocked(config)
	desktopApp.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { v, _ := mygo.EvalAs[bool](desktopPage(), `window.ready === true`); return v })
	got = pageValue(t, desktopPage(), `try { await mygo.call("PWA.Receive",{title:"已关闭"}); return "allowed"; } catch { return Notification.permission; }`)
	if got != "denied" {
		t.Fatal("disabled notifications still allowed")
	}
	select {
	case d := <-delivered:
		t.Fatalf("unexpected notification: %+v", d.options)
	default:
	}
}
