package rag

import (
	"fmt"
	"math"
	"sync"

	"gonum.org/v1/gonum/graph"

	"gonum.org/v1/gonum/graph/community"
	"gonum.org/v1/gonum/graph/network"
	"gonum.org/v1/gonum/graph/path"
	"gonum.org/v1/gonum/graph/simple"
	"gonum.org/v1/gonum/graph/topo"
)

// Node representa um ponto no conhecimento (Nota, Agente, Task)
type Node struct {
	ID   string
	Name string
	Type string // "note", "agent", "source"
}

// Edge representa uma conexão semântica
type Edge struct {
	Source string
	Target string
	Weight float64
	Label  string
}

// GraphEngine é o cérebro relacional nativo do Lumaestro (V20).
// Utiliza a Gonum para matemática pesada e Adjacência nativa para velocidade de navegação.
type GraphEngine struct {
	mu           sync.RWMutex
	nodes        map[string]*Node
	adj          map[string]map[string]float64 // Lista de Adjacência em RAM com pesos
	pageRank     map[string]float64
	
	// Gonum integration for heavy math (v0.17.0 Weighted)
	gonumGraph   *simple.WeightedDirectedGraph
	nodeToGonum  map[string]int64
	gonumToNode  map[int64]string
	
	// 🏷️ Predicados (Rótulos) - Armazena a relação semântica entre pares
	labels       map[string]string

	// 📊 Novas Métricas de Elite
	betweenness  map[string]float64
	communities  map[string]int // NodeID -> CommunityID
	hubs         map[string]float64
	authorities  map[string]float64
}

// NewGraphEngine inicializa o motor de elite com suporte nativo a pesos.
func NewGraphEngine() *GraphEngine {
	return &GraphEngine{
		nodes:       make(map[string]*Node),
		adj:         make(map[string]map[string]float64),
		pageRank:    make(map[string]float64),
		gonumGraph:  simple.NewWeightedDirectedGraph(0, math.Inf(1)),
		nodeToGonum: make(map[string]int64),
		gonumToNode: make(map[int64]string),
		labels:      make(map[string]string),
		betweenness: make(map[string]float64),
		communities: make(map[string]int),
		hubs:        make(map[string]float64),
		authorities: make(map[string]float64),
	}
}

// AddNode insere ou atualiza um nó no grafo.
func (ge *GraphEngine) AddNode(id, name, docType string) {
	ge.mu.Lock()
	defer ge.mu.Unlock()

	if _, exists := ge.nodes[id]; !exists {
		ge.nodes[id] = &Node{ID: id, Name: name, Type: docType}
		
		// Map for Gonum
		gID := int64(len(ge.nodeToGonum))
		ge.nodeToGonum[id] = gID
		ge.gonumToNode[gID] = id
		ge.gonumGraph.AddNode(simple.Node(gID))
	}
}

// AddEdge cria uma sinapse com peso e rótulo (Explicação semântica).
func (ge *GraphEngine) AddEdge(sourceID, targetID string, weight float64, label string) {
	ge.mu.Lock()
	defer ge.mu.Unlock()

	// Garante que os nós existem (Sem recursão de trava)
	if _, ok := ge.nodes[sourceID]; !ok {
		gID := int64(len(ge.nodeToGonum))
		ge.nodeToGonum[sourceID] = gID
		ge.gonumToNode[gID] = sourceID
		ge.nodes[sourceID] = &Node{ID: sourceID, Name: sourceID, Type: "unknown"}
		ge.gonumGraph.AddNode(simple.Node(gID))
	}
	if _, ok := ge.nodes[targetID]; !ok {
		gID := int64(len(ge.nodeToGonum))
		ge.nodeToGonum[targetID] = gID
		ge.gonumToNode[gID] = targetID
		ge.nodes[targetID] = &Node{ID: targetID, Name: targetID, Type: "unknown"}
		ge.gonumGraph.AddNode(simple.Node(gID))
	}

	// Adjacência Interna com Suporte a Predicados (Rótulos)
	if _, ok := ge.adj[sourceID]; !ok {
		ge.adj[sourceID] = make(map[string]float64)
	}
	ge.adj[sourceID][targetID] = weight
	ge.labels[fmt.Sprintf("%s-%s", sourceID, targetID)] = label

	// Aresta Gonum (Sincroniza peso real para algoritmos de elite)
	u := ge.nodeToGonum[sourceID]
	v := ge.nodeToGonum[targetID]
	ge.gonumGraph.SetWeightedEdge(simple.WeightedEdge{
		F: simple.Node(u),
		T: simple.Node(v),
		W: weight,
	})
}

