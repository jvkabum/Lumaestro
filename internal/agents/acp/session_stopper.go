package acp

import (
	"fmt"
	"time"
)

// StopSession encerra uma sessão ativa de forma graciosa para garantir o salvamento do histórico.
func (e *ACPExecutor) StopSession(sessionID string) error {
	e.Mu.Lock()
	s, ok := e.ActiveSessions[sessionID]
	if !ok || s == nil {
		e.Mu.Unlock()
		return fmt.Errorf("sessão %s não encontrada", sessionID)
	}
	delete(e.ActiveSessions, sessionID)
	e.Mu.Unlock()

	fmt.Printf("[ACP] Encerrando sessão %s e subagentes de forma graciosa...\n", sessionID)

	// 🌳 Cleanup Recursivo de Subagentes
	s.SubagentMu.Lock()
	for subID := range s.Subagents {
		_ = e.StopSession(subID) // Chamada recursiva
	}
	s.SubagentMu.Unlock()

	if s.Stdin != nil {
		// 🚪 FECHAMENTO GRACIOSO: Fechar o Stdin sinaliza EOF para o Gemini CLI,
		// forçando-o a salvar o histórico e encerrar por conta própria.
		_ = s.Stdin.Close()
	}

	if s.Cancel != nil {
		s.Cancel()
	}

	// Aguarda um breve momento para o flush do sistema de arquivos
	time.Sleep(300 * time.Millisecond)

	if s.Cmd != nil && s.Cmd.Process != nil {
		// Se ainda estiver vivo após o sinal gracioso, aí sim usamos a força
		_ = s.Cmd.Process.Kill()
	}

	return nil
}
