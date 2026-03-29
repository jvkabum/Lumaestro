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
	prompt := `Extraia triplas estruturadas do texto abaixo no formato JSON.
Cada tripla deve ter "subject", "predicate" e "object".
Use entidades e relações curtas e claras.

Texto:
` + text + `

Formato de Saída (JSON Array):
[{"subject": "A", "predicate": "B", "object": "C"}]`

	// Gerar triplas usando Gemini 2.0 Flash (alta velocidade para extração)
	res, err := s.GenAI.Models.GenerateContent(ctx, "gemini-2.0-flash", genai.Text(prompt), nil)
	if err != nil {
		return nil, fmt.Errorf("erro na extração de ontologia: %w", err)
	}

	// Parsing do JSON retornado
	if len(res.Candidates) > 0 && len(res.Candidates[0].Content.Parts) > 0 {
		rawJSON := fmt.Sprintf("%v", res.Candidates[0].Content.Parts[0])
		
		// Limpeza básica de markdown blocks se houver
		rawJSON = strings.TrimPrefix(rawJSON, "```json")
		rawJSON = strings.TrimSuffix(rawJSON, "```")
		rawJSON = strings.TrimSpace(rawJSON)

		var triples []Triple
		err := json.Unmarshal([]byte(rawJSON), &triples)
		if err != nil {
			return nil, fmt.Errorf("erro no parsing de triplas: %w", err)
		}
		return triples, nil
	}

	return nil, nil
}
