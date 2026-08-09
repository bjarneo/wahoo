import http from "node:http";
import { createServer } from "vite";
import { handleSSRRequest, workerHost, workerPort } from "./worker.mjs";

const vite = await createServer({ server: { middlewareMode: true }, appType: "custom" });
const configuredOrigin = new URL(vite.config.server.origin ?? "");

if (
  configuredOrigin.protocol !== "http:"
  || configuredOrigin.hostname !== workerHost
  || configuredOrigin.port !== String(workerPort)
  || configuredOrigin.username
  || configuredOrigin.password
  || configuredOrigin.pathname !== "/"
  || configuredOrigin.search
  || configuredOrigin.hash
) {
  throw new Error(`Vite server origin must be http://${workerHost}:${workerPort}`);
}

const devOrigin = configuredOrigin.origin;

vite.middlewares.use(async (req, res, next) => {
  const handled = await handleSSRRequest(
    req,
    res,
    async () => {
      const mod = await vite.ssrLoadModule("/src/entry-server.tsx");
      return mod.render({
        viteClientSrc: `${devOrigin}/@vite/client`,
        scriptSrc: `${devOrigin}/src/entry-client.tsx`,
      });
    },
    (error) => {
      vite.ssrFixStacktrace(error);
    },
  );
  if (handled) {
    return;
  }
  next();
});

const server = http.createServer(vite.middlewares);

function closeServer() {
  return new Promise((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
}

let stopping = false;

async function shutdown(signal) {
  if (stopping) {
    return;
  }
  stopping = true;
  console.log(`Wahoo development SSR worker received ${signal}; shutting down`);
  const forceExit = setTimeout(() => process.exit(1), 10_000);
  forceExit.unref();
  try {
    await Promise.all([vite.close(), closeServer()]);
    clearTimeout(forceExit);
    process.exit(0);
  } catch (error) {
    clearTimeout(forceExit);
    console.error("Wahoo development SSR worker shutdown failed", error);
    process.exit(1);
  }
}

process.once("SIGINT", () => void shutdown("SIGINT"));
process.once("SIGTERM", () => void shutdown("SIGTERM"));

await new Promise((resolve, reject) => {
  server.once("error", reject);
  server.listen(workerPort, workerHost, () => {
    server.off("error", reject);
    resolve();
  });
});
console.log(`Wahoo development SSR worker listening on ${devOrigin}`);
