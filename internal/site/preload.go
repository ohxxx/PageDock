// [INPUT]: The current MyGo locale, saved origin, and explicit website notification setting.
// [OUTPUT]: A narrow bridge from website Notification and page-side showNotification to MyGo.
// [POS]: WebView adapter script; creates no app frontend and does not handle Service Worker Push.
// [PROTOCOL]: Update this header and internal/site/AGENTS.md; update the root AGENTS.md when directory boundaries change.
package site

import (
	"encoding/json"
	"fmt"

	l10n "github.com/ohxxx/PageDock/internal/locale"
)

func Preload(source string, enabled bool) string {
	encoded, _ := json.Marshal(source)
	denied, _ := json.Marshal(l10n.Text("notificationDisabled"))
	return fmt.Sprintf("(%s)(%s, %t, %s);", pwaNotificationPreloadScript, encoded, enabled, denied)
}

const pwaNotificationPreloadScript = `function (source, enabled, deniedMessage) {
  if (location.origin !== source || window !== window.top) return;
  const runtime = globalThis.mygo;
  if (!runtime || typeof runtime.call !== "function") return;
  const clickProtocol = "NOTIFICATION_CLICK";
  const permissionDenied = "pagedock:notifications-denied";
  let permission = enabled ? "granted" : "denied";
  const pending = new Map();
  const tags = new Map();
  const documentKey = Array.from(crypto.getRandomValues(new Uint32Array(4)), x => x.toString(16)).join("-");
  let sequence = 0;
  const maxPending = 256;

  function forget(key, replaced = false) {
    const entry = pending.get(key);
    if (replaced && entry && entry.target) entry.target._replaced = true;
    if (entry && tags.get(entry.tag) === key) tags.delete(entry.tag);
    pending.delete(key);
  }
  function emit(target, type) {
    const event = new Event(type);
    target.dispatchEvent(event);
    if (typeof target["on" + type] === "function") target["on" + type].call(target, event);
  }
  async function send(title, options, target, serviceWorker) {
    if (permission !== "granted") throw new DOMException(deniedMessage, "NotAllowedError");
    const key = documentKey + ":" + (++sequence);
    const tag = typeof options.tag === "string" ? options.tag : "";
    const entry = {target, tag, data: options.data, serviceWorker};
    pending.set(key, entry);
    try {
      const id = await runtime.call("PWA.Receive", {
        title: String(title), body: typeof options.body === "string" ? options.body : "",
        tag, key, silent: options.silent === true
      });
      if (tag) {
        const previous = tags.get(tag);
        if (previous) forget(previous, true);
        tags.set(tag, key);
      }
      while (pending.size > maxPending) forget(pending.keys().next().value);
      return {id, key};
    } catch (error) {
      forget(key);
      if (String(error).includes(permissionDenied)) permission = "denied";
      throw error;
    }
  }

  runtime.on("pagedock:notification-click", key => {
    const entry = pending.get(key);
    if (!entry) return;
    forget(key);
    if (entry.serviceWorker) {
      // Match the website's existing Service Worker notificationclick-to-page event protocol.
      window.dispatchEvent(new CustomEvent(clickProtocol, {detail: entry.data || {}}));
    } else {
      emit(entry.target, "click");
    }
  });

  class PageDockNotification extends EventTarget {
    static get permission() { return permission; }
    static async requestPermission(callback) {
      // Native settings authorize the bridge; MyGo Show still requests and enforces system permission.
      if (typeof callback === "function") callback(permission);
      return permission;
    }
    constructor(title, options = {}) {
      super();
      if (permission !== "granted") throw new DOMException(deniedMessage, "NotAllowedError");
      this.title = String(title);
      this.body = options.body || "";
      this.tag = options.tag || "";
      this.data = options.data;
      this.onclick = this.onshow = this.onerror = this.onclose = null;
      this._closed = false;
      this._delivery = send(title, options, this, false);
      this._delivery.then(() => { if (!this._closed) emit(this, "show"); }, () => emit(this, "error"));
    }
    close() {
      if (this._closed) return;
      this._closed = true;
      if (this._replaced) { emit(this, "close"); return; }
      this._delivery.then(({id, key}) => {
        forget(key);
        return runtime.call("PWA.Close", id);
      }).then(() => emit(this, "close"), () => {});
    }
  }
  globalThis.Notification = PageDockNotification;
  if (typeof globalThis.ServiceWorkerRegistration === "function") {
    ServiceWorkerRegistration.prototype.showNotification = function (title, options = {}) {
      return send(title, options, null, true).then(() => undefined);
    };
  }
}`
