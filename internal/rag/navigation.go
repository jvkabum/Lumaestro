package rag

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"Lumaestro/internal/config"
	"Lumaestro/internal/provider"
	"Lumaestro/internal/rag/neural"
	"Lumaestro/internal/utils"
)

// ScoredNode representa um resultado de busca com sua pontuação de relevância.
type ScoredNode struct {
	ID      string
	Name    string
	Content string
	Type    string
	Score   int
	Raw     map[string]interface{}
}

// NavStore define a interface mínima necessária para o DuckDBStore no navegador.
type NavStore interface {
	SearchNodesByKeyword(keyword string, allowedPaths []string, limit int) ([]map[string]interface{}, error)
	GetNeighbors(nodeID string, allowedPaths []string) ([]map[string]interface{}, error)
	FindNodeInText(text string, allowedPaths []string) (string, error)
}

// GraphNavigator gerencia a expansão de contexto baseada em links com suporte a Destaque Visual.
type GraphNavigator struct {
	ctx    context.Context // Contexto persistente do Wails
	Qdrant *provider.QdrantClient
	Ranker *neural.Ranker
	LStore NavStore
}

var stopWords = map[string]bool{
	"o": true, "a": true, "os": true, "as": true, "de": true, "do": true, "da": true,
	"dos": true, "das": true, "em": true, "no": true, "na": true, "nos": true, "nas": true,
	"um": true, "uma": true, "uns": true, "umas": true, "ao": true, "aos": true,
	"que": true, "me": true, "meu": true, "minha": true, "seu": true, "sua": true,
	"ele": true, "ela": true, "eles": true, "isso": true, "este": true, "esta": true,
	"fale": true, "sobre": true, "quem": true, "onde": true, "como": true,
	"explica": true, "explique": true, "quero": true, "preciso": true, "mostre": true,
	"mostra": true, "diga": true, "fala": true, "oque": true, "qual": true,
	"para": true, "por": true, "com": true, "sem": true, "mais": true, "muito": true,
	"tem": true, "ter": true, "ser": true, "está": true, "pode": true, "vai": true,
	"the": true, "is": true, "are": true, "was": true, "what": true, "how": true,
	"who": true, "where": true, "when": true, "this": true, "that": true,
	"and": true, "for": true, "with": true, "from": true, "can": true, "has": true,
	"have": true, "does": true, "show": true, "tell": true, "about": true,
}

// SetContext injeta o contexto oficial do Wails.
func (n *GraphNavigator) SetContext(ctx context.Context) {
	n.ctx = ctx
}

// NewGraphNavigatorV2 inicializa o navegador com foco em Trajetória Semântica e Aprendizado Ativo.
func NewGraphNavigatorV2(qdrant *provider.QdrantClient, ranker *neural.Ranker, lStore NavStore) *GraphNavigator {
	return &GraphNavigator{
		Qdrant: qdrant,
		Ranker: ranker,
		LStore: lStore,
	}
}

// SearchByKeyword realiza uma busca direta no Qdrant filtrando por termos contidos na pergunta, restrito às órbitas autorizadas.
func (n *GraphNavigator) SearchByKeyword(ctx context.Context, input string, allowedPaths []string) []map[string]interface{} {
	fmt.Printf("[RADAR] 📡 Iniciando Deep Scan Híbrido: \"%s\"\n", input)

	cleanWords := n.extractKeywords(input)
	if len(cleanWords) == 0 {
		return []map[string]interface{}{}
	}

	results := make([]map[string]interface{}, 0)
	seenIDs := make(map[string]bool)

	// 1. FAST PATH: DuckDB (Radar Relacional Multi-Órbita)
	if n.LStore != nil && len(allowedPaths) > 0 {
		dbResults, err := n.LStore.SearchNodesByKeyword(input, allowedPaths, 30)
		if err == nil {
			for _, res := range dbResults {
				nodeID, ok := res["id"].(string)
				if !ok || nodeID == "" || seenIDs[nodeID] {
					continue
				}
				fullNode, err := n.Qdrant.SearchByField("obsidian_knowledge", "id", nodeID)
				if err == nil && fullNode != nil {
					results = append(results, fullNode)
					seenIDs[nodeID] = true
				}
			}
			if len(results) >= 3 {
				fmt.Printf("[RADAR] ✅ DuckDB entregou %d resultados de elite.\n", len(results))
				return results
			}
		}
	}

	// 2. DEEP SCAN: Qdrant + Ranking Manual (Filtrado por Órbita)
	scored := make([]ScoredNode, 0)
	collections := []string{"obsidian_knowledge", "knowledge_graph"}

	for _, col := range collections {
		candidates, err := n.Qdrant.Search(col, nil, 1000)
		if err != nil {
			fmt.Printf("[DEBUG-RAG] ⚠️ Erro ao varrer coleção %s: %v\n", col, err)
			continue
		}

		for _, raw := range candidates {
			id := n.safeID(raw)
			if id == "" || seenIDs[id] {
				continue
			}

			// FILTRO DE SOBERANIA MULTI-ÓRBITA
			if col == "obsidian_knowledge" && len(allowedPaths) > 0 {
				nodePath, _ := raw["path"].(string)
				found := false
				for _, p := range allowedPaths {
					if nodePath != "" && strings.HasPrefix(nodePath, p) {
						found = true
						break
					}
				}
				if !found { continue }
			}

			// Validação estrita de nome/sujeito
			name, ok := raw["name"].(string)
			if !ok || name == "" {
				name, ok = raw["subject"].(string) // Fallback para memória
			}
			if !ok || name == "" { continue }
			
			content, _ := raw["content"].(string)
			score := n.calculateScore(name, content, cleanWords)
			if score > 0 {
				if col == "knowledge_graph" {
					raw["document-type"] = "memory"
					raw["name"] = name
				}
				scored = append(scored, ScoredNode{
					ID: id, Name: name, Content: content, Score: score, Raw: raw,
				})
				seenIDs[id] = true
			}
		}
	}

	// Ordenação por relevância
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})

	final := make([]map[string]interface{}, 0, len(scored))
	for i, s := range scored {
		if i == 0 {
			fmt.Printf("[RADAR] 🏆 Top Match: \"%s\" (Score: %d)\n", s.Name, s.Score)
		}
		final = append(final, s.Raw)
	}

	return final
}

