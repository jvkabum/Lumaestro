package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

// Identity representa um perfil de usuário para um provedor específico
type Identity struct {
	Provider  string `json:"provider"`  // "google", "claude", "openai", "qwen"
	Name      string `json:"name"`      // Nome amigável (ex: "Trabalho", "Pessoal")
	HomeDir   string `json:"home_dir"`  // Caminho da pasta de sessão (específico para Google/OAuth)
	APIKey    string `json:"api_key"`   // Chave individual desta identidade (opcional)
	Active    bool   `json:"active"`    // Se esta identidade é a ativa para o seu provedor
	Exhausted bool   `json:"exhausted"` // Flag de cota estourada
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
	Identities          []Identity      `json:"identities"`       // 🎭 Nova lista universal de identidades
	GeminiAccounts      []Identity      `json:"gemini_accounts,omitempty"` // ⚠️ Legado (apenas para migração)
	ClaudeAPIKey        string          `json:"claude_api_key"`
	UseClaudeAPIKey     bool            `json:"use_claude_api_key"`
	GroqAPIKey          string          `json:"groq_api_key"`
	GroqKeyIndex        int             `json:"groq_key_index"` // Índice da chave ativa no pool Groq
	ActiveAgent         string          `json:"active_agent"`
	AutoStartAgents     []string        `json:"auto_start_agents"`
	AgentLanguage       string          `json:"agent_language"`
	MaxConcurrentAgents int             `json:"max_concurrent_agents"` // 🌟 Limite de Enxame (Swarm)
	ExternalProjects    []ProjectScan   `json:"external_projects"`     // 🌟 Repositórios e Aglomerados Code RAG
	GraphDepth          int             `json:"graph_depth"`           // Profundidade de navegação de links (padrão: 1)
	GraphNeighborLimit  int             `json:"graph_neighbor_limit"`  // Máximo de vizinhos por nó (padrão: 5)
	GraphContextLimit   int             `json:"graph_context_limit"`   // Limite de chars do contexto expandido (padrão: 4000)
	EnableNeuralEdges   bool            `json:"enable_neural_edges"`   // 🧠 Ativa a visualização de sinapses dinâmicas no Grafo 3D
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
	RAGProvider           string   `json:"rag_provider"` // "gemini", "lmstudio", "claude" ou "native"
	RAGModel              string   `json:"rag_model"`    // Ex: "google/gemma-4-26b-a4b", "claude-3-5-sonnet-latest"
	HybridFailoverEnabled bool     `json:"hybrid_failover_enabled"`
	FailoverPriority      []string `json:"failover_priority"` // Ex: ["groq", "gemini", "native"]
	GeminiModel           string   `json:"gemini_model"`      // Modelo padrão para chat (auto, 2.5-flash, etc)
	GroqModel             string   `json:"groq_model"`        // Modelo padrão para Groq (ex: qwen/qwen3-32b)
	ActiveGroqModels      []string `json:"active_groq_models"` // 🚀 Lista de modelos ativos na Frota de Resiliência Groq
	ActiveGoogleModels    []string `json:"active_google_models"` // 🌟 Lista de modelos ativos na Frota de Resiliência Google
	ActiveNativeModels    []string `json:"active_native_models"` // 🧩 Lista de modelos GGUF locais (Qwen, Gemma, etc)

	// 📂 Workspace Ativo (diretório de trabalho da IA)
	ActiveWorkspace       string   `json:"active_workspace"` // Caminho absoluto do projeto alvo (vazio = raiz do Lumaestro)
}

