# __APP_NAME__

Generated with Wahoo. Go owns HTTP and APIs. React renders HTML in the private SSR worker and serves Vite modules in development.

## Start

```bash
npm ci --prefix web
npm run dev --prefix web
```

- Go app: `http://localhost:8080`
- SSR worker and Vite modules: `http://localhost:4173`
- Health: `http://localhost:8080/healthz`
- Readiness: `http://localhost:8080/readyz`

Run worker and SSR document checks after a production build:

```bash
npm test --prefix web
npm run typecheck --prefix web
npm run build --prefix web
npm run test:ssr --prefix web
```

## Configuration

| Variable | Default | Purpose |
| --- | --- | --- |
| `WAHOO_HTTP_ADDR` | `PORT` or `:8080` | Go HTTP listen address |
| `WAHOO_SSR_URL` | empty | Private SSR worker URL. Required when `NODE_ENV=production`. |
| `WAHOO_VITE_DEV_URL` | `http://127.0.0.1:4173` | Vite origin used only for development fallback. |
| `WAHOO_JSON_BODY_LIMIT` | `1048576` | Maximum JSON request size in bytes. Maximum is 8 MiB. |

`internal/config` enables a basic, application-owned response header policy. Review and extend it for your browser and deployment requirements.
## Modules

__MODULES__

Add a module later:

```bash
wahoo add . auth
```

Optional HTTP modules return `501 Not Implemented` until the application configures their provider, authorization, and limits. The `jobs` module creates `cmd/worker`; it exits until `app.ConfigureJobs` returns a puller and handler. The `openapi` module serves the application-owned static document at `/openapi.json` and uses `/api/v1` as its API server path.