// Auxiliares de Refatoração

func (n *GraphNavigator) extractKeywords(input string) []string {
	input = n.removeAccents(strings.ToLower(input))
	words := strings.Fields(input)
	clean := make([]string, 0)
	for _, w := range words {
		cw := strings.Trim(w, "?!.,;\"' ")
		if len(cw) >= 3 && !stopWords[cw] {
			clean = append(clean, cw)
		}
	}
	return clean
}

func (n *GraphNavigator) calculateScore(name, content string, keywords []string) int {
	score := 0
	lowerName := n.removeAccents(strings.ToLower(name))
	lowerContent := n.removeAccents(strings.ToLower(content))

	for _, kw := range keywords {
		if strings.Contains(lowerName, kw) {
			score += 100
			if lowerName == kw {
				score += 50
			}
		}
		if strings.Contains(lowerContent, kw) {
			score += 10
		}
	}
	return score
}

func (n *GraphNavigator) removeAccents(s string) string {
	r := strings.NewReplacer(
		"á", "a", "à", "a", "â", "a", "ã", "a", "ä", "a",
		"é", "e", "è", "e", "ê", "e", "ë", "e",
		"í", "i", "ì", "i", "î", "i", "ï", "i",
		"ó", "o", "ò", "o", "ô", "o", "õ", "o", "ö", "o",
		"ú", "u", "ù", "u", "û", "u", "ü", "u",
		"ç", "c",
	)
	return r.Replace(s)
}

func (n *GraphNavigator) safeID(node map[string]interface{}) string {
	if id, ok := node["id"].(string); ok && id != "" {
		return id
	}
	if id, ok := node["id"].(float64); ok {
		return fmt.Sprintf("id-%v", id)
	}
	return ""
}

// FindNodeInText identifica um nó citado no texto, restrito aos caminhos autorizados.
func (n *GraphNavigator) FindNodeInText(ctx context.Context, text string, allowedPaths []string) string {
	id, err := n.LStore.FindNodeInText(text, allowedPaths)
	if err != nil {
		return ""
	}
	return id
}

