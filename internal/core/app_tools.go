package core

import (
	"Lumaestro/internal/config"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GetToolsStatus verifica se as IAs CLIs estão instaladas no PATH e os status de autenticação
func (a *App) GetToolsStatus() map[string]bool {
	// Reduzimos o ruído no log para esse porque ele é feito a cada refresh
	return map[string]bool{
		"gemini":      a.installer.CheckStatus("gemini"),
		"claude":      a.installer.CheckStatus("claude"),
		"obsidian":    a.installer.CheckStatus("obsidian"),
		"claude_auth": a.installer.CheckClaudeAuth(),
		"gemini_auth": a.installer.CheckGeminiAuth(),
		"groq":        a.config != nil && a.config.GroqAPIKey != "",
	}
}

// InstallTool dispara a instalação via CLI oficial
func (a *App) InstallTool(name string) string {
	var err error
	switch name {
	case "gemini":
		err = a.installer.InstallGemini()
	case "claude":
		err = a.installer.InstallClaude()
	case "obsidian":
		err = a.installer.InstallObsidian()
	default:
		return "Ferramenta desconhecida."
	}

	if err != nil {
		return "Erro na instalação: " + err.Error()
	}
	return "Instalação de " + name + " concluída com sucesso!"
}

// FixEnvironment tenta corrigir caminhos de ambiente manualmente
func (a *App) FixEnvironment() string {
	err := a.installer.FixClaudePath()
	if err != nil {
		return "Erro ao corrigir ambiente: " + err.Error()
	}
	return "Ambiente corrigido com sucesso! Reinicie o aplicativo."
}

// GetConfig retorna as configurações atuais para o Vue
func (a *App) GetConfig() *config.Config {
	cfg, _ := config.Load()
	fmt.Printf("[BACKEND-UI] GetConfig disparado pelo frontend. Enviando URL Qdrant: %s\n", cfg.QdrantURL)
	return cfg
}

// SaveConfig persiste as novas configurações no config.json
func (a *App) SaveConfig(cfg config.Config) string {
	err := config.Save(cfg)
	if err != nil {
		return "Erro ao salvar: " + err.Error()
	}

	// Anula serviços obsoletos e reinicializa com a nova configuração.
	a.config = &cfg
	a.resetServicesForReload()
	go a.initServices()
	return "Configurações salvas e serviços reiniciados!"
}

// SetupTool abre um terminal externo - Legado.
func (a *App) SetupTool(name string) string {
	err := a.installer.SetupTool(name)
	if err != nil {
		return "Erro ao abrir terminal: " + err.Error()
	}
	return "Janela de configuração aberta!"
}

// GetProjectDoc retorna um arquivo de documentação do projeto.
func (a *App) GetProjectDoc(name string) (string, error) {
	if !strings.HasSuffix(name, ".md") {
		name += ".md"
	}
	fmt.Printf("[App] Lendo documentação: %s\n", name)
	path := filepath.Join(".", "docs", name)
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("erro ao ler %s: %v", name, err)
	}

	return string(content), nil
}

// OpenFileInEditor abre o arquivo fonte usando o handler padrão do SO.
func (a *App) OpenFileInEditor(path string) error {
	fmt.Printf("[App] Abrindo arquivo na fonte: %s\n", path)
	// No Windows usamos 'cmd /c start'
	cmd := exec.Command("cmd", "/c", "start", "", path)
	return cmd.Run()
}

// GenerateGeminiMD cria um arquivo base GEMINI.md no diretório atual
func (a *App) GenerateGeminiMD() string {
	content := `# Project Instructions

Você agora está sendo orquestrado pelo Lumaestro (Modo ACP).

- **Manejo de Arquivos**: O Backend ditará suas permissões. Se receber "Acesso Negado", pergunte ao usuário.
- **Autonomia Limitada**: Só prossiga ativamente se a sessão permitir.

`
	err := os.WriteFile("GEMINI.md", []byte(content), 0644)
	if err != nil {
		return "Erro ao gerar arquivo de contexto: " + err.Error()
	}
	return "Contexto GEMINI.md gerado com sucesso no diretório atual!"
}

