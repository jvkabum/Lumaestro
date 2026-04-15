package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"Lumaestro/internal/prompts"
)

// Triple representa a unidade básica de conhecimento semântico.
type Triple struct {
	Subject   string `json:"subject"`
	Predicate string `json:"predicate"`
	Object    string `json:"object"`
}

// Entity representa um nó no grafo do Lumaestro.
type Entity struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Label       string                 `json:"label"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// Relation representa uma aresta conectando duas entidades.
type Relation struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Predicate string `json:"predicate"`
}

// OntologyService gerencia a extração de fatos estruturados via qualquer motor generativo.
type OntologyService struct {
	Embedder  ContentGenerator
	ctx       context.Context
}

// NewOntologyService inicializa o serviço com um motor generativo (Gemini ou LM Studio).
func NewOntologyService(ctx context.Context, embedder ContentGenerator) *OntologyService {
	return &OntologyService{Embedder: embedder, ctx: ctx}
}

// ExtractTriples extrai fatos estruturados usando a API oficial do GenAI (bypassando o frágil CLI).
func (s *OntologyService) ExtractTriples(ctx context.Context, text string, contextHint string) ([]Triple, error) {
	prompt := prompts.GetNeuroSymbolicExtractorPrompt(contextHint, text)

	if s.Embedder == nil {
		return nil, fmt.Errorf("serviço de motor generativo (Embedder) indisponível")
	}

	responseText, err := s.Embedder.GenerateText(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("falha na extração de triplas (Motor Resiliente): %w", err)
	}
	if responseText == "" {
		return nil, fmt.Errorf("resposta sem texto do motor generativo")
	}

	return parseTriples(responseText)
}

// ValidateConflict decide entre informações contraditórias usando API nativa GenAI.
func (s *OntologyService) ValidateConflict(ctx context.Context, oldFact, newFact, contextStr string) (string, error) {
	prompt := prompts.GetConflictValidatorPrompt(oldFact, newFact, contextStr)

	if s.Embedder == nil {
		return "CONFLICT", fmt.Errorf("motor generativo indisponível")
	}

	resText, err := s.Embedder.GenerateText(ctx, prompt)
	if err != nil {
		return "CONFLICT", err
	}

	if strings.Contains(strings.ToUpper(resText), "UPDATE") {
		return "UPDATE", nil
	}
	return "CONFLICT", nil
}

// ProcessMedia extrai conhecimento de arquivos visuais via API Nativa GenAI.
func (s *OntologyService) ProcessMedia(ctx context.Context, data []byte, mimeType string) (string, []Triple, error) {
	prompt := `Analise este arquivo. Forneça uma descrição detalhada e extraia triplas semânticas (Sujeito, Predicado, Objeto).
	
Formato:
---DESCRICAO---
[Texto]
---TRIPLAS---
[JSON]`

	// 📸 Delega ao motor generativo — suporta multimodal (Gemini) ou fallback de texto (LM Studio).
	response, err := s.Embedder.GenerateMultimodalText(ctx, prompt, data, mimeType)
	if err != nil {
		return "", nil, fmt.Errorf("falha ao processar media multimodal: %w", err)
	}
	if response == "" {
		return "", nil, fmt.Errorf("resposta vazia no ProcessMedia")
	}

	parts := strings.Split(response, "---TRIPLAS---")
	description := strings.TrimSpace(strings.TrimPrefix(parts[0], "---DESCRICAO---"))
	
	var triples []Triple
	if len(parts) > 1 {
		triples, _ = parseTriples(parts[1])
	}
	return description, triples, nil
}

func repairJSON(s string) string {
	// Remove blocos <thought>, <think>, etc (comuns no Qwen3/DeepSeek)
	s = regexp.MustCompile(`(?s)<thought>.*?</thought>`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`(?s)<think>.*?</think>`).ReplaceAllString(s, "")
	
	// Converte aspas simples para duplas (RE2 compatível, sem lookarounds)
	// Captura o texto interno $1 e o caractere delimitador (+ espaços) $2
	s = regexp.MustCompile(`'([^']*)'(\s*[:,}\]])`).ReplaceAllString(s, `"$1"$2`)
	
	// Remove vírgulas trailing antes de ] ou } que quebram o Go JSON
	s = regexp.MustCompile(`,(\s*[\]}])`).ReplaceAllString(s, "$1")
	
	return strings.TrimSpace(s)
}

func parseTriples(rawJSON string) ([]Triple, error) {
	// 🛡️ Limpeza Avançada: Regex repair para remover thoughts e consertar varreduras ruidosas
	rawJSON = repairJSON(rawJSON)

	// 🛡️ Blindagem contra modelos reasoning que usam preambles
	startIndex := strings.Index(rawJSON, "[")
	endIndex := strings.LastIndex(rawJSON, "]")

	if startIndex != -1 && endIndex != -1 && endIndex > startIndex {
		rawJSON = rawJSON[startIndex : endIndex+1]
	} else {
		// A Inteligência não gerou nenhum array (provável recusa de leitura do documento)
		rawLog := rawJSON
		if len(rawLog) > 100 {
			rawLog = rawLog[:100] + "..."
		}
		fmt.Printf("[RAG] 🛡️ Modelo recusou formatação JSON ou não encontrou fatos. RAW: %s\n", rawLog)
		return nil, fmt.Errorf("ausência de array JSON na resposta do gerador")
	}

	var triples []Triple
	err := json.Unmarshal([]byte(rawJSON), &triples)
	if err != nil {
		fmt.Printf("[RAG] ⚠️ IA alucinou texto que não é JSON: %s\n", rawJSON[:minCustom(len(rawJSON), 60)])
		return nil, nil // Silencia o erro, tratando como arquivo sem triplas extraíveis úteis
	}
	return triples, nil
}

// Helper min() tolerante a versões de compilador antigas.
func minCustom(a, b int) int {
	if a < b {
		return a
	}
	return b
}
