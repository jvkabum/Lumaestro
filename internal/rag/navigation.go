package rag

import (
	"context"
	"fmt"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"Lumaestro/internal/config"
	"Lumaestro/internal/provider"
)

// GraphNavigator gerencia a expansão de contexto baseada em links.
type GraphNavigator struct {
	Qdrant *provider.QdrantClient
}

// NewGraphNavigator inicializa o navegador com acesso à memória semântica.
func NewGraphNavigator(qdrant *provider.QdrantClient) *GraphNavigator {
	return &GraphNavigator{Qdrant: qdrant}
}

// ExpandContext busca as notas vizinhas de forma recursiva (com controle de depth e size).
func (n *GraphNavigator) ExpandContext(ctx context.Context, initialNotes []map[string]interface{}) []string {
	var fullContext []string
	visited := make(map[string]bool)

	// Carregar Limites Essenciais de Configuração
	cfg, err := config.Load()
	depthLimit := 1
	contextLimit := 4000
	if err == nil && cfg != nil {
		if cfg.GraphDepth > 0 {
			depthLimit = cfg.GraphDepth
		}
		if cfg.GraphContextLimit > 0 {
			contextLimit = cfg.GraphContextLimit
		}
	}

	totalChars := 0

	// 🧠 Fila para BFS (Breadth-First Search) no RAG
	type node struct {
		data  map[string]interface{}
		depth int
	}

	var queue []node
	for _, note := range initialNotes {
		queue = append(queue, node{data: note, depth: 0})
	}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		title, _ := current.data["name"].(string)
		if visited[title] || title == "" {
			continue
		}
		visited[title] = true
		
		// Evento Visual: Acende o nó brilhantemente ao iniciar sua leitura formal
		runtime.EventsEmit(ctx, "node:active", title)

		content, _ := current.data["content"].(string)

		// Tracker de Contexto: Evitar explosão de tokens que causam lentidão e custo
		if totalChars+len(content) > contextLimit && totalChars > 0 {
			fullContext = append(fullContext, fmt.Sprintf("[LIMITE EXCEDIDO] Vizinhos ou partes da rede foram omitidos para preservar seu foco e custo."))
			break
		}

		totalChars += len(content)
		fullContext = append(fullContext, fmt.Sprintf("=== Nota: %s ===\n%s", title, content))

		// 🧱 Expansão Baseada no Grafo (Links do Obsidian)
		if current.depth < depthLimit {
			if linksRaw, ok := current.data["links"].([]interface{}); ok {
				for _, linkRaw := range linksRaw {
					if linkName, ok := linkRaw.(string); ok {
						if !visited[linkName] {
							// Buscamos a nota conectada de forma cirúrgica na Collection do Qdrant
							neighborData, err := n.Qdrant.SearchByName("obsidian_knowledge", linkName)
							if err == nil && neighborData != nil {
								// Avisa Frontend das Viagens Visuais
								runtime.EventsEmit(ctx, "graph:log", fmt.Sprintf("[%s] 🔗 seguindo link → %s", time.Now().Format("15:04"), linkName))
								runtime.EventsEmit(ctx, "graph:edge", map[string]string{"source": title, "target": linkName})
								runtime.EventsEmit(ctx, "graph:node", map[string]string{"id": linkName, "name": linkName})

								// Adiciona o vizinho à fila para próxima iteração
								queue = append(queue, node{data: neighborData, depth: current.depth + 1})
							}
						}
					}
				}
			}
		}

		// ⚡ Navegação de Sinapses Mistas (Ontologia Extrapolada)
		if current.depth == 0 { // Triplas são focadas no núcleo para não gerar devaneios e "alucinação mista"
			synapses, err := n.Qdrant.Search("knowledge_graph", nil, 3) // Simulado: Buscando por similaridade nula temporariamente
			if err == nil {
				for _, syn := range synapses {
					subj, _ := syn["subject"].(string)
					obj, _ := syn["object"].(string)
					
					if subj == title || obj == title {
						fact, _ := syn["content"].(string)
						synapseStr := fmt.Sprintf("[SINAPSE APRENDIDA]: %s", fact)
						
						if totalChars+len(synapseStr) < contextLimit {
							fullContext = append(fullContext, synapseStr)
							totalChars += len(synapseStr)
						}
					}
				}
			}
		}
	}

	return fullContext
}
