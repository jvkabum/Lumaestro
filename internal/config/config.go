package config

import (
	"encoding/json"
	"os"
)

// Config representa as configurações globais do orquestrador.
type Config struct {
	ObsidianVaultPath string `json:"obsidian_vault_path"`
	QdrantURL         string `json:"qdrant_url"`
	GeminiAPIKey       string `json:"gemini_api_key"`
	UseGeminiAPIKey    bool   `json:"use_gemini_api_key"`
	ClaudeAPIKey       string `json:"claude_api_key"`
	UseClaudeAPIKey    bool   `json:"use_claude_api_key"`
	ActiveAgent       string `json:"active_agent"` // "gemini" ou "claude"
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
