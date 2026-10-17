// [INPUT]: Standard Notification/Service Worker APIs, user forms, and real system clicks.
// [OUTPUT]: Independent sends, explicit replacement, closing, and origin diagnostics; folded details retain messageId request/receipt correlation.
// [POS]: Standalone test site without MyGo mocks or PageDock config access; update playground/AGENTS.md.
const $ = id => document.getElementById(id);
const MAX_EVENTS = 200;
const TIMEOUT_MS = 10000;
const session = crypto.randomUUID();
const events = [];
let registration;
let latest;
let scheduled;
let reportChain = Promise.resolve();

function record(type, detail = {}, level = 'info') {
  const event = { session, time: new Date().toISOString(), type, detail, level };
  events.push(event);
  if (events.length > MAX_EVENTS) events.shift();
  const row = document.createElement('li');
  row.dataset.level = level;
  const time = document.createElement('time');
  time.textContent = new Date(event.time).toLocaleTimeString();
  const title = document.createElement('strong');
  title.textContent = type;
  const body = document.createElement('pre');
  body.textContent = JSON.stringify(detail);
  const details = document.createElement('details');
  const summary = document.createElement('summary');
  summary.append(time, title);
  details.append(summary, body);
  row.append(details);
  $('events').prepend(row);
  if ($('events').children.length > MAX_EVENTS) $('events').lastElementChild.remove();
  $('feedback').textContent = `${type}${detail.error ? ' · ' + detail.error : ''}`;
  $('feedback').dataset.level = level;
  reportChain = reportChain.then(async () => {
    const response = await fetch('/api/events', {
      method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(event),
    });
    if (!response.ok) throw new Error(`HTTP ${response.status}`);
  }).catch(error => { $('report-status').textContent = `服务端证据写入失败：${error}；请导出页面日志。`; });
  refreshStatus();
}

function refreshStatus() {
  $('runtime').textContent = typeof globalThis.mygo?.call === 'function' ? '泊页 WebView' : '外部浏览器';
  $('permission').textContent = typeof Notification === 'function' ? Notification.permission : 'API 不可用';
  $('visibility').textContent = `${document.visibilityState} / ${document.hasFocus() ? '聚焦' : '失焦'}`;
  $('origin').textContent = location.origin;
}

function payload() {
  if (!$('title').value.trim()) throw new Error('请填写通知标题');
  return {
    title: $('title').value,
    options: {
      body: $('body').value, tag: $('tag').value, silent: $('silent').checked,
      data: { roomId: $('room').value, messageId: crypto.randomUUID(), url: location.href },
    },
  };
}

function worker() {
  if (!registration?.active) throw new Error('Service Worker 尚未就绪');
  return registration;
}

function clicked(kind, data) {
  $('last-click').textContent = `${kind} → ${data?.roomId || '已返回'}`;
  record('点击回传', { kind, data });
}

function direct(message) {
  record('普通通知请求', message);
  const notice = new Notification(message.title, message.options);
  latest = notice;
  notice.addEventListener('click', () => clicked('普通通知', notice.data));
  const receipt = { title: notice.title, tag: notice.tag, messageId: message.options.data.messageId };
  notice.addEventListener('close', () => record('普通通知关闭', receipt));
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => reject(new Error('普通通知 10 秒内未收到 show/error')), TIMEOUT_MS);
    notice.addEventListener('show', () => {
      clearTimeout(timer);
      record('普通通知 API 已接收', receipt);
      resolve(notice);
    }, { once: true });
    notice.addEventListener('error', () => {
      clearTimeout(timer);
      reject(new Error(`普通通知发送失败；当前权限 ${Notification.permission}`));
    }, { once: true });
  });
}

async function pwa(message) {
  record('PWA 通知请求', message);
  await worker().showNotification(message.title, message.options);
  record('PWA API 已接收', { title: message.title, tag: message.options.tag, messageId: message.options.data.messageId });
}

function action(id, run) {
  $(id).addEventListener('click', async () => {
    $(id).disabled = true;
    try { await run(); }
    catch (error) { record('操作失败', { action: id, error: String(error) }, 'error'); }
    finally { $(id).disabled = false; refreshStatus(); }
  });
}

