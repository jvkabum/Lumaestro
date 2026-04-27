package utils

import (
	"context"
	"fmt"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// SafeEmit é o canal de comunicação blindado com o frontend.
// Protege a aplicação contra crashes causados por contextos inválidos, nulos ou já encerrados.
func SafeEmit(ctx context.Context, name string, data interface{}) {
	if ctx == nil {
		fmt.Printf("❌ [SafeEmit] ERRO CRÍTICO: Tentativa de enviar evento '%s' com CONTEXTO NULO!\n", name)
		return
	}

	// 🛡️ Recuperação de Desastres: Captura qualquer panic interno do Wails runtime
	defer func() {
		if r := recover(); r != nil {
			// Apenas loga o erro no terminal, mas impede que o App morra
			fmt.Printf("⚠️  [SafeEmit] Bloqueado crash do Wails no evento '%s': %v\n", name, r)
		}
	}()

	// 🛡️ Validação de Vida: Verifica se o contexto ainda é legítimo
	select {
	case <-ctx.Done():
		// App fechando ou contexto invalidado: ignora a emissão silenciosamente
		return
	default:
		// Contexto saudável: delega a emissão para o runtime oficial
		if name != "agent:log" { // Evita floodar o terminal com logs de agente
			fmt.Printf("📡 [SafeEmit] Enviando evento '%s' para o Frontend...\n", name)
		}
		runtime.EventsEmit(ctx, name, data)
	}
}