// ExpandContext realiza uma travessia inteligente e emite a "Trilha de Raciocínio" para o frontend, restrito às órbitas autorizadas.
func (n *GraphNavigator) ExpandContext(ctx context.Context, initialNotes []map[string]interface{}, allowedPaths []string) []string {
	var fullContext []string
	visited := make(map[string]bool)

	cfg, _ := config.Load()
	depthLimit := 1
	neighborLimit := 5
	contextLimit := 8000
	if cfg != nil {
		if cfg.GraphDepth > 0 { depthLimit = cfg.GraphDepth }
		if cfg.GraphNeighborLimit > 0 { neighborLimit = cfg.GraphNeighborLimit }
		if cfg.GraphContextLimit > 0 { contextLimit = cfg.GraphContextLimit }
	}

	totalChars := 0
	activePath := ""
	if len(allowedPaths) > 0 {
		activePath = allowedPaths[0]
	}

	// 🚀 FASE 1: Processar Núcleos (Depth 0)
	for _, note := range initialNotes {
		name, _ := note["name"].(string)
		id := n.safeID(note)
		nodePath, _ := note["workspace_path"].(string)

		if name != "" {
			visited[name] = true
			content, _ := note["content"].(string)
			
			// Identificação de Órbita
			originLabel := "[PROJETO ATUAL]"
			if activePath != "" && nodePath != "" && !strings.HasPrefix(nodePath, activePath) {
				originLabel = "[REFERÊNCIA EXTERNA]"
			}

			fullContext = append(fullContext, fmt.Sprintf("=== %s: %s ===\n%s", originLabel, name, content))
			totalChars += len(content)

			// ✨ Eventos de Zoom e Destaque Visual
			utils.SafeEmit(n.ctx, "node:active", name)
			utils.SafeEmit(n.ctx, "graph:highlight", map[string]interface{}{
				"name": name,
				"id":   id,
			})

			// 🚀 FASE 2: Expansão de Vizinhança (N-Hop) guiada pelo Grafo Visual
			if depthLimit > 0 {
				type TrailHop struct {
					From   string  `json:"from"`
					To     string  `json:"to"`
					Weight float32 `json:"weight"` 
				}
				var trail []TrailHop

				// 1. Expansão via Links Explícitos (Obsidian Style)
				if links, ok := note["links"].([]interface{}); ok {
					for _, linkIntf := range links {
						if len(trail) >= neighborLimit { break }
						linkName, ok := linkIntf.(string)
						if !ok || visited[linkName] { continue }
						
						nb, err := n.Qdrant.SearchByField("obsidian_knowledge", "name", linkName)
						if err == nil && nb != nil {
							sNodePath, _ := nb["path"].(string)
							
							// Filtro de Órbita
							found := false
							for _, p := range allowedPaths {
								if sNodePath != "" && strings.HasPrefix(sNodePath, p) {
									found = true
									break
								}
							}
							if !found { continue }

							visited[linkName] = true
							sName, _ := nb["name"].(string)
							sContent, _ := nb["content"].(string)

							if totalChars+len(sContent) > contextLimit { continue }

							sOriginLabel := "[PROJETO ATUAL]"
							if activePath != "" && sNodePath != "" && !strings.HasPrefix(sNodePath, activePath) {
								sOriginLabel = "[REFERÊNCIA EXTERNA]"
							}

							fullContext = append(fullContext, fmt.Sprintf("=== %s (Relacionado): %s ===\n%s", sOriginLabel, sName, sContent))
							totalChars += len(sContent)
							trail = append(trail, TrailHop{From: name, To: sName, Weight: n.Ranker.GetWeight(sName)})
						}
					}
				}
				
				// 🧠 2. Expansão via Sinapses Semânticas (ID Vinculador)
				if n.LStore != nil && id != "" {
					semanticNeighbors, err := n.LStore.GetNeighbors(id, allowedPaths)
					if err == nil {
						for _, snb := range semanticNeighbors {
							if len(trail) >= neighborLimit { break }
							sName, _ := snb["name"].(string)
							if sName == "" || visited[sName] { continue }

							sContent, _ := snb["content"].(string)
							sNodePath, _ := snb["workspace_path"].(string)

							if totalChars+len(sContent) > contextLimit { continue }

							visited[sName] = true
							sOriginLabel := "[PROJETO ATUAL]"
							if activePath != "" && sNodePath != "" && !strings.HasPrefix(sNodePath, activePath) {
								sOriginLabel = "[REFERÊNCIA EXTERNA]"
							}

							fullContext = append(fullContext, fmt.Sprintf("=== %s (Vinculado): %s ===\n%s", sOriginLabel, sName, sContent))
							totalChars += len(sContent)
							trail = append(trail, TrailHop{From: name, To: sName, Weight: 0.3})
						}
					}
				}
				
				if len(trail) > 0 {
					utils.SafeEmit(n.ctx, "graph:traverse", map[string]interface{}{
						"hops":  trail,
						"total": len(trail),
					})
				}
			}
		}
	}

	// 🚀 FASE 3: Sinapses de Chat (Memória Longa)
	// Filtramos sinapses para garantir que apenas memórias vinculadas às órbitas autorizadas apareçam
	synapses, err := n.Qdrant.Search("knowledge_graph", nil, 20) // Aumentado para compensar filtros
	if err == nil && synapses != nil {
		for _, syn := range synapses {
			fact, _ := syn["content"].(string)
			wsPath, _ := syn["workspace_path"].(string)

			// FILTRO DE SOBERANIA DE MEMÓRIA
			if len(allowedPaths) > 0 {
				found := false
				for _, p := range allowedPaths {
					if wsPath != "" && strings.HasPrefix(wsPath, p) {
						found = true
						break
					}
				}
				if !found { continue }
			} else {
				continue // Modo Cego total
			}

			if fact != "" && totalChars+len(fact) < contextLimit {
				fullContext = append(fullContext, fmt.Sprintf("[SINAPSE]: %s", fact))
				totalChars += len(fact)
			}
		}
	}

	return fullContext
}
