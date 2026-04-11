#!/usr/bin/env node
'use strict';

// Pre-publish smoke test for the assembled @txtscape/mcp npm package.
// Verifies that the JS shim can find and launch the platform binary,
// and that the binary responds to a valid MCP initialize request.

const { spawn } = require('child_process');
const path = require('path');
const os = require('os');

const shimPath = path.join(__dirname, '..', 'txtscape-mcp', 'bin', 'txtscape-mcp.js');

const key = `${os.platform()}-${os.arch()}`;
console.log(`platform: ${key}`);
console.log(`shim: ${shimPath}`);

const initRequest = JSON.stringify({
    jsonrpc: '2.0',
    id: 1,
    method: 'initialize',
    params: {}
});

const child = spawn('node', [shimPath], {
    stdio: ['pipe', 'pipe', 'pipe']
});

let stdout = '';
let stderr = '';

child.stdout.on('data', (data) => { stdout += data.toString(); });
child.stderr.on('data', (data) => { stderr += data.toString(); });

child.on('error', (err) => {
    console.error(`failed to start: ${err.message}`);
    process.exit(1);
});

child.stdin.write(initRequest + '\n');
child.stdin.end();

child.on('close', (code) => {
    if (stderr) {
        console.error(`stderr: ${stderr}`);
    }

    if (!stdout.trim()) {
        console.error('no output from binary');
        process.exit(1);
    }

    let response;
    try {
        response = JSON.parse(stdout.trim());
    } catch (e) {
        console.error(`invalid JSON response: ${stdout}`);
        process.exit(1);
    }

    // Verify it's a valid MCP initialize response
    const checks = [
        [response.jsonrpc === '2.0', 'jsonrpc should be 2.0'],
        [response.id === 1, 'id should be 1'],
        [response.result != null, 'result should exist'],
        [response.result?.serverInfo?.name === 'txtscape', 'serverInfo.name should be txtscape'],
        [typeof response.result?.serverInfo?.version === 'string', 'serverInfo.version should be a string'],
        [response.result?.protocolVersion != null, 'protocolVersion should exist'],
        [response.result?.capabilities?.tools != null, 'capabilities.tools should exist'],
        [typeof response.result?.instructions === 'string', 'instructions should be a string'],
    ];

    let failed = false;
    for (const [ok, msg] of checks) {
        if (ok) {
            console.log(`  ✓ ${msg}`);
        } else {
            console.error(`  ✗ ${msg}`);
            failed = true;
        }
    }

    if (failed) {
        console.error('\nprepublish test failed');
        console.error('response:', JSON.stringify(response, null, 2));
        process.exit(1);
    }

    console.log('\nprepublish test passed');
});