action('direct', () => direct(payload()));
action('pwa', () => pwa(payload()));
action('permission-request', async () => {
  if (typeof Notification !== 'function') throw new Error('当前环境没有 Notification API');
  const permission = await Promise.race([
    Notification.requestPermission(),
    new Promise((_, reject) => setTimeout(() => reject(new Error('权限请求 10 秒内没有返回；请检查浏览器或系统通知设置')), TIMEOUT_MS)),
  ]);
  record('权限结果', { permission });
});
action('replace', async () => {
  const message = payload();
  if (!message.options.tag) throw new Error('同 tag 替换需要填写 tag');
  const old = await direct({ ...message, title: `${message.title} · 旧` });
  await direct({
    ...message, title: `${message.title} · 新`,
    options: { ...message.options, data: { ...message.options.data, messageId: crypto.randomUUID() } },
  });
  old.close();
  record('旧对象已请求关闭', { expected: '只保留新通知；旧对象 close 不应移除新通知' });
});
action('close', () => {
  if (!latest) throw new Error('请先发送普通通知');
  latest.close();
  record('普通通知关闭请求', { tag: latest.tag });
});
action('list-pwa', async () => {
  const notices = await worker().getNotifications({ tag: $('tag').value });
  record('PWA 查询结果', { count: notices.length, titles: notices.map(n => n.title) });
});
action('close-pwa', async () => {
  const notices = await worker().getNotifications({ tag: $('tag').value });
  notices.forEach(notice => notice.close());
  record('PWA 关闭请求', { count: notices.length });
});
action('worker-send', async () => {
  const { title, options } = payload();
  const active = worker().active;
  record('Worker 内部通知请求', { title });
  const result = await new Promise((resolve, reject) => {
    const channel = new MessageChannel();
    const timer = setTimeout(() => { channel.port1.close(); reject(new Error('Worker 10 秒内未返回结果')); }, TIMEOUT_MS);
    channel.port1.onmessage = event => { clearTimeout(timer); channel.port1.close(); resolve(event.data); };
    active.postMessage({ type: 'LAB_WORKER_SEND', title, options }, [channel.port2]);
  });
  if (!result.ok) throw new Error(result.error);
  record('Worker API 已接收', { title });
});
action('schedule', () => {
  const seconds = Number($('delay').value);
  if (!Number.isInteger(seconds) || seconds < 1 || seconds > 60) throw new Error('延迟必须在 1–60 秒之间');
  const message = payload();
  const backgroundOnly = $('background-only').checked;
  clearTimeout(scheduled);
  $('cancel').disabled = false;
  $('timer').textContent = `${seconds} 秒后发送`;
  record('延迟已安排', { seconds, backgroundOnly });
  scheduled = setTimeout(async () => {
    scheduled = undefined;
    $('cancel').disabled = true;
    $('timer').textContent = '倒计时结束';
    try {
      if (backgroundOnly && document.visibilityState !== 'hidden') {
        record('网站规则跳过通知', { visibility: document.visibilityState, focused: document.hasFocus() });
        return;
      }
      await pwa(message);
    } catch (error) { record('延迟通知失败', { error: String(error) }, 'error'); }
  }, seconds * 1000);
});
$('cancel').addEventListener('click', () => {
  clearTimeout(scheduled);
  scheduled = undefined;
  $('cancel').disabled = true;
  $('timer').textContent = '已取消';
  record('延迟已取消');
});
action('seen', () => record('人工确认横幅可见', { source: '用户按钮确认' }));
action('clear', () => { events.length = 0; $('events').replaceChildren(); $('feedback').textContent = '已清空页面日志；服务端记录仍保留。'; });
action('export', () => {
  const url = URL.createObjectURL(new Blob([JSON.stringify({ session, origin: location.origin, events }, null, 2)], { type: 'application/json' }));
  const link = document.createElement('a');
  link.href = url;
  link.download = `pagedock-notifications-${session}.json`;
  link.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
});

window.addEventListener('NOTIFICATION_CLICK', event => clicked('PWA', event.detail));
navigator.serviceWorker?.addEventListener('message', event => {
  if (event.data?.type === 'LAB_NOTIFICATION_CLICK') clicked('Worker', event.data.data);
});
for (const type of ['focus', 'blur', 'visibilitychange']) {
  const target = type === 'visibilitychange' ? document : window;
  target.addEventListener(type, () => record('页面状态变化', { event: type, visibility: document.visibilityState, focused: document.hasFocus() }));
}
window.addEventListener('error', event => record('脚本错误', { error: event.message }, 'error'));
window.addEventListener('unhandledrejection', event => record('未处理的异步错误', { error: String(event.reason) }, 'error'));

refreshStatus();
try {
  const response = await fetch('/api/environment');
  if (!response.ok) throw new Error(`环境请求 HTTP ${response.status}`);
  const environment = await response.json();
  $('foreign').href = environment.foreignOrigin;
  record('页面已就绪', { secureContext: isSecureContext, origin: location.origin, permission: $('permission').textContent });
  if (!navigator.serviceWorker) throw new Error('当前环境不支持 Service Worker');
  await navigator.serviceWorker.register('/sw.js');
  registration = await Promise.race([
    navigator.serviceWorker.ready,
    new Promise((_, reject) => setTimeout(() => reject(new Error('Service Worker 10 秒内未激活')), TIMEOUT_MS)),
  ]);
  $('worker').textContent = '已激活';
  record('Service Worker 已就绪', { scope: registration.scope });
} catch (error) {
  $('worker').textContent = '初始化失败';
  record('初始化失败', { error: String(error) }, 'error');
}
