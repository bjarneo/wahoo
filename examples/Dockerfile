# syntax=docker/dockerfile:1

FROM node:22-alpine@sha256:c610fcdfb1d5b4740dd70c284ed3cb16bb857e0f7166196e36a5501df7a3aa32 AS web
WORKDIR /src/web

COPY web/package.json web/package-lock.json ./
RUN npm ci

COPY web/ ./
RUN npm run build

FROM golang:1.25-alpine@sha256:56961d79ea8129efddcc0b8643fd8a5416b4e6228cfd477e3fd61deb2672c587 AS go
WORKDIR /src

COPY go.mod ./
RUN go mod download

COPY . ./
COPY --from=web /src/web/dist ./web/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/app .

FROM node:22-alpine@sha256:c610fcdfb1d5b4740dd70c284ed3cb16bb857e0f7166196e36a5501df7a3aa32 AS runtime
RUN apk add --no-cache tini && addgroup -S wahoo && adduser -S -G wahoo wahoo

WORKDIR /app/web
COPY web/package.json web/package-lock.json ./
RUN npm ci --omit=dev

WORKDIR /app
COPY --chown=wahoo:wahoo --from=web /src/web/dist ./web/dist
COPY --chown=wahoo:wahoo --from=web /src/web/ssr-server.mjs ./web/ssr-server.mjs
COPY --chown=wahoo:wahoo --from=web /src/web/supervisor.mjs ./web/supervisor.mjs
COPY --chown=wahoo:wahoo --from=web /src/web/worker.mjs ./web/worker.mjs
COPY --chown=wahoo:wahoo --from=go /out/app ./app

ENV NODE_ENV=production
ENV PORT=:8080
ENV WAHOO_SSR_URL=http://127.0.0.1:4173/__wahoo_ssr

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 CMD wget -q -O /dev/null http://127.0.0.1:8080/healthz && wget -q -O /dev/null http://127.0.0.1:4173/__wahoo_ready || exit 1

USER wahoo

CMD ["/sbin/tini", "--", "node", "web/supervisor.mjs", "./app"]
