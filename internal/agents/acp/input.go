package acp

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// SendInput envia texto para uma sessão ativa da IA via RPC 'prompt', suportando imagens em base64.
func (e *ACPExecutor) SendInput(sessionID string, input string, images []map[string]string, agentCWD string) error {
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

	// ⏳ Aguarda o Handshake terminar de forma assíncrona e limpa
	select {
	case <-session.initDone:
		if session.ACPSessID == "" {
			return fmt.Errorf("inicialização concluída, mas sem ACP sessionId")
		}
	case <-time.After(90 * time.Second): // Tolerância alta para slow boots/downloads
		return fmt.Errorf("timeout crítico aguardando a estabilização do motor")
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

	// 🐕 WATCHDOG DE TURNO: Se a IA não responder em 45s, destrava o frontend
	go func() {
		time.Sleep(45 * time.Second)
		
		e.Mu.Lock()
		_, stillActive := e.ActiveSessions[sessionID]
		e.Mu.Unlock()

		if !stillActive {
			return // Sessão encerrada, nada a fazer
		}

		// Verifica se a mensagem ainda não foi respondida (sem turn_complete)
		e.turnMu.Lock()
		_, turnPending := e.turnChannels[sessionID]
		e.turnMu.Unlock()

		if turnPending {
			fmt.Printf("[ACP] ⚠️ WATCHDOG: Turno ID %d sem resposta após 45s. Destravando frontend.\n", promptID)
			e.LogChan <- ExecutionLog{
				Source:  "SYSTEM",
				Content: "🟡 A IA demorou mais de 45s para responder. O processo pode estar processando em background ou a conexão com o Google pode ter falhado.",
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

	err := e.SendInput(sessionID, prompt, images, "")
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
