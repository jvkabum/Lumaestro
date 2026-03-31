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

// ExtractTriples extrai fatos estruturados de um texto usando o prompt TrustGraph.
func (s *OntologyService) ExtractTriples(ctx context.Context, text string) ([]Triple, error) {
	prompt := `Você é um especialista em extração de conhecimento estruturado (triplas).
Extraia triplas semânticas do texto abaixo no formato JSON.

## Classes: Person, Project, Task, Concept, Technology.
## Relações: works_on, uses, defines, part_of, mentions.

Texto:
` + text

	res, err := s.GenAI.Models.GenerateContent(ctx, "gemini-2.0-flash", []*genai.Content{{Parts: []*genai.Part{{Text: prompt}}}}, nil)
	if err != nil {
		return nil, fmt.Errorf("erro na extração de ontologia: %w", err)
	}

	if len(res.Candidates) > 0 && len(res.Candidates[0].Content.Parts) > 0 {
		return parseTriples(fmt.Sprintf("%v", res.Candidates[0].Content.Parts[0]))
	}
	return nil, nil
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
