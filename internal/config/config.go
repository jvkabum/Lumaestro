package config

import (
	"encoding/json"
	"os"
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

// Config representa as configurações globais do orquestrador.
type Config struct {
	ObsidianVaultPath string         `json:"obsidian_vault_path"`
	QdrantURL         string         `json:"qdrant_url"`
	GeminiAPIKey      string         `json:"gemini_api_key"`
	UseGeminiAPIKey   bool           `json:"use_gemini_api_key"`
	ClaudeAPIKey      string         `json:"claude_api_key"`
	UseClaudeAPIKey   bool           `json:"use_claude_api_key"`
	ActiveAgent       string         `json:"active_agent"`
	AutoStartAgents   []string       `json:"auto_start_agents"`
	AgentLanguage     string         `json:"agent_language"`
	GraphDepth        int            `json:"graph_depth"`         // Profundidade de navegação de links (padrão: 1)
	GraphContextLimit int            `json:"graph_context_limit"` // Limite de chars do contexto expandido (padrão: 4000)
	Security          SecurityConfig `json:"security"`
}

const configPath = "config.json"

// Save armazena as configurações em um arquivo JSON.
func Save(cfg Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, data, 0644)
}

// Load recupera as configurações do arquivo JSON.
func Load() (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return &Config{}, nil // Retorna config vazia se o arquivo não existir
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
