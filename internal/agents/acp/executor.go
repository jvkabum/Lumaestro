package acp

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	config "Lumaestro/internal/config"
	"Lumaestro/internal/utils"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// NewACPExecutor inicializa o novo executor JSON-RPC.
func NewACPExecutor() *ACPExecutor {
	return &ACPExecutor{
		ActiveSessions:  make(map[string]*ACPSession),
		LogChan:         make(chan ExecutionLog, 100),
		TerminalOutput:  make(chan TerminalData, 256),
		Proxy:           NewFSProxy(),
		Tools:           NewToolRegistry(), // 🛠️ Inicializa as ferramentas Obsidian
		pendingReviews:  make(map[string]chan bool),
		pendingRequests: make(map[int]chan JSONRPCMessage),
		execLock:        make(chan struct{}, 1), // Apenas 1 ferramenta por vez
		NetLog:          utils.NewNetworkLogger(5 * time.Second),
		turnChannels:    make(map[string]chan string),
	}
}

func isPotentiallyDestructiveCommand(details string) bool {
	d := strings.ToLower(strings.TrimSpace(details))
	if d == "" {
		return false
	}

	markers := []string{
		" rm ", " rm -", "rm -rf", "del /f", "del /s", "rmdir /s", "rd /s", "format ",
		"mkfs", "diskpart", "shutdown ", "reboot", "poweroff", "taskkill /f", "kill -9",
		"reg delete", "drop database", "truncate table", "remove-item -recurse", "remove-item -force",
	}

	for _, m := range markers {
		if strings.Contains(d, m) {
			return true
		}
	}

	return false
}

// waitForResponse aguarda a resposta de um ID específico por um tempo determinado.
func (e *ACPExecutor) waitForResponse(id int, timeout time.Duration) (JSONRPCMessage, error) {
	ch := make(chan JSONRPCMessage, 1)

	e.requestsMu.Lock()
	e.pendingRequests[id] = ch
	e.requestsMu.Unlock()

	defer func() {
		e.requestsMu.Lock()
		delete(e.pendingRequests, id)
		e.requestsMu.Unlock()
	}()

	select {
	case msg := <-ch:
		if msg.Error != nil {
			return msg, fmt.Errorf("erro RPC [%d]: %s", msg.Error.Code, msg.Error.Message)
		}
		return msg, nil
	case <-time.After(timeout):
		return JSONRPCMessage{}, fmt.Errorf("timeout aguardando resposta para ID %d", id)
	}
}

func (e *ACPExecutor) getNextID() int {
	return int(atomic.AddUint64(&e.msgIDCounter, 1))
}

// SendRPC envia uma mensagem JSON-RPC para o processo ACP via stdin usando o formato ndJSON.
func (e *ACPExecutor) SendRPC(s *ACPSession, msg JSONRPCMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	// Garante que apenas um pacote seja escrito por vez no Pipe
	s.WriteMu.Lock()
	defer s.WriteMu.Unlock()

	// 📡 TRANSPARÊNCIA: Mostra no terminal o JSON exato sendo enviado para a IA
	fmt.Printf(">> [ACP SEND] %s\n", string(data))
	
	_, err = fmt.Fprintln(s.Stdin, string(data))
	return err
}

// IsTurnPending verifica se o agente ainda está processando uma mensagem.
func (e *ACPExecutor) IsTurnPending(sessionID string) bool {
	e.turnMu.Lock()
	_, pending := e.turnChannels[sessionID]
	e.turnMu.Unlock()
	return pending
}

// RequestReview emite um evento para o Wails e aguarda a resposta do usuário.
func (e *ACPExecutor) RequestReview(reviewID string, action string, details string) bool {
	if e.AutonomousMode && strings.EqualFold(strings.TrimSpace(action), "EXECUTAR COMANDO") {
		if !isPotentiallyDestructiveCommand(details) {
			if e.Ctx != nil {
				runtime.EventsEmit(e.Ctx, "agent:status", map[string]string{
					"agent":  "system",
					"action": "Modo autônomo: comando autoaprovado",
					"kind":   "status",
				})
			}
			return true
		}
		if e.Ctx != nil {
			runtime.EventsEmit(e.Ctx, "agent:status", map[string]string{
				"agent":  "system",
				"action": "Modo autônomo: comando potencialmente destrutivo requer aprovação",
				"kind":   "warning",
			})
		}
	}

	ch := make(chan bool)

	e.reviewMu.Lock()
	e.pendingReviews[reviewID] = ch
	e.reviewMu.Unlock()

	// Emite evento para o Frontend (Wails)
	runtime.EventsEmit(e.Ctx, "agent:review_request", map[string]string{
		"id":      reviewID,
		"action":  action,
		"details": details,
	})

	// Aguarda a resposta (bloqueia a goroutine do RPC Handler)
	approved := <-ch

	e.reviewMu.Lock()
	delete(e.pendingReviews, reviewID)
	e.reviewMu.Unlock()

	return approved
}

// SubmitReview é chamado pelo Frontend via Wails para aprovar/rejeitar.
func (e *ACPExecutor) SubmitReview(reviewID string, approved bool) {
	e.reviewMu.Lock()
	ch, ok := e.pendingReviews[reviewID]
	e.reviewMu.Unlock()
	if ok {
		ch <- approved
	}
}

