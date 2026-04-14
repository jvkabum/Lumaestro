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
	"syscall"
	"time"
)

// Repositório especializado na versão Q4_K_M para garantir compatibilidade
const defaultHFModel = "enacimie/Qwen3-Embedding-0.6B-Q4_K_M-GGUF"

// NativeEmbedder gerencia um processo interno do llama-server
// que baixa e carrega o modelo automaticamente via HuggingFace.
type NativeEmbedder struct {
	hfModel string // Ex: "Qwen/Qwen3-Embedding-0.6B-GGUF:Q8_0"
	port    int
	cmd     *exec.Cmd
	client  *http.Client
	OnLog   func(string) // Callback para progresso (download/hf)
}

func NewNativeEmbedder(hfModel string) *NativeEmbedder {
	if hfModel == "" {
		hfModel = defaultHFModel
	}
	return &NativeEmbedder{
		hfModel: hfModel,
		port:    8085,
		client:  &http.Client{Timeout: 60 * time.Second},
	}
}

// Start inicia o llama-server com download automático do modelo via -hf.
// Pré-requisito: o usuário ter rodado "winget install llama.cpp" uma única vez.
func (n *NativeEmbedder) Start() error {
	// 1. Localiza o binário no PATH do sistema (instalado via winget/brew)
	binName := "llama-server"
	if runtime.GOOS == "windows" {
		binName = "llama-server.exe"
	}

	finalBin, err := exec.LookPath(binName)
	if err != nil {
		return fmt.Errorf(
			"llama-server não encontrado no PATH. Instale com: winget install llama.cpp (Windows) ou brew install llama.cpp (macOS)",
		)
	}

	// 2. Monta os argumentos usando -hf (download automático do HuggingFace)
	//    Na primeira execução, o modelo será baixado e cacheado automaticamente.
	//    Nas próximas, ele usa o cache local — instantâneo.
	args := []string{
		"--port", fmt.Sprintf("%d", n.port),
		"-hf", n.hfModel,
		"--embedding",
		"--pooling", "cls",
		"--ctx-size", "2048",
		// --log-disable removido para permitir captura de progresso de download
	}

	fmt.Printf("[NativeEngine] 🚀 Iniciando: %s %v\n", finalBin, args)

	n.cmd = exec.Command(finalBin, args...)

	// Ocultar janela no Windows (sem terminal piscando)
	if runtime.GOOS == "windows" {
		n.cmd.SysProcAttr = &syscall.SysProcAttr{
			HideWindow:    true,
			CreationFlags: 0x08000000, // CREATE_NO_WINDOW
		}
	}

	stdout, _ := n.cmd.StdoutPipe()
	stderr, _ := n.cmd.StderrPipe()

	if err := n.cmd.Start(); err != nil {
		return fmt.Errorf("erro ao disparar llama-server: %w", err)
	}

	// Scanner assíncrono para monitorar download/HF progress (Une Stdout e Stderr)
	go func() {
		multi := io.MultiReader(stdout, stderr)
		scanner := bufio.NewScanner(multi)
		for scanner.Scan() {
			line := scanner.Text()
			if n.OnLog != nil {
				lLine := strings.ToLower(line)
				if strings.Contains(lLine, "download") || 
				   strings.Contains(lLine, "progress") || 
				   strings.Contains(lLine, "%") || 
				   strings.Contains(lLine, "error") {
					n.OnLog(line)
				}
			}
		}
	}()

	fmt.Println("[NativeEngine] ⏳ Aguardando motor ficar pronto (pode levar ~30s no primeiro download)...")
	return n.waitForReady()
}

func (n *NativeEmbedder) waitForReady() error {
	url := fmt.Sprintf("http://localhost:%d/health", n.port)

	// Timeout maior na primeira vez (download do modelo)
	for i := 0; i < 120; i++ {
		resp, err := n.client.Get(url)
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			fmt.Println("[NativeEngine] ✅ Motor Nativo ONLINE e pronto.")
			return nil
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("timeout: motor nativo não respondeu após 120 segundos")
}

func (n *NativeEmbedder) GenerateEmbedding(ctx context.Context, text string, fastTrack bool) ([]float32, error) {
	url := fmt.Sprintf("http://localhost:%d/embedding", n.port)

	payload := map[string]interface{}{
		"content": text,
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("erro no motor nativo: status %d", resp.StatusCode)
	}

	// Lógica flexível: o llama-server pode retornar {"embedding": [...]} ou direto o array [...]
	resBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("falha ao ler corpo da resposta: %w", err)
	}

	// Tenta formato Objeto 1 (Standard OpenAI/Legacy)
	var objResult struct {
		Embedding []float32 `json:"embedding"`
	}
	if err := json.Unmarshal(resBytes, &objResult); err == nil && len(objResult.Embedding) > 0 {
		return objResult.Embedding, nil
	}

	// Tenta formato Array Direto (Novo padrão detectado)
	var arrayResult []float32
	if err := json.Unmarshal(resBytes, &arrayResult); err == nil && len(arrayResult) > 0 {
		return arrayResult, nil
	}

	return nil, fmt.Errorf("formato de resposta de embedding desconhecido ou vazio")
}

func (n *NativeEmbedder) GenerateMultimodalEmbedding(ctx context.Context, data []byte, mimeType string, fastTrack bool) ([]float32, error) {
	return nil, fmt.Errorf("modo nativo não suporta multimodal (mídia)")
}

func (n *NativeEmbedder) Stop() {
	if n.cmd != nil && n.cmd.Process != nil {
		fmt.Println("[NativeEngine] 🛑 Encerrando motor nativo...")
		n.cmd.Process.Kill()
	}
}
