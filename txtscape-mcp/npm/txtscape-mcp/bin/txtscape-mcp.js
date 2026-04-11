#!/usr/bin/env node
'use strict';

const { spawn } = require('child_process');
const path = require('path');
const os = require('os');

const BINARIES = {
    'darwin-arm64': 'txtscape-mcp-darwin-arm64',
    'darwin-x64': 'txtscape-mcp-darwin-x64',
    'linux-x64': 'txtscape-mcp-linux-x64',
    'linux-arm64': 'txtscape-mcp-linux-arm64',
    'win32-x64': 'txtscape-mcp-win32-x64.exe',
};

const key = `${os.platform()}-${os.arch()}`;
const name = BINARIES[key];

if (!name) {
    process.stderr.write(`txtscape-mcp: unsupported platform: ${key}\n`);
    process.exit(1);
}

const binary = path.join(__dirname, name);

const child = spawn(binary, process.argv.slice(2), { stdio: 'inherit' });

child.on('exit', (code, signal) => {
    if (signal) {
        process.kill(process.pid, signal);
    } else {
        process.exit(code ?? 1);
    }
});
