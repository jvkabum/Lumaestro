package core

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"Lumaestro/internal/agents/acp"
	"Lumaestro/internal/config"
)

// CodeSearchMatch representa uma linha casada com o seu contexto.
type CodeSearchMatch struct {
	LineNumber    int    `json:"lineNumber"`
	ContextBefore string `json:"contextBefore"`
	LineContent   string `json:"lineContent"`
	ContextAfter  string `json:"contextAfter"`
}

// CodeSearchFile agrupa todas as correspondencias de um determinado arquivo.
type CodeSearchFile struct {
	Path    string            `json:"path"`
	Matches []CodeSearchMatch `json:"matches"`
}

// CodeSearchResult contem o resultado completo de uma busca no workspace.
type CodeSearchResult struct {
	Query      string           `json:"query"`
	PathFilter string           `json:"pathFilter"`
	TotalCount int              `json:"totalCount"`
	Files      []CodeSearchFile `json:"files"`
}

// WorkspaceDiffResult traz o resumo do git status e o diff unificado.
type WorkspaceDiffResult struct {
	StatusText string   `json:"statusText"`
	Modified   []string `json:"modified"`
	Untracked  []string `json:"untracked"`
	DiffText   string   `json:"diffText"`
}

func (a *App) getActiveWorkspace() string {
	ws := ""
	if a.executor != nil {
		ws = a.executor.Workspace
	}
	if ws == "" && a.config != nil {
		ws = a.config.ActiveWorkspace
	}
	return ws
}

// SlashCommandItem representa um item de comando slash com metadados para o autocomplete do chat.
type SlashCommandItem struct {
	Command  string `json:"command"`
	Icon     string `json:"icon"`
	Type     string `json:"type"` // "builtin", "skill", "agent"
	Desc     string `json:"desc"`
	Template string `json:"template,omitempty"`
	Action   string `json:"action,omitempty"`
	Scope    string `json:"scope,omitempty"`
}

// GetAvailableSlashCommands reúne comandos built-in, agentes customizados e skills dinâmicas do Antigravity.
func (a *App) GetAvailableSlashCommands() ([]SlashCommandItem, error) {
	ws := a.getActiveWorkspace()

	// 1. Comandos nativos e essenciais do Lumaestro / Antigravity CLI
	commands := []SlashCommandItem{
		{
			Command:  "/boost",
			Icon:     "🚀",
			Type:     "builtin",
			Desc:     "Raciocínio profundo estruturado em 3 fases (Decomposição, Solução, Auto-correção)",
			Template: "/boost ",
			Scope:    "builtin",
		},
		{
			Command:  "/teamwork-preview",
			Icon:     "🤝",
			Type:     "builtin",
			Desc:     "Enxame multi-agente colaborativo com portas de validação",
			Template: "/teamwork-preview ",
			Scope:    "builtin",
		},
		{
			Command: "/agents",
			Icon:    "🐝",
			Type:    "builtin",
			Desc:    "Abrir Agent Manager (monitorar subagentes e descobrir agentes customizados)",
			Action:  "agents",
			Scope:   "builtin",
		},
		{
			Command: "/codesearch",
			Icon:    "🔎",
			Type:    "builtin",
			Desc:    "Pesquisa avançada de símbolos e código no workspace (ou /cs)",
			Action:  "codesearch",
			Scope:   "builtin",
		},
		{
			Command: "/diff",
			Icon:    "📑",
			Type:    "builtin",
			Desc:    "Visualizar status do Git e diff unificado de alterações",
			Action:  "diff",
			Scope:   "builtin",
		},
		{
			Command: "/artifact",
			Icon:    "📜",
			Type:    "builtin",
			Desc:    "Revisão e co-steering de artefatos de código (atalho Ctrl+R)",
			Action:  "artifact",
			Scope:   "builtin",
		},
		{
			Command: "/fork",
			Icon:    "🌿",
			Type:    "builtin",
			Desc:    "Ramificar sessão de chat atual em uma nova trilha independente",
			Action:  "fork",
			Scope:   "builtin",
		},
		{
			Command:  "/rename",
			Icon:     "✏️",
			Type:     "builtin",
			Desc:     "Renomear o título da sessão ativa",
			Template: "/rename ",
			Scope:    "builtin",
		},
		{
			Command: "/permissions",
			Icon:    "🛡️",
			Type:    "builtin",
			Desc:    "Inspecionar políticas de segurança, sandbox e ferramentas",
			Action:  "permissions",
			Scope:   "builtin",
		},
		{
			Command: "/plan",
			Icon:    "🔒",
			Type:    "builtin",
			Desc:    "Alternar Modo de Execução (default / accept-edits / plan)",
			Action:  "plan",
			Scope:   "builtin",
		},
		{
			Command: "/voice",
			Icon:    "🎙️",
			Type:    "builtin",
			Desc:    "Alternar ditado por voz em tempo real (atalho F5)",
			Action:  "voice",
			Scope:   "builtin",
		},
		{
			Command: "/usage",
			Icon:    "📊",
			Type:    "builtin",
			Desc:    "Exibir telemetria de consumo de tokens, contexto e cotas",
			Action:  "usage",
			Scope:   "builtin",
		},
		{
			Command: "/clear",
			Icon:    "🧹",
			Type:    "builtin",
			Desc:    "Limpar mensagens da janela de chat atual",
			Action:  "clear",
			Scope:   "builtin",
		},
	}

	seen := make(map[string]bool)
	for _, c := range commands {
		seen[strings.ToLower(c.Command)] = true
	}

	// 2. Agentes Customizados (.agents/agents/ ou ~/.gemini/config/agents/)
	if customAgents, err := acp.DiscoverCustomAgents(ws); err == nil {
		for _, agent := range customAgents {
			cmd := "/" + agent.Name
			if !seen[strings.ToLower(cmd)] {
				seen[strings.ToLower(cmd)] = true
				commands = append(commands, SlashCommandItem{
					Command:  cmd,
					Icon:     "🤖",
					Type:     "agent",
					Desc:     agent.Description,
					Template: cmd + " ",
					Scope:    agent.Scope,
				})
			}
		}
	}

	// 3. Habilidades Dinâmicas (.agents/skills/, ~/.gemini/config/skills/, etc.)
	if skills, err := acp.DiscoverSkills(ws); err == nil {
		for _, skill := range skills {
			if !seen[strings.ToLower(skill.Command)] {
				seen[strings.ToLower(skill.Command)] = true
				commands = append(commands, SlashCommandItem{
					Command:  skill.Command,
					Icon:     "🛠️",
					Type:     "skill",
					Desc:     skill.Description,
					Template: skill.Command + " ",
					Scope:    skill.Scope,
				})
			}
		}
	}

	return commands, nil
}

