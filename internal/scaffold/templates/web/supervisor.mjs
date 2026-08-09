import { spawn } from "node:child_process";

const [command, ...args] = process.argv.slice(2);

if (!command) {
  throw new Error("usage: node supervisor.mjs <go-app> [args...]");
}

const children = [
  { name: "SSR worker", process: spawn(process.execPath, ["web/ssr-server.mjs"], { stdio: "inherit" }) },
  { name: "Go app", process: spawn(command, args, { stdio: "inherit" }) },
];
const closed = children.map(({ process: child }) => new Promise((resolve) => child.once("close", resolve)));

let stopping = false;
let forceExit;

function isRunning(child) {
  return child.exitCode === null && child.signalCode === null;
}

function stop(exitCode, signal) {
  if (stopping) {
    return;
  }
  stopping = true;
  for (const child of children) {
    if (isRunning(child.process)) {
      child.process.kill(signal);
    }
  }
  forceExit = setTimeout(() => {
    for (const child of children) {
      if (isRunning(child.process)) {
        child.process.kill("SIGKILL");
      }
    }
    process.exit(1);
  }, 10_000);
  forceExit.unref();

  Promise.all(closed)
    .then(() => {
      clearTimeout(forceExit);
      process.exit(exitCode);
    });
}

for (const child of children) {
  child.process.once("error", (error) => {
    console.error(`Wahoo ${child.name} failed to start`, error);
    stop(1, "SIGTERM");
  });
  child.process.once("close", (code, signal) => {
    if (!stopping) {
      console.error(`Wahoo ${child.name} exited unexpectedly`, { code, signal });
      stop(code && code > 0 ? code : 1, "SIGTERM");
    }
  });
}

process.once("SIGINT", () => stop(0, "SIGTERM"));
process.once("SIGTERM", () => stop(0, "SIGTERM"));
