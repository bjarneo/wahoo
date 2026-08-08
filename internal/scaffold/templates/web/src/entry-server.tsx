import { renderToString } from "react-dom/server";
import App from "./App";
import "./styles.css";

type RenderOptions = {
  scriptSrc?: string;
  styleHrefs?: string[];
};

export function render(options: RenderOptions = {}) {
  const assets = options.scriptSrc
    ? `${(options.styleHrefs ?? []).map((href) => `<link rel="stylesheet" href="${href}"/>`).join("")}<script type="module" src="${options.scriptSrc}"></script>`
    : import.meta.env.DEV
    ? `<script type="module" src="/src/entry-client.tsx"></script>`
    : "";
  return `<!doctype html><html lang="en"><head><meta charset="UTF-8"/><meta name="viewport" content="width=device-width, initial-scale=1.0"/><title>__APP_NAME__</title></head><body><div id="root">${renderToString(<App />)}</div>${assets}</body></html>`;
}
