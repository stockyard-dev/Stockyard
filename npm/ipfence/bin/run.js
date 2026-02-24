#!/usr/bin/env node
const { execFileSync } = require('child_process');
const { join } = require('path');

const name = 'ipfence';
const ext = process.platform === 'win32' ? '.exe' : '';
const bin = join(__dirname, name + ext);

try {
  execFileSync(bin, process.argv.slice(2), { stdio: 'inherit' });
} catch (err) {
  if (err.status !== null) process.exit(err.status);
  console.error(`Failed to run ${name}: ${err.message}`);
  process.exit(1);
}
