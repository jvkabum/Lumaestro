package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// SecurityConfig define as permissões granulares para a IA
type SecurityConfig struct {
	AllowRead         bool     `json:"allow_read"`
	AllowWrite        bool     `json:"allow_write"`
	AllowCreate       bool     `json:"allow_create"`
	AllowDelete       bool     `json:"allow_delete"`
	AllowMove         bool     `json:"allow_move"`
	AllowRunCommands  bool     `json:"allow_run_commands"`
	FullMachineAccess bool     `json:"full_machine_access"` // Se falso, restringe aos Workspaces
	Workspaces        []string `json:"workspaces"`          // Lista de pastas autorizadas (Whitelist)
}

// GeminiAccount representa um perfil de login do Gemini
type GeminiAccount struct {
	Name      string `json:"name"`
	HomeDir   string `json:"home_dir"` // Caminho da pasta de sessão (.gemini_accounts/nome)
	Active    bool   `json:"active"`
	Exhausted bool   `json:"exhausted"`
}

// ProjectScan mapeia uma pasta que serve como repositório secundário (satélite/aglomerado)
type ProjectScan struct {
	Path        string `json:"path"`
	CoreNode    string `json:"core_node"`    // Nó raíz radial (ex: MóduloAuth, Gesttik)
	IncludeCode bool   `json:"include_code"` // Se true, o Code RAG roda em todo o source no diretório
}

// Config representa as configurações globais do orquestrador.
type Config struct {
	ObsidianVaultPath   string          `json:"obsidian_vault_path"`
	QdrantURL           string          `json:"qdrant_url"`
	QdrantAPIKey        string          `json:"qdrant_api_key"`
	GeminiAPIKey        string          `json:"gemini_api_key"` // Aceita múltiplas chaves separadas por vírgula
	UseGeminiAPIKey     bool            `json:"use_gemini_api_key"`
	GeminiKeyIndex      int             `json:"gemini_key_index"` // Índice da chave ativa no pool
	GeminiAccounts      []GeminiAccount `json:"gemini_accounts"`  // 🌟 Nova lista de contas
	ClaudeAPIKey        string          `json:"claude_api_key"`
	UseClaudeAPIKey     bool            `json:"use_claude_api_key"`
	ActiveAgent         string          `json:"active_agent"`
	AutoStartAgents     []string        `json:"auto_start_agents"`
	AgentLanguage       string          `json:"agent_language"`
	MaxConcurrentAgents int             `json:"max_concurrent_agents"` // 🌟 Limite de Enxame (Swarm)
	ExternalProjects    []ProjectScan   `json:"external_projects"`     // 🌟 Repositórios e Aglomerados Code RAG
	GraphDepth          int             `json:"graph_depth"`           // Profundidade de navegação de links (padrão: 1)
	GraphNeighborLimit  int             `json:"graph_neighbor_limit"`  // Máximo de vizinhos por nó (padrão: 5)
	GraphContextLimit   int             `json:"graph_context_limit"`   // Limite de chars do contexto expandido (padrão: 4000)
	Security            SecurityConfig  `json:"security"`

	// ⚡ Configurações do Motor Lightning (Aprendizado por Reforço)
	LightningEnabled   bool   `json:"lightning_enabled"`    // Ativa o rastreamento e aprendizado
	LightningProxyPort string `json:"lightning_proxy_port"` // Porta do proxy local (padrão: 8001)

	// 🤖 LM Studio (Motor Local OpenAI-Compatível)
	LMStudioURL     string `json:"lmstudio_url"`     // URL base do servidor LM Studio (ex: http://localhost:1234)
	LMStudioModel   string `json:"lmstudio_model"`   // ID do modelo carregado no LM Studio
	LMStudioEnabled bool   `json:"lmstudio_enabled"` // Habilita o LM Studio como motor de IA

	// 🎛️ Pool de Motores Ativos (Blend entre Gemini/Claude/LM Studio)
	BlendActiveModels    bool     `json:"blend_active_models"`
	ActiveModelProviders []string `json:"active_model_providers"`
	PrimaryProvider      string   `json:"primary_provider"`

	// 🔬 Motor de Embeddings (vetores para busca semântica no Qdrant)
	EmbeddingsProvider string `json:"embeddings_provider"` // "gemini", "lmstudio", ou "native"
	EmbeddingsModel    string `json:"embeddings_model"`    // Ex: "nomic-embed-text", "text-embedding-nomic-embed-text-v1.5", etc
	EmbeddingDimension int    `json:"embedding_dimension"` // 3072 para Gemini, 768 para nomic, etc.

	// 🧠 Motor de RAG/Ontologia (geração textual para extração de triplas e chat semântico)
	RAGProvider string `json:"rag_provider"` // "gemini", "lmstudio", "claude" ou "native"
	RAGModel    string `json:"rag_model"`    // Ex: "google/gemma-4-26b-a4b", "claude-3-5-sonnet-latest"
	GeminiModel string `json:"gemini_model"` // Modelo padrão para chat (auto, 2.5-flash, etc)
}