// AddMCPServer instala um novo servidor MCP na CLI local
func (a *App) AddMCPServer(name string, command string) string {
	cmd := exec.Command("cmd", "/c", "gemini", "mcp", "add", name, command)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return fmt.Sprintf("Erro ao adicionar MCP: %s\nOutput: %s", err.Error(), string(output))
	}
	return fmt.Sprintf("MCP '%s' adicionado com sucesso!\n%s", name, string(output))
}

// ListMCPServers retorna a lista de MCPs instalados
func (a *App) ListMCPServers() string {
	cmd := exec.Command("cmd", "/c", "gemini", "mcp", "list")
	output, err := cmd.CombinedOutput()

	if err != nil {
		return fmt.Sprintf("Erro ao listar MCPs: %s\nOutput: %s", err.Error(), string(output))
	}
	return string(output)
}

// AddIdentity adiciona uma nova identidade para o provedor especificado
func (a *App) AddIdentity(provider, name string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	homeDir := ""
	if provider == "google" {
		cwd, _ := os.Getwd()
		homeDir = filepath.Join(cwd, ".gemini_accounts", name)
		// Cria o diretório de sessão se não existir
		if err := os.MkdirAll(homeDir, 0755); err != nil {
			return fmt.Errorf("falha ao criar pasta de conta: %w", err)
		}
	}

	// Verifica se já existe na config
	for i := range cfg.Identities {
		if cfg.Identities[i].Provider == provider && cfg.Identities[i].Name == name {
			cfg.Identities[i].HomeDir = homeDir
			return config.Save(*cfg)
		}
	}

	cfg.Identities = append(cfg.Identities, config.Identity{
		Provider: provider,
		Name:     name,
		HomeDir:  homeDir,
		Active:   false,
	})

	return config.Save(*cfg)
}

// LoginIdentity abre um terminal para realizar o login (específico para provedores com OAuth/CLI)
func (a *App) LoginIdentity(provider, name string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	var targetDir string
	for _, id := range cfg.Identities {
		if id.Provider == provider && id.Name == name {
			targetDir = id.HomeDir
			break
		}
	}

	if provider == "google" {
		if targetDir == "" {
			return fmt.Errorf("identidade '%s' no Google não encontrada ou sem diretório configurado", name)
		}

		// Comando para abrir o terminal com GEMINI_CLI_HOME isolado
		binaryPath := "gemini"
		if _, err := exec.LookPath("gemini"); err != nil {
			cwd, _ := os.Getwd()
			binaryPath = filepath.Join(cwd, "node_modules", ".bin", "gemini.cmd")
		}

		script := fmt.Sprintf(`$env:GEMINI_CLI_HOME='%s'; & '%s'`, targetDir, binaryPath)
		fmt.Printf("[Maestro] 🔑 Iniciando fluxo de Login OAuth para: %s (%s)\n", name, provider)
		return exec.Command("cmd", "/c", "start", "powershell", "-NoExit", "-Command", script).Run()
	}

	return fmt.Errorf("o provedor '%s' não exige login via terminal (use chaves de API)", provider)
}

// SwitchIdentity alterna a identidade ativa de um provedor e reinicia a sessão (se necessário)
func (a *App) SwitchIdentity(provider, name string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	found := false
	for i := range cfg.Identities {
		if cfg.Identities[i].Provider == provider {
			if cfg.Identities[i].Name == name {
				cfg.Identities[i].Active = true
				found = true
			} else {
				cfg.Identities[i].Active = false
			}
		}
	}

	if !found {
		return fmt.Errorf("identidade '%s' para o provedor '%s' não encontrada", name, provider)
	}

	if err := config.Save(*cfg); err != nil {
		return err
	}

	fmt.Printf("[Maestro] 🔄 Trocando para identidade: %s (%s)\n", name, provider)

	// Se for Google, precisamos reiniciar a sessão do agente CLI
	if provider == "google" {
		return a.StartAgentSession("gemini")
	}
	return nil
}

// RemoveIdentity exclui uma identidade do registro
func (a *App) RemoveIdentity(provider, name string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	newIdentities := make([]config.Identity, 0, len(cfg.Identities))
	for _, id := range cfg.Identities {
		if id.Provider == provider && id.Name == name {
			// Se for Google, poderíamos opcionalmente apagar a pasta HomeDir, 
			// mas por segurança vamos apenas remover o registro.
			continue
		}
		newIdentities = append(newIdentities, id)
	}

	cfg.Identities = newIdentities
	return config.Save(*cfg)
}
