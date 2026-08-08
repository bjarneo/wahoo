# Wahoo

> **Work in progress:** Do not use Wahoo yet. It is not ready for production. APIs, templates, and release behavior can change.

Wahoo is a package monorepo for Go, React, and Tailwind SaaS applications. Go owns the public server. React renders pages through an SSR worker.

## Quick Start

```bash
go run ./cmd/wahoo new --local ./acme
cd acme
npm ci --prefix web
npm run dev --prefix web
```

Answer the module prompts to add only the parts the application needs. A basic project has the Go server, React pages, Tailwind styles, and SSR worker. It does not add auth, SSE, or WebSocket routes.

Use `--module example.com/acme` when the project will be published outside this repository. `--local` adds a local Go module replacement for framework development.

## Features

- HTTP lifecycle, request IDs, timeouts, panic recovery, and graceful shutdown.
- React SSR with Vite development and production asset manifests.
- Tailwind CSS and React hydration.
- SSE hub and WebSocket upgrade helper.
- Password hash and opaque token primitives.
- Prompted, opt-in auth, SSE, and WebSocket modules.
- Add modules later with `wahoo add`.

## Install

Install a pinned release binary on Linux or macOS:

```bash
curl --fail --location --silent --show-error https://raw.githubusercontent.com/bjarneo/wahoo/v0.3.0/install.sh --output /tmp/wahoo-install.sh
WAHOO_VERSION=v0.3.0 sh /tmp/wahoo-install.sh
```

The script installs `wahoo` to `~/.local/bin` and verifies the release checksum. Set `WAHOO_INSTALL_DIR` to use a different directory.

Install from a local checkout:

```bash
make install
```

Run `wahoo new --module example.com/acme ./acme` to create an application. Use `wahoo add ./acme auth sse` to add modules later.

## Documentation

Read the [Wahoo documentation](http://whiterose.org.contextowl.co/docs/wahoo).
