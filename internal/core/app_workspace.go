package core

import (
	"fmt"
	"os"
	"path/filepath"

	"Lumaestro/internal/config"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ============================================================
// 📂 WORKSPACE — Isolamento de Projeto para a IA
// ============================================================

// SetWorkspace define o diretório de trabalho ativo da IA.
// Todas as sessões ACP criadas após esta chamada usarão este diretório como CWD.
func (a *App) SetWorkspace(path string) error {
	if path == "" {
		// Limpar workspace = voltar para o diretório do Lumaestro
		a.executor.Workspace = ""
		if a.config != nil {
			a.config.ActiveWorkspace = ""
			config.Save(*a.config)
		}
		fmt.Println("[Cosmos] 🏛️ Retornando ao ponto zero do Universo.")
		a.emitEvent("workspace:changed", map[string]string{
			"path": "",
			"name": "Universo Lumaestro",
		})
		return nil
	}

	// Validar que o caminho existe e é um diretório
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("caminho não encontrado: %s", path)
	}
	if !info.IsDir() {
		return fmt.Errorf("o caminho não é um diretório: %s", path)
	}

	absPath, _ := filepath.Abs(path)
	a.executor.Workspace = absPath

	if a.config != nil {
		a.config.ActiveWorkspace = absPath
		config.Save(*a.config)
	}

	projectName := filepath.Base(absPath)
	fmt.Printf("[Cosmos] 🌌 Galáxia detectada: %s (%s)\n", projectName, absPath)

	// 🕸️ Atualiza o Crawler para apontar para o novo projeto (sinapse Workspace→Crawler)
	if a.crawler != nil {
		a.crawler.VaultPath = absPath
		fmt.Printf("[Workspace] 🕸️ Crawler reconfigurado para: %s\n", absPath)
	}

	a.emitEvent("workspace:changed", map[string]string{
		"path": absPath,
		"name": projectName,
	})

	return nil
}

// GetWorkspace retorna o workspace ativo atual.
func (a *App) GetWorkspace() map[string]string {
	ws := a.executor.Workspace
	if ws == "" {
		cwd, _ := os.Getwd()
		return map[string]string{
			"path": "",
			"name": filepath.Base(cwd) + " (Padrão)",
		}
	}
	return map[string]string{
		"path": ws,
		"name": filepath.Base(ws),
	}
}

// SelectWorkspace abre o seletor de pasta nativo do S.O. e define o workspace.
func (a *App) SelectWorkspace() (map[string]string, error) {
	dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "📂 Selecione o Projeto para a IA trabalhar",
	})
	if err != nil {
		return nil, err
	}
	if dir == "" {
		// Usuário cancelou
		return a.GetWorkspace(), nil
	}

	err = a.SetWorkspace(dir)
	if err != nil {
		return nil, err
	}

	return a.GetWorkspace(), nil
}

// ClearWorkspace limpa o workspace ativo (volta para o diretório do Lumaestro).
func (a *App) ClearWorkspace() map[string]string {
	a.SetWorkspace("")
	return a.GetWorkspace()
}

// UnlinkProject apenas remove o vínculo do projeto na configuração, sem apagar os arquivos físicos
func (a *App) UnlinkProject(projectPath string) error {
	absPath, _ := filepath.Abs(projectPath)

	// 1. Remover da lista de ExternalProjects
	var newList []config.ProjectScan
	modified := false
	for _, p := range a.config.ExternalProjects {
		pAbs, _ := filepath.Abs(p.Path)
		if pAbs != absPath {
			newList = append(newList, p)
		} else {
			modified = true
		}
	}
	
	if modified {
		a.config.ExternalProjects = newList
		config.Save(*a.config)
		fmt.Println("[Config] 🧹 Vínculo removido da lista de projetos externos.")
	}

	// 2. Se era o workspace ativo, volta para o ponto zero
	currActive, _ := filepath.Abs(a.config.ActiveWorkspace)
	if currActive == absPath {
		a.SetWorkspace("")
	}

	a.emitEvent("project:unlinked", map[string]string{
		"path": absPath,
	})

	return nil
}
