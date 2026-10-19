//go:build windows

// [INPUT]: A new evidence directory and a bounded lifetime.
// [OUTPUT]: Two isolated Go HTTP origins and independent server handle counts.
// [POS]: Diagnostic separating the soak's HTTP server from its WebView host.
// [PROTOCOL]: Update tests/soak/AGENTS.md; never bind outside loopback.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

func main() {
	output := flag.String("output", "", "new absolute evidence directory")
	duration := flag.Duration("duration", time.Hour, "maximum server lifetime")
	flag.Parse()
	if !filepath.IsAbs(*output) || *duration <= 0 {
		panic("absolute output and positive duration are required")
	}
	if err := os.Mkdir(*output, 0700); err != nil {
		panic(err)
	}
	var requests atomic.Uint64
	serve := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		fmt.Fprint(w, `<!doctype html><meta charset="utf-8"><style>body{background:#182230;color:white}article{height:4px}</style>
<h1>PageDock isolated endurance test</h1><script>
window.soakPayload=new Uint8Array(2*1024*1024);soakPayload.fill(71);
const fragment=document.createDocumentFragment();
for(let i=0;i<500;i++){const row=document.createElement('article');row.textContent='row '+i;fragment.append(row);}
document.body.append(fragment);window.soakClicks=0;window.soakPath=location.pathname;window.soakReady=true;
</script>`)
	})
	source, foreign := httptest.NewServer(serve), httptest.NewServer(serve)
	defer source.Close()
	defer foreign.Close()
	data, err := json.Marshal(map[string]any{"pid": os.Getpid(), "origins": source.URL + "," + foreign.URL})
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(filepath.Join(*output, "origins.json"), data, 0600); err != nil {
		panic(err)
	}
	log, err := os.Create(filepath.Join(*output, "handles.jsonl"))
	if err != nil {
		panic(err)
	}
	defer log.Close()
	count := syscall.NewLazyDLL("kernel32.dll").NewProc("GetProcessHandleCount")
	started := time.Now()
	for time.Since(started) < *duration {
		var handles uint32
		if ok, _, err := count.Call(^uintptr(0), uintptr(unsafe.Pointer(&handles))); ok == 0 {
			panic(err)
		}
		if err := json.NewEncoder(log).Encode(map[string]any{"elapsed_s": time.Since(started).Seconds(), "handles": handles, "requests": requests.Load()}); err != nil {
			panic(err)
		}
		if _, err := os.Stat(filepath.Join(*output, "stop")); err == nil {
			return
		} else if !os.IsNotExist(err) {
			panic(err)
		}
		time.Sleep(time.Second)
	}
}
