import http from "node:http";
import { readFileSync } from "node:fs";
import { render } from "./dist/server/entry-server.js";

const manifest = JSON.parse(readFileSync(new URL("./dist/.vite/manifest.json", import.meta.url), "utf8"));
const client = manifest["index.html"];
if (!client) {
  throw new Error("Vite client entry is missing from the manifest");
}

http.createServer((req, res) => {
  if (!req.url?.startsWith("/__wahoo_ssr")) {
    res.statusCode = 404;
    res.end();
    return;
  }
  res.setHeader("content-type", "text/html; charset=utf-8");
  res.end(render({
    scriptSrc: `/${client.file}`,
    styleHrefs: (client.css ?? []).map((file) => `/${file}`),
  }));
}).listen(4173, () => console.log("Wahoo SSR worker listening on :4173"));
