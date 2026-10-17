// [INPUT]: Real loopback HTTP; [OUTPUT]: Notification/media pages, logo, dual origins, diagnostic events, and error-path tests.
// [POS]: Native Node behavior tests without static contracts; update tests/AGENTS.md and playground/AGENTS.md.
import test from 'node:test';
import assert from 'node:assert/strict';
import http from 'node:http';
import { startLab } from '../../playground/server.mjs';

test('测试服务隔离来源、限制暴露的资源并记录真实事件', async t => {
  const lab = await startLab();
  t.after(() => lab.close());
  const environment = await (await fetch(`${lab.url}/api/environment`)).json();
  assert.equal(environment.foreignOrigin, lab.foreignURL);
  assert.notEqual(lab.url, lab.foreignURL);
  const page = await fetch(lab.url);
  assert.equal(page.status, 200);
  assert.match(page.headers.get('content-type'), /text\/html/);
  const script = await fetch(`${lab.url}/sw.js`);
  assert.equal(script.status, 200);
  assert.match(script.headers.get('content-type'), /javascript/);
  for (const origin of [lab.url, lab.foreignURL]) {
    for (const [path, type] of [['/media.html', 'text/html'], ['/media.js', 'text/javascript']]) {
      const resource = await fetch(origin + path);
      assert.equal(resource.status, 200);
      assert.equal(resource.headers.get('content-type'), `${type}; charset=utf-8`);
      assert.equal(resource.headers.get('cache-control'), 'no-store');
    }
    const icon = await fetch(`${origin}/icon.png`);
    assert.equal(icon.status, 200);
    assert.equal(icon.headers.get('content-type'), 'image/png');
    const png = Buffer.from(await icon.arrayBuffer());
    assert.equal(png.subarray(0, 8).toString('hex'), '89504e470d0a1a0a');
    assert.ok(png.readUInt32BE(16) > 0);
    assert.equal(png.readUInt32BE(16), png.readUInt32BE(20));
  }
  assert.equal((await fetch(`${lab.url}/../go.mod`)).status, 404);
  assert.equal((await fetch(`${lab.url}/server.mjs`)).status, 404);
  const spoofedHostStatus = await new Promise((resolve, reject) => {
    http.get(lab.url, { headers: { Host: 'other.invalid' } }, response => {
      response.resume();
      resolve(response.statusCode);
    }).on('error', reject);
  });
  assert.equal(spoofedHostStatus, 403);
  const post = (origin, body) => fetch(`${lab.url}/api/events`, {
    method: 'POST', headers: { Origin: origin, 'Content-Type': 'application/json' }, body,
  });
  assert.equal((await post(lab.foreignURL, '{}')).status, 403);
  assert.equal((await post(lab.url, '{')).status, 400);
  assert.equal((await post(lab.url, '{}')).status, 400);
  assert.equal((await post(lab.url, 'x'.repeat(17000))).status, 413);
  const event = { session: 'local-test', type: '点击回传', detail: { roomId: '房间-42' } };
  assert.equal((await post(lab.url, JSON.stringify(event))).status, 204);
  const events = await (await fetch(`${lab.url}/api/events`)).json();
  assert.equal(events.length, 1);
  assert.deepEqual(events[0].detail, event.detail);
  assert.equal(events[0].origin, lab.url);
  assert.deepEqual(await (await fetch(`${lab.foreignURL}/api/events`)).json(), []);
});
