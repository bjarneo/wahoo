# syntax=docker/dockerfile:1

FROM node:22-alpine AS web
WORKDIR /src/web

COPY web/package.json ./
RUN npm install

COPY web/ ./
RUN npm run build

FROM golang:1.25-alpine AS go
WORKDIR /src

COPY go.mod ./
RUN go mod download

COPY . ./
COPY --from=web /src/web/dist ./web/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/app .

FROM node:22-alpine AS runtime
RUN apk add --no-cache tini

WORKDIR /app/web
COPY web/package.json ./
RUN npm install --omit=dev

WORKDIR /app
COPY --from=web /src/web/dist ./web/dist
COPY --from=web /src/web/ssr-server.mjs ./web/ssr-server.mjs
COPY --from=go /out/app ./app

ENV PORT=:8080
ENV WAHOO_SSR_URL=http://127.0.0.1:4173/__wahoo_ssr

EXPOSE 8080

CMD ["/sbin/tini", "--", "sh", "-c", "node web/ssr-server.mjs & exec ./app"]
