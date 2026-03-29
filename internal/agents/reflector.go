package agents

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/genai"
)

// Reflector analisa o sucesso das respostas da IA.
type Reflector struct {
	GenAI *genai.Client
}

// NewReflector inicializa o refletor com Gemini.
func NewReflector(client *genai.Client) *Reflector {
	return &Reflector{GenAI: client}
}

// ReflectResult contém o veredito da reflexão.
type ReflectResult struct {
	Success    bool
	Improvement string
	NewSkill   string
}

// Reflect analisa uma interação e decide se o sistema deve aprender algo novo.
func (r *Reflector) Reflect(ctx context.Context, query, response string) (*ReflectResult, error) {
	prompt := fmt.Sprintf(`Como especialista em qualidade de agentes de IA, analise a interação abaixo.
Pergunta: %s
Resposta: %s

O orquestrador foi preciso? Responda no formato:
SUCCESS: true/false
LEARNING: {descrição de como melhorar o contexto ou a estratégia}
NEW_SKILL: {se necessário, descreva uma nova regra de busca ou comportamento}`, query, response)

	res, err := r.GenAI.Models.GenerateContent(ctx, "gemini-2.0-flash", genai.Text(prompt), nil)
	if err != nil {
		return nil, fmt.Errorf("falha na reflexão: %w", err)
	}

	result := &ReflectResult{Success: true}
	if len(res.Candidates) > 0 && len(res.Candidates[0].Content.Parts) > 0 {
		analysis := fmt.Sprintf("%v", res.Candidates[0].Content.Parts[0])
		if strings.Contains(analysis, "SUCCESS: false") {
			result.Success = false
		}
		// Lógica simples para detectar nova skill
		if strings.Contains(analysis, "NEW_SKILL:") {
			result.NewSkill = strings.Split(analysis, "NEW_SKILL:")[1]
		}
	}

	return result, nil
}