// ComputePageRank calcula a autoridade das notas (Algoritmo de Elite Google).
func (ge *GraphEngine) ComputePageRank() {
	ge.mu.Lock()
	defer ge.mu.Unlock()

	if len(ge.nodes) == 0 {
		return
	}

	// PageRank (Power Iteration via Gonum)
	results := network.PageRank(ge.gonumGraph, 0.85, 1e-6)
	
	for gID, rank := range results {
		id := ge.gonumToNode[int64(gID)]
		ge.pageRank[id] = rank
	}
	
	fmt.Printf("[GraphEngine] 🧠 PageRank recalculado para %d nós\n", len(ge.pageRank))
}

// ComputeBetweenness calcula notas que servem de "ponte" entre diferentes áreas.
func (ge *GraphEngine) ComputeBetweenness() {
	ge.mu.Lock()
	defer ge.mu.Unlock()

	if len(ge.nodes) == 0 { return }

	// Betweenness Centrality (Brandes Algorithm)
	results := network.Betweenness(ge.gonumGraph)
	
	for gID, b := range results {
		id := ge.gonumToNode[int64(gID)]
		ge.betweenness[id] = b
	}
}

// ComputeCommunities agrupa notas por afinidade semântica (Algoritmo de Louvain).
func (ge *GraphEngine) ComputeCommunities() {
	ge.mu.Lock()
	defer ge.mu.Unlock()

	if len(ge.nodes) == 0 { return }

	// Louvain exige um grafo não-direcionado para clustering por afinidade.
	undirect := simple.NewWeightedUndirectedGraph(0, math.Inf(1))
	nodes := ge.gonumGraph.Nodes()
	for nodes.Next() {
		undirect.AddNode(nodes.Node())
	}
	edges := ge.gonumGraph.WeightedEdges()
	for edges.Next() {
		e := edges.WeightedEdge()
		undirect.SetWeightedEdge(simple.WeightedEdge{F: e.From(), T: e.To(), W: e.Weight()})
	}

	// Louvain v0.17.0: A função 'Modularize' implementa o algoritmo de Louvain.
	reduced := community.Modularize(undirect, 1.0, nil)
	// Cast para obter as comunidades (Modularize retorna ReducedGraph)
	var communities [][]graph.Node
	if r, ok := reduced.(interface{ Communities() [][]graph.Node }); ok {
		communities = r.Communities()
	}
	
	for i, c := range communities {
		for _, gNode := range c {
			id := ge.gonumToNode[gNode.ID()]
			ge.communities[id] = i
		}
	}
	fmt.Printf("[GraphEngine] 🏘️ %d comunidades detectadas.\n", len(communities))
}

// ComputeHITS calcula Hubs (notas que citam muito) e Authorities (notas muito citadas).
func (ge *GraphEngine) ComputeHITS() {
	ge.mu.Lock()
	defer ge.mu.Unlock()

	if len(ge.nodes) == 0 { return }
	
	hits := network.HITS(ge.gonumGraph, 1e-6)
	for gID, val := range hits {
		ge.hubs[ge.gonumToNode[int64(gID)]] = val.Hub
		ge.authorities[ge.gonumToNode[int64(gID)]] = val.Authority
	}
}

// GetMSTEdges retorna apenas as arestas do "Esqueleto Vital" do conhecimento.
func (ge *GraphEngine) GetMSTEdges() []Edge {
	ge.mu.RLock()
	defer ge.mu.RUnlock()

	// path.Prim v0.17.0: Popula o grafo 'dst' com o MST de 'src'.
	undirect := simple.NewWeightedUndirectedGraph(0, math.Inf(1))
	nodes := ge.gonumGraph.Nodes()
	for nodes.Next() {
		undirect.AddNode(nodes.Node())
	}
	edges := ge.gonumGraph.WeightedEdges()
	for edges.Next() {
		e := edges.WeightedEdge()
		undirect.SetWeightedEdge(simple.WeightedEdge{F: e.From(), T: e.To(), W: e.Weight()})
	}

	mstGraph := simple.NewWeightedUndirectedGraph(0, math.Inf(1))
	path.Prim(mstGraph, undirect)
	
	var resEdges []Edge
	mstEdges := mstGraph.WeightedEdges()
	for mstEdges.Next() {
		e := mstEdges.WeightedEdge()
		src := ge.gonumToNode[e.From().ID()]
		tgt := ge.gonumToNode[e.To().ID()]
		label := ge.labels[fmt.Sprintf("%s-%s", src, tgt)]
		resEdges = append(resEdges, Edge{
			Source: src,
			Target: tgt,
			Weight: e.Weight(), 
			Label:  label,
		})
	}
	return resEdges
}

// DetectCycles encontra componentes fortemente conectados (SCC).
func (ge *GraphEngine) DetectCycles() [][]string {
	ge.mu.RLock()
	defer ge.mu.RUnlock()

	sccs := topo.TarjanSCC(ge.gonumGraph)
	var result [][]string
	for _, component := range sccs {
		if len(component) > 1 { // Apenas ciclos reais
			var ids []string
			for _, n := range component {
				ids = append(ids, ge.gonumToNode[n.ID()])
			}
			result = append(result, ids)
		}
	}
	return result
}

