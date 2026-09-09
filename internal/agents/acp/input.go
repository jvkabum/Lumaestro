package acp

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"Lumaestro/internal/utils"
)

// SendInput envia texto para uma sessão ativa da IA via RPC 'prompt', suportando imagens em base64.
func (e *ACPExecutor) SendInput(sessionID string, input string, images []map[string]string) error {
	fmt.Printf("[ACP] >> SendInput recebido! Session: %s, Msg: %s...\n", sessionID, input)
	
	e.Mu.Lock()
	session, ok := e.ActiveSessions[sessionID]
	if ok && session != nil {
		session.LastInput = input
		imgData, _ := json.Marshal(images)
		session.LastImagesJSON = string(imgData)
	}
	e.Mu.Unlock()

	if !ok || session == nil {
		fmt.Printf("[ACP] ❌ Erro: Sessão %s não encontrada!\n", sessionID)
		return fmt.Errorf("sessão %s não encontrada", sessionID)
	}

	// 🚀 Antigravity CLI: Envia evento stream-json diretamente pelo stdin (dispensa ACP sessionId prévio)
	if session.IsAntigravity {
		userEvt := map[string]interface{}{
			"event": "user",
			"message": map[string]interface{}{
				"content": input,
			},
		}
		data, err := json.Marshal(userEvt)
		if err != nil {
			return err
		}

		session.WriteMu.Lock()
		fmt.Printf(">> [AGY SEND] %s\n", string(data))
		_, err = fmt.Fprintln(session.Stdin, string(data))
		session.WriteMu.Unlock()
		if err != nil {
			return err
		}

		// Registra canal de turno para IsTurnPending
		e.turnMu.Lock()
		if _, exists := e.turnChannels[sessionID]; !exists {
			e.turnChannels[sessionID] = make(chan string, 100)
		}
		e.turnMu.Unlock()

		session.UpdateActivity()

		// 🐕 WATCHDOG DE TURNO BASEADO EM INATIVIDADE:
		// Verifica periodicamente se houve silêncio total do motor por mais de 120s.
		// Se o motor estiver chamando ferramentas ou gerando texto, a atividade se renova e ele NÃO é interrompido.
		go func() {
			silenceThreshold := 120 * time.Second
			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()

			for range ticker.C {
				e.Mu.Lock()
				s, stillActive := e.ActiveSessions[sessionID]
				e.Mu.Unlock()
				if !stillActive || s == nil {
					return
				}

				e.turnMu.Lock()
				_, turnPending := e.turnChannels[sessionID]
				e.turnMu.Unlock()
				if !turnPending {
					return
				}

				if time.Since(s.GetLastActivity()) > silenceThreshold {
					fmt.Printf("[AGY] ⚠️ WATCHDOG: Motor Antigravity silencioso por mais de %v. Destravando frontend.\n", silenceThreshold)
					e.LogChan <- ExecutionLog{
						Source:  "SYSTEM",
						Content: fmt.Sprintf("🟡 O motor Antigravity ficou inativo por mais de %v. Destravando frontend.", silenceThreshold),
					}
					if e.Ctx != nil {
						utils.SafeEmit(e.Ctx, "agent:turn_complete", session.AgentName)
					}
					return
				}
			}
		}()

		return nil
	}

	// ⏳ Aguarda o Handshake terminar se ele ainda estiver rolando em background (apenas JSON-RPC clássico)
	if session.ACPSessID == "" {
		fmt.Printf("[ACP] ⏳ Sessão %s ainda sem ID ACP. Aguardando estabilização...\n", sessionID)
		for i := 0; i < 10; i++ {
			time.Sleep(500 * time.Millisecond)
			if session.ACPSessID != "" { break }
		}
		if session.ACPSessID == "" {
			return fmt.Errorf("sessão não initializada completamente (sem ACP sessionId)")
		}
	}

	// 🧠 Construção do Prompt Multimodal (Texto + Imagens)
	var promptData []interface{}
	promptData = append(promptData, map[string]string{
		"type": "text",
		"text": input,
	})

	for _, img := range images {
		promptData = append(promptData, map[string]interface{}{
			"type": "image",
			"source": map[string]string{
				"type":      "base64",
				"mediaType": img["type"],
				"data":      img["data"],
			},
		})
	}

	promptID := e.getNextID()

	params := map[string]interface{}{
		"sessionId": session.ACPSessID,
		"prompt":    promptData,
	}
	paramsJSON, _ := json.Marshal(params)

	err := e.SendRPC(session, JSONRPCMessage{
		JSONRPC: JSONRPCVersion,
		ID:      promptID,
		Method:  "session/prompt",
		Params:  paramsJSON,
	})
	
	if err != nil {
		return err
	}

	session.UpdateActivity()

	// 🐕 WATCHDOG DE TURNO: Se a IA ficar inativa por mais de 120s, destrava o frontend
	go func() {
		silenceThreshold := 120 * time.Second
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			e.Mu.Lock()
			s, stillActive := e.ActiveSessions[sessionID]
			e.Mu.Unlock()
			if !stillActive || s == nil {
				return
			}

			e.turnMu.Lock()
			_, turnPending := e.turnChannels[sessionID]
			e.turnMu.Unlock()
			if !turnPending {
				return
			}

			if time.Since(s.GetLastActivity()) > silenceThreshold {
				fmt.Printf("[ACP] ⚠️ WATCHDOG: Turno ID %d sem resposta por mais de %v. Destravando frontend.\n", promptID, silenceThreshold)
				e.LogChan <- ExecutionLog{
					Source:  "SYSTEM",
					Content: fmt.Sprintf("🟡 A IA ficou inativa por mais de %v. O processo pode ter travado ou a conexão falhou.", silenceThreshold),
				}
				return
			}
		}
	}()
	
	return nil
}

// AskSync envia um prompt e aguarda a resposta completa da IA (Bloqueante).
func (e *ACPExecutor) AskSync(sessionID string, prompt string, images []map[string]string) (string, error) {
	e.Mu.Lock()
	_, ok := e.ActiveSessions[sessionID]
	e.Mu.Unlock()

	if !ok { return "", fmt.Errorf("sessão '%s' não encontrada para AskSync", sessionID) }

	ch := make(chan string, 512)
	e.turnMu.Lock()
	e.turnChannels[sessionID] = ch
	e.turnMu.Unlock()

	err := e.SendInput(sessionID, prompt, images)
	if err != nil { return "", err }

	var fullResponse strings.Builder
	timeout := time.After(60 * time.Second)

	for {
		select {
		case chunk, ok := <-ch:
			if !ok { return fullResponse.String(), nil }
			fullResponse.WriteString(chunk)
		case <-timeout:
			return "", fmt.Errorf("timeout aguardando resposta completa do agente")
		}
	}
}

// SendSteeringHint envia uma dica de direcionamento em tempo real para o canal da sessão.
func (e *ACPExecutor) SendSteeringHint(sessionID string, hint string) error {
	e.Mu.Lock()
	session, ok := e.ActiveSessions[sessionID]
	e.Mu.Unlock()

	if !ok || session == nil {
		return fmt.Errorf("sessão %s não encontrada para steering", sessionID)
	}

	select {
	case session.SteeringChan <- hint:
		fmt.Printf("[Steering] ⚡ Hint injetado para %s: %s\n", sessionID, hint)
		return nil
	default:
		return fmt.Errorf("canal de steering de %s está cheio ou bloqueado", sessionID)
	}
}
