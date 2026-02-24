/**
 * Stockyard OpenClaw Shared Core
 * 
 * Handles binary download, proxy lifecycle, and API communication
 * for all Stockyard OpenClaw skills.
 */

const { execSync, spawn } = require("child_process");
const { existsSync, mkdirSync, createWriteStream, chmodSync } = require("fs");
const { join } = require("path");
const https = require("https");
const http = require("http");

const VERSION = "0.1.0";
const DATA_DIR = join(process.env.HOME || "/tmp", ".stockyard");
const BIN_DIR = join(DATA_DIR, "bin");

/**
 * Ensure the Stockyard binary is downloaded for the current platform.
 */
async function ensureBinary(product = "stockyard") {
  mkdirSync(BIN_DIR, { recursive: true });

  const platform = { darwin: "darwin", linux: "linux", win32: "windows" }[process.platform];
  const arch = { x64: "amd64", arm64: "arm64" }[process.arch];
  const ext = platform === "windows" ? ".exe" : "";
  const binPath = join(BIN_DIR, product + ext);

  if (existsSync(binPath)) return binPath;

  const url = `https://github.com/stockyard-dev/stockyard/releases/download/v${VERSION}/${product}_${platform}_${arch}.tar.gz`;
  console.log(`[Stockyard] Downloading ${product} v${VERSION}...`);

  const tarPath = join(BIN_DIR, "tmp.tar.gz");
  await downloadFile(url, tarPath);
  execSync(`tar -xzf "${tarPath}" -C "${BIN_DIR}" ${product}${ext}`, { stdio: "pipe" });
  try { chmodSync(binPath, 0o755); } catch {}
  try { require("fs").unlinkSync(tarPath); } catch {}

  console.log(`[Stockyard] ✓ ${product} installed`);
  return binPath;
}

function downloadFile(url, dest) {
  return new Promise((resolve, reject) => {
    const follow = (u) => {
      https.get(u, (res) => {
        if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
          return follow(res.headers.location);
        }
        if (res.statusCode !== 200) return reject(new Error(`HTTP ${res.statusCode}`));
        const file = createWriteStream(dest);
        res.pipe(file);
        file.on("finish", () => { file.close(); resolve(); });
      }).on("error", reject);
    };
    follow(url);
  });
}

/**
 * Start a Stockyard proxy as a background process.
 */
function startProxy(binPath, configPath, port = 4000) {
  const proc = spawn(binPath, ["-config", configPath], {
    stdio: "pipe",
    detached: true,
    env: { ...process.env },
  });
  proc.unref();

  proc.stdout?.on("data", (d) => console.log(`[Stockyard] ${d.toString().trim()}`));
  proc.stderr?.on("data", (d) => console.error(`[Stockyard] ${d.toString().trim()}`));

  return proc;
}

/**
 * Make an HTTP request to the proxy API.
 */
function apiCall(port, path, method = "GET", body = null) {
  return new Promise((resolve, reject) => {
    const opts = {
      hostname: "127.0.0.1",
      port,
      path,
      method,
      headers: { "Content-Type": "application/json" },
    };
    const req = http.request(opts, (res) => {
      let data = "";
      res.on("data", (chunk) => (data += chunk));
      res.on("end", () => {
        try { resolve(JSON.parse(data)); }
        catch { resolve({ raw: data }); }
      });
    });
    req.on("error", reject);
    req.setTimeout(5000, () => { req.destroy(); reject(new Error("Timeout")); });
    if (body) req.write(JSON.stringify(body));
    req.end();
  });
}

/**
 * Check if proxy is healthy.
 */
async function checkHealth(port) {
  try {
    const result = await apiCall(port, "/health");
    return result?.status === "ok" || result?.raw?.includes("ok");
  } catch {
    return false;
  }
}

/**
 * Wait for proxy to become healthy (up to 10s).
 */
async function waitForProxy(port, maxWaitMs = 10000) {
  const start = Date.now();
  while (Date.now() - start < maxWaitMs) {
    if (await checkHealth(port)) return true;
    await new Promise((r) => setTimeout(r, 500));
  }
  return false;
}

module.exports = { ensureBinary, startProxy, apiCall, checkHealth, waitForProxy, VERSION, DATA_DIR, BIN_DIR };
