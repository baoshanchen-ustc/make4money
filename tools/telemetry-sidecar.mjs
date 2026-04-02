import { createRequire } from 'module';
import { fileURLToPath, pathToFileURL, URL as NodeURL } from 'url';
import fs from 'fs/promises';
import path from 'path';
import net from 'net';
import tls from 'tls';
import dns from 'dns/promises';
import { once } from 'events';
import http from 'http';
import https from 'https';

async function loadUndici() {
  const scriptDir = path.dirname(fileURLToPath(import.meta.url));
  const candidates = [
    path.resolve(scriptDir, '../../restored-src/node_modules/undici/index.js'),
    path.resolve(scriptDir, '../frontend/node_modules/undici/index.js'),
  ];

  for (const candidate of candidates) {
    try {
      await fs.access(candidate);
      const mod = await import(pathToFileURL(candidate).href);
      return mod.default ?? mod;
    } catch {
      // try next
    }
  }

  return null;
}

async function loadAxios() {
  const scriptDir = path.dirname(fileURLToPath(import.meta.url));
  const candidates = [
    path.resolve(scriptDir, '../../restored-src/node_modules/axios/index.js'),
    path.resolve(scriptDir, '../frontend/node_modules/axios/index.js'),
  ];

  for (const candidate of candidates) {
    try {
      await fs.access(candidate);
      const requireFromCandidate = createRequire(pathToFileURL(candidate));
      const mod = requireFromCandidate(candidate);
      return mod.default ?? mod;
    } catch {
      // try next
    }
  }

  return null;
}

async function readStdin() {
  const chunks = [];
  for await (const chunk of process.stdin) {
    chunks.push(chunk);
  }
  return Buffer.concat(chunks).toString('utf8');
}

function sanitizeHeaders(headers = {}) {
  const out = {};
  for (const [key, value] of Object.entries(headers)) {
    if (typeof value === 'string') {
      if (value !== '') out[key] = value;
      continue;
    }
    if (Array.isArray(value)) {
      const arr = value.filter((v) => typeof v === 'string' && v !== '');
      if (arr.length > 0) out[key] = arr;
    }
  }
  return out;
}

function normalizeResponseHeaders(headers = {}) {
  const out = {};
  for (const [key, value] of Object.entries(headers)) {
    if (Array.isArray(value)) {
      out[key] = value.map((v) => String(v));
      continue;
    }
    if (value == null) continue;
    out[key] = [String(value)];
  }
  return out;
}

function parseProxyURL(proxyUrl) {
  if (typeof proxyUrl !== 'string' || proxyUrl.trim() === '') return undefined;
  let parsed;
  try {
    parsed = new URL(proxyUrl);
  } catch {
    throw new Error(`invalid proxy_url: ${proxyUrl}`);
  }
  const protocol = parsed.protocol.replace(':', '').toLowerCase();
  const supported = new Set(['http', 'https', 'socks', 'socks5', 'socks5h']);
  if (!supported.has(protocol)) {
    throw new Error(`unsupported proxy scheme for sidecar: ${protocol}`);
  }
  const port = parsed.port
    ? Number(parsed.port)
    : (protocol === 'https' ? 443 : (protocol.startsWith('socks') ? 1080 : 80));
  if (!Number.isFinite(port) || port <= 0 || port > 65535) {
    throw new Error(`invalid proxy port in proxy_url: ${proxyUrl}`);
  }

  const proxy = {
    protocol,
    host: parsed.hostname,
    port,
  };
  if (parsed.username || parsed.password) {
    proxy.auth = {
      username: decodeURIComponent(parsed.username || ''),
      password: decodeURIComponent(parsed.password || ''),
    };
  }
  return proxy;
}

async function readExact(socket, n) {
  let out = Buffer.alloc(0);
  while (out.length < n) {
    const chunk = socket.read(n - out.length);
    if (chunk) {
      out = Buffer.concat([out, chunk]);
      continue;
    }
    await once(socket, 'readable');
  }
  return out;
}

