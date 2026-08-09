import http from "node:http";

export const workerHost = "127.0.0.1";
export const workerPort = 4173;

function requestPath(request) {
  return new URL(request.url ?? "/", "http://wahoo-worker.invalid").pathname;
}

function send(response, statusCode, body, contentType = "text/plain; charset=utf-8") {
  response.writeHead(statusCode, { "content-type": contentType });
  response.end(body);
}

export async function handleSSRRequest(request, response, render, onError = console.error) {
  if (request.method !== "GET") {
    return false;
  }

  const path = requestPath(request);
  if (path === "/__wahoo_ready") {
    send(response, 200, "ok\n");
    return true;
  }
  if (path !== "/__wahoo_ssr") {
    return false;
  }

  try {
    send(response, 200, await render(), "text/html; charset=utf-8");
  } catch (error) {
    onError(error);
    send(response, 500, "SSR rendering failed\n");
  }
  return true;
}

export function createSSRWorker(render) {
  return http.createServer(async (request, response) => {
    if (!(await handleSSRRequest(request, response, render))) {
      send(response, 404, "not found\n");
    }
  });
}

export function installWorkerShutdown(server) {
  let stopping = false;

  async function shutdown(signal) {
    if (stopping) {
      return;
    }
    stopping = true;
    console.log(`Wahoo SSR worker received ${signal}; shutting down`);

    const forceExit = setTimeout(() => {
      server.closeAllConnections();
      process.exit(1);
    }, 10_000);
    forceExit.unref();

    server.close((error) => {
      clearTimeout(forceExit);
      if (error) {
        console.error("Wahoo SSR worker shutdown failed", error);
        process.exit(1);
      }
      process.exit(0);
    });
  }

  process.once("SIGINT", () => void shutdown("SIGINT"));
  process.once("SIGTERM", () => void shutdown("SIGTERM"));
}
