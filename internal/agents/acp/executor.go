package acp

import (
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"

	"Lumaestro/internal/utils"
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

// RequestReview emite um evento para o Wails e aguarda a resposta do usuário.
func (e *ACPExecutor) RequestReview(reviewID string, action string, details string) bool {
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
