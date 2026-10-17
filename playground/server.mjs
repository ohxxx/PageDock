// [INPUT]: PORT/FOREIGN_PORT and local browser requests for static resources and diagnostic events.
// [OUTPUT]: Notification/media pages and bounded in-memory event records on two loopback origins.
// [POS]: Dependency-free local test server without a database, excluded from application packaging.
// [PROTOCOL]: Update playground/AGENTS.md, the root AGENTS.md, and validation records.
import http from 'node:http';
import { readFile } from 'node:fs/promises';
import { pathToFileURL } from 'node:url';

const HOST = '127.0.0.1';
const DEFAULT_PORT = 4173;
const MAX_EVENTS = 300;
const MAX_BODY_BYTES = 16384;
const assets = new Map([
  ['/', ['index.html', 'text/html; charset=utf-8']],
  ['/app.js', ['app.js', 'text/javascript; charset=utf-8']],
  ['/style.css', ['style.css', 'text/css; charset=utf-8']],
  ['/sw.js', ['sw.js', 'text/javascript; charset=utf-8']],
  ['/media.html', ['media.html', 'text/html; charset=utf-8']],
  ['/media.js', ['media.js', 'text/javascript; charset=utf-8']],
  ['/icon.png', ['../internal/brand/icon.png', 'image/png']],
]);

function port(value, fallback) {
  const number = value === undefined ? fallback : Number(value);
  if (!Number.isInteger(number) || number < 0 || number > 65535 || value === '') {
    throw new Error('端口必须是 0–65535 的整数');
  }
  return number;
}

function json(response, status, value) {
  response.writeHead(status, { 'Content-Type': 'application/json; charset=utf-8' });
  response.end(JSON.stringify(value));
}

export async function startLab({ primaryPort = 0, foreignPort = 0 } = {}) {
  const events = [];
  let origins;
  async function handle(request, response) {
    response.setHeader('Cache-Control', 'no-store');
    response.setHeader('X-Content-Type-Options', 'nosniff');
    const ownOrigin = `http://${HOST}:${request.socket.localPort}`;
    if (request.headers.host !== new URL(ownOrigin).host) {
      json(response, 403, { error: '仅允许通过本机地址访问' });
      return;
    }
    const url = new URL(request.url, ownOrigin);
    if (request.method === 'GET' && url.pathname === '/api/environment') {
      json(response, 200, { origin: ownOrigin, foreignOrigin: origins.find(o => o !== ownOrigin) });
    } else if (request.method === 'GET' && url.pathname === '/api/events') {
      json(response, 200, events.filter(e => e.origin === ownOrigin));
    } else if (request.method === 'POST' && url.pathname === '/api/events') {
      if (request.headers.origin !== ownOrigin || request.headers['content-type'] !== 'application/json') {
        json(response, 403, { error: '诊断事件必须来自同源页面' });
        return;
      }
      let body = '';
      let size = 0;
      for await (const chunk of request) {
        size += chunk.length;
        if (size > MAX_BODY_BYTES) {
          json(response, 413, { error: '诊断事件过大' });
          return;
        }
        body += chunk;
      }
      let event;
      try { event = JSON.parse(body); } catch {
        json(response, 400, { error: '无效 JSON' });
        return;
      }
      if (!event || typeof event.type !== 'string' || typeof event.session !== 'string') {
        json(response, 400, { error: '事件缺少 type / session' });
        return;
      }
      events.push({ ...event, origin: ownOrigin, receivedAt: new Date().toISOString() });
      if (events.length > MAX_EVENTS) events.shift();
      response.writeHead(204).end();
    } else if (request.method === 'GET' && assets.has(url.pathname)) {
      const [file, type] = assets.get(url.pathname);
      const data = await readFile(new URL(file, import.meta.url));
      response.writeHead(200, { 'Content-Type': type });
      response.end(data);
    } else {
      json(response, 404, { error: '未找到资源' });
    }
  }
  const servers = [];
  try {
    for (const requestedPort of [primaryPort, foreignPort]) {
      const server = http.createServer((request, response) => {
        handle(request, response).catch(error => {
          console.error(error);
          if (!response.headersSent) json(response, 500, { error: '本地测试服务异常' });
          else response.destroy();
        });
      });
      await new Promise((resolve, reject) => {
        server.once('error', reject);
        server.listen(requestedPort, HOST, resolve);
      });
      servers.push(server);
    }
  } catch (error) {
    await Promise.all(servers.map(server => new Promise(resolve => server.close(resolve))));
    throw error;
  }
  origins = servers.map(server => `http://${HOST}:${server.address().port}`);
  return {
    url: origins[0], foreignURL: origins[1],
    close: () => Promise.all(servers.map(server => new Promise(resolve => server.close(resolve)))),
  };
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  const lab = await startLab({
    primaryPort: port(process.env.PORT, DEFAULT_PORT),
    foreignPort: port(process.env.FOREIGN_PORT, 0),
  });
  console.log(JSON.stringify({ url: lab.url, foreignURL: lab.foreignURL }));
  for (const signal of ['SIGINT', 'SIGTERM']) process.once(signal, async () => { await lab.close(); });
}
