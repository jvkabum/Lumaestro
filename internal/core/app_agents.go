package core

import (
	"Lumaestro/internal/agents"
	"fmt"
	"os/exec"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// StartLoginSession inicia uma sessão de terminal interativa interna para login.
func (a *App) StartLoginSession(agent string) string {
	binary, args := a.installer.GetSetupCommand(agent)
	sessionID := "login-session-" + agent

	err := a.legacyExec.StartCustomSession(a.ctx, agent, binary, args, sessionID)
	if err != nil {
		return "Erro ao iniciar sessão de login: " + err.Error()
	}

	runtime.EventsEmit(a.ctx, "terminal:started", map[string]interface{}{
		"agent":     agent,
		"mode":      "Configuração/Login",
		"isRealPTY": true,
	})

	return "Sessão de login iniciada no terminal interno."
}

// ============================================================
// TERMINAL ACP — JSON RPC 2.0 (O CÉREBRO)
// ============================================================

// StartAgentSession inicia a CLI do Gemini em modo seguro ACP (JSON RPC 2.0).
func (a *App) StartAgentSession(agent string) error {
	sessionID := agent // 🚨 Unificação de ID: Usar o nome do agente diretamente casas sesão ACP

	// 🕵️⚡ Trava de Segurança: Não inicia se já houver uma sessão ativa ou iniciando para este agente.
	a.executor.Mu.Lock()
	_, exists := a.executor.ActiveSessions[sessionID]
	a.executor.Mu.Unlock()

	if exists {
		fmt.Printf("[App] Agente %s já está no Ar. Orquestra pronta.\n", agent)
		return nil
	}

	fmt.Printf("[App] Iniciando agente: %s\n", agent)
	// No primeiro boot ou reinício, passamos loadSessionID como "LATEST" para carregar a última Sinfonia.
	return a.executor.StartSession(a.ctx, agent, sessionID, "LATEST", uuid.Nil, nil)
}

// StartBackgroundAgentSession cria uma instância paralela silenciosa exclusiva para o processamento de RAG
func (a *App) StartBackgroundAgentSession(agent string) error {
	sessionID := "background-" + agent // Mantém prefixo apenas para background para evitar colisão

	a.executor.Mu.Lock()
	_, exists := a.executor.ActiveSessions[sessionID]
	a.executor.Mu.Unlock()

	if exists {
		fmt.Printf("[App] Agente de Background (%s) já está online.\n", agent)
		return nil
	}

	fmt.Printf("[App] Iniciando Agente de BACKGROUND (Black): %s\n", agent)
	// Background NUNCA deve carregar histórico (LATEST) para não misturar os contextos. Inicia sempre limpo.
	return a.executor.StartSession(a.ctx, agent, sessionID, "", uuid.Nil, nil)
}

// ListAgentSessions retorna a lista de conversas salvas para o agente
func (a *App) ListAgentSessions(agent string) ([]agents.SessionInfo, error) {
	sessionID := agent
	a.executor.Mu.Lock()
	session, ok := a.executor.ActiveSessions[sessionID]
	a.executor.Mu.Unlock()

	if !ok {
		return nil, fmt.Errorf("inicie o agente antes de listar o histórico")
	}

	return a.executor.ListSessions(session)
}

// LoadAgentSession encerra a atual e carrega uma antiga (Checkpoint)
func (a *App) LoadAgentSession(agent string, acpSessionID string) error {
	fmt.Printf("[App] Trocando para sessão: %s\n", acpSessionID)
	sessionID := agent
	return a.executor.StartSession(a.ctx, agent, sessionID, acpSessionID, uuid.Nil, nil)
}

// NewAgentSession força a criação de um novo chat (limpa o contexto)
func (a *App) NewAgentSession(agent string) error {
	fmt.Println("[App] Iniciando NOVO chat (limpando contexto)...")
	sessionID := agent
	return a.executor.StartSession(a.ctx, agent, sessionID, "", uuid.Nil, nil)
}

func (a *App) ResizeTerminal(agent string, cols int, rows int) {
	// Ignored on JSON RPC mode.
}

// StopAgentSession encerra a sessão ativa.
func (a *App) StopAgentSession(agent string) error {
	sessionID := agent
	err := a.executor.StopSession(sessionID)
	if err != nil {
		return fmt.Errorf("nenhuma sessão ativa ACP encontrada para %s", agent)
	}

	runtime.EventsEmit(a.ctx, "terminal:closed", agent)
	return nil
}

// ============================================================
// NOVAS INTEGRAÇÕES (Autonomia, Regras e MCP)
// ============================================================

// SetAutonomousMode ativa ou desativa globalmente o modo YOLO
func (a *App) SetAutonomousMode(enabled bool) string {
	a.executor.AutonomousMode = enabled
	if enabled {
		return "Modo Autônomo ATIVADO. Executará tarefas de terminal sem permissão (Comandos destrutivos ainda requerem review de Hands Security)."
	}
	return "Modo Autônomo DESATIVADO. A CLI voltará a pedir aprovação."
}

// SubmitReview aprova ou rejeita uma ação pendente da IA
func (a *App) SubmitReview(id string, approved bool) {
	a.executor.SubmitReview(id, approved)
}

// GetAutonomousMode retorna o estado atual da autonomia do enxame.
func (a *App) GetAutonomousMode() bool {
	return a.executor.AutonomousMode
}

// RunInTerminal abre um terminal nativo do Windows (Powershell) para tarefas externas.
func (a *App) RunInTerminal(command string) string {
	cmd := exec.Command("cmd", "/c", "start", "powershell", "-NoExit", "-Command", command)
	err := cmd.Run()
	if err != nil {
		return "Erro: " + err.Error()
	}
	return "Terminal aberto!"
}
