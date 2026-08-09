import { renderToString } from "react-dom/server";
import App from "./App";
import "./styles.css";

type RenderOptions = {
  scriptSrc?: string;
  styleHrefs?: string[];
  viteClientSrc?: string;
};

function escapeAttribute(value: string) {
  return value.replaceAll("&", "&amp;").replaceAll('"', "&quot;").replaceAll("<", "&lt;");
}

export function render(options: RenderOptions = {}) {
  const assets = [
    ...(options.styleHrefs ?? []).map((href) => `<link rel="stylesheet" href="${escapeAttribute(href)}">`),
    options.viteClientSrc ? `<script type="module" src="${escapeAttribute(options.viteClientSrc)}"></script>` : "",
    options.scriptSrc ? `<script type="module" src="${escapeAttribute(options.scriptSrc)}"></script>` : "",
  ].join("");
  return `<!doctype html><html lang="en"><head><meta charset="UTF-8"/><meta name="viewport" content="width=device-width, initial-scale=1.0"/><title>__APP_NAME__</title></head><body><div id="root">${renderToString(<App />)}</div>${assets}</body></html>`;
}