// SetSessionModel altera o modelo da IA em runtime via RPC 'unstable_setSessionModel'.
func (e *ACPExecutor) SetSessionModel(sessionID string, model string) error {
	e.Mu.Lock()
	session, ok := e.ActiveSessions[sessionID]
	e.Mu.Unlock()

	if !ok || session == nil {
		return fmt.Errorf("sessão %s não encontrada", sessionID)
	}

	if session.ACPSessID == "" {
		return fmt.Errorf("sessão ainda não inicializada via ACP")
	}

	fmt.Printf("[ACP] >> Solicitando troca de modelo para: %s (Sessão: %s)\n", model, session.ACPSessID)

	params, _ := json.Marshal(map[string]interface{}{
		"sessionId": session.ACPSessID,
		"model":     model,
	})

	id := e.getNextID()
	err := e.SendRPC(session, JSONRPCMessage{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Method:  "unstable_setSessionModel",
		Params:  params,
	})

	if err != nil {
		return err
	}

	// Aguarda um breve retorno ou timeout para confirmar o recebimento
	_, err = e.waitForResponse(id, 5*time.Second)
	return err
}
// HandleQuotaExhausted aciona a rotação da frota de resiliência.
func (e *ACPExecutor) HandleQuotaExhausted(sessionID string) {
	e.Mu.Lock()
	session, ok := e.ActiveSessions[sessionID]
	e.Mu.Unlock()

	if !ok || session == nil {
		return
	}

	// 1. Rotacionar a chave no pool do Config
	cfg, err := config.Load()
	if err != nil {
		return
	}

	// Se houver mais chaves, rotaciona. 
	// Se for modelo Pro, tentamos o fallback para Flash na mesma chave antes de rotacionar (opcional).
	_ = cfg.RotateGeminiKey() // Rotaciona e ignora o retorno (o índice interno já é atualizado)
	
	// 2. Notificar UI
	if e.Ctx != nil {
		runtime.EventsEmit(e.Ctx, "agent:status", map[string]string{
			"agent":  session.AgentName,
			"action": fmt.Sprintf("🔄 Cota exaurida! Trocando chave API (Pool %d/%d) e reiniciando motor...", cfg.GeminiKeyIndex+1, cfg.GeminiKeyCount()),
			"kind":   "warning",
		})
	}

	// 3. Reiniciar a sessão CLI (O StartSession já mata a anterior se o ID colidir)
	// Como estamos dentro do Executor, podemos chamar StartSession.
	// Precisamos apenas dos parâmetros originais.
	time.Sleep(1 * time.Second) // Delay tático para limpeza de pipes
			if err := e.StartSession(e.Ctx, session.AgentName, session.ID, session.ACPSessID, session.AgentID, session.CurrentIssueID, session.PlanMode, nil); err != nil {
		fmt.Printf("[Resilience] Erro ao reiniciar motor: %v\n", err)
		return
	}

	// 4. Auto-Retry (Se houver input salvo)
	if session.LastInput != "" {
		fmt.Printf("[Resilience] Re-enviando último input após rotação...\n")
		time.Sleep(2 * time.Second) // Aguarda o boot do novo processo
		
		// Injetamos um aviso de log para o usuário saber que o maestro voltou
		e.LogChan <- ExecutionLog{
			Source:  "SYSTEM",
			Content: "🛡️ Frota rotacionada com sucesso. Tentando novamente...",
			Type:    "system",
		}

		// Chama SendInput (precisamos do método que está no input.go)
		var images []map[string]string
		if session.LastImagesJSON != "" {
			json.Unmarshal([]byte(session.LastImagesJSON), &images)
		}
		
		go e.SendInput(session.ID, session.LastInput, images)
	}
}

// SpawnSubagent cria uma nova sessão ACP efêmera vinculada a esta sessão pai.
func (e *ACPExecutor) SpawnSubagent(parent *ACPSession, agentName string, goal string) (*ACPSession, error) {
	subSessID := fmt.Sprintf("%s-sub-%s", parent.ID, uuid.NewString()[:8])
	
	fmt.Printf("[Subagent] 🚀 Spawning subagent '%s' para: %s\n", agentName, goal)
	
	err := e.StartSession(parent.Ctx, agentName, subSessID, "LATEST", parent.AgentID, parent.CurrentIssueID, parent.PlanMode, parent)
	if err != nil {
		return nil, err
	}
	
	e.Mu.Lock()
	subSess := e.ActiveSessions[subSessID]
	e.Mu.Unlock()
	
	return subSess, nil
}

// StopSession encerra uma sessão ACP e todos os seus subagentes recursivamente.
func (e *ACPExecutor) StopSession(sessionID string) error {
	e.Mu.Lock()
	session, ok := e.ActiveSessions[sessionID]
	e.Mu.Unlock()

	if !ok || session == nil {
		return fmt.Errorf("sessão %s não encontrada", sessionID)
	}

	fmt.Printf("[ACP] 🛑 Encerrando sessão %s e subagentes...\n", sessionID)

	// 🌳 Cleanup Recursivo de Subagentes
	session.SubagentMu.Lock()
	for subID := range session.Subagents {
		e.StopSession(subID) // Chamada recursiva
	}
	session.SubagentMu.Unlock()

	// 🔪 Finalização do Processo
	if session.Cancel != nil {
		session.Cancel()
	}

	e.Mu.Lock()
	delete(e.ActiveSessions, sessionID)
	e.Mu.Unlock()

	return nil
}