// GetDiscoveredSkills retorna a lista de todas as habilidades escaneadas.
func (a *App) GetDiscoveredSkills() ([]acp.SkillItem, error) {
	ws := a.getActiveWorkspace()
	return acp.DiscoverSkills(ws)
}

// GetCustomAgents lista os agentes customizados descobertos nos diretorios locais e globais.
func (a *App) GetCustomAgents() ([]acp.CustomAgent, error) {
	ws := a.getActiveWorkspace()
	return acp.DiscoverCustomAgents(ws)
}

// KillSubagent cancela e encerra imediatamente um subagente em execucao.
func (a *App) KillSubagent(sessionID string) error {
	if a.executor == nil {
		return fmt.Errorf("executor nao inicializado")
	}

	err := a.executor.StopSession(sessionID)
	if err == nil {
		a.emitEvent("agent:subagent_stopped", map[string]string{
			"sessionID": sessionID,
			"status":    "killed",
		})
	}
	return err
}

// RunCodeSearch realiza uma busca interativa de codigo no workspace atual conforme o /codesearch.
func (a *App) RunCodeSearch(query string, pathFilter string, isLiteral bool) (*CodeSearchResult, error) {
	ws := ""
	if a.executor != nil {
		ws = a.executor.Workspace
	}
	if ws == "" && a.config != nil {
		ws = a.config.ActiveWorkspace
	}
	if ws == "" || ws == "." {
		return &CodeSearchResult{Query: query, Files: []CodeSearchFile{}}, nil
	}

	var re *regexp.Regexp
	var errRe error
	if !isLiteral {
		flags := "(?i)"
		// Se contem letra maiuscula, respeita smart-case
		if strings.ToLower(query) != query {
			flags = ""
		}
		re, errRe = regexp.Compile(flags + query)
		if errRe != nil {
			// Fallback para literal caso a regex seja invalida
			isLiteral = true
		}
	}

	result := &CodeSearchResult{
		Query:      query,
		PathFilter: pathFilter,
		Files:      []CodeSearchFile{},
	}

	ignoredDirs := map[string]bool{
		".git":         true,
		"node_modules": true,
		".lumaestro":   true,
		"dist":         true,
		".gemini":      true,
		"deps":         true,
	}

	errWalk := filepath.WalkDir(ws, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if ignoredDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		rel, _ := filepath.Rel(ws, p)
		rel = filepath.ToSlash(rel)

		// Filtro de arquivo (se fornecido)
		if pathFilter != "" {
			matched, _ := filepath.Match("*" + pathFilter + "*", filepath.Base(rel))
			if !matched && !strings.Contains(strings.ToLower(rel), strings.ToLower(pathFilter)) {
				return nil
			}
		}

		// Filtra apenas arquivos de texto/codigo conhecidos
		ext := strings.ToLower(filepath.Ext(rel))
		if ext == ".exe" || ext == ".dll" || ext == ".db" || ext == ".png" || ext == ".jpg" || ext == ".ico" || ext == ".bin" {
			return nil
		}

		fileData, errRead := os.ReadFile(p)
		if errRead != nil {
			return nil
		}

		lines := strings.Split(string(fileData), "\n")
		var fileMatches []CodeSearchMatch

		for i, line := range lines {
			isMatch := false
			if isLiteral {
				isMatch = strings.Contains(strings.ToLower(line), strings.ToLower(query))
			} else if re != nil {
				isMatch = re.MatchString(line)
			}

			if isMatch {
				before := ""
				if i > 0 {
					before = strings.TrimRight(lines[i-1], "\r")
				}
				after := ""
				if i < len(lines)-1 {
					after = strings.TrimRight(lines[i+1], "\r")
				}

				fileMatches = append(fileMatches, CodeSearchMatch{
					LineNumber:    i + 1,
					ContextBefore: before,
					LineContent:   strings.TrimRight(line, "\r"),
					ContextAfter:  after,
				})
				result.TotalCount++

				if result.TotalCount >= 100 { // Limite de 100 ocorrencias para resposta instantanea
					break
				}
			}
		}

		if len(fileMatches) > 0 {
			result.Files = append(result.Files, CodeSearchFile{
				Path:    rel,
				Matches: fileMatches,
			})
		}

		if result.TotalCount >= 100 {
			return filepath.SkipAll
		}
		return nil
	})

	if errWalk != nil {
		fmt.Printf("[CodeSearch] Erro ao escanear workspace: %v\n", errWalk)
	}

	return result, nil
}

