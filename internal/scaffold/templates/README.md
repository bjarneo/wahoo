# __APP_NAME__

Generated with Wahoo. Go owns HTTP, APIs, realtime, and graceful shutdown. React renders HTML for SSR and serves frontend modules in development.

## Start

```bash
npm install --prefix web
npm run dev --prefix web
```

- Go app: `http://localhost:8080`
- SSR worker and Vite modules: `http://localhost:4173`
- Health: `http://localhost:8080/healthz`
- SSE example: `http://localhost:8080/events`
- WebSocket example: `ws://localhost:8080/ws`

The auth routes in `app/routes.go` return `501 Not Implemented`. Add user storage, session cookies, CSRF checks, and mail before you expose them. Read the Wahoo authentication guide at `https://contextowl.co/docs/cowl-wahoo/authentication`.
