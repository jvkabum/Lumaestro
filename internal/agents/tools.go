package agents

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Tool representa uma função executável por um agente.
type Tool struct {
	Name        string
	Description string
	Function    func(args map[string]interface{}) (string, error)
}

// ToolRegistry mantém as ferramentas disponíveis.
type ToolRegistry struct {
	Tools map[string]Tool
}

// NewToolRegistry inicializa a biblioteca de ferramentas do Lumaestro.
func NewToolRegistry() *ToolRegistry {
	r := &ToolRegistry{Tools: make(map[string]Tool)}

	// 1. Ferramenta de Listagem de Vault
	r.Tools["ListVaultFiles"] = Tool{
		Name:        "ListVaultFiles",
		Description: "Lista todos os arquivos do Obsidian Vault.",
		Function: func(args map[string]interface{}) (string, error) {
			vaultPath, _ := args["path"].(string)
			var files []string
			filepath.Walk(vaultPath, func(p string, info os.FileInfo, err error) error {
				if !info.IsDir() {
					files = append(files, info.Name())
				}
				return nil
			})
			return fmt.Sprintf("Arquivos encontrados: %v", files), nil
		},
	}

	// 2. Ferramenta de Nota Diária (Obsidian CLI)
	r.Tools["AppendDaily"] = Tool{
		Name:        "AppendDaily",
		Description: "Adiciona conteúdo à nota diária de hoje no Obsidian.",
		Function: func(args map[string]interface{}) (string, error) {
			content, _ := args["content"].(string)
			cmd := exec.Command("obsidian", "daily:append", "content="+content)
			err := cmd.Run()
			if err != nil {
				return "", fmt.Errorf("falha ao anexar à nota diária: %w (verifique se a CLI está ativa)", err)
			}
			return "Conteúdo anexado com sucesso!", nil
		},
	}

	// 3. Ferramenta de Busca Global (Obsidian CLI)
	r.Tools["ObsidianSearch"] = Tool{
		Name:        "ObsidianSearch",
		Description: "Abre a busca do Obsidian para uma consulta específica.",
		Function: func(args map[string]interface{}) (string, error) {
			query, _ := args["query"].(string)
			cmd := exec.Command("obsidian", "search", "query="+query)
			err := cmd.Run()
			if err != nil {
				return "", fmt.Errorf("falha na busca do Obsidian: %w", err)
			}
			return "Busca iniciada no Obsidian.", nil
		},
	}

	return r
}