// NormalizeProviders garante defaults seguros para o pool de provedores e motores.
func (c *Config) NormalizeProviders() {
	if c == nil {
		return
	}

	if len(c.ActiveModelProviders) == 0 {
		c.ActiveModelProviders = []string{"gemini", "claude", "lmstudio", "native"}
	}

	if strings.TrimSpace(c.PrimaryProvider) == "" {
		c.PrimaryProvider = "gemini"
	}

	if strings.TrimSpace(c.EmbeddingsProvider) == "" {
		c.EmbeddingsProvider = "gemini"
	}

	if c.EmbeddingDimension <= 0 {
		if c.EmbeddingsProvider == "lmstudio" {
			c.EmbeddingDimension = 768 // Default para modelos nomic/local
		} else if c.EmbeddingsProvider == "native" {
			c.EmbeddingDimension = 1024 // Default para Qwen3-0.6B local
		} else {
			c.EmbeddingDimension = 3072 // Gemini embedding v2
		}
	}

	if strings.TrimSpace(c.RAGProvider) == "" {
		c.RAGProvider = "gemini"
	}
}

// GetActiveProviders retorna a lista de provedores ativos normalizada e sem duplicatas.
func (c *Config) GetActiveProviders() []string {
	c.NormalizeProviders()

	allowed := map[string]bool{
		"gemini":   true,
		"claude":   true,
		"lmstudio": true,
		"native":   true,
	}
	seen := map[string]bool{}
	providers := make([]string, 0, len(c.ActiveModelProviders))

	for _, p := range c.ActiveModelProviders {
		k := strings.ToLower(strings.TrimSpace(p))
		if !allowed[k] || seen[k] {
			continue
		}
		seen[k] = true
		providers = append(providers, k)
	}

	if len(providers) == 0 {
		providers = []string{"gemini"}
	}

	return providers
}

// GetGeminiKeys retorna a lista de chaves API do Gemini (split por vírgula).
func (c *Config) GetGeminiKeys() []string {
	raw := strings.TrimSpace(c.GeminiAPIKey)
	if raw == "" {
		return nil
	}
	var keys []string
	for _, k := range strings.Split(raw, ",") {
		k = strings.TrimSpace(k)
		if k != "" {
			keys = append(keys, k)
		}
	}
	return keys
}

// GetActiveGeminiKey retorna a chave API ativa do pool (com base no índice atual).
func (c *Config) GetActiveGeminiKey() string {
	keys := c.GetGeminiKeys()
	if len(keys) == 0 {
		return ""
	}
	idx := c.GeminiKeyIndex % len(keys)
	return keys[idx]
}

// RotateGeminiKey avança para a próxima chave no pool e persiste a mudança.
// Retorna a nova chave ativa ou "" se não houver mais chaves.
func (c *Config) RotateGeminiKey() string {
	keys := c.GetGeminiKeys()
	if len(keys) <= 1 {
		return c.GetActiveGeminiKey() // Sem rotação possível
	}
	c.GeminiKeyIndex = (c.GeminiKeyIndex + 1) % len(keys)
	// Persiste o novo índice
	Save(*c)
	fmt.Printf("[KeyPool] 🔄 Rotação de chave Gemini: Agora usando chave #%d de %d\n", c.GeminiKeyIndex+1, len(keys))
	return keys[c.GeminiKeyIndex]
}

// GeminiKeyCount retorna quantas chaves estão no pool.
func (c *Config) GeminiKeyCount() int {
	return len(c.GetGeminiKeys())
}

func getConfigPath() string {
	// Migração automática de config.json legado
	if _, err := os.Stat("config.json"); err == nil && !strings.Contains(os.Getenv("WAILS_WASM"), "true") {
		_ = os.Rename("config.json", ".lumaestro.json")
	}
	return ".lumaestro.json"
}

// Save armazena as configurações em um arquivo JSON.
func Save(cfg Config) error {
	cfg.NormalizeProviders()
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(getConfigPath(), data, 0644)
}

// Load recupera as configurações do arquivo JSON com resiliência a Race Conditions.
func Load() (*Config, error) {
	path := getConfigPath()

	var data []byte
	var err error

	// 🔄 Retry Loop: Tenta ler até 3 vezes com pequeno delay se o arquivo estiver vazio/corrompido
	// Isso evita o erro 'unexpected end of JSON input' durante o Save() simultâneo.
	for i := 0; i < 3; i++ {
		data, err = os.ReadFile(path)
		if err == nil && len(data) > 2 { // Verifica se tem conteúdo mínimo
			break
		}
		if i < 2 {
			time.Sleep(100 * time.Millisecond)
		}
	}

	if err != nil {
		fmt.Printf("[Config] Aviso: %s não encontrado no diretorio (%v)\n", path, err)
		return &Config{}, nil
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		// Se falhar o parse, tentamos uma última vez após um sono maior
		time.Sleep(200 * time.Millisecond)
		data, _ = os.ReadFile(path)
		if errRetry := json.Unmarshal(data, &cfg); errRetry == nil {
			return &cfg, nil
		}
		fmt.Printf("[Config] ERRO CRITICO Parse JSON: %v\n", err)
		return nil, err
	}
	cfg.NormalizeProviders()
	return &cfg, nil
}
