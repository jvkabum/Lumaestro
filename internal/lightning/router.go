package lightning

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// LLMRouter gerencia a resiliência entre múltiplos provedores de IA.
type LLMRouter struct {
	Providers []string // Ex: ["gemini", "openai", "claude"]
}

// NewLLMRouter inicializa o roteador com os provedores disponíveis.
func NewLLMRouter() *LLMRouter {
	return &LLMRouter{
		Providers: []string{"gemini", "openai", "claude"},
	}
}

// ExecuteWithFallback tenta executar o prompt no provedor primário e alterna em caso de falha.
func (r *LLMRouter) ExecuteWithFallback(ctx context.Context, systemPrompt, userMessage string) (string, string, error) {
	var lastErr error
	
	for _, provider := range r.Providers {
		// Pular provedores sem chave configurada (exceto Gemini que usa CLI por padrão)
		if provider == "openai" && os.Getenv("OPENAI_API_KEY") == "" { continue }
		if provider == "claude" && os.Getenv("ANTHROPIC_API_KEY") == "" { continue }

		fmt.Printf("[🛡️ Resiliência] Tentando Provedor: %s...\n", provider)
		
		output, err := r.executeCLI(ctx, provider, systemPrompt, userMessage)
		if err == nil && output != "" {
			return output, provider, nil
		}
		
		lastErr = err
		fmt.Printf("[⚠️ Falha] Provedor %s indisponível: %v. Tentando fallback...\n", provider, err)
		time.Sleep(1 * time.Second)
	}

	return "", "", fmt.Errorf("todos os provedores de IA falharam. Último erro: %w", lastErr)
}

// executeCLI é o wrapper interno para as CLIs configuradas no PATH.
func (r *LLMRouter) executeCLI(ctx context.Context, tool, system, user string) (string, error) {
	input := user
	if system != "" {
		input = fmt.Sprintf("System: %s\n\nUser: %s", system, user)
	}

	cmd := exec.CommandContext(ctx, tool, input)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("falha ao executar %s: %w (Output: %s)", tool, err, string(output))
	}

	return strings.TrimSpace(string(output)), nil
}
