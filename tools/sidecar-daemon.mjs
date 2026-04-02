import fs from 'fs/promises';
import net from 'net';
import path from 'path';
import { handleSidecarRequest } from './telemetry-sidecar.mjs';

function parseSocketPath() {
  const idx = process.argv.indexOf('--socket');
  if (idx >= 0 && process.argv[idx + 1]) {
    return process.argv[idx + 1];
  }
  return process.env.SUB2API_SIDECAR_DAEMON_SOCKET || '/tmp/sub2api-node-sidecar.sock';
}

async function readSocketJSON(socket) {
  const chunks = [];
  for await (const chunk of socket) {
    chunks.push(chunk);
  }
  return JSON.parse(Buffer.concat(chunks).toString('utf8'));
}

async function main() {
  const socketPath = parseSocketPath();
  await fs.mkdir(path.dirname(socketPath), { recursive: true });
  await fs.rm(socketPath, { force: true });

  const server = net.createServer(async (socket) => {
    try {
      const input = await readSocketJSON(socket);
      await handleSidecarRequest(input, socket);
    } catch (error) {
      socket.write(JSON.stringify({
        status: 0,
        error: error instanceof Error ? error.message : String(error),
      }));
    } finally {
      socket.end();
    }
  });

  server.on('error', (err) => {
    console.error(String(err));
    process.exitCode = 1;
  });

  process.on('SIGINT', async () => {
    server.close();
    await fs.rm(socketPath, { force: true }).catch(() => {});
    process.exit(0);
  });
  process.on('SIGTERM', async () => {
    server.close();
    await fs.rm(socketPath, { force: true }).catch(() => {});
    process.exit(0);
  });

  server.listen(socketPath);
}

main().catch((error) => {
  console.error(error instanceof Error ? error.stack ?? error.message : String(error));
  process.exitCode = 1;
});
