//go:build desktop && pagedock_soak && darwin

// [INPUT]: Explicit soak flags, isolated loopback pages, and test-only native notification transport.
// [OUTPUT]: Repeated real WebView notification/navigation/window lifetimes with behavioral assertions.
// [POS]: Opt-in macOS endurance test; tests/soak/run.py prepares the isolated dependency overlay.
// [PROTOCOL]: Update tests/internal/AGENTS.md, tests/soak/AGENTS.md, and validation records.
package pagedock

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"
	"weak"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	prefs "github.com/ohxxx/PageDock/internal/config"
	website "github.com/ohxxx/PageDock/internal/site"
	views "github.com/ohxxx/PageDock/internal/view"
)

var (
	soakDuration = flag.Duration("soak-duration", 30*time.Minute, "active endurance duration")
	soakRecovery = flag.Duration("soak-recovery", 5*time.Minute, "idle recovery duration")
	soakPace     = flag.Duration("soak-pace", 3*time.Second, "minimum interval between window cycles")
	soakOutput   = flag.String("soak-output", "", "absolute evidence directory")
	soakBatch    = flag.Int("soak-batch", 128, "notifications per trusted page; zero isolates navigation/window churn")
	soakVisits   = flag.Int("soak-visits", 3, "trusted page visits per window")
)

const soakPage = `<!doctype html><meta charset="utf-8"><style>
body{background:#182230;color:white}article{height:4px}</style><h1>PageDock isolated endurance test</h1>
<script>
window.soakPayload=new Uint8Array(2*1024*1024);soakPayload.fill(71);
const fragment=document.createDocumentFragment();
for(let i=0;i<500;i++){const row=document.createElement('article');row.textContent='row '+i;fragment.append(row);}
document.body.append(fragment);window.soakClicks=0;window.soakPath=location.pathname;window.soakReady=true;
</script>`

type soakReferences struct {
	windows []weak.Pointer[mygo.Window]
	pages   []weak.Pointer[mygo.Page]
	views   []weak.Pointer[views.Browser]
}

func TestDesktopResourceSoak(t *testing.T) {
	if *soakOutput == "" || !filepath.IsAbs(*soakOutput) || *soakDuration <= 0 || *soakRecovery <= 0 || *soakPace <= 0 || *soakBatch < 0 || *soakVisits < 1 {
		t.Fatal("use tests/soak/run.py with positive duration/recovery/pace and an absolute evidence directory")
	}
	serve := func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		fmt.Fprint(w, soakPage)
	}
	site := httptest.NewServer(http.HandlerFunc(serve))
	defer site.Close()
	foreign := httptest.NewServer(http.HandlerFunc(serve))
	defer foreign.Close()
	config := prefs.Config{URL: site.URL + "/home", AllowHTTP: true, AllowNotifications: true}
	if err := desktopApp.store.Save(config); err != nil {
		t.Fatal(err)
	}
	desktopApp.mu.Lock()
	desktopApp.config = config
	desktopApp.mu.Unlock()
	defer func() {
		desktopApp.mu.Lock()
		desktopApp.stopSiteLocked()
		desktopApp.mu.Unlock()
		appNotifications.clear()
		mygo.ClearNotifications()
	}()
	stats := newSoakMetrics(t)
	defer stats.close()
	refs := &soakReferences{}
	stats.checkpoint(t, "cold", refs, true)
	deadline := time.Now().Add(*soakDuration)
	nextCheckpoint := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		started := time.Now()
		soakCycle(t, site.URL, foreign.URL, config, stats, refs)
		stats.sample(t, "loaded", refs, false)
		if time.Now().After(nextCheckpoint) {
			stats.checkpoint(t, "loaded", refs, true)
			nextCheckpoint = time.Now().Add(30 * time.Second)
		}
		if remaining := *soakPace - time.Since(started); remaining > 0 {
			time.Sleep(remaining)
		}
	}
	stats.checkpoint(t, "loaded_final", refs, true)
	appNotifications.clear()
	mygo.ClearNotifications()
	stats.checkpoint(t, "cleared", refs, true)
	deadline = time.Now().Add(*soakRecovery)
	for time.Now().Before(deadline) {
		time.Sleep(min(30*time.Second, time.Until(deadline)))
		stats.checkpoint(t, "recovery", refs, true)
	}
	stats.finish(t, refs)
}

