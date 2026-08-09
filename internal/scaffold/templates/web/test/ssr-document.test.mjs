import assert from "node:assert/strict";
import test from "node:test";

test("SSR document has accessible landmarks and production assets", async () => {
  const { render } = await import("../dist/server/entry-server.js");
  const document = render({
    scriptSrc: "/assets/client.js",
    styleHrefs: ["/assets/client.css"],
  });

  assert.match(document, /<a class="skip-link" href="#main-content">/);
  assert.match(document, /<main class="shell" id="main-content">/);
  assert.match(document, /<nav class="nav" aria-label="Primary navigation">/);
  assert.match(document, /<h1 id="hero-heading">/);
  assert.match(document, /<link rel="stylesheet" href="\/assets\/client.css">/);
  assert.match(document, /<script type="module" src="\/assets\/client.js"><\/script>/);
});
