package agents

import (
	"context"
	"fmt"
	"strings"

	"Lumaestro/internal/prompts"
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
	prompt := prompts.GetReflectorPrompt(query, response)

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
