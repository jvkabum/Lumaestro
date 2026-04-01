package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/genai"
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

// OntologyService gerencia a extração de fatos estruturados.
type OntologyService struct {
	GenAI *genai.Client
}

// NewOntologyService inicializa o serviço com o cliente Gemini.
func NewOntologyService(client *genai.Client) *OntologyService {
	return &OntologyService{GenAI: client}
}

// ExtractTriples extrai fatos estruturados de um texto usando o prompt TrustGraph, com suporte a desambiguação.
func (s *OntologyService) ExtractTriples(ctx context.Context, text string, contextHint string) ([]Triple, error) {
	prompt := fmt.Sprintf(`Extraia triplas semânticas (Sujeito-Predicado-Objeto) do texto abaixo.
Retorne APENAS um ARRAY JSON puro. NÃO use wrappers como {"triples": [...]}.
Exemplo exato do formato esperado:
[
  {"subject": "Lumaestro", "predicate": "uses", "object": "Qdrant"}
]

## DICA DE CONTEXTO GLOBAL:
Use esta informação para resolver pronomes como "ele", "ela", "o projeto", "a empresa": 
> %s

## BLUEPRINT OBRIGATÓRIO:
1. CLASSES: [Person, Project, Task, Concept, Technology, Milestone, Bug, Decision]
2. RELAÇÕES: [is_part_of, works_on, uses, defines, explains, mentions, created, resolved, depends_on]
3. REGRA: Use apenas os termos acima. Atomize os fatos.

Texto:
%s`, contextHint, text)

	res, err := s.GenAI.Models.GenerateContent(ctx, "gemini-2.0-flash", []*genai.Content{{Parts: []*genai.Part{{Text: prompt}}}}, nil)
	if err != nil {
		return nil, fmt.Errorf("erro na extração de ontologia: %w", err)
	}

	if len(res.Candidates) > 0 && len(res.Candidates[0].Content.Parts) > 0 {
		return parseTriples(fmt.Sprintf("%v", res.Candidates[0].Content.Parts[0]))
	}
	return nil, nil
}

// ValidateConflict atua como o "Agente da Verdade", decidindo entre informações contraditórias.
func (s *OntologyService) ValidateConflict(ctx context.Context, oldFact, newFact, contextStr string) (string, error) {
	prompt := fmt.Sprintf(`Você é o Agente Validador de Verdade do Lumaestro.
Detectamos um conflito de informação no Grafo de Conhecimento.

FATO ANTIGO: %s
FATO NOVO: %s
CONTEXTO RECENTE: %s

Sua tarefa:
Responda APENAS "UPDATE" se o Fato Novo for claramente uma atualização ou correção válida.
Responda APENAS "CONFLICT" se houver dúvida real ou se as informações forem contraditórias sem uma justificativa clara.

Decisão:`, oldFact, newFact, contextStr)

	res, err := s.GenAI.Models.GenerateContent(ctx, "gemini-2.0-flash", []*genai.Content{{Parts: []*genai.Part{{Text: prompt}}}}, nil)
	if err != nil {
		return "CONFLICT", err
	}

	if len(res.Candidates) > 0 && len(res.Candidates[0].Content.Parts) > 0 {
		decision := strings.TrimSpace(fmt.Sprintf("%v", res.Candidates[0].Content.Parts[0]))
		if strings.Contains(decision, "UPDATE") {
			return "UPDATE", nil
		}
	}
	return "CONFLICT", nil
}

// ProcessMedia extrai conhecimento de arquivos visuais ou documentos (Imagens/PDFs).
// Retorna (descrição, triplas, erro)
func (s *OntologyService) ProcessMedia(ctx context.Context, data []byte, mimeType string) (string, []Triple, error) {
	prompt := `Você é um especialista em visão computacional e extração de conhecimento.
Analise este arquivo. Forneça uma descrição detalhada e extraia triplas semânticas (Person, Project, Task, Concept, Technology).

Formato:
---DESCRICAO---
[Texto]
---TRIPLAS---
[JSON]`

	contents := []*genai.Content{
		{
			Parts: []*genai.Part{
				{InlineData: &genai.Blob{MIMEType: mimeType, Data: data}},
				{Text: prompt},
			},
		},
	}

	res, err := s.GenAI.Models.GenerateContent(ctx, "gemini-2.0-flash", contents, nil)
	if err != nil {
		return "", nil, fmt.Errorf("erro no processamento multimodal: %w", err)
	}

	if len(res.Candidates) > 0 && len(res.Candidates[0].Content.Parts) > 0 {
		fullText := fmt.Sprintf("%v", res.Candidates[0].Content.Parts[0])
		parts := strings.Split(fullText, "---TRIPLAS---")
		description := strings.TrimSpace(strings.TrimPrefix(parts[0], "---DESCRICAO---"))
		
		var triples []Triple
		if len(parts) > 1 {
			triples, _ = parseTriples(parts[1])
		}
		return description, triples, nil
	}
	return "", nil, fmt.Errorf("nenhum conteúdo gerado")
}

func parseTriples(rawJSON string) ([]Triple, error) {
	rawJSON = strings.TrimPrefix(rawJSON, "```json")
	rawJSON = strings.TrimPrefix(rawJSON, "```")
	rawJSON = strings.TrimSuffix(rawJSON, "```")
	rawJSON = strings.TrimSpace(rawJSON)

	var triples []Triple
	err := json.Unmarshal([]byte(rawJSON), &triples)
	return triples, err
}
