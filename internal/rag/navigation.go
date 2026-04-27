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
	SearchNodesByKeyword(keyword string, limit int) ([]map[string]interface{}, error)
	GetNeighbors(nodeID string) ([]map[string]interface{}, error)
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

// ExpandContext realiza uma travessia inteligente e emite a "Trilha de Raciocínio" para o frontend.
// SearchByKeyword realiza uma busca direta no Qdrant filtrando por termos contidos na pergunta.
// Usado como fallback automático quando a API de Embeddings falha (Quota/429).
func (n *GraphNavigator) SearchByKeyword(ctx context.Context, input string) []map[string]interface{} {
	fmt.Printf("[RADAR] 📡 Iniciando Deep Scan Híbrido: \"%s\"\n", input)

	cleanWords := n.extractKeywords(input)
	if len(cleanWords) == 0 {
		return []map[string]interface{}{}
	}

	results := make([]map[string]interface{}, 0)
	seenIDs := make(map[string]bool)

	// 1. FAST PATH: DuckDB (Radar Relacional)
	if n.LStore != nil {
		dbResults, err := n.LStore.SearchNodesByKeyword(input, 30)
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
			if len(results) >= 3 { // Reduzi para 3 para ser mais ágil
				fmt.Printf("[RADAR] ✅ DuckDB entregou %d resultados de elite.\n", len(results))
				return results
			}
		}
	}

	// 2. DEEP SCAN: Qdrant + Ranking Manual
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

			// Validação estrita de nome/sujeito
			name, ok := raw["name"].(string)
			if !ok || name == "" {
				name, ok = raw["subject"].(string) // Fallback para memória
			}
			if !ok || name == "" {
				continue
			} // Ignora nós sem identificação válida
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
	// Normalização: Minúsculas + Remoção de Acentos (Simplificado para evitar dependências externas pesadas)
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
	// Normaliza para comparação (ignora acentos no ranking)
	lowerName := n.removeAccents(strings.ToLower(name))
	lowerContent := n.removeAccents(strings.ToLower(content))

	for _, kw := range keywords {
		if strings.Contains(lowerName, kw) {
			score += 100
			if lowerName == kw {
				score += 50
			} // Match exato
		}
		if strings.Contains(lowerContent, kw) {
			score += 10
		}
	}
	return score
}

func (n *GraphNavigator) removeAccents(s string) string {
	// Replacer focado em português para garantir recall em "ação", "vovô", etc.
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
		// Formata com precisão para evitar colisões em números grandes
		return fmt.Sprintf("id-%v", id)
	}
	return ""
}

func (n *GraphNavigator) ExpandContext(ctx context.Context, initialNotes []map[string]interface{}) []string {
	var fullContext []string
	visited := make(map[string]bool)

	cfg, _ := config.Load()
	depthLimit := 1
	neighborLimit := 5
	contextLimit := 4000
	if cfg != nil {
		if cfg.GraphDepth > 0 {
			depthLimit = cfg.GraphDepth
		}
		if cfg.GraphNeighborLimit > 0 {
			neighborLimit = cfg.GraphNeighborLimit
		}
		if cfg.GraphContextLimit > 0 {
			contextLimit = cfg.GraphContextLimit
		}
	}

	totalChars := 0

	// 🚀 FASE 1: Processar Núcleos (Depth 0)
	for _, note := range initialNotes {
		name, _ := note["name"].(string)
		id := n.safeID(note)

		if name != "" {
			visited[name] = true
			content, _ := note["content"].(string)
			fullContext = append(fullContext, fmt.Sprintf("=== [NÚCLEO]: %s ===\n%s", name, content))
			totalChars += len(content)

			// ✨ RESTAURAÇÃO: Eventos de Zoom e Destaque Visual para o Frontend
			utils.SafeEmit(n.ctx, "node:active", name)
			utils.SafeEmit(n.ctx, "graph:highlight", map[string]interface{}{
				"name": name,
				"id":   id,
			})
		}
	}

	// 🚀 FASE 2: Expansão de Vizinhança (N-Hop) guiada pelo Grafo Visual
	if depthLimit > 0 {
		type TrailHop struct {
			From   string  `json:"from"`
			To     string  `json:"to"`
			Weight float32 `json:"weight"` // Peso aprendido para visualização visual
		}
		var trail []TrailHop

		for _, note := range initialNotes {
			parentName, _ := note["name"].(string)
			if links, ok := note["links"].([]interface{}); ok {
				for _, linkIntf := range links {
					if len(trail) >= neighborLimit {
						break
					}

					linkName, ok := linkIntf.(string)
					if !ok {
						continue
					}

					if visited[linkName] {
						continue
					}
					visited[linkName] = true

					nb, err := n.Qdrant.SearchByField("obsidian_knowledge", "name", linkName)
					if err == nil && nb != nil {
						name, _ := nb["name"].(string)
						content, _ := nb["content"].(string)

						if totalChars+len(content) > contextLimit {
							continue
						}

						fullContext = append(fullContext, fmt.Sprintf("=== [CONTEXTO_RELACIONADO]: %s ===\n%s", name, content))
						totalChars += len(content)

						neuralWeight := n.Ranker.GetWeight(name)
						trail = append(trail, TrailHop{From: parentName, To: name, Weight: neuralWeight})
					}
				}
			}

			// 🧠 [NOVO] Expansão via Sinapses Semânticas (ID Vinculador)
			// Busca vizinhos implícitos que citam este nó ou são citados por ele
			if n.LStore != nil {
				nodeID := n.safeID(note)
				if nodeID != "" {
					semanticNeighbors, err := n.LStore.GetNeighbors(nodeID)
					if err == nil {
						for _, snb := range semanticNeighbors {
							if len(trail) >= neighborLimit {
								break
							}

							sName, _ := snb["name"].(string)
							if sName == "" || visited[sName] {
								continue
							}

							sContent, _ := snb["content"].(string)
							if totalChars+len(sContent) > contextLimit {
								continue
							}

							visited[sName] = true
							fullContext = append(fullContext, fmt.Sprintf("=== [REFERÊNCIA_VINCULADA]: %s ===\n%s", sName, sContent))
							totalChars += len(sContent)

							// Adiciona à trilha visual (mas como é semântico, o frontend não desenha linhas se não quiser)
							trail = append(trail, TrailHop{
								From:   parentName,
								To:     sName,
								Weight: 0.3, // Peso menor para vínculos implícitos
							})
						}
					}
				}
			}
		}

		// 🚀 Emite o percurso completo como uma única mensagem animável no frontend
		if len(trail) > 0 {
			utils.SafeEmit(n.ctx, "graph:traverse", map[string]interface{}{
				"hops":  trail,
				"total": len(trail),
			})
		}
	}

	// 🚀 FASE 3: Sinapses de Chat (Memória Longa)
	// (Busca os últimos fatos relevantes da sessão ou conhecimentos consolidados)
	synapses, err := n.Qdrant.Search("knowledge_graph", nil, 5)
	if err == nil && synapses != nil {
		for _, syn := range synapses {
			fact, _ := syn["content"].(string)
			if fact != "" && totalChars+len(fact) < contextLimit {
				fullContext = append(fullContext, fmt.Sprintf("[SINAPSE]: %s", fact))
				totalChars += len(fact)
			}
		}
	} else if err != nil {
		fmt.Printf("[DEBUG-RAG] ⚠️ Falha ao buscar sinapses (ignorando): %v\n", err)
	}

	return fullContext
}
