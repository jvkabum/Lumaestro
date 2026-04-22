package lightning

/*
#cgo CFLAGS: -I${SRCDIR}/../../deps/duckdb
#cgo LDFLAGS: -L${SRCDIR}/../../deps/duckdb -lduckdb
*/
import "C"
import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	_ "github.com/marcboeker/go-duckdb"
)

// DuckDBStore gerencia a persistência analítica do Lightning em Go.
type DuckDBStore struct {
	db   *sql.DB
	path string
	mu   sync.Mutex
}

// NewDuckDBStore inicializa o store analítico.
func NewDuckDBStore(dbPath string) (*DuckDBStore, error) {
	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		return nil, fmt.Errorf("falha ao abrir DuckDB: %w", err)
	}

	store := &DuckDBStore{
		db:   db,
		path: dbPath,
	}

	if err := store.InitSchema(); err != nil {
		return nil, fmt.Errorf("falha ao inicializar esquema DuckDB: %w", err)
	}

	return store, nil
}

// InitSchema cria as tabelas analíticas otimizadas para colunas.
func (s *DuckDBStore) InitSchema() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS spans (
			rollout_id VARCHAR,
			attempt_id VARCHAR,
			sequence_id INTEGER,
			trace_id VARCHAR,
			span_id VARCHAR,
			parent_id VARCHAR,
			name VARCHAR,
			status_code VARCHAR,
			status_description VARCHAR,
			attributes JSON,
			events JSON,
			start_time DOUBLE,
			end_time DOUBLE,
			prompt_tokens INTEGER,
			completion_tokens INTEGER
		)`,
		`CREATE TABLE IF NOT EXISTS rollouts (
			rollout_id VARCHAR PRIMARY KEY,
			input JSON,
			start_time DOUBLE,
			end_time DOUBLE,
			mode VARCHAR,
			status VARCHAR,
			metadata JSON
		)`,
		`CREATE TABLE IF NOT EXISTS rewards (
			rollout_id VARCHAR,
			reward DOUBLE,
			timestamp DOUBLE,
			source VARCHAR
		)`,
		`CREATE TABLE IF NOT EXISTS prompts (
			id VARCHAR PRIMARY KEY,
			agent_name VARCHAR,
			content VARCHAR,
			avg_reward DOUBLE,
			created_at DOUBLE
		)`,
		`CREATE TABLE IF NOT EXISTS prompt_candidates (
			id VARCHAR PRIMARY KEY,
			agent_name VARCHAR,
			name VARCHAR,
			content VARCHAR,
			critique VARCHAR,
			status VARCHAR,
			accuracy_score DOUBLE DEFAULT 0.0,
			created_at DOUBLE
		)`,
		`CREATE TABLE IF NOT EXISTS gold_samples (
			id VARCHAR PRIMARY KEY,
			agent_name VARCHAR,
			input VARCHAR,
			output VARCHAR,
			created_at DOUBLE
		)`,
		`CREATE TABLE IF NOT EXISTS graph_nodes (
			id VARCHAR PRIMARY KEY,
			workspace_path VARCHAR,
			name VARCHAR,
			type VARCHAR,
			metadata JSON,
			pos_x DOUBLE DEFAULT 0.0,
			pos_y DOUBLE DEFAULT 0.0,
			pos_z DOUBLE DEFAULT 0.0,
			created_at DOUBLE
		)`,
		`CREATE TABLE IF NOT EXISTS graph_edges (
			workspace_path VARCHAR,
			source_id VARCHAR,
			target_id VARCHAR,
			weight DOUBLE DEFAULT 1.0,
			relation_type VARCHAR DEFAULT 'mentions',
			created_at DOUBLE
		)`,
		// Migração automática: Adiciona colunas de posição se não existirem (v20)
		`ALTER TABLE graph_nodes ADD COLUMN IF NOT EXISTS pos_x DOUBLE DEFAULT 0.0`,
		`ALTER TABLE graph_nodes ADD COLUMN IF NOT EXISTS pos_y DOUBLE DEFAULT 0.0`,
		`ALTER TABLE graph_nodes ADD COLUMN IF NOT EXISTS pos_z DOUBLE DEFAULT 0.0`,
	}

	for _, q := range queries {
		if _, err := s.db.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

// InsertSpan insere um rastro (Span) no DuckDB.
func (s *DuckDBStore) InsertSpan(span Span) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	attrJSON, _ := json.Marshal(span.Attributes)
	eventsJSON, _ := json.Marshal(span.Events)

	var parentID interface{} = nil
	if span.ParentID != nil {
		parentID = *span.ParentID
	}

	var endTime interface{} = nil
	if span.EndTime != nil {
		endTime = *span.EndTime
	}

	query := `INSERT INTO spans (
		rollout_id, attempt_id, sequence_id, trace_id, span_id, parent_id, 
		name, status_code, status_description, attributes, events, start_time, end_time,
		prompt_tokens, completion_tokens
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := s.db.Exec(query,
		span.RolloutID, span.AttemptID, span.SequenceID, span.TraceID, span.SpanID, parentID,
		span.Name, span.Status.StatusCode, span.Status.Description, string(attrJSON), string(eventsJSON),
		span.StartTime, endTime, span.PromptTokens, span.CompletionTokens,
	)

	return err
}

