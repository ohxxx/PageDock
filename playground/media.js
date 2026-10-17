// [INPUT]: User actions and real getUserMedia/MediaRecorder; [OUTPUT]: Timed local capture and statistics, with silence excluded from success.
// [POS]: Standalone media validation page with folded event details; update playground/AGENTS.md and validation records.
const DURATION_MS = 3000;
const MAX_EVENTS = 30;
const session = crypto.randomUUID();
const byId = id => document.getElementById(id);
const preview = byId('preview');
const playback = byId('recording');
const buttons = ['microphone', 'camera', 'combined'].map(byId);
const labels = {
  'media-environment': '环境已就绪', 'media-request': '请求设备权限',
  'media-granted': '设备已授权', 'media-pass': '采集通过',
  'media-warning': '未检测到声音', 'media-error': '采集失败', 'media-released': '设备已释放',
};
let active;
let recordingURL;

function report(type, detail) {
  const event = { session, type, detail, time: new Date().toISOString() };
  const row = document.createElement('li');
  const title = document.createElement('strong');
  const data = document.createElement('pre');
  const time = document.createElement('time');
  const details = document.createElement('details');
  const summary = document.createElement('summary');
  time.textContent = new Date(event.time).toLocaleTimeString();
  title.textContent = labels[type] || type;
  if (type === 'media-error') row.dataset.level = 'error';
  data.textContent = JSON.stringify(detail, null, 2);
  summary.append(time, title);
  details.append(summary, data);
  row.append(details);
  byId('events').prepend(row);
  while (byId('events').children.length > MAX_EVENTS) byId('events').lastChild.remove();
  fetch('/api/events', {
    method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(event),
  }).then(response => {
    if (!response.ok) throw new Error(`HTTP ${response.status}`);
  }).catch(error => { byId('report-status').textContent = `诊断记录写入失败：${error.message}`; });
}

function releaseRecording() {
  playback.pause();
  playback.removeAttribute('src');
  playback.load();
  playback.hidden = true;
  if (recordingURL) URL.revokeObjectURL(recordingURL);
  recordingURL = undefined;
}

function waitForCapture(signal) {
  return new Promise(resolve => {
    const finish = () => {
      clearTimeout(timer);
      signal.removeEventListener('abort', finish);
      resolve();
    };
    const timer = setTimeout(finish, DURATION_MS);
    signal.addEventListener('abort', finish, { once: true });
    if (signal.aborted) finish();
  });
}

