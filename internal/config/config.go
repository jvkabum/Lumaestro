package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// SecurityConfig define as permissões granulares para a IA
type SecurityConfig struct {
	AllowRead        bool     `json:"allow_read"`
	AllowWrite       bool     `json:"allow_write"`
	AllowCreate      bool     `json:"allow_create"`
	AllowDelete      bool     `json:"allow_delete"`
	AllowMove        bool     `json:"allow_move"`
	AllowRunCommands bool     `json:"allow_run_commands"`
	FullMachineAccess bool     `json:"full_machine_access"` // Se falso, restringe aos Workspaces
	Workspaces       []string `json:"workspaces"`          // Lista de pastas autorizadas (Whitelist)
}

// GeminiAccount representa um perfil de login do Gemini
type GeminiAccount struct {
	Name      string `json:"name"`
	HomeDir   string `json:"home_dir"` // Caminho da pasta de sessão (.gemini_accounts/nome)
	Active    bool   `json:"active"`
	Exhausted bool   `json:"exhausted"`
}

// Config representa as configurações globais do orquestrador.
type Config struct {
	ObsidianVaultPath string         `json:"obsidian_vault_path"`
	QdrantURL         string         `json:"qdrant_url"`
	QdrantAPIKey      string         `json:"qdrant_api_key"`
	GeminiAPIKey      string         `json:"gemini_api_key"` // Aceita múltiplas chaves separadas por vírgula
	UseGeminiAPIKey   bool           `json:"use_gemini_api_key"`
	GeminiKeyIndex    int            `json:"gemini_key_index"` // Índice da chave ativa no pool
	GeminiAccounts    []GeminiAccount `json:"gemini_accounts"` // 🌟 Nova lista de contas
	ClaudeAPIKey      string         `json:"claude_api_key"`
	UseClaudeAPIKey   bool           `json:"use_claude_api_key"`
	ActiveAgent       string         `json:"active_agent"`
	AutoStartAgents   []string       `json:"auto_start_agents"`
	AgentLanguage     string         `json:"agent_language"`
	MaxConcurrentAgents int          `json:"max_concurrent_agents"` // 🌟 Limite de Enxame (Swarm)
	GraphDepth        int            `json:"graph_depth"`         // Profundidade de navegação de links (padrão: 1)
	GraphNeighborLimit int            `json:"graph_neighbor_limit"` // Máximo de vizinhos por nó (padrão: 5)
	GraphContextLimit int            `json:"graph_context_limit"` // Limite de chars do contexto expandido (padrão: 4000)
	Security          SecurityConfig `json:"security"`
	
	// ⚡ Configurações do Motor Lightning (Aprendizado por Reforço)
	LightningEnabled   bool   `json:"lightning_enabled"`    // Ativa o rastreamento e aprendizado
	LightningProxyPort string `json:"lightning_proxy_port"` // Porta do proxy local (padrão: 8001)
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
	// Se existir um arquivo na raiz do projeto durante o Wails Dev, usar ele!
	if _, err := os.Stat("../../.lumaestro.json"); err == nil {
		return "../../.lumaestro.json"
	}
	// Migração automática de config.json legado
	if _, err := os.Stat("config.json"); err == nil && !strings.Contains(os.Getenv("WAILS_WASM"), "true") {
		_ = os.Rename("config.json", ".lumaestro.json")
	}
	return ".lumaestro.json"
}

// Save armazena as configurações em um arquivo JSON.
func Save(cfg Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(getConfigPath(), data, 0644)
}

// Load recupera as configurações do arquivo JSON.
func Load() (*Config, error) {
	path := getConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Printf("[Config] Aviso: %s não encontrado no diretorio (%v)\n", path, err)
		return &Config{}, nil // Retorna config vazia se o arquivo não existir
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		fmt.Printf("[Config] ERRO CRITICO Parse JSON: %v\n", err)
		return nil, err
	}
	return &cfg, nil
}