// GetDB retorna a conexão bruta com o DuckDB (para Binding Wails).
func (s *DuckDBStore) GetDB() *sql.DB {
	return s.db
}

// InsertPrompt registra uma nova versão do System Prompt de um agente.
func (s *DuckDBStore) InsertPrompt(agentName, content string, avgReward float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	now := float64(time.Now().Unix())
	id := fmt.Sprintf("pmp-%.0f", now)
	query := `INSERT INTO prompts (id, agent_name, content, avg_reward, created_at) VALUES (?, ?, ?, ?, ?)`
	_, err := s.db.Exec(query, id, agentName, content, avgReward, now)
	return err
}

// GetLatestPrompt retorna a versão mais recente do prompt para um agente específico.
func (s *DuckDBStore) GetLatestPrompt(agentName string) (string, error) {
	var content string
	err := s.db.QueryRow(`SELECT content FROM prompts WHERE agent_name = ? ORDER BY created_at DESC LIMIT 1`, agentName).Scan(&content)
	return content, err
}

// InsertCandidate registra uma proposta de evolução (Beam Search).
func (s *DuckDBStore) InsertCandidate(agentName, name, content, critique string, accuracy float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	now := float64(time.Now().Unix())
	id := fmt.Sprintf("cand-%.0f-%s", now, name)
	query := `INSERT INTO prompt_candidates (id, agent_name, name, content, critique, status, accuracy_score, created_at) VALUES (?, ?, ?, ?, ?, 'pending', ?, ?)`
	_, err := s.db.Exec(query, id, agentName, name, content, critique, accuracy, now)
	return err
}

