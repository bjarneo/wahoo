import assert from "node:assert/strict";
import test from "node:test";
import { createSSRWorker, workerHost } from "../worker.mjs";

async function start(server) {
  await new Promise((resolve) => server.listen(0, workerHost, resolve));
  const address = server.address();
  return `http://${workerHost}:${address.port}`;
}

async function close(server) {
  await new Promise((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
}

test("SSR worker exposes readiness and renders only its private endpoint", async () => {
  const server = createSSRWorker(() => "<main>Rendered</main>");
  const origin = await start(server);

  try {
    const ready = await fetch(`${origin}/__wahoo_ready`);
    assert.equal(ready.status, 200);
    assert.equal(await ready.text(), "ok\n");

    const page = await fetch(`${origin}/__wahoo_ssr?url=%2F`);
    assert.equal(page.status, 200);
    assert.match(page.headers.get("content-type"), /^text\/html/);
    assert.equal(await page.text(), "<main>Rendered</main>");

    const missing = await fetch(`${origin}/not-public`);
    assert.equal(missing.status, 404);
  } finally {
    await close(server);
  }
});
