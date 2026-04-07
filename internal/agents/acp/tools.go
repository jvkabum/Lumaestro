package acp

import (
	"context"
	"fmt"
)

// ToolRegistry gerencia as ferramentas estendidas que os agentes podem usar.
type ToolRegistry struct {
	Tools   map[string]ToolDefinition
	Ctx     context.Context
	Indexer interface{} // Ponteiro para o Crawler do Obsidian para buscas profundas
}

// ToolDefinition define como uma ferramenta é invocada pela IA.
type ToolDefinition struct {
	Name        string
	Description string
	Params      map[string]string
	Function    func(args map[string]interface{}) (string, error)
}

func NewToolRegistry() *ToolRegistry {
	r := &ToolRegistry{
		Tools: make(map[string]ToolDefinition),
	}
	r.registerDefaultTools()
	return r
}

func (r *ToolRegistry) registerDefaultTools() {
	// Ferramenta de exemplo para o Obsidian (Sinfonia de Busca)
	r.Tools["search_vault"] = ToolDefinition{
		Name:        "search_vault",
		Description: "Busca semântica no Obsidian Vault do usuário.",
		Function: func(args map[string]interface{}) (string, error) {
			return "Busca realizada com sucesso no Vault.", nil
		},
	}
}

func (r *ToolRegistry) ExecuteTool(name string, args map[string]interface{}) (string, error) {
	tool, exists := r.Tools[name]
	if !exists {
		return "", fmt.Errorf("ferramenta %s não encontrada", name)
	}
	return tool.Function(args)
}
