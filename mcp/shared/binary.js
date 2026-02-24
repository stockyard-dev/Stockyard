#!/usr/bin/env node
/**
 * Stockyard MCP Server — Binary Manager
 * Downloads, verifies, and manages the Stockyard binary for the current platform.
 */
const { execSync, spawn } = require("child_process");
const { existsSync, mkdirSync, createWriteStream, chmodSync, unlinkSync, readFileSync, writeFileSync } = require("fs");
const { join, dirname } = require("path");
const https = require("https");
const os = require("os");

const PLATFORMS = { darwin: "darwin", linux: "linux", win32: "windows" };
const ARCHS = { x64: "amd64", arm64: "arm64" };
const STOCKYARD_DIR = join(os.homedir(), ".stockyard");
const BIN_DIR = join(STOCKYARD_DIR, "bin");

/**
 * Resolve binary name for a product.
 * @param {string} product - e.g. "costcap", "llmcache", "stockyard"
 * @returns {string} Full path to binary
 */
function binPath(product) {
  const ext = process.platform === "win32" ? ".exe" : "";
  return join(BIN_DIR, product + ext);
}

/**
 * Download a file following redirects.
 */
function download(url, dest) {
  return new Promise((resolve, reject) => {
    const follow = (url, redirects = 0) => {
      if (redirects > 5) return reject(new Error("Too many redirects"));
      https.get(url, (res) => {
        if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
          return follow(res.headers.location, redirects + 1);
        }
        if (res.statusCode !== 200) return reject(new Error(`HTTP ${res.statusCode} from ${url}`));
        const file = createWriteStream(dest);
        res.pipe(file);
        file.on("finish", () => { file.close(); resolve(); });
        file.on("error", reject);
      }).on("error", reject);
    };
    follow(url);
  });
}

/**
 * Ensure the Stockyard binary is available for the given product.
 * @param {string} product - Binary name (e.g. "costcap")
 * @param {string} version - Version string (e.g. "0.1.0")
 */
async function ensureBinary(product, version) {
  const bin = binPath(product);
  if (existsSync(bin)) return bin;

  const platform = PLATFORMS[process.platform];
  const arch = ARCHS[process.arch];
  if (!platform || !arch) {
    throw new Error(`Unsupported platform: ${process.platform}/${process.arch}`);
  }

  mkdirSync(BIN_DIR, { recursive: true });

  const ext = platform === "windows" ? ".zip" : ".tar.gz";
  const url = `https://github.com/stockyard-dev/stockyard/releases/download/v${version}/${product}_${platform}_${arch}${ext}`;
  const tmpFile = join(BIN_DIR, `tmp-${product}${ext}`);

  console.error(`[stockyard-mcp] Downloading ${product} v${version} for ${platform}/${arch}...`);

  try {
    await download(url, tmpFile);
    if (ext === ".tar.gz") {
      execSync(`tar -xzf "${tmpFile}" -C "${BIN_DIR}" ${product}`, { stdio: "pipe" });
    } else {
      execSync(`unzip -o "${tmpFile}" ${product}.exe -d "${BIN_DIR}"`, { stdio: "pipe" });
    }
    try { chmodSync(bin, 0o755); } catch {}
    try { unlinkSync(tmpFile); } catch {}
    console.error(`[stockyard-mcp] ✓ ${product} installed at ${bin}`);
  } catch (err) {
    console.error(`[stockyard-mcp] ✗ Failed to download ${product}: ${err.message}`);
    console.error(`[stockyard-mcp]   Manual download: ${url}`);
    throw err;
  }

  return bin;
}

/**
 * Write a YAML config file for a product.
 * @param {string} product - Product name
 * @param {object} config - Config object to serialize as YAML
 * @returns {string} Path to config file
 */
function writeConfig(product, config) {
  const configDir = join(STOCKYARD_DIR, "mcp");
  mkdirSync(configDir, { recursive: true });
  const configPath = join(configDir, `${product}.yaml`);
  
  // Simple YAML serializer (no dep needed for flat configs)
  const yaml = serializeYaml(config, 0);
  writeFileSync(configPath, yaml, "utf-8");
  console.error(`[stockyard-mcp] Config written to ${configPath}`);
  return configPath;
}

function serializeYaml(obj, indent) {
  const pad = "  ".repeat(indent);
  let out = "";
  for (const [k, v] of Object.entries(obj)) {
    if (v === null || v === undefined) continue;
    if (typeof v === "object" && !Array.isArray(v)) {
      out += `${pad}${k}:\n${serializeYaml(v, indent + 1)}`;
    } else if (Array.isArray(v)) {
      out += `${pad}${k}:\n`;
      for (const item of v) {
        if (typeof item === "object") {
          out += `${pad}  -\n${serializeYaml(item, indent + 2)}`;
        } else {
          out += `${pad}  - ${item}\n`;
        }
      }
    } else if (typeof v === "string" && v.includes("${")) {
      // Env var interpolation — don't quote
      out += `${pad}${k}: ${v}\n`;
    } else if (typeof v === "string") {
      out += `${pad}${k}: "${v}"\n`;
    } else {
      out += `${pad}${k}: ${v}\n`;
    }
  }
  return out;
}

/**
 * Start the Stockyard proxy as a background process.
 * @param {string} product - Binary name
 * @param {string} configPath - Path to YAML config
 * @returns {{ process: ChildProcess, port: number }}
 */
function startProxy(product, configPath) {
  const bin = binPath(product);
  if (!existsSync(bin)) {
    throw new Error(`Binary not found: ${bin}. Run ensureBinary() first.`);
  }

  const proc = spawn(bin, ["--config", configPath], {
    stdio: ["ignore", "pipe", "pipe"],
    detached: false,
  });

  proc.stdout.on("data", (d) => console.error(`[${product}] ${d.toString().trim()}`));
  proc.stderr.on("data", (d) => console.error(`[${product}] ${d.toString().trim()}`));

  return proc;
}

/**
 * Check if a proxy is running by hitting its health endpoint.
 * @param {number} port
 * @returns {Promise<boolean>}
 */
function checkHealth(port) {
  return new Promise((resolve) => {
    const req = require("http").get(`http://127.0.0.1:${port}/health`, (res) => {
      resolve(res.statusCode === 200);
    });
    req.on("error", () => resolve(false));
    req.setTimeout(2000, () => { req.destroy(); resolve(false); });
  });
}

/**
 * Call the Stockyard management API.
 * @param {number} port
 * @param {string} path - e.g. "/api/spend"
 * @returns {Promise<object>}
 */
function apiCall(port, path) {
  return new Promise((resolve, reject) => {
    require("http").get(`http://127.0.0.1:${port}${path}`, (res) => {
      let data = "";
      res.on("data", (chunk) => data += chunk);
      res.on("end", () => {
        try { resolve(JSON.parse(data)); }
        catch { resolve({ raw: data }); }
      });
    }).on("error", reject);
  });
}

module.exports = { ensureBinary, binPath, writeConfig, startProxy, checkHealth, apiCall, STOCKYARD_DIR, BIN_DIR };
