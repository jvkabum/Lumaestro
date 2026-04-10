package acp

import (
	"fmt"
	"strings"
)

// HookContext contém o estado para a execução do hook
type HookContext struct {
	Session *ACPSession
	Method  string
	Params  interface{}
}

// HookResult define o que o hook pode fazer (interromper ou modificar fluxo)
type HookResult struct {
	Abort   bool
	Message string
	Error   *RPCError
}

// ACPHook define a interface para hooks de pré/pós processamento
type ACPHook interface {
	Name() string
	BeforeTool(ctx *HookContext) *HookResult
	AfterTool(ctx *HookContext, result interface{}, err *RPCError)
}

// LoggingHook implementa telemetria básica de ferramentas
type LoggingHook struct{}

func (h *LoggingHook) Name() string { return "LoggingHook" }
func (h *LoggingHook) BeforeTool(ctx *HookContext) *HookResult {
	fmt.Printf("[Hook] Pre-Tool: %s em %s\n", ctx.Method, ctx.Session.ID)
	return nil
}
func (h *LoggingHook) AfterTool(ctx *HookContext, result interface{}, err *RPCError) {
	if err != nil {
		fmt.Printf("[Hook] Post-Tool Error: %s -> %v\n", ctx.Method, err.Message)
	} else {
		fmt.Printf("[Hook] Post-Tool Success: %s\n", ctx.Method)
	}
}

// SafetyHook implementa verificações de segurança globais
type SafetyHook struct{}

func (h *SafetyHook) Name() string { return "SafetyHook" }
func (h *SafetyHook) BeforeTool(ctx *HookContext) *HookResult {
	// Exemplo: Bloquear acesso a pastas sensíveis se não estiver via FSProxy (FSProxy já faz isso, mas aqui é redundância)
	if strings.Contains(ctx.Method, "read") || strings.Contains(ctx.Method, "write") {
		// Lógica adicional de auditoria poderia entrar aqui
	}
	return nil
}
func (h *SafetyHook) AfterTool(ctx *HookContext, result interface{}, err *RPCError) {}

// GlobalHooks registro de hooks ativos
var GlobalHooks = []ACPHook{
	&LoggingHook{},
	&SafetyHook{},
}
