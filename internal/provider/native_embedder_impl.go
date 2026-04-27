package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync" // Adicionado para proteção de concorrência
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
	mu      sync.Mutex // Mutex para evitar que o llama-server receba chamadas paralelas
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

	// Cria um alias com nome descritivo para aparecer no Gerenciador de Tarefas
	finalBin = createProcessAlias(finalBin, "lumaestro-embedder")

	// 2. Monta os argumentos usando -hf (download automático do HuggingFace)
	//    Na primeira execução, o modelo será baixado e cacheado automaticamente.
	//    Nas próximas, ele usa o cache local — instantâneo.
	args := []string{
		"--port", fmt.Sprintf("%d", n.port),
	}

	// 🛠️ Proteção Hugging Face: Mapeador Inteligente de Repositório e Arquivo
	// Se houver ":", separamos precisamente (--hf-repo e --hf-file)
	if parts := strings.Split(n.hfModel, ":"); len(parts) == 2 {
		args = append(args, "--hf-repo", parts[0], "--hf-file", parts[1])
	} else {
		args = append(args, "-hf", n.hfModel)
	}

	args = append(args,
		"--embedding",
		"--pooling", "cls",
		"--ctx-size", "32768",
		"--n-gpu-layers", "-1", // -1 delega 100% das camadas que couberem na VRAM para a Placa de Vídeo; o resto fica na CPU.
	)

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
	url := fmt.Sprintf("http://127.0.0.1:%d/health", n.port)

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
	// 🔒 Proteção: enfileira requisições. O llama-server nativo (CPU) não lida bem com paralelo.
	n.mu.Lock()
	defer n.mu.Unlock()

	url := fmt.Sprintf("http://127.0.0.1:%d/embedding", n.port)
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
		// Captura a mensagem de erro real do servidor para facilitar debug
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("erro no motor nativo: status %d — %s", resp.StatusCode, string(errBody)[:min(len(errBody), 300)])
	}

	// Lógica flexível: o llama-server retorna em múltiplos formatos dependendo da versão
	resBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("falha ao ler corpo da resposta: %w", err)
	}

	// Formato 1 (REAL llama-server /embedding): [{"index":0,"embedding":[[0.01,0.02,...]]}]
	// O embedding é uma array 2D! Precisamos pegar o primeiro sub-array.
	var nativeResult []struct {
		Index     int         `json:"index"`
		Embedding [][]float64 `json:"embedding"`
	}
	if err := json.Unmarshal(resBytes, &nativeResult); err == nil && len(nativeResult) > 0 && len(nativeResult[0].Embedding) > 0 && len(nativeResult[0].Embedding[0]) > 0 {
		raw := nativeResult[0].Embedding[0]
		result := make([]float32, len(raw))
		for i, v := range raw {
			result[i] = float32(v)
		}
		return result, nil
	}

	// Formato 2: Objeto simples {"embedding": [...]} (Legacy)
	var objResult struct {
		Embedding []float32 `json:"embedding"`
	}
	if err := json.Unmarshal(resBytes, &objResult); err == nil && len(objResult.Embedding) > 0 {
		return objResult.Embedding, nil
	}

	// Formato 3: Array de objetos com embedding 1D [{"embedding": [...]}]
	var arrayObjResult []struct {
		Embedding []float32 `json:"embedding"`
	}
	if err := json.Unmarshal(resBytes, &arrayObjResult); err == nil && len(arrayObjResult) > 0 && len(arrayObjResult[0].Embedding) > 0 {
		return arrayObjResult[0].Embedding, nil
	}

	// Formato 4: OpenAI Compatible {"data": [{"embedding": [...]}]} (/v1/embeddings)
	var openAIResult struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resBytes, &openAIResult); err == nil && len(openAIResult.Data) > 0 && len(openAIResult.Data[0].Embedding) > 0 {
		return openAIResult.Data[0].Embedding, nil
	}

	// Formato 5: Array direto de floats [0.1, 0.2, ...]
	var arrayResult []float32
	if err := json.Unmarshal(resBytes, &arrayResult); err == nil && len(arrayResult) > 0 {
		return arrayResult, nil
	}

	return nil, fmt.Errorf("formato de resposta de embedding desconhecido. Raw: %.300s", string(resBytes))
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

// createProcessAlias cria um hard link do binário com um nome descritivo NA MESMA PASTA do original.
// Isso faz com que o processo apareça no Gerenciador de Tarefas com um nome identificável
// (ex: "lumaestro-embedder" ao invés de "llama-server"), e garante acesso às DLLs companheiras.
func createProcessAlias(originalBin, aliasName string) string {
	if runtime.GOOS != "windows" {
		return originalBin // Em Linux/macOS, não é necessário
	}

	// Cria o alias na MESMA pasta do binário original (essencial para DLLs do llama.cpp)
	aliasDir := filepath.Dir(originalBin)
	aliasPath := filepath.Join(aliasDir, aliasName+".exe")

	// Se o alias já existe e é válido, usa direto
	if info, err := os.Stat(aliasPath); err == nil && info.Size() > 0 {
		return aliasPath
	}

	// Remove alias antigo/corrompido se existir
	os.Remove(aliasPath)

	// Tenta criar hard link (sem duplicar espaço em disco)
	if err := os.Link(originalBin, aliasPath); err != nil {
		fmt.Printf("[Alias] ⚠️ Não foi possível criar alias '%s': %v\n", aliasName, err)
		return originalBin
	}

	fmt.Printf("[Alias] ✅ Processo '%s' registrado como '%s'\n", filepath.Base(originalBin), aliasName)
	return aliasPath
}
