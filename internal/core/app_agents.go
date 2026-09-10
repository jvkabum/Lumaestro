package core

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"Lumaestro/internal/agents/acp"

	"github.com/google/uuid"
)

// StartLoginSession inicia uma sessão de terminal interativa interna para login.
func (a *App) StartLoginSession(agent string) string {
	binary, args := a.installer.GetSetupCommand(agent)
	sessionID := "login-session-" + agent

	err := a.legacyExec.StartCustomSession(a.ctx, agent, binary, args, sessionID)
	if err != nil {
		return "Erro ao iniciar sessão de login: " + err.Error()
	}

	a.emitEvent("terminal:started", map[string]interface{}{
		"agent":     agent,
		"mode":      "Configuração/Login",
		"isRealPTY": true,
	})

	return "Sessão de login iniciada no terminal interno."
}

// ============================================================
// TERMINAL ACP — JSON RPC 2.0 (O CÉREBRO)
// ============================================================

func normalizeAgentKey(agent string) string {
	norm := strings.ToLower(strings.TrimSpace(agent))
	if norm == "gemini" || norm == "agy" || norm == "antigravity" || norm == "" {
		return "antigravity"
	}
	return norm
}

// StartAgentSession inicia a CLI do Antigravity/Gemini em modo seguro ACP (JSON RPC 2.0).
func (a *App) StartAgentSession(agent string) error {
	normAgent := normalizeAgentKey(agent)
	sessionID := normAgent // 🚨 Unificação de ID: Usar o nome do agente diretamente para sessão ACP

	// 🕵️⚡ Trava Imediata: Se já houver uma sessão ativa para este agente, retorna instantaneamente (0ms!)
	a.executor.Mu.Lock()
	_, exists := a.executor.ActiveSessions[sessionID]
	a.executor.Mu.Unlock()

	if exists {
		return nil
	}

	// 🛡️ Gatekeeper de Autenticação: Bloqueia inicialização de instâncias "zumbis" se não houver credenciais.
	if (normAgent == "gemini" || normAgent == "antigravity" || normAgent == "agy") && a.config != nil && !a.config.UseGeminiAPIKey && !a.installer.CheckGeminiAuth() {
		return fmt.Errorf("falha de Autenticação: O motor Antigravity/Gemini requer uma API Key ou Login OAuth (GCloud ADC) para iniciar o processo ACP")
	}
	if agent == "claude" && a.config != nil && !a.config.UseClaudeAPIKey && !a.installer.CheckClaudeAuth() {
		return fmt.Errorf("falha de Autenticação: Claude Code requer setup de credenciais antes de operar via ACP")
	}

	// ⏳ RESILIÊNCIA DE BOOT: Apenas motores locais (native) dependem de NLP local pré-carregado
	if agent == "native" && !a.NLPReady {
		fmt.Printf("[App] ⏳ Aguardando motores locais ficarem ONLINE antes de iniciar %s...\n", agent)
		for i := 0; i < 5; i++ { // Espera até 2.5s no máximo
			if a.NLPReady {
				break
			}
			time.Sleep(500 * time.Millisecond)
		}
	}

	fmt.Printf("[App] Iniciando agente: %s\n", agent)
	// No primeiro boot ou reinício, passamos loadSessionID como "LATEST" para carregar a última Sinfonia.
	err := a.executor.StartSession(a.ctx, agent, sessionID, "LATEST", uuid.Nil, nil, false, nil)
	if err == nil {
		// 📡 SINCRONIZAÇÃO: Puxa o ID real da sessão restaurada e avisa o frontend
		go func() {
			time.Sleep(2 * time.Second) // Espera o initialize e session/load concluírem
			a.executor.Mu.Lock()
			realID := ""
			if sess, ok := a.executor.ActiveSessions[sessionID]; ok {
				realID = sess.ACPSessID
			}
			a.executor.Mu.Unlock()

			if realID != "" {
				fmt.Printf("[App] 📡 Sincronizando UI com a Sinfonia Real: %s\n", realID)
				a.emitEvent("sessions:updated", nil)
				a.emitEvent("sessions:current", realID)
			}
		}()
	}
	return err
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
	return a.executor.StartSession(a.ctx, agent, sessionID, "", uuid.Nil, nil, false, nil)
}