async function capture(kind) {
  if (active) return;
  const controller = new AbortController();
  const { signal } = controller;
  active = controller;
  buttons.forEach(button => { button.disabled = true; });
  byId('stop').disabled = false;
  releaseRecording();
  const constraints = { audio: kind !== 'camera', video: kind !== 'microphone' };
  let stream, context, recorder, frameCallback, meterTimer;
  let recordingError;
  let frames = 0;
  let peakRMS = 0;
  const chunks = [];
  const result = { kind, constraints };
  let recordingStopped;
  try {
    byId('feedback').textContent = '等待设备授权…';
    report('media-request', { kind, constraints });
    stream = await navigator.mediaDevices.getUserMedia(constraints);
    if (signal.aborted) throw new DOMException('测试已取消', 'AbortError');
    result.tracks = stream.getTracks().map(track => ({ kind: track.kind, state: track.readyState, muted: track.muted }));
    report('media-granted', { kind, tracks: result.tracks });
    if (constraints.video) {
      preview.hidden = false;
      preview.srcObject = stream;
      await preview.play();
      if (typeof preview.requestVideoFrameCallback !== 'function') throw new Error('此 WebView 不支持视频帧计数，无法完成摄像头验收');
      const countFrame = () => {
        frames++;
        if (!signal.aborted) frameCallback = preview.requestVideoFrameCallback(countFrame);
      };
      frameCallback = preview.requestVideoFrameCallback(countFrame);
    }
    if (constraints.audio) {
      context = new AudioContext();
      await context.resume();
      const analyser = context.createAnalyser();
      context.createMediaStreamSource(stream).connect(analyser);
      const samples = new Float32Array(analyser.fftSize);
      meterTimer = setInterval(() => {
        analyser.getFloatTimeDomainData(samples);
        const rms = Math.sqrt(samples.reduce((sum, value) => sum + value * value, 0) / samples.length);
        peakRMS = Math.max(peakRMS, rms);
      }, 50);
      recorder = new MediaRecorder(new MediaStream(stream.getAudioTracks()));
      recorder.addEventListener('dataavailable', event => { if (event.data.size) chunks.push(event.data); });
      recorder.addEventListener('error', event => { recordingError = event.error; });
      recordingStopped = new Promise(resolve => recorder.addEventListener('stop', resolve, { once: true }));
      recorder.start();
    }
    if (signal.aborted) throw new DOMException('测试已取消', 'AbortError');
    byId('feedback').textContent = '正在采集，3 秒后自动停止…';
    await waitForCapture(signal);
    if (signal.aborted) throw new DOMException('测试已取消', 'AbortError');
    if (recorder) {
      if (recorder.state !== 'inactive') recorder.stop();
      await recordingStopped;
      if (recordingError) throw recordingError;
    }
    if (constraints.video) {
      result.video = { width: preview.videoWidth, height: preview.videoHeight, frames };
      if (!frames || !preview.videoWidth || !preview.videoHeight) throw new Error('已授权但未收到有效视频帧');
    }
    // Release devices as soon as capture ends; decoding and playback must not keep the camera or microphone occupied.
    stream.getTracks().forEach(track => track.stop());
    if (constraints.audio) {
      const blob = new Blob(chunks, { type: recorder.mimeType });
      if (!blob.size) throw new Error('已授权但录音为空');
      const decoded = await context.decodeAudioData(await blob.arrayBuffer());
      result.audio = { bytes: blob.size, mimeType: blob.type, duration: decoded.duration, samples: decoded.length, peakRMS };
      if (!decoded.length || !decoded.duration) throw new Error('录音无法解码为有效音频');
      recordingURL = URL.createObjectURL(blob);
      playback.src = recordingURL;
      playback.hidden = false;
    }
    result.tracksAfterStop = stream.getTracks().map(track => track.readyState);
    report(constraints.audio && peakRMS === 0 ? 'media-warning' : 'media-pass', result);
    byId('feedback').textContent = constraints.audio
      ? (peakRMS === 0 ? '未检测到声音，请检查静音或输入设备。' : '录音完成，可回放。设备已释放。')
      : '采集通过，设备已释放。';
  } catch (error) {
    report('media-error', { kind, name: error.name, message: error.message });
    byId('feedback').textContent = `${error.name}：${error.message}`;
  } finally {
    clearInterval(meterTimer);
    if (frameCallback !== undefined) preview.cancelVideoFrameCallback(frameCallback);
    if (recorder && recorder.state !== 'inactive') recorder.stop();
    stream?.getTracks().forEach(track => track.stop());
    preview.srcObject = null;
    preview.hidden = true;
    if (context) await context.close();
    active = undefined;
    buttons.forEach(button => { button.disabled = false; });
    byId('stop').disabled = true;
    report('media-released', { kind, tracks: stream ? stream.getTracks().map(track => track.readyState) : [] });
  }
}

buttons.forEach(button => button.addEventListener('click', () => { void capture(button.id); }));
byId('stop').addEventListener('click', () => active?.abort());
window.addEventListener('pagehide', () => { active?.abort(); releaseRecording(); });
byId('origin').textContent = location.origin;
const environment = { secureContext: isSecureContext, getUserMedia: typeof navigator.mediaDevices?.getUserMedia === 'function', mediaRecorder: typeof MediaRecorder === 'function' };
byId('environment').textContent = `安全上下文：${environment.secureContext} · 设备采集 API：${environment.getUserMedia} · 录音 API：${environment.mediaRecorder}`;
report('media-environment', environment);
