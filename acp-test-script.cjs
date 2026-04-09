const { spawn } = require('child_process');
const path = require('path');
const fs = require('fs');

const geminiScript = path.join(__dirname, 'node_modules', '@google', 'gemini-cli', 'bundle', 'gemini.js');
const diagLog = path.join(__dirname, 'gemini-telemetry.json');

console.log(`[TEST] Iniciando Gemini CLI em: ${geminiScript}`);
console.log(`[TEST] Usando HOME: ${__dirname}`);

const child = spawn('node', ['--no-warnings=DEP0040', geminiScript, '--acp', '--debug'], {
    cwd: __dirname,
    env: { 
        ...process.env, 
        GEMINI_CLI_HOME: __dirname,
        GEMINI_TELEMETRY_ENABLED: "true",
        GEMINI_TELEMETRY_TARGET: "local",
        GEMINI_TELEMETRY_OUTFILE: diagLog
    },
    stdio: ['pipe', 'pipe', 'pipe']
});

let buffer = '';

child.stdout.on('data', (data) => {
    const str = data.toString();
    console.log(`[STDOUT RAW] ${str.trim()}`);
    buffer += str;
    
    // Tenta detectar JSON-RPC vindo do Gemini
    if (buffer.includes('\n')) {
        const lines = buffer.split('\n');
        buffer = lines.pop();
        for (const line of lines) {
            if (line.trim()) {
                console.log(`[JSON RECEBIDO] ${line.trim()}`);
                handleMessage(line.trim());
            }
        }
    }
});

child.stderr.on('data', (data) => {
    console.error(`[STDERR] ${data.toString().trim()}`);
});

let lastSessionId = "dummy";

function handleMessage(jsonStr) {
    try {
        const msg = JSON.parse(jsonStr);
        // Atualiza ID se receber de 'session/new'
        if (msg.result && msg.result.sessionId) {
            lastSessionId = msg.result.sessionId;
            console.log(`[TEST] ✅ Capturado SessionID do Gemini: ${lastSessionId}`);
        }

        // Se o Gemini pedir algo, respondemos para não travar
        if (msg.method === 'client/readFile') {
            console.log(`[AUTO-REPLY] Respondendo pedido de leitura de arquivo: ${msg.params.path}`);
            send({
                jsonrpc: "2.0",
                id: msg.id,
                result: { content: "" } // Simula arquivo vazio
            });
        }
    } catch (e) {
        // Não é JSON, ignora
    }
}

function send(obj) {
    const json = JSON.stringify(obj) + '\n';
    console.log(`[SENDING] ${json.trim()}`);
    child.stdin.write(json);
}

// Pequeno delay para o Node carregar
setTimeout(() => {
    console.log('--- 1. INITIALIZE ---');
    send({
        jsonrpc: "2.0",
        id: 1,
        method: "initialize",
        params: {
            protocolVersion: 1,
            clientInfo: { name: "TestScript", version: "1.0.0" },
            clientCapabilities: { fs: { readTextFile: true, writeTextFile: true } }
        }
    });

    setTimeout(() => {
        console.log('--- 2. AUTHENTICATE ---');
        send({
            jsonrpc: "2.0",
            id: 2,
            method: "authenticate",
            params: { methodId: "gemini-api-key" }
        });

        setTimeout(() => {
            console.log('--- 3. SESSION NEW ---');
            send({
                jsonrpc: "2.0",
                id: 3,
                method: "newSession",
                params: { cwd: __dirname, mcpServers: [] }
            });

            setTimeout(() => {
                console.log('--- 4. SESSION PROMPT ---');
                send({
                    jsonrpc: "2.0",
                    id: 4,
                    method: "prompt",
                    params: {
                        sessionId: lastSessionId, 
                        prompt: [{ type: "text", text: "ola gemini" }]
                    }
                });
            }, 3000);
        }, 3000);
    }, 3000);
}, 3000);

// Timeout de segurança do script
setTimeout(() => {
    console.log('--- FIM DO TESTE ---');
    child.kill();
    process.exit(0);
}, 18000);