func soakCycle(t *testing.T, source, foreign string, config prefs.Config, stats *soakMetrics, refs *soakReferences) {
	t.Helper()
	v := &views.Browser{Menu: views.NewMenu(config)}
	win, err := newWindow(mygo.WindowOptions{Title: "PageDock isolated endurance test", Width: 620, Height: 420,
		Hidden: true, TitleBarStyle: mygo.TitleBarHidden, Content: ui.View(v.Render)}, config.DisplayName())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		desktopApp.mu.Lock()
		desktopApp.stopSiteLocked()
		desktopApp.mu.Unlock()
	}()
	web, err := win.NewWebView(mygo.WebViewOptions{Page: mygo.PageOptions{
		TrustedOrigins: []string{source}, PreloadScript: website.Preload(source, true),
	}})
	if err != nil {
		win.Destroy()
		t.Fatal(err)
	}
	win.Update(func() { v.Web = web })
	desktopApp.mu.Lock()
	desktopApp.main, desktopApp.mainWeb = win, web
	desktopApp.mu.Unlock()
	page := web.Page()
	v.FollowLoading(win, page)
	v.FollowBackground(win, page)
	refs.windows = append(refs.windows, weak.Make(win))
	refs.pages = append(refs.pages, weak.Make(page))
	refs.views = append(refs.views, weak.Make(v))
	win.ShowInactive()
	for visit := range *soakVisits {
		url := fmt.Sprintf("%s/cycle-%d/page-%d", source, stats.cycles, visit)
		soakNavigate(t, page, url)
		stats.navigations++
		soakWait(t, func() bool { return soakActiveCallbacks() == 0 }, "old document callbacks")
		if *soakBatch == 0 {
			continue
		}
		batch := *soakBatch
		code := fmt.Sprintf(`(async()=>{
const count=%d;
for(let start=0;start<count;start+=16){await Promise.all(Array.from({length:Math.min(16,count-start)},(_,offset)=>new Promise((resolve,reject)=>{
const i=start+offset;const n=new Notification('soak '+i,{body:'message '+i+' '+ 'x'.repeat(2000),
tag:i%%8===0?'replacement-'+(i%%4):'',silent:true,data:{payload:'d'.repeat(1024)}});
n.onclick=()=>{window.soakClicks++};n.onshow=resolve;n.onerror=()=>reject(new Error('notification failed'));window.soakLast=n;
})));}return %d;})()`, batch, batch)
		value := soakEval(t, page, code)
		if value != float64(batch) {
			t.Fatalf("notification batch: %v", value)
		}
		stats.notifications += batch
		soakEval(t, page, `(async()=>{await new Promise(resolve=>{soakLast.onclose=resolve;soakLast.close()});return true;})()`)
		before := soakEval(t, page, "window.soakClicks").(float64)
		appNotifications.mu.Lock()
		var ids []string
		for i := len(appNotifications.records) - 1; i >= 0 && len(ids) < 2; i-- {
			record := appNotifications.records[i]
			if record.click != nil {
				ids = append(ids, record.id)
			}
		}
		appNotifications.mu.Unlock()
		if len(ids) != 2 {
			t.Fatal("active notification click routes missing")
		}
		for _, id := range ids {
			mygo.PageDockSoakClick(id)
			stats.clicks++
		}
		soakWait(t, func() bool {
			value, err := mygo.EvalAs[float64](page, "window.soakClicks")
			return err == nil && value == before+2
		}, "notification click delivery")
		for range 8 {
			if err := sendTestNotification(config.DisplayName()); err != nil {
				t.Fatal(err)
			}
			stats.notifications++
		}
		soakCheckNative(t)
	}
	soakNavigate(t, page, foreign+"/foreign-"+strconv.Itoa(stats.cycles))
	stats.navigations++
	soakWait(t, func() bool { return soakActiveCallbacks() == 0 }, "cross-origin callback release")
	if got := soakEval(t, page, `(async()=>{try{await mygo.call('PWA.Receive',{title:'foreign'});return 'allowed'}catch{return 'denied'}})()`); got != "denied" {
		t.Fatalf("foreign notification authorization: %v", got)
	}
	win.Hide()
	time.Sleep(50 * time.Millisecond)
	win.ShowInactive()
	stats.cycles++
}

func soakNavigate(t *testing.T, page *mygo.Page, url string) {
	t.Helper()
	var mu sync.Mutex
	var events []string
	record := func(event string) {
		mu.Lock()
		defer mu.Unlock()
		if len(events) < 20 {
			events = append(events, event)
		}
	}
	off := []func(){
		page.OnDidNavigate(func(url string) { record("commit " + url) }),
		page.OnDOMReady(func() { record("DOM ready") }),
		page.OnDidFinishLoad(func() { record("finished") }),
		page.OnDidFailLoad(func(err *mygo.LoadError) { record(fmt.Sprintf("failed %+v", err)) }),
		page.OnRenderProcessGone(func(reason string) { record("render process gone: " + reason) }),
	}
	defer func() {
		for _, unsubscribe := range off {
			unsubscribe()
		}
	}()
	if err := page.LoadURL(url); err != nil {
		t.Fatal(err)
	}
	expression := "window.soakReady === true && location.href === " + strconv.Quote(url)
	deadline := time.Now().Add(20 * time.Second)
	var value any
	var lastErr error
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		value, lastErr = page.EvalContext(ctx, expression)
		cancel()
		if lastErr == nil && value == true {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	document, inspectErr := page.EvalContext(ctx, `JSON.stringify({url:location.href,state:document.readyState,ready:window.soakReady,body:document.body?.innerText.slice(0,100)})`)
	mu.Lock()
	trace := append([]string(nil), events...)
	mu.Unlock()
	t.Fatalf("navigation readiness: target=%s current=%s loading=%t value=%v error=%v document=%v inspect_error=%v events=%v",
		url, page.URL(), page.IsLoading(), value, lastErr, document, inspectErr, trace)
}

func soakEval(t *testing.T, page *mygo.Page, script string) any {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	value, err := page.EvalContext(ctx, script)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func soakWait(t *testing.T, check func() bool, reason string) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("timed out: " + reason)
}

func soakActiveCallbacks() int {
	appNotifications.mu.Lock()
	defer appNotifications.mu.Unlock()
	n := 0
	for _, record := range appNotifications.records {
		if record.click != nil {
			n++
		}
	}
	return n
}

func soakCheckNative(t *testing.T) {
	t.Helper()
	appNotifications.mu.Lock()
	defer appNotifications.mu.Unlock()
	native := mygo.PageDockSoakNativeCount()
	if native != len(appNotifications.records) || native > maxNativeNotifications {
		t.Fatalf("native records=%d queue=%d limit=%d", native, len(appNotifications.records), maxNativeNotifications)
	}
}