function connectTCP(host, port, timeoutMs) {
  return new Promise((resolve, reject) => {
    const socket = net.connect({ host, port });
    const onError = (err) => reject(err);
    socket.once('error', onError);
    socket.setTimeout(timeoutMs, () => {
      socket.destroy(new Error('proxy connection timeout'));
    });
    socket.once('connect', () => {
      socket.off('error', onError);
      socket.setTimeout(0);
      resolve(socket);
    });
  });
}

function readUntilHeaderEnd(socket, timeoutMs) {
  return new Promise((resolve, reject) => {
    let buf = Buffer.alloc(0);
    const timer = setTimeout(() => {
      cleanup();
      reject(new Error('proxy CONNECT response timeout'));
    }, timeoutMs);
    const cleanup = () => {
      clearTimeout(timer);
      socket.off('data', onData);
      socket.off('error', onErr);
      socket.off('end', onEnd);
    };
    const onErr = (err) => {
      cleanup();
      reject(err);
    };
    const onEnd = () => {
      cleanup();
      reject(new Error('proxy closed connection before CONNECT response'));
    };
    const onData = (chunk) => {
      buf = Buffer.concat([buf, chunk]);
      const idx = buf.indexOf('\r\n\r\n');
      if (idx >= 0) {
        cleanup();
        const header = buf.slice(0, idx + 4);
        const rest = buf.slice(idx + 4);
        if (rest.length > 0) socket.unshift(rest);
        resolve(header.toString('latin1'));
      }
    };
    socket.on('data', onData);
    socket.once('error', onErr);
    socket.once('end', onEnd);
  });
}

async function dialViaHttpProxy(proxy, targetHost, targetPort, timeoutMs) {
  let socket = await connectTCP(proxy.host, proxy.port, timeoutMs);
  try {
    if (proxy.protocol === 'https') {
      socket = await new Promise((resolve, reject) => {
        const tlsSock = tls.connect({
          socket,
          servername: proxy.host,
          ALPNProtocols: ['http/1.1'],
        });
        tlsSock.once('secureConnect', () => resolve(tlsSock));
        tlsSock.once('error', reject);
      });
    }

    const lines = [
      `CONNECT ${targetHost}:${targetPort} HTTP/1.1`,
      `Host: ${targetHost}:${targetPort}`,
      'Proxy-Connection: Keep-Alive',
    ];
    if (proxy.auth?.username || proxy.auth?.password) {
      const authRaw = `${proxy.auth.username ?? ''}:${proxy.auth.password ?? ''}`;
      const auth = Buffer.from(authRaw, 'utf8').toString('base64');
      lines.push(`Proxy-Authorization: Basic ${auth}`);
    }
    lines.push('', '');
    socket.write(lines.join('\r\n'));

    const head = await readUntilHeaderEnd(socket, timeoutMs);
    const firstLine = head.split('\r\n', 1)[0] || '';
    const m = firstLine.match(/^HTTP\/\d+\.\d+\s+(\d{3})/i);
    const status = m ? Number(m[1]) : 0;
    if (status !== 200) {
      throw new Error(`HTTP proxy CONNECT failed: ${firstLine || 'invalid response'}`);
    }
    return socket;
  } catch (err) {
    socket.destroy();
    throw err;
  }
}

function encodeIPv4(ip) {
  return Buffer.from(ip.split('.').map((x) => Number(x)));
}

function encodeIPv6(ip) {
  const parts = ip.split('::');
  let left = parts[0] ? parts[0].split(':').filter(Boolean) : [];
  let right = parts[1] ? parts[1].split(':').filter(Boolean) : [];
  if (parts.length === 1) {
    right = [];
  }
  const missing = 8 - (left.length + right.length);
  const full = [...left, ...Array(Math.max(missing, 0)).fill('0'), ...right];
  if (full.length !== 8) throw new Error(`invalid IPv6 address: ${ip}`);
  const buf = Buffer.alloc(16);
  for (let i = 0; i < 8; i += 1) {
    const v = parseInt(full[i], 16);
    if (!Number.isFinite(v) || v < 0 || v > 0xffff) throw new Error(`invalid IPv6 segment: ${full[i]}`);
    buf.writeUInt16BE(v, i * 2);
  }
  return buf;
}

