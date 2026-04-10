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

// AddGeminiAccount adiciona uma nova conta e prepara seu diretório de sessão
func (a *App) AddGeminiAccount(name string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	cwd, _ := os.Getwd()
	accountPath := filepath.Join(cwd, ".gemini_accounts", name)

	// Cria o diretório de sessão se não existir
	if err := os.MkdirAll(accountPath, 0755); err != nil {
		return fmt.Errorf("falha ao criar pasta de conta: %w", err)
	}

	// Verifica se já existe na config
	for i := range cfg.GeminiAccounts {
		if cfg.GeminiAccounts[i].Name == name {
			cfg.GeminiAccounts[i].HomeDir = accountPath
			return config.Save(*cfg)
		}
	}

	cfg.GeminiAccounts = append(cfg.GeminiAccounts, config.GeminiAccount{
		Name:    name,
		HomeDir: accountPath,
		Active:  false,
	})

	return config.Save(*cfg)
}

// LoginGeminiAccount abre um terminal para realizar o login OAuth em uma conta específica
func (a *App) LoginGeminiAccount(name string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	var targetDir string
	for _, acc := range cfg.GeminiAccounts {
		if acc.Name == name {
			targetDir = acc.HomeDir
			break
		}
	}

	if targetDir == "" {
		return fmt.Errorf("conta '%s' não encontrada ou sem diretório configurado", name)
	}

	// Comando para abrir o terminal com GEMINI_CLI_HOME isolado
	binaryPath := "gemini"
	if _, err := exec.LookPath("gemini"); err != nil {
		cwd, _ := os.Getwd()
		binaryPath = filepath.Join(cwd, "node_modules", ".bin", "gemini.cmd")
	}

	// Script para o PowerShell forçar o ambiente de sessão desta conta.
	// NOTA: Removemos NO_BROWSER para permitir o fluxo visual de login no navegador.
	script := fmt.Sprintf(`$env:GEMINI_CLI_HOME='%s'; & '%s'`, targetDir, binaryPath)

	fmt.Printf("[Maestro] 🔑 Iniciando fluxo de Login OAuth para: %s\n", name)
	return exec.Command("cmd", "/c", "start", "powershell", "-NoExit", "-Command", script).Run()
}

// SwitchGeminiAccount alterna a conta ativa do Gemini e reinicia a sessão
func (a *App) SwitchGeminiAccount(name string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	found := false
	for i := range cfg.GeminiAccounts {
		if cfg.GeminiAccounts[i].Name == name {
			cfg.GeminiAccounts[i].Active = true
			found = true
		} else {
			cfg.GeminiAccounts[i].Active = false
		}
	}

	if !found {
		return fmt.Errorf("conta '%s' não encontrada", name)
	}

	if err := config.Save(*cfg); err != nil {
		return err
	}

	fmt.Printf("[Maestro] 🔄 Trocando para sessão de login: %s\n", name)
	return a.StartAgentSession("gemini")
}
