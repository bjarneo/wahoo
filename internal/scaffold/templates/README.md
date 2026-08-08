# __APP_NAME__

Generated with Wahoo. Go owns HTTP, APIs, and graceful shutdown. React renders HTML for SSR and serves frontend modules in development.

## Start

```bash
npm ci --prefix web
npm run dev --prefix web
```

- Go app: `http://localhost:8080`
- SSR worker and Vite modules: `http://localhost:4173`
- Health: `http://localhost:8080/healthz`
## Modules

__MODULES__

Add a module later:

```bash
wahoo add . auth
```

The `auth` module adds route stubs only. Add user storage, session cookies, CSRF checks, and mail before you expose them. Read the Wahoo authentication guide at `http://whiterose.org.contextowl.co/docs/wahoo/authentication`.
