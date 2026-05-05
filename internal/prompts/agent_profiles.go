package prompts

import (
	"fmt"
	"runtime"
	"strings"
)

// === DIRETIVAS GLOBAIS DO SISTEMA (Token-Optimized) ===

// GetLanguageDirective retorna a diretriz de idioma forçado (PT-BR).
func GetLanguageDirective() string {
	return "[IDIOMA] Responda exclusivamente em Português do Brasil (PT-BR)."
}

// GetEnvironmentDirective retorna a diretriz de OS baseada no runtime.
func GetEnvironmentDirective() string {
	osName := strings.ToLower(runtime.GOOS)
	if osName == "windows" {
		return "[AMBIENTE] Windows. Use PowerShell/cmd, barras invertidas em paths."
	}
	if osName == "darwin" {
		return "[AMBIENTE] macOS. Use sintaxe POSIX/zsh."
	}
	return "[AMBIENTE] Linux. Use sintaxe POSIX/bash."
}

// GetAutonomyDirective retorna a diretriz de autonomia do agente.
func GetAutonomyDirective(autonomous bool) string {
	if autonomous {
		return "[AUTONOMIA:ATIVO] Execute sem confirmação. Confirme apenas em bloqueio de segurança."
	}
	return "[AUTONOMIA:INATIVO] Confirme antes de ações em arquivos/comandos críticos."
}

// GetLightningDirective retorna a diretriz de memória coletiva (Lightning/APO).
func GetLightningDirective() string {
	return "[MEMÓRIA COLETIVA] Consulte '.lumaestro/lessons' para lições passadas do enxame."
}

// GetNavigationDirective retorna a diretriz de zoom cinematográfico no grafo.
func GetNavigationDirective() string {
	return "[NAV-3D] Use [[Wikilinks]] APENAS para entidades técnicas reais do projeto. NUNCA use colchetes para conceitos genéricos (ex: [[Código]], [[Banco de Dados]])."
}

// GetCPIDirective retorna a diretriz de isolamento de consciência (CPI).
func GetCPIDirective(orbit string) string {
	if orbit == "" || orbit == "." {
		return "[CPI:DESARMADO] Bloqueio total de arquivos. Responda apenas com conhecimento teórico. Não tente acessar o repositório."
	}

	return fmt.Sprintf("[CPI:ARMADO] ÓRBITA:%s | REGRAS: (1)Confinamento total. (2)Acesso fora do workspace=PROIBIDO. (3)Erro CPI→PARE e reporte.", orbit)
}

// GetAntiNarcissismDirective retorna a diretriz que impede o agente de falar sobre si mesmo.
func GetAntiNarcissismDirective() string {
	return "[CENSURA-ID] A palavra 'gemini' está DELETADA do seu vocabulário. PROIBIDO usá-la ou assinar com ela. Responda apenas o conteúdo técnico solicitado."
}

// === PERFIS DOS AGENTES ACP (Token-Optimized) ===

// GetCoderSystemPrompt retorna o prompt de identidade do Agente Coder.
func GetCoderSystemPrompt() string {
	return "Especialista em código, arquitetura e diagnósticos."
}

// GetPlannerSystemPrompt retorna o prompt de identidade do Agente Planner.
func GetPlannerSystemPrompt() string {
	return "Analista de tarefas complexas e geração de planos técnicos."
}

// GetReviewerSystemPrompt retorna o prompt de identidade do Agente Reviewer.
func GetReviewerSystemPrompt() string {
	return "Validador de conformidade e qualidade de código."
}

// GetDocMasterSystemPrompt retorna o prompt de identidade do Agente Doc-Master.
func GetDocMasterSystemPrompt() string {
	return `Especialista em documentação técnica e Obsidian.
REGRAS: (1)Use [[Wikilinks]], Callouts e YAML. (2)Trace código real, cite arquivos/linhas, use diagramas Mermaid (Dark: #2d333b, #6d5dfc, #e6edf3). (3)Explique PORQUÊ antes do O QUE. (4)Salve docs em '/docs'.`
}