// ListAgentSessions retorna a lista de conversas salvas para o agente
func (a *App) ListAgentSessions(agent string) ([]acp.SessionInfo, error) {
	// 🛰️ Agora permitimos listar o histórico mesmo sem sessão ativa (Standby Mode)
	return a.executor.ListSessions(nil)
}

// LoadAgentSession encerra a atual e carrega uma antiga (Checkpoint)
func (a *App) LoadAgentSession(agent string, acpSessionID string) error {
	normAgent := normalizeAgentKey(agent)
	fmt.Printf("[App] Trocando para sessão: %s (motor: %s)\n", acpSessionID, normAgent)
	sessionID := normAgent

	// 🛡️ HOT SWAP DIRETO: Se já houver uma sessão ativa, verifica o tipo de motor
	a.executor.Mu.Lock()
	existingSession, exists := a.executor.ActiveSessions[sessionID]
	isAlive := exists && existingSession.Cmd != nil && existingSession.Cmd.ProcessState == nil
	isAntigravity := isAlive && existingSession.IsAntigravity
	a.executor.Mu.Unlock()

	if isAlive {
		if isAntigravity {
			// Antigravity opera via stream-json e deve ser reiniciado apontando para a conversa desejada
			fmt.Printf("[App] 🔄 Reiniciando motor Antigravity CLI com a conversa: %s\n", acpSessionID)
			_ = a.executor.StopSession(sessionID)
			return a.executor.StartSession(a.ctx, normAgent, sessionID, acpSessionID, uuid.Nil, nil, false, nil)
		}
		fmt.Printf("[App] ♻️ Hot Swap Direto: Carregando sessão %s no processo ativo\n", acpSessionID)
		return a.executor.LoadSession(existingSession, acpSessionID)
	}

	// Se não houver processo ativo, faz o fluxo completo (inicia processo + carrega sessão)
	return a.executor.StartSession(a.ctx, normAgent, sessionID, acpSessionID, uuid.Nil, nil, false, nil)
}

// NewAgentSession força a criação de um novo chat (limpa o contexto)
func (a *App) NewAgentSession(agent string) error {
	normAgent := normalizeAgentKey(agent)
	fmt.Printf("[App] Iniciando NOVO chat para '%s' (limpando contexto)...\n", normAgent)
	sessionID := normAgent

	// 1. Encerra qualquer sessão ativa existente para este motor (limpeza total)
	a.executor.Mu.Lock()
	var sessionsToStop []string
	for id, sess := range a.executor.ActiveSessions {
		if sess != nil && (id == sessionID || (normAgent == "antigravity" && (id == "gemini" || id == "agy"))) {
			sessionsToStop = append(sessionsToStop, id)
		}
	}
	a.executor.Mu.Unlock()

	for _, id := range sessionsToStop {
		_ = a.executor.StopSession(id)
	}

	// 2. Remove o last_session.json da órbita ativa para não apontar mais para a conversa anterior
	ws := a.getActiveWorkspace()
	if ws != "" && ws != "." {
		lastSessionPath := filepath.Join(ws, ".lumaestro", "last_session.json")
		_ = os.Remove(lastSessionPath)
	}

	// 3. Inicia um novo processo CLI em modo "NEW"
	return a.executor.StartSession(a.ctx, normAgent, sessionID, "NEW", uuid.Nil, nil, false, nil)
}

func (a *App) ResizeTerminal(agent string, cols int, rows int) {
	// Ignored on JSON RPC mode.
}

// StopAgentSession encerra a sessão ativa.
func (a *App) StopAgentSession(agent string) error {
	normAgent := normalizeAgentKey(agent)
	sessionID := normAgent
	err := a.executor.StopSession(sessionID)
	if err != nil {
		return fmt.Errorf("nenhuma sessão ativa ACP encontrada para %s", normAgent)
	}

	a.emitEvent("terminal:closed", normAgent)
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
