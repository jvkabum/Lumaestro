package core

import (
	"Lumaestro/internal/rag"
	"fmt"
	"strings"
)

// ============================================================
// 🧠 CÓRTEX RELACIONAL E VISUALIZAÇÃO (O MAPA)
// ============================================================

func (a *App) GetNeuralNodeContext(nodeID string) (map[string]interface{}, error) {
	fmt.Printf("[Audit] Buscando origem de: %s\n", nodeID)

	var result map[string]interface{}

	// 1. Tentar buscar em Notas do Obsidian ou Sistema (Chave: name)
	res, err := a.qdrant.SearchByField("obsidian_knowledge", "name", nodeID)

	if err == nil && res != nil {
		result = map[string]interface{}{
			"path":    res["path"],
			"content": res["content"],
			"type":    res["type"],
			"source":  res["document-type"], // Retorna se é "system" ou "vault"
		}
	} else {
		// 2. Tentar buscar em Memórias de Chat (Chave: subject)
		res, err = a.qdrant.SearchByField("knowledge_graph", "subject", nodeID)
		if err == nil && res != nil {
			result = map[string]interface{}{
				"path":    "Memória de Chat",
				"content": res["content"],
				"type":    "memory",
				"source":  "RAG Synapse",
			}
		} else {
			// 3. Fallback: Se não existe no banco, é uma dedução/especulação da IA (Nó Virtual)
			result = map[string]interface{}{
				"path":    "Conceito Neural",
				"content": fmt.Sprintf("O nó '%s' é um conceito abstrato detectado pela IA durante a tecelagem do conhecimento. Ele ainda não possui uma nota física dedicada no seu Obsidian.", nodeID),
				"type":    "virtual",
				"source":  "Inteligência Artificial",
			}
		}
	}

	// 🔍 Adiciona conexões relacionadas (Vizinhos) para o efeito de laços dourados
	if a.GEngine != nil {
		result["related_edges"] = a.GEngine.GetNeighborEdges(nodeID)
	} else {
		result["related_edges"] = []string{}
	}

	return result, nil
}


// AnalyzeGraphHealth analisa a integridade semântica do grafo.
func (a *App) AnalyzeGraphHealth() (map[string]interface{}, error) {
	fmt.Println("[Audit] Analisando saúde do Grafo de Contexto...")

	if a.qdrant == nil {
		return nil, fmt.Errorf("banco vetorial offline")
	}
	// 📂 Contagem Isolada por Projeto (Órbita Atual)
	count := 0
	if a.LStore != nil && a.executor.Workspace != "" {
		c, err := a.LStore.GetNodeCount(a.executor.Workspace)
		if err == nil {
			count = c
		}
	} else {
		// Fallback para global se não houver workspace (Obsidian Base)
		obsidianCount, _ := a.qdrant.CountPoints("obsidian_knowledge")
		memoryCount, _ := a.qdrant.CountPoints("knowledge_graph")
		count = obsidianCount + memoryCount
	}

	// Cálculo de Densidade Orgânica (Progressão Logarítmica)
	// Com 816 notas, queremos um valor que faça sentido visual.
	densityValue := 0.05 // Base 5%
	if count > 0 {
		// Quanto mais notas, mais o cérebro se torna denso (Log10)
		densityValue += (float64(count) / 1000.0) * 0.2 // Linear suave até 1000 notas
	}
	if densityValue > 1.0 {
		densityValue = 1.0
	}

	stats := map[string]interface{}{
		"density":      densityValue,
		"conflicts":    0,
		"active_nodes": count,
	}

	// 🧠 Dispara cálculos pesados do Cérebro Relacional de forma assíncrona ou síncrona dependendo da escala
	if a.GEngine != nil {
		a.GEngine.ComputePageRank()    // Centralidade de Autoridade
		a.GEngine.ComputeCommunities() // Afinidade Semântica
		a.GEngine.ComputeBetweenness() // Notas Ponte (Gargalos)
		a.GEngine.ComputeHITS()        // Hubs vs Authorities

		cycles := a.GEngine.DetectCycles()
		stats["conflicts"] = len(cycles)
		stats["communities"] = countCommunities(a.GEngine)
	}

	// Gatilho: Se o usuário pediu saúde, aproveitamos para tecer pontes neurais
	// Aumentamos o lote de processamento conforme o tamanho do cofre
	batchSize := 100
	if count > 500 {
		batchSize = 250
	}
	go a.WeaveNeuralLinks(batchSize)

	return stats, nil
}

