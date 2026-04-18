package rag

import (
	"context"
	"fmt"
	"hash/fnv"
	"time"

	"Lumaestro/internal/provider"
	"Lumaestro/internal/utils"
)

// KnowledgeWeaver é o "Tecelão de Conhecimento" que transforma conversas em sinapses.
type KnowledgeWeaver struct {
	ctx      context.Context // Contexto persistente do Wails
	Ontology *provider.OntologyService
	Qdrant   *provider.QdrantClient
	Embedder provider.Embedder
}

// SetContext injeta o contexto oficial do Wails para processos de background.
func (w *KnowledgeWeaver) SetContext(ctx context.Context) {
	w.ctx = ctx
}

// NewKnowledgeWeaver inicializa o tecelão.
func NewKnowledgeWeaver(ontology *provider.OntologyService, qdrant *provider.QdrantClient, embedder provider.Embedder) *KnowledgeWeaver {
	return &KnowledgeWeaver{
		Ontology: ontology,
		Qdrant:   qdrant,
		Embedder: embedder,
	}
}

// WeaveChatKnowledge analisa o texto do chat, extrai fatos e os integra ao grafo com consciência de sessão.
func (w *KnowledgeWeaver) WeaveChatKnowledge(ctx context.Context, sessionID string, chatText string) error {
	// 📡 Sinalização de Início: Avisa o Frontend que a WEAVER começou a tecer
	utils.SafeEmit(w.ctx, "weaver:started", nil)
	defer utils.SafeEmit(w.ctx, "weaver:finished", nil)

	// 1. Extração de Triplas (Sinapses)
	contextHint := fmt.Sprintf("Memória de Chat - Sessão: %s", sessionID)
	triples, err := w.Ontology.ExtractTriples(ctx, chatText, contextHint)
	if err != nil {
		return fmt.Errorf("falha ao extrair sinapses: %w", err)
	}

	if len(triples) == 0 {
		return nil
	}

	for _, t := range triples {
		factText := fmt.Sprintf("%s %s %s", t.Subject, t.Predicate, t.Object)
		vector, _ := w.Embedder.GenerateEmbedding(ctx, factText, false)

		// 2. DETECÇÃO DE CONFLITO: Busca se já sabemos algo sobre este (Sujeito, Predicado)
		existing, _ := w.Qdrant.SearchByField("knowledge_graph", "subject", t.Subject)
		if existing != nil && existing["predicate"] == t.Predicate && existing["object"] != t.Object && existing["status"] != "legacy" {
			
			// AGENTE VALIDADOR: Decidir se é uma atualização ou conflito duvidoso
			resolution, err := w.Ontology.ValidateConflict(ctx, existing["object"].(string), t.Object, chatText)
			
			if err == nil && resolution == "UPDATE" {
				// Marca o antigo como LEGADO (Conhecimento Morto)
				oldID := uint64(existing["id"].(float64)) // ID original
				w.Qdrant.SetPayload("knowledge_graph", oldID, map[string]interface{}{
					"status": "legacy",
					"archived_at": time.Now().Format(time.RFC3339),
				})
				utils.SafeEmit(w.ctx, "agent:log", map[string]string{
					"source":  "WEAVER",
					"content": fmt.Sprintf("📜 Conhecimento Legado: '%s' foi superado por '%s'.", existing["object"], t.Object),
				})
			} else {
				// CONFLITO TOTAL: Emite Alerta Vermelho para o Frontend com os dados completos para resolução
				utils.SafeEmit(w.ctx, "graph:conflict", map[string]interface{}{
					"subject":    t.Subject,
					"predicate":  t.Predicate,
					"old":        existing["object"],
					"new":        t.Object,
					"old_id":     uint64(existing["id"].(float64)),
					"session_id": sessionID,
				})
				continue // Não salva enquanto não houver certeza
			}
		}

		// 3. Salvar Nova Sinapse
		h := fnv.New64a()
		h.Write([]byte(factText + sessionID)) // ID único por sessão também
		id := h.Sum64()

		payload := map[string]interface{}{
			"id":         id,
			"session_id": sessionID,
			"subject":    t.Subject,
			"predicate":  t.Predicate,
			"object":     t.Object,
			"source":     "chat_memory",
			"status":     "active",
			"timestamp":  time.Now().Format(time.RFC3339),
			"content":    factText,
		}

		w.Qdrant.UpsertPoint("knowledge_graph", id, vector, payload)

		// 4. ATUALIZAÇÃO VISUAL
		utils.SafeEmit(w.ctx, "graph:node", map[string]string{
			"id":            t.Subject,
			"name":          t.Subject,
			"document-type": "memory",
			"session-id":    sessionID,
		})
		utils.SafeEmit(w.ctx, "graph:node", map[string]string{
			"id":            t.Object,
			"name":          t.Object,
			"document-type": "memory",
			"session-id":    sessionID,
		})
		utils.SafeEmit(w.ctx, "graph:edge", map[string]string{
			"source": t.Subject,
			"target": t.Object,
		})
	}

	return nil
}
