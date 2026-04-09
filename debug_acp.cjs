const { spawn } = require('child_process');
const path = require('path');
const fs = require('fs');

const geminiScript = path.join(__dirname, 'node_modules', '@google', 'gemini-cli', 'bundle', 'gemini.js');

console.log('=== DIAGNÓSTICO ACP v4 (ndJSON + Protocol Fix) ===');
console.log('Gemini CLI:', geminiScript);
console.log('');

const child = spawn('node', ['--no-warnings=DEP0040', geminiScript, '--acp'], {
    cwd: __dirname,
    env: process.env,
    stdio: ['pipe', 'pipe', 'pipe']
});

let msgId = 0;

function sendRPC(msg) {
    const data = JSON.stringify(msg) + '\n';
    console.log(`>> [ID:${msg.id}] ${msg.method || 'response'}`);
    console.log(`   ${JSON.stringify(msg)}`);
    child.stdin.write(data);
}

let buffer = '';
let step = 0;
let sessionId = null;

child.stdout.on('data', (chunk) => {
    buffer += chunk.toString('utf-8');
    const lines = buffer.split('\n');
    buffer = lines.pop() || '';
    
    for (const line of lines) {
        const trimmed = line.trim();
        if (!trimmed) continue;
        
        try {
            const msg = JSON.parse(trimmed);
            step++;
            console.log(`\n<< [STEP ${step}] RESPOSTA:`);
            console.log(JSON.stringify(msg, null, 2));
            
            if (msg.error) {
                console.error(`!! ERRO: Code=${msg.error.code} Msg=${msg.error.message}`);
            }

            // Resposta do initialize (id: 1) -> chamar authenticate
            if (msg.id === 1 && msg.result) {
                console.log('\n✅ INITIALIZE OK!');
                
                setTimeout(() => {
                    console.log('\n--- FASE 2: AUTHENTICATE ---');
                    sendRPC({
                        jsonrpc: '2.0',
                        id: ++msgId,
                        method: 'authenticate',
                        params: { methodId: 'gemini-api-key' }
                    });
                }, 500);
            }
            
            // Resposta do authenticate (id: 2) -> chamar newSession
            if (msg.id === 2 && !msg.error) {
                console.log('\n✅ AUTHENTICATE OK!');
                
                setTimeout(() => {
                    console.log('\n--- FASE 3: NEW SESSION ---');
                    sendRPC({
                        jsonrpc: '2.0',
                        id: ++msgId,
                        method: 'session/new',
                        params: { cwd: process.cwd(), mcpServers: [] }
                    });
                }, 500);
            }
            
            // Resposta do newSession -> captura sessionId e envia prompt
            if (msg.result && msg.result.sessionId) {
                sessionId = msg.result.sessionId;
                console.log(`\n✅ SESSION CRIADA! ID: ${sessionId}`);
                
                setTimeout(() => {
                    console.log('\n--- FASE 4: PROMPT ---');
                    sendRPC({
                        jsonrpc: '2.0',
                        id: ++msgId,
                        method: 'session/prompt',
                        params: {
                            sessionId: sessionId,
                            prompt: [{ type: 'text', text: 'Olá!' }]
                        }
                    });
                }, 500);
            }
            
            if (msg.method === 'sessionUpdate') {
                 console.log('Update:', JSON.stringify(msg.params.update));
            }
            
        } catch (e) {
            console.error('!! Parse error:', e.message, 'Line:', trimmed);
        }
    }
});

child.stderr.on('data', (data) => {
    console.error(`[stderr] ${data.toString()}`);
});

console.log('Aguardando Gemini CLI (2s)...\n');
setTimeout(() => {
    console.log('--- FASE 1: INITIALIZE ---');
    msgId = 1;
    sendRPC({
        jsonrpc: '2.0',
        id: msgId,
        method: 'initialize',
        params: {
            protocolVersion: 2,
            clientCapabilities: {
                fs: { readTextFile: true, writeTextFile: true }
            }
        }
    });
}, 2000);

setTimeout(() => {
    child.kill();
    process.exit();
}, 30000);
