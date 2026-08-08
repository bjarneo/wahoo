# Wahoo

Wahoo is a Go, React, and Tailwind foundation for SaaS applications. Go owns the public server. React renders pages through an SSR worker.

## Quick Start

```bash
go run ./cmd/wahoo new --local ./acme
cd acme
npm install --prefix web
npm run dev --prefix web
```

Open `http://localhost:8080`. The generator creates a Go server, React pages, Tailwind styles, an SSR worker, SSE, WebSocket, and auth route stubs.

Use `--module example.com/acme` when the project will be published outside this repository. `--local` adds a local Go module replacement for framework development.

## Features

- HTTP lifecycle, request IDs, timeouts, panic recovery, and graceful shutdown.
- React SSR with Vite development and production asset manifests.
- Tailwind CSS and React hydration.
- SSE hub and WebSocket upgrade helper.
- Password hash and opaque token primitives.
- One-command project generation.

## Install

Install the latest release binary on Linux or macOS:

```bash
curl --fail --location --silent --show-error https://raw.githubusercontent.com/bjarneo/wahoo/main/install.sh | sh
```

The script installs `wahoo` to `~/.local/bin`. Set `WAHOO_VERSION=v0.1.0` to install a release tag. Set `WAHOO_INSTALL_DIR` to use a different directory.

Install from a local checkout:

```bash
make install
```

Run `wahoo new --module example.com/acme ./acme` to create an application.

## Documentation

Read the [Wahoo documentation](https://contextowl.co/docs/cowl-wahoo).
