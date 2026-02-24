#!/usr/bin/env node
const { execSync } = require('child_process');
const { existsSync, mkdirSync, createWriteStream, chmodSync, renameSync } = require('fs');
const { join } = require('path');
const https = require('https');

const pkg = require('../package.json');
const name = Object.keys(pkg.bin)[0];
const version = pkg.version;

const platform = { darwin: 'darwin', linux: 'linux', win32: 'windows' }[process.platform];
const arch = { x64: 'amd64', arm64: 'arm64' }[process.arch];
if (!platform || !arch) { console.error(`Unsupported platform: ${process.platform}/${process.arch}`); process.exit(1); }

const ext = platform === 'windows' ? '.zip' : '.tar.gz';
const url = `https://github.com/stockyard-dev/stockyard/releases/download/v${version}/${name}_${platform}_${arch}${ext}`;
const binDir = join(__dirname);
const binPath = join(binDir, platform === 'windows' ? `${name}.exe` : name);

if (existsSync(binPath)) { process.exit(0); }

console.log(`Downloading ${name} v${version} for ${platform}/${arch}...`);

function download(url, dest) {
  return new Promise((resolve, reject) => {
    const follow = (url) => {
      https.get(url, (res) => {
        if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
          return follow(res.headers.location);
        }
        if (res.statusCode !== 200) return reject(new Error(`HTTP ${res.statusCode}`));
        const file = createWriteStream(dest);
        res.pipe(file);
        file.on('finish', () => { file.close(); resolve(); });
      }).on('error', reject);
    };
    follow(url);
  });
}

(async () => {
  const tmpFile = join(binDir, `tmp${ext}`);
  try {
    await download(url, tmpFile);
    if (ext === '.tar.gz') {
      execSync(`tar -xzf "${tmpFile}" -C "${binDir}" ${name}`, { stdio: 'pipe' });
    } else {
      execSync(`unzip -o "${tmpFile}" ${name}.exe -d "${binDir}"`, { stdio: 'pipe' });
    }
    try { chmodSync(binPath, 0o755); } catch {}
    try { require('fs').unlinkSync(tmpFile); } catch {}
    console.log(`✓ ${name} installed successfully`);
  } catch (err) {
    console.error(`Failed to install ${name}: ${err.message}`);
    console.error(`You can download manually from: ${url}`);
    process.exit(1);
  }
})();