// GetWorkspaceDiff extrai as alteracoes ativas no repositorio Git do workspace para o /diff.
func (a *App) GetWorkspaceDiff() (*WorkspaceDiffResult, error) {
	ws := ""
	if a.executor != nil {
		ws = a.executor.Workspace
	}
	if ws == "" && a.config != nil {
		ws = a.config.ActiveWorkspace
	}
	if ws == "" || ws == "." {
		return &WorkspaceDiffResult{}, nil
	}

	res := &WorkspaceDiffResult{
		Modified:  []string{},
		Untracked: []string{},
	}

	// 1. git status -s
	cmdStatus := exec.Command("git", "status", "--short")
	cmdStatus.Dir = ws
	if out, err := cmdStatus.Output(); err == nil {
		res.StatusText = strings.TrimSpace(string(out))
		scanner := bufio.NewScanner(strings.NewReader(res.StatusText))
		for scanner.Scan() {
			l := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(l, "M ") || strings.HasPrefix(l, " M") {
				res.Modified = append(res.Modified, strings.TrimSpace(l[2:]))
			} else if strings.HasPrefix(l, "??") {
				res.Untracked = append(res.Untracked, strings.TrimSpace(l[2:]))
			}
		}
	}

	// 2. git diff
	cmdDiff := exec.Command("git", "diff")
	cmdDiff.Dir = ws
	if out, err := cmdDiff.Output(); err == nil {
		res.DiffText = string(out)
	}

	return res, nil
}

// GetSecurityPermissions retorna as politicas e permissoes ativas para o /permissions.
func (a *App) GetSecurityPermissions() (*config.SecurityConfig, error) {
	if a.config != nil {
		return &a.config.Security, nil
	}
	cfg := a.GetConfig()
	if cfg == nil {
		return &config.SecurityConfig{}, nil
	}
	return &cfg.Security, nil
}

// GetAntigravitySettings retorna as configurações e permissões granulares do Antigravity CLI.
func (a *App) GetAntigravitySettings() (*config.AntigravitySettings, error) {
	settings, _, err := config.LoadAntigravitySettings()
	return settings, err
}

// SaveAntigravityPermissionRule adiciona ou atualiza uma regra action(target) em allow, ask ou deny.
func (a *App) SaveAntigravityPermissionRule(level string, action string, target string) error {
	return config.AddPermissionRule(level, action, target)
}

// RemoveAntigravityPermissionRule remove uma regra action(target) do settings.json.
func (a *App) RemoveAntigravityPermissionRule(rule string) error {
	return config.RemovePermissionRule(rule)
}

// EvaluateAntigravityAction avalia se uma ação pretendida pelo agente é permitida, requer aprovação ou deve ser negada.
func (a *App) EvaluateAntigravityAction(action string, target string) string {
	settings, _, err := config.LoadAntigravitySettings()
	if err != nil {
		return "ask"
	}
	return config.EvaluatePermission(settings, action, target)
}

