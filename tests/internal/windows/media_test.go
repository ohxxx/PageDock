//go:build desktop && windows

// [INPUT]: Windows WebView2, explicit test-device arguments, local dual origins, and real native settings saves.
// [OUTPUT]: Repeated camera/microphone grants and revocation, origin isolation, session retention, and local WebRTC send/receive tests.
// [POS]: Devices use Chromium test sources while permissions and codecs use the real runtime; this does not replace physical-device validation.
// [PROTOCOL]: Update internal/AGENTS.md, tests/AGENTS.md, patches/mygo/AGENTS.md, and validation records.
package pagedock

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/egoist/mygo"
	prefs "github.com/ohxxx/PageDock/internal/config"
	views "github.com/ohxxx/PageDock/internal/view"
)

func TestDesktopWindowsMediaPermissionRecovery(t *testing.T) {
	arguments := os.Getenv("WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS")
	if !strings.Contains(arguments, "--use-file-for-fake-video-capture=") || !strings.Contains(arguments, "--use-file-for-fake-audio-capture=") {
		t.Skip("请通过 tests/playground/windows/media.mjs 提供测试设备；不自动开启物理设备")
	}
	serve := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!doctype html><title>Media permission test</title><h1>Local media test</h1>`)
	})
	site, foreign := httptest.NewServer(serve), httptest.NewServer(serve)
	defer site.Close()
	defer foreign.Close()
	config := prefs.Config{URL: site.URL, AllowHTTP: true}
	if err := desktopApp.store.Save(config); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(desktopApp.store.Path)
	desktopApp.start()
	desktopApp.showSettings()
	settings, _ := desktopWindows()
	defer settings.Destroy()
	defer func() {
		_, win := desktopWindows()
		if win != nil {
			win.Destroy()
		}
	}()
	ready := func() {
		t.Helper()
		eventually(t, func() bool {
			value, _ := mygo.EvalAs[string](desktopPage(), `document.title`)
			return value == "Media permission test"
		})
	}
	ready()
	pageValue(t, desktopPage(), `document.cookie='media_session=retained;path=/'; localStorage.setItem('media_session','retained'); return 'ok';`)
	request := func(audio, video bool, allowed bool) {
		t.Helper()
		script := fmt.Sprintf(`try {
 const stream=await navigator.mediaDevices.getUserMedia({audio:%t,video:%t});
 const kinds=stream.getTracks().map(t=>t.kind).sort().join(',');
 stream.getTracks().forEach(t=>t.stop());
 return kinds+':'+stream.getTracks().every(t=>t.readyState==='ended');
} catch(e) { return e.name; }`, audio, video)
		want := "NotAllowedError"
		if allowed {
			kinds := []string{}
			if audio {
				kinds = append(kinds, "audio")
			}
			if video {
				kinds = append(kinds, "video")
			}
			want = strings.Join(kinds, ",") + ":true"
		}
		if got := pageValue(t, desktopPage(), script); got != want {
			t.Fatalf("audio=%t video=%t: got %q, want %q", audio, video, got, want)
		}
	}
	for _, permissions := range [][2]bool{{false, false}, {true, false}, {false, true}, {true, true}, {false, false}, {true, true}} {
		_, previous := desktopWindows()
		eventually(t, func() bool {
			idle := false
			nativeSettingsUpdate(t, func(v *views.Settings) { idle = !v.Busy })
			return idle
		})
		nativeSettingsUpdate(t, func(v *views.Settings) {
			v.Draft.AllowMicrophone, v.Draft.AllowCamera = permissions[0], permissions[1]
			v.Begin(views.Save)
		})
		eventually(t, previous.IsDestroyed)
		ready()
		request(true, false, permissions[0])
		request(false, true, permissions[1])
		request(true, true, permissions[0] && permissions[1])
		if got := pageValue(t, desktopPage(), `return String(document.cookie.includes('media_session=retained') && localStorage.getItem('media_session')==='retained');`); got != "true" {
			t.Fatal("permission update lost website login data")
		}
	}
	for _, target := range []string{foreign.URL, site.URL} {
		if err := desktopPage().LoadURL(target); err != nil {
			t.Fatal(err)
		}
		eventually(t, func() bool { return strings.TrimSuffix(desktopPage().URL(), "/") == target })
		ready()
		request(true, true, target == site.URL)
	}
	settings.Hide()
	_, main := desktopWindows()
	focus(main)
	var call struct {
		AudioPackets, VideoFrames int
		Width, Height             int
		AudioPeak                 float64
		Stopped                   bool
		Error                     string
	}
	raw := pageValue(t, desktopPage(), mediaLoopbackScript)
	if err := json.Unmarshal([]byte(raw), &call); err != nil {
		t.Fatal(raw, err)
	}
	if call.Error != "" || call.AudioPackets == 0 || call.VideoFrames < 5 || call.Width == 0 || call.Height == 0 || call.AudioPeak == 0 || !call.Stopped {
		t.Fatalf("local WebRTC call failed: %s", raw)
	}
	t.Logf("WebRTC audio/video received and stopped: %s", raw)
	// Capture must work again after a request finishes, without leaving devices occupied.
	request(true, true, true)
}

const mediaLoopbackScript = `
let stream, local, remote, video, preview, recorder, context, timer;
let result={}, stage='capture';
try {
 await Promise.race([(async()=>{
  stream=await navigator.mediaDevices.getUserMedia({audio:true,video:true});
  stage='local preview'; preview=document.createElement('video'); preview.muted=true; preview.playsInline=true;
  preview.style.width='320px'; preview.srcObject=stream; document.body.append(preview); await preview.play();
  await new Promise(resolve=>preview.requestVideoFrameCallback(resolve));
  stage='create peers'; local=new RTCPeerConnection({iceServers:[]}); remote=new RTCPeerConnection({iceServers:[]});
  const received=new MediaStream();
  remote.ontrack=e=>received.addTrack(e.track);
  stream.getTracks().forEach(track=>local.addTrack(track,stream));
  const gather=pc=>pc.iceGatheringState==='complete'?Promise.resolve():new Promise(resolve=>{
   pc.addEventListener('icegatheringstatechange',()=>{if(pc.iceGatheringState==='complete')resolve()});
  });
  stage='offer ICE'; await local.setLocalDescription(await local.createOffer()); await gather(local);
  await remote.setRemoteDescription(local.localDescription);
  stage='answer ICE'; await remote.setLocalDescription(await remote.createAnswer()); await gather(remote);
  await local.setRemoteDescription(remote.localDescription);
  video=document.createElement('video'); video.muted=true; video.playsInline=true;
  stage='play remote video'; video.style.width='320px'; video.srcObject=received; document.body.append(video); await video.play();
  const chunks=[]; recorder=new MediaRecorder(new MediaStream(received.getAudioTracks()));
  recorder.ondataavailable=e=>{if(e.data.size)chunks.push(e.data)};
  const recorded=new Promise((resolve,reject)=>{recorder.onstop=resolve;recorder.onerror=e=>reject(e.error)});
  recorder.start();
  await new Promise(resolve=>setTimeout(resolve,2500));
  stage='decode remote recording'; recorder.stop(); await recorded;
  context=new AudioContext(); const decoded=await context.decodeAudioData(await new Blob(chunks,{type:recorder.mimeType}).arrayBuffer());
  result.audioPeak=0; for(const sample of decoded.getChannelData(0)) result.audioPeak=Math.max(result.audioPeak,Math.abs(sample));
  result.width=video.videoWidth; result.height=video.videoHeight;
  for(const stat of (await remote.getStats()).values()) {
   if(stat.type==='inbound-rtp' && stat.kind==='audio') result.audioPackets=stat.packetsReceived;
   if(stat.type==='inbound-rtp' && stat.kind==='video') result.videoFrames=stat.framesDecoded;
  }
 })(),new Promise((_,reject)=>{timer=setTimeout(()=>reject(new Error('Local call timed out')),12000)})]);
} catch(e) {
 result.error=stage+': '+e.name+': '+e.message; result.connection=[local?.connectionState,remote?.connectionState];
 result.diagnostics=[];
 for(const pc of [local,remote]) if(pc) for(const s of (await pc.getStats()).values()) {
  if(s.type==='inbound-rtp'||s.type==='outbound-rtp'||s.type==='media-source') result.diagnostics.push({type:s.type,kind:s.kind,packetsReceived:s.packetsReceived,packetsSent:s.packetsSent,framesDecoded:s.framesDecoded,framesEncoded:s.framesEncoded,frames:s.frames,framesPerSecond:s.framesPerSecond});
 }
}
finally {
 clearTimeout(timer); stream?.getTracks().forEach(t=>t.stop()); local?.close(); remote?.close();
 if(recorder && recorder.state!=='inactive') recorder.stop(); if(context) await context.close();
 if(video) { video.pause(); video.srcObject=null; video.remove(); }
 if(preview) { preview.pause(); preview.srcObject=null; preview.remove(); }
 result.stopped=!!stream&&stream.getTracks().every(t=>t.readyState==='ended');
}
return JSON.stringify(result);`