async function resolveTargetAddress(host, remoteDNS) {
  if (remoteDNS) {
    return { atyp: 0x03, addrBuf: Buffer.concat([Buffer.from([Buffer.byteLength(host)]), Buffer.from(host, 'utf8')]) };
  }
  const ipKind = net.isIP(host);
  if (ipKind === 4) return { atyp: 0x01, addrBuf: encodeIPv4(host) };
  if (ipKind === 6) return { atyp: 0x04, addrBuf: encodeIPv6(host) };
  const lookup = await dns.lookup(host);
  if (lookup.family === 4) return { atyp: 0x01, addrBuf: encodeIPv4(lookup.address) };
  return { atyp: 0x04, addrBuf: encodeIPv6(lookup.address) };
}

async function dialViaSocks5(proxy, targetHost, targetPort, timeoutMs) {
  const socket = await connectTCP(proxy.host, proxy.port, timeoutMs);
  try {
    const hasAuth = Boolean(proxy.auth?.username || proxy.auth?.password);
    const methods = hasAuth ? [0x00, 0x02] : [0x00];
    socket.write(Buffer.from([0x05, methods.length, ...methods]));

    const greeting = await readExact(socket, 2);
    if (greeting[0] !== 0x05 || greeting[1] === 0xff) {
      throw new Error('SOCKS5 method negotiation failed');
    }

    if (greeting[1] === 0x02) {
      const user = Buffer.from(proxy.auth?.username ?? '', 'utf8');
      const pass = Buffer.from(proxy.auth?.password ?? '', 'utf8');
      if (user.length > 255 || pass.length > 255) throw new Error('SOCKS5 auth credentials too long');
      socket.write(Buffer.concat([Buffer.from([0x01, user.length]), user, Buffer.from([pass.length]), pass]));
      const authResp = await readExact(socket, 2);
      if (authResp[1] !== 0x00) throw new Error('SOCKS5 authentication failed');
    }

    const remoteDNS = proxy.protocol !== 'socks5';
    const target = await resolveTargetAddress(targetHost, remoteDNS);
    const portBuf = Buffer.alloc(2);
    portBuf.writeUInt16BE(targetPort, 0);
    socket.write(Buffer.concat([Buffer.from([0x05, 0x01, 0x00, target.atyp]), target.addrBuf, portBuf]));

    const respHead = await readExact(socket, 4);
    if (respHead[0] !== 0x05 || respHead[1] !== 0x00) {
      throw new Error(`SOCKS5 CONNECT failed: code=${respHead[1]}`);
    }

    let toRead = 0;
    if (respHead[3] === 0x01) toRead = 4 + 2;
    else if (respHead[3] === 0x04) toRead = 16 + 2;
    else if (respHead[3] === 0x03) {
      const lenBuf = await readExact(socket, 1);
      toRead = lenBuf[0] + 2;
    } else {
      throw new Error('SOCKS5 invalid ATYP in response');
    }
    if (toRead > 0) await readExact(socket, toRead);

    return socket;
  } catch (err) {
    socket.destroy();
    throw err;
  }
}

function createSocksAgent(proxy, endpointProtocol, timeoutMs) {
  const isHTTPS = endpointProtocol === 'https:';
  const AgentClass = isHTTPS ? https.Agent : http.Agent;
  return new AgentClass({
    keepAlive: true,
    createConnection: (options, callback) => {
      (async () => {
        const targetHost = options.host;
        const targetPort = Number(options.port || (isHTTPS ? 443 : 80));
        const rawSocket = await dialViaSocks5(proxy, targetHost, targetPort, timeoutMs);
        if (!isHTTPS) {
          callback(null, rawSocket);
          return;
        }
        const tlsSocket = tls.connect({
          socket: rawSocket,
          servername: options.servername || targetHost,
          ALPNProtocols: ['http/1.1'],
        });
        tlsSocket.once('secureConnect', () => callback(null, tlsSocket));
        tlsSocket.once('error', (err) => callback(err));
      })().catch((err) => callback(err));
    },
  });
}

