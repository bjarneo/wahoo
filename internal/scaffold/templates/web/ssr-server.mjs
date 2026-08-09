import { readFileSync } from "node:fs";
import { render } from "./dist/server/entry-server.js";
import { createSSRWorker, installWorkerShutdown, workerHost, workerPort } from "./worker.mjs";

const manifest = JSON.parse(readFileSync(new URL("./dist/.vite/manifest.json", import.meta.url), "utf8"));
const client = manifest["index.html"];
if (!client) {
  throw new Error("Vite client entry is missing from the manifest");
}

const server = createSSRWorker(() => render({
  scriptSrc: `/${client.file}`,
  styleHrefs: (client.css ?? []).map((file) => `/${file}`),
}));

installWorkerShutdown(server);
server.once("error", (error) => {
  console.error("Wahoo SSR worker failed", error);
  process.exit(1);
});
server.listen(workerPort, workerHost, () => {
  console.log(`Wahoo SSR worker listening on http://${workerHost}:${workerPort}`);
});
