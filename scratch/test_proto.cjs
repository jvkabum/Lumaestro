const { spawn } = require('child_process');
const path = require('path');

const geminiScript = path.join(__dirname, 'node_modules', '@google', 'gemini-cli', 'bundle', 'gemini.js');
const API_KEY = "AIzaSyBkwoV7XgZBgssxvwJ4NNSvi7mGqOQOIrg"; // Uma das chaves do pool

console.log('=== TESTE DE PROTOCOLO v0.37 ===');

const child = spawn('node', ['--no-warnings=DEP0040', geminiScript, '--acp'], {
    cwd: __dirname,
    env: { ...process.env, GOOGLE_API_KEY: API_KEY },
    stdio: ['pipe', 'pipe', 'pipe']
});

let msgId = 0;
function send(method, params = {}) {
    msgId++;
    const msg = { jsonrpc: '2.0', id: msgId, method, params };
    console.log(`>> SEND: ${JSON.stringify(msg)}`);
    child.stdin.write(JSON.stringify(msg) + '\n');
}

child.stdout.on('data', (data) => {
    console.log(`<< RECV: ${data.toString().trim()}`);
    const lines = data.toString().split('\n');
    for (const line of lines) {
        if (!line.trim()) continue;
        try {
            const res = JSON.parse(line);
            if (res.id === 1) { // Initialize OK
                send('authenticate', { methodId: 'gemini-api-key' });
            } else if (res.id === 2) { // Authenticate OK
                console.log('--- TESTANDO session/new ---');
                send('session/new', { cwd: __dirname });
            } else if (res.id === 3) { // session/new Response
                if (res.error) {
                    console.log('❌ session/new FALHOU. Tentando newSession...');
                    send('newSession', { cwd: __dirname });
                } else {
                    console.log('✅ session/new FUNCIONA!');
                    process.exit(0);
                }
            } else if (res.id === 4) { // newSession Response
                if (res.error) {
                    console.log('❌ newSession TAMBÉM FALHOU.');
                } else {
                    console.log('✅ newSession FUNCIONA!');
                }
                process.exit(0);
            }
        } catch(e) {}
    }
});

child.stderr.on('data', (data) => console.error(`ERR: ${data.toString()}`));

setTimeout(() => {
    send('initialize', { protocolVersion: 1 });
}, 1000);

setTimeout(() => {
    console.log('Timeout!');
    child.kill();
    process.exit(1);
}, 10000);