// WeaveNeuralLinks percorre o grafo e cria conexões por similaridade (brain mapping).
func (a *App) WeaveNeuralLinks(limit int) {
	// ⚡ Captura local de referências (Escudo Anti-Panic)
	// Isso evita que o sistema quebre se os motores forem reiniciados durante a execução
	qdrant := a.qdrant
	embedder := a.embedder
	ctx := a.ctx

	if qdrant == nil || embedder == nil || ctx == nil {
		return
	}

	// 1. Busca as notas (as 50 mais recentes + uma amostra aleatória se possível)
	notes, err := qdrant.Search("obsidian_knowledge", nil, limit)
	if err != nil || len(notes) == 0 {
		return
	}

	for _, note := range notes {
		name, _ := note["name"].(string)
		content, _ := note["content"].(string)
		if name == "" || content == "" {
			continue
		}

		// 🛡️ Health Check antes de processar cada nota (caso o contexto tenha sido cancelado)
		select {
		case <-ctx.Done():
			return
		default:
		}

		// 2. Usamos o embedding para encontrar vizinhos próximos
		vector, err := embedder.GenerateEmbedding(ctx, content, false)
		if err != nil {
			continue
		}

		// 3. Busca os 5 vizinhos mais próximos (aumentado de 3 para 5)
		similars, err := qdrant.SearchWithScores("obsidian_knowledge", vector, 6)
		if err != nil {
			continue
		}

		for _, sim := range similars {
			targetName, _ := sim["name"].(string)
			score, _ := sim["_score"].(float64)

			// Filtro de Qualidade: Score > 0.82 (Sensibilidade ajustada)
			if targetName == "" || targetName == name || score < 0.82 {
				continue
			}

			// Emite link visual (Peso maior para similaridade alta)
			a.emitEvent("graph:edge", map[string]interface{}{
				"source": strings.ToLower(name),
				"target": strings.ToLower(targetName),
				"weight": int(score * 6), // Reforço visual
				"type":   "neural",
			})
		}
	}
	fmt.Println("[Neural] ✅ Tecelagem concluída para o lote.")
}

// 🧠 NEURAL BINDINGS: Métodos que expõem o aprendizado ativo para a UI

// HandleNodeClick recebe o feedback positivo (clique) e aplica reforço sináptico.
func (a *App) HandleNodeClick(nodeID string) {
	if a.ranker != nil {
		a.ranker.Reinforce(nodeID)

		a.emitEvent("agent:log", map[string]string{
			"source":  "NEURAL",
			"content": fmt.Sprintf("🧠 Reforço sináptico aplicado ao nó: %s", nodeID),
		})
	}
}

// SetExplorationMode ativa ou desativa o filtro neural no grafo.
func (a *App) SetExplorationMode(enabled bool) string {
	if a.ranker != nil {
		a.ranker.SetExplorationMode(enabled)
		if enabled {
			return "Modo Exploração Ativado (Pesos neurais ignorados)."
		}
		return "Modo Neural Ativado (Pesos aprendidos influenciam o grafo)."
	}
	return "Motor neural não inicializado."
}

// IsExplorationMode retorna o estado atual para sincronização da UI.
func (a *App) IsExplorationMode() bool {
	if a.ranker != nil {
		return a.ranker.IsExplorationMode()
	}
	return false
}

// RunReconScan dispara a busca por conexões perdidas e informa o frontend das propostas.
func (a *App) RunReconScan() string {
	if a.Recon == nil {
		return "🔴 Agente Recon offline."
	}

	proposals, err := a.Recon.ScanMissingLinks(a.ctx, a.executor.Workspace)
	if err != nil {
		return "🔴 Erro no Scan: " + err.Error()
	}

	count := 0
	for _, p := range proposals {
		if !p.Auto {
			count++
			a.emitEvent("agent:proposal", p)
		}
	}

	return fmt.Sprintf("🕵️🌐 Recon Scan concluído! %d novas sinapses propostas para sua revisão.", count)
}

// PruneGraph executa a poda neural baseada em PageRank para limpar o Dashboard.
func (a *App) PruneGraph(threshold float64) string {
	if a.GEngine == nil {
		return "🔴 Motor de grafos offline."
	}

	removed := a.GEngine.Prune(threshold)
	if len(removed) > 0 {
		// Sincroniza visualmente (Reset de Grafo no Front)
		a.SyncAllNodes()
		return fmt.Sprintf("🧹 Poda Neural concluída: %d nós irrelevantes removidos.", len(removed))
	}

	return "✅ O grafo já está otimizado (sem nós abaixo do threshold)."
}

// GetSkeletalGraph retorna apenas as arestas vitais (MST) para despoluir a visão.
func (a *App) GetSkeletalGraph() map[string]interface{} {
	if a.GEngine == nil {
		return nil
	}

	mstEdges := a.GEngine.GetMSTEdges()
	return map[string]interface{}{
		"edges": mstEdges,
	}
}

// helper para contar comunidades únicas
func countCommunities(ge *rag.GraphEngine) int {
	if ge == nil { return 0 }
	
	unique := make(map[int]struct{})
	// Precisamos expor ou acessar os IDs de comunidade
	for _, id := range ge.GetCommunityIDs() {
		unique[id] = struct{}{}
	}
	return len(unique)
}