// GetPendingCandidates retorna todos os candidatos aguardando aprovação.
func (s *DuckDBStore) GetPendingCandidates() ([]map[string]interface{}, error) {
	rows, err := s.db.Query(`SELECT id, agent_name, name, content, critique, accuracy_score, created_at FROM prompt_candidates WHERE status = 'pending' ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var candidates []map[string]interface{}
	for rows.Next() {
		var id, agent, name, content, critique string
		var accuracy, createdAt float64
		if err := rows.Scan(&id, &agent, &name, &content, &critique, &accuracy, &createdAt); err == nil {
			candidates = append(candidates, map[string]interface{}{
				"id": id, "agent": agent, "name": name, "content": content, "critique": critique, "accuracy": accuracy, "date": time.Unix(int64(createdAt), 0).Format("02/01 15:04"),
			})
		}
	}
	return candidates, nil
}

// ApproveCandidate move um candidato para a tabela oficial de prompts e encerra os outros do mesmo agente.
func (s *DuckDBStore) ApproveCandidate(candidateID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 1. Obter dados do candidato
	var agentName, content string
	err := s.db.QueryRow(`SELECT agent_name, content FROM prompt_candidates WHERE id = ?`, candidateID).Scan(&agentName, &content)
	if err != nil { return err }

	// 2. Inserir como Prompt oficial (Faremos isso via transação ou comando direto)
	now := float64(time.Now().Unix())
	pmpID := fmt.Sprintf("pmp-%.0f", now)
	_, err = s.db.Exec(`INSERT INTO prompts (id, agent_name, content, avg_reward, created_at) VALUES (?, ?, ?, 0.0, ?)`, pmpID, agentName, content, now)
	if err != nil { return err }

	// 3. Marcar todos os candidatos pendentes desse agente como resolvidos
	_, err = s.db.Exec(`UPDATE prompt_candidates SET status = 'rejected' WHERE agent_name = ? AND status = 'pending'`, agentName)
	if err != nil { return err }
	
	_, err = s.db.Exec(`UPDATE prompt_candidates SET status = 'approved' WHERE id = ?`, candidateID)
	return err
}

// FindNodeByName busca um nó no grafo pelo nome exato ou similar (Busca Rápida/Léxica).
func (s *DuckDBStore) FindNodeByName(workspacePath, name string) (string, error) {
	var id string
	// Tenta busca exata primeiro
	err := s.db.QueryRow(`SELECT id FROM graph_nodes WHERE workspace_path = ? AND name = ? LIMIT 1`, workspacePath, name).Scan(&id)
	if err == nil {
		return id, nil
	}

	// Tenta busca por similaridade (ILIKE) se a exata falhar
	err = s.db.QueryRow(`SELECT id FROM graph_nodes WHERE workspace_path = ? AND name ILIKE ? LIMIT 1`, workspacePath, "%"+name+"%").Scan(&id)
	return id, err
}

// InsertGoldSample registra uma amostra "Gold" no DuckDB.
func (s *DuckDBStore) InsertGoldSample(agentName, input, output string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	now := float64(time.Now().Unix())
	id := fmt.Sprintf("gold-%.0f", now)
	query := `INSERT INTO gold_samples (id, agent_name, input, output, created_at) VALUES (?, ?, ?, ?, ?)`
	_, err := s.db.Exec(query, id, agentName, input, output, now)
	return err
}

// GetGoldSamples retorna os casos de ouro de um agente.
func (s *DuckDBStore) GetGoldSamples(agentName string) ([]map[string]string, error) {
	rows, err := s.db.Query(`SELECT input, output FROM gold_samples WHERE agent_name = ?`, agentName)
	if err != nil { return nil, err }
	defer rows.Close()

	var samples []map[string]string
	for rows.Next() {
		var in, out string
		if err := rows.Scan(&in, &out); err == nil {
			samples = append(samples, map[string]string{"input": in, "output": out})
		}
	}
	return samples, nil
}

// Close fecha a conexão com o DuckDB.
func (s *DuckDBStore) Close() error {
	return s.db.Close()
}

// --- Métodos do Cérebro Relacional (Grafo) ---

// UpdateNodePositions atualiza as coordenadas de uma lista de nós em massa.
func (s *DuckDBStore) UpdateNodePositions(nodes []map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare(`UPDATE graph_nodes SET pos_x = ?, pos_y = ?, pos_z = ? WHERE id = ?`)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()

	for _, n := range nodes {
		id, _ := n["id"].(string)
		x, _ := n["x"].(float64)
		y, _ := n["y"].(float64)
		z, _ := n["z"].(float64)
		if id != "" {
			_, _ = stmt.Exec(x, y, z, id)
		}
	}

	return tx.Commit()
}

// UpsertGraphNode insere ou atualiza um nó no grafo analítico vinculado a um workspace.
func (s *DuckDBStore) UpsertGraphNode(workspacePath, id, name, nodeType string, metadata map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	metaJSON, _ := json.Marshal(metadata)
	query := `INSERT INTO graph_nodes (id, workspace_path, name, type, metadata, pos_x, pos_y, pos_z, created_at)
			  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
			  ON CONFLICT (id) DO UPDATE SET 
			  workspace_path = excluded.workspace_path, name = excluded.name, 
			  type = excluded.type, metadata = excluded.metadata,
			  pos_x = excluded.pos_x, pos_y = excluded.pos_y, pos_z = excluded.pos_z`
	
	x, _ := metadata["x"].(float64)
	y, _ := metadata["y"].(float64)
	z, _ := metadata["z"].(float64)

	_, err := s.db.Exec(query, id, workspacePath, name, nodeType, string(metaJSON), x, y, z, time.Now().UnixNano())
	return err
}

// InsertGraphEdge insere uma relação semântica entre dois nós em um workspace.
func (s *DuckDBStore) InsertGraphEdge(workspacePath, sourceID, targetID string, weight float64, relationType string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `INSERT INTO graph_edges (workspace_path, source_id, target_id, weight, relation_type, created_at)
			  VALUES (?, ?, ?, ?, ?, ?)`
	
	_, err := s.db.Exec(query, workspacePath, sourceID, targetID, weight, relationType, time.Now().UnixNano())
	return err
}

// GetNodeCount retorna o número de notas vinculadas a um workspace específico.
func (s *DuckDBStore) GetNodeCount(workspacePath string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var count int
	err := s.db.QueryRow(`SELECT count(*) FROM graph_nodes WHERE workspace_path = ?`, workspacePath).Scan(&count)
	return count, err
}

// GetFullGraph recupera todos os nós e arestas de um workspace específico.
func (s *DuckDBStore) GetFullGraph(workspacePath string) ([]map[string]interface{}, []map[string]interface{}, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 1. Recuperar Nós do Workspace
	rowsN, err := s.db.Query(`SELECT id, name, type, pos_x, pos_y, pos_z FROM graph_nodes WHERE workspace_path = ?`, workspacePath)
	if err != nil {
		return nil, nil, err
	}
	defer rowsN.Close()

	var nodes []map[string]interface{}
	for rowsN.Next() {
		var id, name, t string
		var x, y, z float64
		if err := rowsN.Scan(&id, &name, &t, &x, &y, &z); err == nil {
			nodes = append(nodes, map[string]interface{}{
				"id": id, "name": name, "type": t,
				"x": x, "y": y, "z": z,
			})
		}
	}

	// 2. Recuperar Arestas do Workspace
	rowsE, err := s.db.Query(`SELECT source_id, target_id, weight, relation_type FROM graph_edges WHERE workspace_path = ?`, workspacePath)
	if err != nil {
		return nil, nil, err
	}
	defer rowsE.Close()

	var edges []map[string]interface{}
	for rowsE.Next() {
		var src, tgt, rel string
		var w float64
		if err := rowsE.Scan(&src, &tgt, &w, &rel); err == nil {
			edges = append(edges, map[string]interface{}{
				"source": src, "target": tgt, "weight": w, "relation_type": rel,
			})
		}
	}

	return nodes, edges, nil
}