function createHttpConnectAgent(proxy, endpointProtocol, timeoutMs) {
  const isHTTPS = endpointProtocol === 'https:';
  const AgentClass = isHTTPS ? https.Agent : http.Agent;
  return new AgentClass({
    keepAlive: true,
    createConnection: (options, callback) => {
      (async () => {
        const targetHost = options.host;
        const targetPort = Number(options.port || (isHTTPS ? 443 : 80));
        const tunnel = await dialViaHttpProxy(proxy, targetHost, targetPort, timeoutMs);
        if (!isHTTPS) {
          callback(null, tunnel);
          return;
        }
        const tlsSocket = tls.connect({
          socket: tunnel,
          servername: options.servername || targetHost,
          ALPNProtocols: ['http/1.1'],
        });
        tlsSocket.once('secureConnect', () => callback(null, tlsSocket));
        tlsSocket.once('error', (err) => callback(err));
      })().catch((err) => callback(err));
    },
  });
}

function createSocksUndiciDispatcher(undici, proxy, timeoutMs) {
  return new undici.Agent({
    connect: (opts, callback) => {
      (async () => {
        const targetHost = opts.hostname || opts.host;
        const targetPort = Number(opts.port || (opts.protocol === 'https:' ? 443 : 80));
        const rawSocket = await dialViaSocks5(proxy, targetHost, targetPort, timeoutMs);
        if (opts.protocol !== 'https:') {
          callback(null, rawSocket);
          return rawSocket;
        }
        const tlsSocket = tls.connect({
          socket: rawSocket,
          servername: opts.servername || targetHost,
          ALPNProtocols: ['http/1.1'],
        });
        tlsSocket.once('secureConnect', () => callback(null, tlsSocket));
        tlsSocket.once('error', (err) => callback(err));
        return tlsSocket;
      })().catch((err) => callback(err));
    },
  });
}

function writeChunk(output, chunk) {
  return new Promise((resolve, reject) => {
    const ok = output.write(chunk, (err) => {
      if (err) reject(err);
    });
    if (ok) {
      resolve();
      return;
    }
    output.once('drain', resolve);
  });
}

function buildFetchOptions(undici, endpoint, proxyCfg, timeoutMs) {
  if (!proxyCfg) return {};
  if (proxyCfg.protocol === 'http' || proxyCfg.protocol === 'https') {
    const uri = new NodeURL(`${proxyCfg.protocol}://${proxyCfg.host}:${proxyCfg.port}`);
    if (proxyCfg.auth?.username || proxyCfg.auth?.password) {
      uri.username = proxyCfg.auth?.username ?? '';
      uri.password = proxyCfg.auth?.password ?? '';
    }
    return { dispatcher: new undici.ProxyAgent({ uri: uri.toString() }) };
  }
  return { dispatcher: createSocksUndiciDispatcher(undici, proxyCfg, timeoutMs) };
}

function createTimeoutController(timeoutMs) {
  const controller = new AbortController();
  let timer;
  if (timeoutMs > 0) {
    timer = setTimeout(() => controller.abort(new Error('request timeout')), timeoutMs);
  }
  return {
    signal: controller.signal,
    cancel(reason) {
      clearTimeout(timer);
      if (reason) {
        try {
          controller.abort(reason);
        } catch {
          controller.abort();
        }
      }
    },
    clear() {
      clearTimeout(timer);
    },
    refreshIdle() {
      if (!(timeoutMs > 0)) return;
      clearTimeout(timer);
      timer = setTimeout(() => controller.abort(new Error('stream idle timeout')), timeoutMs);
    },
  };
}