// Clear limpa o motor em RAM.
func (ge *GraphEngine) Clear() {
	ge.mu.Lock()
	defer ge.mu.Unlock()
	
	ge.nodes = make(map[string]*Node)
	ge.adj = make(map[string]map[string]float64)
	ge.pageRank = make(map[string]float64)
	ge.gonumGraph = simple.NewWeightedDirectedGraph(0, math.Inf(1))
	ge.nodeToGonum = make(map[string]int64)
	ge.gonumToNode = make(map[int64]string)
}

// GetRank retorna o peso de autoridade de um nó.
func (ge *GraphEngine) GetRank(id string) float64 {
	ge.mu.RLock()
	defer ge.mu.RUnlock()
	return ge.pageRank[id]
}

// GetCommunity retorna o ID do grupo semântico.
func (ge *GraphEngine) GetCommunity(id string) int {
	ge.mu.RLock()
	defer ge.mu.RUnlock()
	return ge.communities[id]
}

// GetCommunityIDs retorna uma lista de todos os IDs de comunidade presentes no grafo.
func (ge *GraphEngine) GetCommunityIDs() []int {
	ge.mu.RLock()
	defer ge.mu.RUnlock()
	
	var ids []int
	for _, id := range ge.communities {
		ids = append(ids, id)
	}
	return ids
}

// GetBetweenness retorna a centralidade de pontilha.
func (ge *GraphEngine) GetBetweenness(id string) float64 {
	ge.mu.RLock()
	defer ge.mu.RUnlock()
	return ge.betweenness[id]
}

// GetHITS retorna o score Hub/Authority.
func (ge *GraphEngine) GetHITS(id string) (float64, float64) {
	ge.mu.RLock()
	defer ge.mu.RUnlock()
	return ge.hubs[id], ge.authorities[id]
}

// BFS realiza uma busca por camadas para expansão de contexto e iluminação de mapa.
func (ge *GraphEngine) BFS(startID string, maxDepth int) []string {
	ge.mu.RLock()
	defer ge.mu.RUnlock()

	visited := make(map[string]bool)
	queue := []string{startID}
	depth := make(map[string]int)
	
	var result []string
	visited[startID] = true
	depth[startID] = 0

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		if depth[curr] > maxDepth {
			continue
		}

		result = append(result, curr)

		for neighbor := range ge.adj[curr] {
			if !visited[neighbor] {
				visited[neighbor] = true
				depth[neighbor] = depth[curr] + 1
				queue = append(queue, neighbor)
			}
		}
	}

	return result
}

// Prune remove nós cuja autoridade (PageRank) esteja abaixo do nível de sobrevivência.
func (ge *GraphEngine) Prune(threshold float64) []string {
	ge.mu.Lock()
	defer ge.mu.Unlock()

	var removed []string
	for id, rank := range ge.pageRank {
		if rank < threshold {
			// Não removemos nós do tipo 'system' ou 'source' (Markdown direto) a menos que explicitamente solicitado.
			node, ok := ge.nodes[id]
			if !ok || (node.Type != "memory" && node.Type != "unknown") {
				continue
			}

			// Remove do Mapa de Nós
			delete(ge.nodes, id)
			// Remove das Adjacências (Entrada e Saída)
			delete(ge.adj, id)
			for source := range ge.adj {
				delete(ge.adj[source], id)
			}
			// Remove do PageRank
			delete(ge.pageRank, id)
			
			// Remove do Gonum
			if gID, ok := ge.nodeToGonum[id]; ok {
				ge.gonumGraph.RemoveNode(gID)
				delete(ge.gonumToNode, gID)
				delete(ge.nodeToGonum, id)
			}

			removed = append(removed, id)
		}
	}

	if len(removed) > 0 {
		fmt.Printf("[GraphEngine] 🧹 Poda Neural: %d nós irrelevantes removidos do Córtex.\n", len(removed))
	}
	return removed
}
// GetNeighborEdges retorna todos os IDs de arestas conectadas a um nó (entrantes e saintes).
func (ge *GraphEngine) GetNeighborEdges(id string) []string {
	ge.mu.RLock()
	defer ge.mu.RUnlock()

	var edges []string
	// Saintes
	if targets, ok := ge.adj[id]; ok {
		for target := range targets {
			edges = append(edges, fmt.Sprintf("%s-%s", id, target))
		}
	}
	// Entrantes
	for source, targets := range ge.adj {
		if _, ok := targets[id]; ok {
			edges = append(edges, fmt.Sprintf("%s-%s", source, id))
		}
	}
	return edges
}
