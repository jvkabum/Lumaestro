package acp

import (
	"os"
	"path/filepath"
	"strings"

	"Lumaestro/internal/prompts"
)

// AgentProfile define a identidade e o comportamento de um agente específico.
type AgentProfile struct {
	Name         string
	SystemPrompt string
}

var (
	ProfileCoder = AgentProfile{
		Name:         "Coder",
		SystemPrompt: prompts.GetCoderSystemPrompt(),
	}

	ProfilePlanner = AgentProfile{
		Name:         "Planner",
		SystemPrompt: prompts.GetPlannerSystemPrompt(),
	}

	ProfileReviewer = AgentProfile{
		Name:         "Reviewer",
		SystemPrompt: prompts.GetReviewerSystemPrompt(),
	}

	ProfileDocMaster = AgentProfile{
		Name:         "Doc-Master",
		SystemPrompt: prompts.GetDocMasterSystemPrompt(),
	}
)

// BuildContext agrupa todos os dados dinâmicos para montagem do prompt.
// Permite injeção condicional: diretivas só entram se o contexto justificar.
type BuildContext struct {
	RAGContext string   // Contexto do Obsidian (pode ser vazio)
	History    []string // Histórico de conversa
	Goal       string   // Objetivo atual do usuário
	Autonomous bool     // Modo YOLO ativo?
	Orbit      string   // Workspace CPI (pode ser vazio = desarmado)
	HasGraph   bool     // Grafo 3D está ativo? (evita injetar NavDirective sem grafo)
	HasLessons bool     // Existem lições Lightning? (evita injetar sem conteúdo)
}

// PromptBuilder organiza as peças da sinfonia em uma string única para o agente.
type PromptBuilder struct{}

func NewPromptBuilder() *PromptBuilder {
	return &PromptBuilder{}
}

// Build gera o prompt final com injeção condicional de diretivas.
//
// Estratégia de economia de tokens:
// - Diretivas core (idioma, agente, CPI) são SEMPRE injetadas.
// - Diretivas situacionais só entram quando relevantes:
//   - EnvironmentDirective → apenas para Coder (executa comandos).
//   - NavigationDirective  → apenas se HasGraph=true.
//   - LightningDirective   → apenas se HasLessons=true.
//   - AutonomyDirective    → apenas se autonomous=true (modo inativo é o default implícito).
func (b *PromptBuilder) Build(profile AgentProfile, ctx BuildContext) string {
	var sb strings.Builder
	sb.Grow(512) // Pre-alocação para reduzir realocações

	// 1. Core Identity (sempre presente)
	sb.WriteString(prompts.GetLanguageDirective())
	sb.WriteByte('\n')
	sb.WriteString(profile.SystemPrompt)
	sb.WriteByte('\n')

	// 2. 🛡️ CPI — Isolamento de Consciência (sempre presente, é segurança)
	sb.WriteString(prompts.GetCPIDirective(ctx.Orbit))
	sb.WriteByte('\n')

	// 2.5. 🎯 Anti-Narcisismo — impede auto-apresentação
	sb.WriteString(prompts.GetAntiNarcissismDirective())
	sb.WriteByte('\n')

	// 🔒 AMNÉSIA SITUACIONAL: Se não há órbita ou se estamos na sandbox, isolamos o conhecimento do sistema
	appRoot, _ := os.Getwd()
	normOrbit := strings.ToLower(filepath.Clean(ctx.Orbit))
	normAppRoot := strings.ToLower(filepath.Clean(appRoot))
	isZeroOrbit := ctx.Orbit == "" || ctx.Orbit == "." || strings.Contains(normOrbit, "sandbox") || normOrbit == normAppRoot
	if isZeroOrbit {
		sb.WriteString("[AMNÉSIA/SANDBOX] Você está operando em uma Sandbox limpa (.lumaestro/sandbox) sem repositório de projeto de usuário ativo. PROIBIDO mencionar ou alterar a base de código do Lumaestro. Responda como uma IA técnica genérica.\n")
	}

	// 3. Diretivas Condicionais (só gasta token se relevante e autorizado pela órbita)
	if ctx.Autonomous {
		sb.WriteString(prompts.GetAutonomyDirective(true))
		sb.WriteByte('\n')
	}
	if profile.Name == "Coder" {
		sb.WriteString(prompts.GetEnvironmentDirective())
		sb.WriteByte('\n')
	}
	if ctx.HasGraph && !isZeroOrbit {
		sb.WriteString(prompts.GetNavigationDirective())
		sb.WriteByte('\n')
	}
	if ctx.HasLessons && !isZeroOrbit {
		sb.WriteString(prompts.GetLightningDirective())
		sb.WriteByte('\n')
	}


	// 4. Contexto RAG (Obsidian) - Silenciado em Órbita Zero
	if ctx.RAGContext != "" && !isZeroOrbit {
		sb.WriteString("CONTEXTO:\n")
		sb.WriteString(ctx.RAGContext)
		sb.WriteByte('\n')
	}

	// 5. Histórico Recente (Memória Viva)
	if len(ctx.History) > 0 {
		sb.WriteString("HISTÓRICO:\n")
		for _, h := range ctx.History {
			sb.WriteString("- ")
			sb.WriteString(h)
			sb.WriteByte('\n')
		}
	}

	// 6. Objetivo
	sb.WriteString("OBJETIVO: ")
	sb.WriteString(ctx.Goal)

	return sb.String()
}

// BuildLegacy mantém compatibilidade com a assinatura antiga durante a transição.
// Deprecated: Use Build(profile, BuildContext{...}) diretamente.
func (b *PromptBuilder) BuildLegacy(profile AgentProfile, context string, history []string, goal string, autonomous bool, orbit string) string {
	return b.Build(profile, BuildContext{
		RAGContext: context,
		History:    history,
		Goal:       goal,
		Autonomous: autonomous,
		Orbit:      orbit,
		HasGraph:   true,  // comportamento antigo: sempre injetava
		HasLessons: true,  // comportamento antigo: sempre injetava
	})
}