// NormalizeProviders garante defaults seguros para o pool de provedores e motores.
func (c *Config) NormalizeProviders() {
	if c == nil {
		return
	}

	if len(c.ActiveModelProviders) == 0 {
		c.ActiveModelProviders = []string{"gemini", "claude", "lmstudio", "native", "groq"}
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

	if c.RAGProvider == "" {
		c.RAGProvider = "gemini"
	}
	if len(c.FailoverPriority) == 0 {
		c.FailoverPriority = []string{"groq", "gemini", "native"}
	}

	if strings.TrimSpace(c.GroqModel) == "" {
		c.GroqModel = "llama-3.3-70b-versatile"
	}
	if len(c.ActiveGroqModels) == 0 {
		c.ActiveGroqModels = []string{
			"llama-3.3-70b-versatile",
			"openai/gpt-oss-120b",
			"qwen/qwen3-32b",
			"moonshotai/kimi-k2-instruct",
			"moonshotai/kimi-k2-instruct-0905",
			"meta-llama/llama-4-scout-17b-16e-instruct",
			"openai/gpt-oss-20b",
			"allam-2-7b",
			"llama-3.1-8b-instant",
			"groq/compound",
			"groq/compound-mini",
		}
	}
	if len(c.ActiveGoogleModels) == 0 {
		c.ActiveGoogleModels = []string{
			"gemini-3.1-flash-lite-preview",
			"gemini-2.5-flash",
			"gemini-3-flash-preview",
			"gemini-2.5-flash-lite",
			"gemma-4-31b-it",
			"gemma-4-26b-a4b-it",
			"gemma-4-E2B-it-text-only",
		}
	}
	if len(c.ActiveNativeModels) == 0 {
		c.ActiveNativeModels = []string{
			"ozgurpolat/gemma-4-E2B-it-text-only-GGUF:Q4_K_M",
			"Jackrong/Qwen3.5-4B-Claude-4.6-Opus-Reasoning-Distilled-v2-GGUF:Q5_K_M",
		}
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
		"groq":     true,
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

// GetGroqKeys retorna a lista de chaves API da Groq (split por vírgula).
func (c *Config) GetGroqKeys() []string {
	raw := strings.TrimSpace(c.GroqAPIKey)
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

// GetActiveGroqKey retorna a chave API ativa do pool Groq.
func (c *Config) GetActiveGroqKey() string {
	keys := c.GetGroqKeys()
	if len(keys) == 0 {
		return ""
	}
	idx := c.GroqKeyIndex % len(keys)
	return keys[idx]
}

// RotateGroqKey avança para a próxima chave no pool Groq e persiste.
func (c *Config) RotateGroqKey() string {
	keys := c.GetGroqKeys()
	if len(keys) <= 1 {
		return c.GetActiveGroqKey()
	}
	c.GroqKeyIndex = (c.GroqKeyIndex + 1) % len(keys)
	Save(*c)
	fmt.Printf("[GroqPool] 🔄 Rotação de chave: Agora usando chave #%d de %d\n", c.GroqKeyIndex+1, len(keys))
	return keys[c.GroqKeyIndex]
}

// GroqKeyCount retorna quantas chaves estão no pool Groq.
func (c *Config) GroqKeyCount() int {
	return len(c.GetGroqKeys())
}

func getConfigPath() string {
	configDir := filepath.Join(".lumaestro", "cache")
	// Garante que o diretorio exista
	_ = os.MkdirAll(configDir, 0755)

	// Migração automática da raiz para a subpasta cache
	oldPath := ".lumaestro.json"
	newPath := filepath.Join(configDir, ".lumaestro.json")
	
	if _, err := os.Stat(oldPath); err == nil {
		fmt.Printf("[Config] 🔄 Migrando arquivo de configuração para %s\n", newPath)
		_ = os.Rename(oldPath, newPath)
	}

	return newPath
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
		fmt.Printf("[Config] ❌ ERRO de Parse JSON em %s: %v\n", path, err)
		fmt.Printf("[Config] Dados brutos capturados (%d bytes)\n", len(data))
		return nil, err
	}
	
	// Garantir que arrays não sejam nulos para o frontend
	if cfg.Identities == nil {
		cfg.Identities = []Identity{}
	}
	if cfg.ActiveModelProviders == nil {
		cfg.ActiveModelProviders = []string{"gemini"}
	}
	if cfg.ActiveGroqModels == nil {
		cfg.ActiveGroqModels = []string{}
	}
	if cfg.ActiveGoogleModels == nil {
		cfg.ActiveGoogleModels = []string{}
	}
	if cfg.ExternalProjects == nil {
		cfg.ExternalProjects = []ProjectScan{}
	}

	cfg.NormalizeProviders()

	// 🕵️ Motor de Migração: GeminiAccount (Legado) -> Identity (Universal)
	if len(cfg.Identities) == 0 && len(cfg.GeminiAccounts) > 0 {
		fmt.Printf("[Config] 🔄 Iniciando migração de %d contas Gemini para Identidades Universais...\n", len(cfg.GeminiAccounts))
		for _, old := range cfg.GeminiAccounts {
			cfg.Identities = append(cfg.Identities, Identity{
				Provider:  "google",
				Name:      old.Name,
				HomeDir:   old.HomeDir,
				Active:    old.Active,
				Exhausted: old.Exhausted,
			})
		}
		cfg.GeminiAccounts = nil // Limpa o legado para não salvar de volta
		Save(cfg)
	}

	return &cfg, nil
}
