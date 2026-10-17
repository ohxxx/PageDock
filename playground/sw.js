// [INPUT]: Page diagnostic messages/system clicks; [OUTPUT]: Worker-native notification results and click data.
// [POS]: No caching or remote push; test separately from the page bridge; update playground/AGENTS.md.
self.addEventListener('install', () => self.skipWaiting());
self.addEventListener('activate', event => event.waitUntil(self.clients.claim()));
self.addEventListener('message', event => {
  if (event.data?.type !== 'LAB_WORKER_SEND') return;
  event.waitUntil((async () => {
    try {
      await self.registration.showNotification(event.data.title, event.data.options);
      event.ports[0].postMessage({ ok: true });
    } catch (error) {
      event.ports[0].postMessage({ ok: false, error: String(error) });
    }
  })());
});
self.addEventListener('notificationclick', event => {
  event.notification.close();
  event.waitUntil((async () => {
    const clients = await self.clients.matchAll({ type: 'window' });
    const page = clients[0];
    if (page) {
      await page.focus();
      page.postMessage({ type: 'LAB_NOTIFICATION_CLICK', data: event.notification.data });
    }
  })());
});
