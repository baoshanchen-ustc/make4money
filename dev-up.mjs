#!/usr/bin/env zx
// dev-up.mjs — start backend and frontend dev servers
// Usage: ./dev-up.mjs
// Run ./dev-setup.sh first if this is a fresh clone.

import { createServer } from "net";
import { execSync } from "child_process";

const root = path.dirname(new URL(import.meta.url).pathname);
const backend = path.join(root, "backend");
const frontend = path.join(root, "frontend");

$.verbose = false;

// ── helpers ───────────────────────────────────────────────────────────────────
const info = (s) => console.log(`${chalk.green("[dev-up]")} ${s}`);
const die  = (s) => { console.error(chalk.red(`[dev-up] ${s}`)); process.exit(1); };

function safeKill(proc, signal = "SIGTERM") {
  try { proc.kill(signal); } catch {}
}

function killPort(port) {
  try {
    const pids = execSync(`lsof -ti:${port}`, { encoding: "utf8" }).trim();
    if (pids) pids.split("\n").forEach(pid => {
      try { process.kill(Number(pid), "SIGKILL"); } catch {}
    });
  } catch {}
}

function isPortInUse(port) {
  return new Promise((resolve) => {
    const srv = createServer();
    srv.once("error", () => resolve(true));
    srv.once("listening", () => { srv.close(); resolve(false); });
    srv.listen(port, "0.0.0.0");
  });
}

async function waitForPort(port, timeoutMs) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    if (await isPortInUse(port)) return true;
    await sleep(1000);
  }
  return false;
}

// ── preflight ─────────────────────────────────────────────────────────────────
if (!fs.existsSync(path.join(backend, "config.yaml"))) {
  die("backend/config.yaml not found. Run ./dev-setup.sh first.");
}
if (await isPortInUse(8080)) die("Port 8080 already in use. Stop the existing process first.");
if (await isPortInUse(3000)) die("Port 3000 already in use. Stop the existing process first.");

// ── start backend ─────────────────────────────────────────────────────────────
info("Starting backend on :8080 ...");
const backendProc = $({ cwd: backend })`go run ./cmd/server`.nothrow();

const backendReady = await waitForPort(8080, 60_000);
if (!backendReady) {
  safeKill(backendProc, "SIGKILL");
  die("Backend did not start within 60s.");
}
info("Backend ready.");

// ── start frontend ────────────────────────────────────────────────────────────
info("Starting frontend on :3000 ...");
const frontendProc = $({ cwd: frontend })`pnpm dev`.nothrow();

const frontendReady = await waitForPort(3000, 30_000);
if (!frontendReady) {
  safeKill(frontendProc, "SIGKILL");
  die("Frontend did not start within 30s.");
}
info("Frontend ready.");

console.log();
console.log(`  Frontend: ${chalk.cyan("http://localhost:3000")}`);
console.log(`  Backend:  ${chalk.cyan("http://localhost:8080")}`);
console.log();
info("Press Ctrl+C to stop all servers.");

// ── shutdown ──────────────────────────────────────────────────────────────────
let shuttingDown = false;
async function shutdown() {
  if (shuttingDown) return;
  shuttingDown = true;
  console.log();
  info("Shutting down...");
  safeKill(backendProc, "SIGTERM");
  safeKill(frontendProc, "SIGTERM");
  await Promise.race([
    Promise.allSettled([backendProc, frontendProc]),
    sleep(3000),
  ]);
  safeKill(backendProc, "SIGKILL");
  safeKill(frontendProc, "SIGKILL");
  killPort(8080);
  killPort(3000);
  info("All servers stopped.");
  process.exit(0);
}

process.on("SIGINT", shutdown);
process.on("SIGTERM", shutdown);

await Promise.race([backendProc, frontendProc]);
await shutdown();
