package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"sync" // Adicionado para proteção de concorrência
	"syscall"
	"time"
)

// Usando o repositório Unsloth com a tag de quantização específica
const defaultHFRAGModel = "unsloth/gemma-4-E4B-it-GGUF:Q5_K_M"

// NativeGenerator implementa ContentGenerator usando llama-server.
type NativeGenerator struct {
	displayName string
	hfModel     string
	port        int
	cmd         *exec.Cmd
	client      *http.Client
	mu          sync.Mutex // Mutex para evitar sobrecarga no motor especialista
	OnLog       func(string) // Callback para progresso (download/hf)
}

func NewNativeGenerator(hfModel string, port int, name string) *NativeGenerator {
	if hfModel == "" {
		hfModel = defaultHFRAGModel
	}
	if port <= 0 {
		port = 8086
	}
	if name == "" {
		name = "NativeChat"
	}
	return &NativeGenerator{
		displayName: name,
		hfModel:     hfModel,
		port:        port,
		client:      &http.Client{Timeout: 120 * time.Second},
	}
}

// Start inicia o motor de chat/RAG via llama-server.
func (n *NativeGenerator) Start() error {
	binName := "llama-server"
	if runtime.GOOS == "windows" {
		binName = "llama-server.exe"
	}

	finalBin, err := exec.LookPath(binName)
	if err != nil {
		return fmt.Errorf("llama-server não encontrado. Instale com 'winget install llama.cpp'")
	}

	// Cria alias descritivo para o Gerenciador de Tarefas
	finalBin = createProcessAlias(finalBin, "lumaestro-specialist")

	// Argumentos otimizados para Chat/RAG
	args := []string{
		"--port", fmt.Sprintf("%d", n.port),
	}

	// 🛠️ Proteção Hugging Face: Mapeador Inteligente de Repositório e Arquivo
	// Se houver ":", separamos precisamente para o llama-server não se perder (Evita erro 404)
	if parts := strings.Split(n.hfModel, ":"); len(parts) == 2 {
		args = append(args, "--hf-repo", parts[0], "--hf-file", parts[1])
	} else {
		args = append(args, "-hf", n.hfModel)
	}

	// Anexamos os argumentos de CPU/GPU
	args = append(args,
		"--ctx-size", "4096", // Contexto reduzido de 8k para 4k para poupar compute e dobrar o TPS inicial
		"--n-gpu-layers", "-1", // Tenta usar GPU máxima se disponível
	)

	fmt.Printf("[%s] 🚀 Iniciando Gerador Local: %s %v\n", n.displayName, finalBin, args)

	n.cmd = exec.Command(finalBin, args...)

	if runtime.GOOS == "windows" {
		n.cmd.SysProcAttr = &syscall.SysProcAttr{
			HideWindow:    true,
			CreationFlags: 0x08000000,
		}
	}

	stdout, _ := n.cmd.StdoutPipe()
	stderr, _ := n.cmd.StderrPipe()

	if err := n.cmd.Start(); err != nil {
		return fmt.Errorf("erro ao disparar llama-server (chat): %w", err)
	}

	// Scanner assíncrono para monitorar download/HF progress (Une Stdout e Stderr)
	go func() {
		multi := io.MultiReader(stdout, stderr)
		scanner := bufio.NewScanner(multi)
		for scanner.Scan() {
			line := scanner.Text()
			if n.OnLog != nil {
				lLine := strings.ToLower(line)
				// Filtra apenas linhas relevantes de download/status para não poluir
				if strings.Contains(lLine, "download") || 
				   strings.Contains(lLine, "progress") || 
				   strings.Contains(lLine, "%") || 
				   strings.Contains(lLine, "failed") || 
				   strings.Contains(lLine, "status") || 
				   strings.Contains(lLine, "error") {
					n.OnLog(line)
				}
			}
		}
	}()

	fmt.Printf("[%s] ⏳ Aguardando motor ficar pronto (download do modelo pode demorar)...\n", n.displayName)
	return n.waitForReady()
}

func (n *NativeGenerator) waitForReady() error {
	url := fmt.Sprintf("http://localhost:%d/health", n.port)
	for i := 0; i < 300; i++ { // 5 minutos de timeout para download inicial
		resp, err := n.client.Get(url)
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			fmt.Printf("[%s] ✅ Motor ONLINE na porta %d.\n", n.displayName, n.port)
			return nil
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("timeout guardando motor de chat")
}

func (n *NativeGenerator) GenerateText(ctx context.Context, prompt string) (string, error) {
	// 🔒 Proteção: enfileira requisições para evitar erro de conexão/timeout na GPU
	n.mu.Lock()
	defer n.mu.Unlock()

	url := fmt.Sprintf("http://localhost:%d/v1/chat/completions", n.port)

	payload := map[string]interface{}{
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"temperature": 0.2, // Baixa temperatura para RAG (mais factual)
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("erro no motor local: status %d", resp.StatusCode)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if len(result.Choices) > 0 {
		return result.Choices[0].Message.Content, nil
	}

	return "", fmt.Errorf("resposta vazia do modelo local")
}

func (n *NativeGenerator) GenerateMultimodalText(ctx context.Context, prompt string, data []byte, mimeType string) (string, error) {
	// Gemma-4 / Llama-3 standard GGUF não suportam visão nativa via llama-server sem mmproj.
	// Por enquanto, mantemos apenas texto para o motor offline nativo.
	return "", fmt.Errorf("gerador nativo suporta apenas Texto/RAG por enquanto")
}

func (n *NativeGenerator) Stop() {
	if n.cmd != nil && n.cmd.Process != nil {
		fmt.Printf("[%s] 🛑 Encerrando motor local na porta %d...\n", n.displayName, n.port)
		n.cmd.Process.Kill()
	}
}
