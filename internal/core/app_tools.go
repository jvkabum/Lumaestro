package core

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"Lumaestro/internal/config"
)

// GetToolsStatus verifica se as IAs CLIs estão instaladas no PATH e os status de autenticação
func (a *App) GetToolsStatus() map[string]bool {
	// Sincroniza o PATH para detectar ferramentas recém-instaladas sem restart
	a.installer.SyncPath()
	
	return map[string]bool{
		"node":        a.installer.CheckStatus("node"),
		"gemini":      a.installer.CheckStatus("gemini"),
		"antigravity": a.installer.CheckStatus("antigravity"),
		"agy":         a.installer.CheckStatus("agy"),
		"claude":      a.installer.CheckStatus("claude"),
		"obsidian":    a.installer.CheckStatus("obsidian"),
		"claude_auth":      a.installer.CheckClaudeAuth(),
		"gemini_auth":      a.installer.CheckGeminiAuth(),
		"antigravity_auth": a.installer.CheckGeminiAuth(),
		"agy_auth":         a.installer.CheckGeminiAuth(),
		"groq":             a.config != nil && a.config.GroqAPIKey != "",
	}
}

// InstallTool dispara a instalação via CLI oficial
func (a *App) InstallTool(name string) string {
	var err error
	switch name {
	case "node":
		err = a.installer.InstallNode()
	case "gemini":
		err = a.installer.InstallGemini()
	case "antigravity", "agy":
		err = a.installer.InstallAntigravity()
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

	// Sincroniza o ambiente imediatamente após a instalação
	a.installer.SyncPath()

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

// MCPServerDefinition representa as configurações de um servidor MCP no mcp_config.json
type MCPServerDefinition struct {
	Command   string            `json:"command,omitempty"`
	Args      []string          `json:"args,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	ServerURL string            `json:"serverUrl,omitempty"`
}

// MCPConfigFile representa o schema oficial do mcp_config.json
type MCPConfigFile struct {
	MCPServers map[string]MCPServerDefinition `json:"mcpServers"`
}

// MCPServerItemInfo representa o servidor MCP formatado para a UI e inspeção
type MCPServerItemInfo struct {
	Name      string   `json:"name"`
	Type      string   `json:"type"` // "stdio" ou "sse"
	Command   string   `json:"command,omitempty"`
	Args      []string `json:"args,omitempty"`
	ServerURL string   `json:"serverUrl,omitempty"`
	Scope     string   `json:"scope"` // "global" ou "workspace"
	FilePath  string   `json:"filePath"`
}

// GetMCPServersList lê os servidores configurados tanto no nível de workspace quanto global.
func (a *App) GetMCPServersList() ([]MCPServerItemInfo, error) {
	var results []MCPServerItemInfo
	seen := make(map[string]bool)

	// 1. Workspace MCP config ({workspace}/.agents/mcp_config.json ou .gemini/mcp_config.json)
	ws := a.getActiveWorkspace()
	if ws != "" && ws != "." {
		wsCandidates := []string{
			filepath.Join(ws, ".agents", "mcp_config.json"),
			filepath.Join(ws, ".gemini", "mcp_config.json"),
		}
		for _, path := range wsCandidates {
			if data, err := os.ReadFile(path); err == nil {
				var file MCPConfigFile
				if errJson := json.Unmarshal(data, &file); errJson == nil && file.MCPServers != nil {
					for name, def := range file.MCPServers {
						if !seen[name] {
							seen[name] = true
							srvType := "stdio"
							if def.ServerURL != "" {
								srvType = "sse"
							}
							results = append(results, MCPServerItemInfo{
								Name:      name,
								Type:      srvType,
								Command:   def.Command,
								Args:      def.Args,
								ServerURL: def.ServerURL,
								Scope:     "workspace",
								FilePath:  path,
							})
						}
					}
					break
				}
			}
		}
	}

	// 2. Global MCP config (~/.gemini/config/mcp_config.json ou ~/.gemini/antigravity-cli/mcp_config.json)
	userHome, errHome := os.UserHomeDir()
	if errHome == nil {
		globalCandidates := []string{
			filepath.Join(userHome, ".gemini", "config", "mcp_config.json"),
			filepath.Join(userHome, ".gemini", "antigravity", "mcp_config.json"),
			filepath.Join(userHome, ".gemini", "antigravity-cli", "mcp_config.json"),
		}
		for _, path := range globalCandidates {
			if data, err := os.ReadFile(path); err == nil {
				var file MCPConfigFile
				if errJson := json.Unmarshal(data, &file); errJson == nil && file.MCPServers != nil {
					for name, def := range file.MCPServers {
						if !seen[name] {
							seen[name] = true
							srvType := "stdio"
							if def.ServerURL != "" {
								srvType = "sse"
							}
							results = append(results, MCPServerItemInfo{
								Name:      name,
								Type:      srvType,
								Command:   def.Command,
								Args:      def.Args,
								ServerURL: def.ServerURL,
								Scope:     "global",
								FilePath:  path,
							})
						}
					}
					break
				}
			}
		}
	}

	return results, nil
}

// SaveMCPServerConfig adiciona ou atualiza um servidor MCP seguindo a especificação do Antigravity CLI e 2.0.
func (a *App) SaveMCPServerConfig(name string, commandOrUrl string, isGlobal bool) (string, error) {
	name = strings.TrimSpace(name)
	commandOrUrl = strings.TrimSpace(commandOrUrl)
	if name == "" || commandOrUrl == "" {
		return "", fmt.Errorf("nome e comando/URL são obrigatórios")
	}

	var targetPath string
	if isGlobal {
		userHome, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("não foi possível identificar diretório do usuário: %w", err)
		}
		targetDir := filepath.Join(userHome, ".gemini", "config")
		_ = os.MkdirAll(targetDir, 0755)
		targetPath = filepath.Join(targetDir, "mcp_config.json")
	} else {
		ws := a.getActiveWorkspace()
		if ws == "" || ws == "." {
			return "", fmt.Errorf("nenhum workspace ativo selecionado para configuração local")
		}
		targetDir := filepath.Join(ws, ".agents")
		_ = os.MkdirAll(targetDir, 0755)
		targetPath = filepath.Join(targetDir, "mcp_config.json")
	}

	// Carrega arquivo existente ou inicia novo
	var file MCPConfigFile
	file.MCPServers = make(map[string]MCPServerDefinition)
	if data, err := os.ReadFile(targetPath); err == nil {
		_ = json.Unmarshal(data, &file)
		if file.MCPServers == nil {
			file.MCPServers = make(map[string]MCPServerDefinition)
		}
	}

	// Define se é SSE (URL) ou Stdio (Comando + Argumentos)
	var def MCPServerDefinition
	if strings.HasPrefix(commandOrUrl, "http://") || strings.HasPrefix(commandOrUrl, "https://") {
		def.ServerURL = commandOrUrl
	} else {
		fields := strings.Fields(commandOrUrl)
		if len(fields) > 0 {
			def.Command = fields[0]
			if len(fields) > 1 {
				def.Args = fields[1:]
			}
		}
	}

	file.MCPServers[name] = def

	encoded, errEnc := json.MarshalIndent(file, "", "  ")
	if errEnc != nil {
		return "", fmt.Errorf("falha ao serializar mcp_config.json: %w", errEnc)
	}

	if errWrite := os.WriteFile(targetPath, encoded, 0644); errWrite != nil {
		return "", fmt.Errorf("falha ao salvar mcp_config.json: %w", errWrite)
	}

	// Opcional: tenta registrar também no gemini CLI caso instalado
	go func() {
		_ = exec.Command("cmd", "/c", "gemini", "mcp", "add", name, commandOrUrl).Run()
	}()

	return fmt.Sprintf("Servidor MCP '%s' configurado com sucesso em %s", name, targetPath), nil
}

// RemoveMCPServerConfig remove um servidor MCP do arquivo correspondente.
func (a *App) RemoveMCPServerConfig(name string, isGlobal bool) (string, error) {
	name = strings.TrimSpace(name)
	var targetPath string
	if isGlobal {
		userHome, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		targetPath = filepath.Join(userHome, ".gemini", "config", "mcp_config.json")
	} else {
		ws := a.getActiveWorkspace()
		targetPath = filepath.Join(ws, ".agents", "mcp_config.json")
	}

	data, err := os.ReadFile(targetPath)
	if err != nil {
		return "", fmt.Errorf("arquivo de configuração não encontrado: %w", err)
	}

	var file MCPConfigFile
	if errJson := json.Unmarshal(data, &file); errJson != nil {
		return "", fmt.Errorf("erro ao ler arquivo de configuração: %w", errJson)
	}

	if file.MCPServers != nil {
		delete(file.MCPServers, name)
	}

	encoded, errEnc := json.MarshalIndent(file, "", "  ")
	if errEnc != nil {
		return "", errEnc
	}

	if errWrite := os.WriteFile(targetPath, encoded, 0644); errWrite != nil {
		return "", errWrite
	}

	return fmt.Sprintf("Servidor MCP '%s' removido com sucesso de %s", name, targetPath), nil
}

// AddMCPServer instala ou atualiza um servidor MCP (compatível com UI atual e CLI oficial)
func (a *App) AddMCPServer(name string, command string) string {
	msg, err := a.SaveMCPServerConfig(name, command, true)
	if err != nil {
		return "Erro ao configurar MCP: " + err.Error()
	}
	return msg
}

// ListMCPServers retorna a lista de MCPs instalados formatada
func (a *App) ListMCPServers() string {
	servers, err := a.GetMCPServersList()
	if err == nil && len(servers) > 0 {
		var sb strings.Builder
		sb.WriteString("Servidores MCP Detectados (Antigravity & Workspace):\n\n")
		for _, s := range servers {
			sb.WriteString(fmt.Sprintf("• [%s] %s (%s)\n", strings.ToUpper(s.Scope), s.Name, s.Type))
			if s.ServerURL != "" {
				sb.WriteString(fmt.Sprintf("  URL: %s\n", s.ServerURL))
			} else {
				argsStr := strings.Join(s.Args, " ")
				if argsStr != "" {
					sb.WriteString(fmt.Sprintf("  Exec: %s %s\n", s.Command, argsStr))
				} else {
					sb.WriteString(fmt.Sprintf("  Exec: %s\n", s.Command))
				}
			}
			sb.WriteString(fmt.Sprintf("  Arquivo: %s\n\n", s.FilePath))
		}
		return sb.String()
	}

	// Fallback para CLI
	cmd := exec.Command("cmd", "/c", "gemini", "mcp", "list")
	output, _ := cmd.CombinedOutput()
	if len(output) > 0 {
		return string(output)
	}
	return "Nenhum servidor MCP configurado no momento."
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