function buildAxiosTransportOptions(endpoint, proxyCfg, timeoutMs) {
  if (!proxyCfg) return {};
  const endpointProtocol = new URL(endpoint).protocol;
  if (proxyCfg.protocol === 'http' || proxyCfg.protocol === 'https') {
    return { proxy: proxyCfg };
  }
  const agent = createSocksAgent(proxyCfg, endpointProtocol, timeoutMs);
  if (endpointProtocol === 'https:') {
    return { proxy: false, httpsAgent: agent };
  }
  return { proxy: false, httpAgent: agent };
}

export async function handleSidecarRequest(input, output = process.stdout) {
  const clientMode = typeof input.client_mode === 'string' ? input.client_mode : 'messages';
  const headers = sanitizeHeaders(input.headers);
  const payload = Buffer.from(input.payload_base64 ?? '', 'base64');
  const method = typeof input.method === 'string' && input.method !== '' ? input.method : 'POST';
  const returnRawBytes = input.return_raw_bytes === true;
  const streamResponse = input.stream_response === true;
  const proxyCfg = parseProxyURL(input.proxy_url);
  const timeoutMs = input.timeout_ms ?? 10000;

  if (clientMode === 'telemetry') {
    const axios = await loadAxios();
    if (!axios) {
      throw new Error('axios runtime not found for telemetry sidecar');
    }
    const response = await axios.request({
      method,
      url: input.endpoint,
      data: payload,
      timeout: timeoutMs,
      headers,
      ...buildAxiosTransportOptions(input.endpoint, proxyCfg, timeoutMs),
      validateStatus: () => true,
    });
    const rawBody = returnRawBytes
      ? Buffer.from(response.data ?? [])
      : Buffer.from(
          typeof response.data === 'string'
            ? response.data
            : JSON.stringify(response.data ?? null),
          'utf8',
        );
    output.write(JSON.stringify({
      status: response.status,
      data: returnRawBytes ? undefined : response.data,
      headers: normalizeResponseHeaders(response.headers ?? {}),
      body_base64: rawBody.toString('base64'),
    }));
    return;
  }

  const undici = await loadUndici();
  if (!undici || typeof undici.fetch !== 'function') {
    throw new Error('undici runtime not found for telemetry sidecar');
  }
  const timeoutCtl = createTimeoutController(timeoutMs);
  const fetchOptions = buildFetchOptions(undici, input.endpoint, proxyCfg, timeoutMs);
  const response = await undici.fetch(input.endpoint, {
    method,
    headers,
    body: payload,
    duplex: 'half',
    signal: timeoutCtl.signal,
    ...fetchOptions,
  });

  if (streamResponse) {
    try {
      const meta = JSON.stringify({
        status: response.status || 0,
        headers: normalizeResponseHeaders(Object.fromEntries(response.headers.entries())),
      }) + '\n';
      await writeChunk(output, meta);
      timeoutCtl.refreshIdle();
      for await (const chunk of response.body ?? []) {
        timeoutCtl.refreshIdle();
        await writeChunk(output, Buffer.from(chunk));
      }
      timeoutCtl.clear();
      return;
    } catch (err) {
      timeoutCtl.cancel(err);
      throw err;
    }
  }

  const rawBody = Buffer.from(await response.arrayBuffer());
  timeoutCtl.clear();
  output.write(JSON.stringify({
    status: response.status,
    data: returnRawBytes ? undefined : rawBody.toString('utf8'),
    headers: normalizeResponseHeaders(Object.fromEntries(response.headers.entries())),
    body_base64: rawBody.toString('base64'),
  }));
}

async function main() {
  const raw = await readStdin();
  const input = JSON.parse(raw);
  await handleSidecarRequest(input, process.stdout);
}

const isMain = process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url);
if (isMain) {
  main().catch((error) => {
    process.stdout.write(JSON.stringify({
      status: 0,
      error: error instanceof Error ? error.message : String(error),
    }));
    process.exitCode = 1;
  });
}
