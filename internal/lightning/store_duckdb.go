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
	"strings"
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
			content VARCHAR,
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
		// Migração automática: Adiciona colunas de posição se não existirem (v2.0)
		`ALTER TABLE graph_nodes ADD COLUMN IF NOT EXISTS pos_x DOUBLE DEFAULT 0.0`,
		`ALTER TABLE graph_nodes ADD COLUMN IF NOT EXISTS pos_y DOUBLE DEFAULT 0.0`,
		`ALTER TABLE graph_nodes ADD COLUMN IF NOT EXISTS pos_z DOUBLE DEFAULT 0.0`,
		`ALTER TABLE graph_nodes ADD COLUMN IF NOT EXISTS workspace_path VARCHAR`,
		`ALTER TABLE graph_edges ADD COLUMN IF NOT EXISTS workspace_path VARCHAR`,
		`ALTER TABLE graph_nodes ADD COLUMN IF NOT EXISTS parent_id VARCHAR`,
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

// FindNodeInText busca o nó mais relevante cujo nome está contido no texto fornecido.
func (s *DuckDBStore) FindNodeInText(text string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var id string

	// Limpeza profunda do texto para evitar que aspas ou pontuação quebrem o match
	cleaner := strings.NewReplacer("\"", "", "'", "", "(", "", ")", "", "[", "", "]", "", "?", "", "!", "", ".", "", ",", "")
	cleanText := cleaner.Replace(strings.TrimSpace(text))

	// Busca Strict: Tenta encontrar o nome da nota como uma palavra isolada no texto
	queryStrict := `
		SELECT id
		FROM graph_nodes
		WHERE length(name) > 2
		  AND (
		    ' ' || ? || ' ' ILIKE '% ' || name || ' %' OR
		    ? ILIKE name || ' %' OR
		    ? ILIKE '% ' || name
		  )
		ORDER BY length(name) DESC
		LIMIT 1
	`

	err := s.db.QueryRow(queryStrict, cleanText, cleanText, cleanText).Scan(&id)
	if err == nil {
		return id, nil
	}

	// Fallback loose: ILIKE normal
	queryLoose := `
		SELECT id
		FROM graph_nodes
		WHERE length(name) > 3
		  AND ? ILIKE '%' || name || '%'
		ORDER BY length(name) DESC
		LIMIT 1
	`

	err = s.db.QueryRow(queryLoose, cleanText).Scan(&id)
	return id, err
}

// --- Métodos do Cérebro Relacional (Grafo) ---

// UpsertGraphNode insere ou atualiza um nó no grafo analítico vinculado a um workspace.
func (s *DuckDBStore) UpsertGraphNode(workspacePath, id, name, nodeType, content string, metadata map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	metaJSON, _ := json.Marshal(metadata)
	
	// 📍 Lógica de Preservação de Layout:
	// 1. Tenta pegar do metadata (caso venha do frontend ou cache de topologia)
	x, _ := metadata["x"].(float64)
	y, _ := metadata["y"].(float64)
	z, _ := metadata["z"].(float64)

	// 2. Se as coordenadas forem zero (caso venha do crawler/indexador), 
	// tenta preservar o que já está salvo no banco para não "resetar" o mapa.
	if x == 0 && y == 0 && z == 0 {
		_ = s.db.QueryRow(`SELECT pos_x, pos_y, pos_z FROM graph_nodes WHERE id = ?`, id).Scan(&x, &y, &z)
	}

	query := `INSERT INTO graph_nodes (id, workspace_path, name, type, content, metadata, pos_x, pos_y, pos_z, parent_id, created_at)
			  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			  ON CONFLICT (id) DO UPDATE SET 
			  workspace_path = excluded.workspace_path, name = excluded.name, type = excluded.type, 
			  content = excluded.content, metadata = excluded.metadata,
			  pos_x = CASE WHEN excluded.pos_x != 0 THEN excluded.pos_x ELSE graph_nodes.pos_x END,
			  pos_y = CASE WHEN excluded.pos_y != 0 THEN excluded.pos_y ELSE graph_nodes.pos_y END,
			  pos_z = CASE WHEN excluded.pos_z != 0 THEN excluded.pos_z ELSE graph_nodes.pos_z END,
			  parent_id = CASE WHEN excluded.parent_id IS NOT NULL THEN excluded.parent_id ELSE graph_nodes.parent_id END`
	
	// Tenta extrair parent do metadata se não for explicitamente passado (compatibilidade)
	parentID, _ := metadata["parent"].(string)
	if parentID == "" {
		parentID, _ = metadata["parent_gravity_id"].(string)
	}

	_, err := s.db.Exec(query, id, workspacePath, name, nodeType, content, string(metaJSON), x, y, z, parentID, time.Now().UnixNano())
	return err
}

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

// GetFullGraph recupera todos os nós e arestas de um workspace específico para carregar na RAM.
func (s *DuckDBStore) GetFullGraph(workspacePath string) ([]map[string]interface{}, []map[string]interface{}, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 1. Recuperar Nós do Workspace
	rowsN, err := s.db.Query(`SELECT id, name, type, pos_x, pos_y, pos_z, parent_id, metadata FROM graph_nodes WHERE workspace_path = ?`, workspacePath)
	if err != nil {
		return nil, nil, err
	}
	defer rowsN.Close()

	var nodes []map[string]interface{}
	for rowsN.Next() {
		var id, name, t string
		var x, y, z float64
		var parent sql.NullString
		var metadataJSON []byte
		if err := rowsN.Scan(&id, &name, &t, &x, &y, &z, &parent, &metadataJSON); err == nil {
			nodeData := map[string]interface{}{
				"id": id, "name": name, "type": t, "x": x, "y": y, "z": z,
			}
			
			// 🧠 Fundir metadados JSON (celestial-type, mass, etc)
			if len(metadataJSON) > 0 {
				var meta map[string]interface{}
				if err := json.Unmarshal(metadataJSON, &meta); err == nil {
					for k, v := range meta {
						nodeData[k] = v
					}
				}
			}
			if parent.Valid {
				nodeData["parent_gravity_id"] = parent.String
			}
			nodes = append(nodes, nodeData)
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

// SearchNodesByKeyword realiza uma busca textual rápida (Radar) no DuckDB.
// Prioriza o nome e depois o conteúdo, com suporte a múltiplas palavras.
func (s *DuckDBStore) SearchNodesByKeyword(keyword string, limit int) ([]map[string]interface{}, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	words := strings.Fields(strings.ToLower(keyword))
	if len(words) == 0 {
		return nil, nil
	}

	// 🛑 STOP WORDS (Português/Inglês) expandidas para evitar ruído na busca SQL
	stopWords := map[string]bool{
		"oque": true, "que": true, "é": true, "o": true, "a": true, "os": true, "as": true,
		"de": true, "do": true, "da": true, "um": true, "uma": true, "sobre": true, "como": true,
		"fale": true, "quem": true, "onde": true, "qual": true, "quais": true, "para": true, "pelo": true,
		"pela": true, "com": true, "num": true, "numa": true, "está": true, "tem": true,
		"what": true, "is": true, "how": true, "the": true, "in": true, "on": true, "at": true, "to": true,
		"for": true, "of": true, "and": true, "or": true, "with": true, "about": true,
	}

	var scoreExprs []string
	var args []interface{}

	for _, word := range words {
		if len(word) < 2 || stopWords[word] {
			continue
		}

		pattern := "%" + word + "%"
		// 1. Match no Nome (Peso 15 - Aumentado)
		scoreExprs = append(scoreExprs, "(CASE WHEN name ILIKE ? THEN 15 ELSE 0 END)")
		args = append(args, pattern)

		// 2. Match no Conteúdo (Peso 2 - Dobrado)
		scoreExprs = append(scoreExprs, "(CASE WHEN content ILIKE ? THEN 2 ELSE 0 END)")
		args = append(args, pattern)

		// 3. Match Exato no Nome (Peso 30 - Bônus Premium)
		scoreExprs = append(scoreExprs, "(CASE WHEN name ILIKE ? THEN 30 ELSE 0 END)")
		args = append(args, word)
	}

	if len(scoreExprs) == 0 {
		return nil, nil
	}

	query := fmt.Sprintf(`
		SELECT id, name, type, (%s) as relevance
		FROM graph_nodes
		WHERE relevance > 0
		ORDER BY relevance DESC, created_at DESC
		LIMIT ?
	`, strings.Join(scoreExprs, " + "))

	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var id, name, nodeType string
		var relevance int
		if err := rows.Scan(&id, &name, &nodeType, &relevance); err == nil {
			results = append(results, map[string]interface{}{
				"id":        id,
				"name":      name,
				"type":      nodeType,
				"relevance": relevance,
			})
		}
	}
	return results, nil
}

// GetNeighbors recupera nós vizinhos baseados em conexões explícitas ou semânticas (ID Vinculador).
func (s *DuckDBStore) GetNeighbors(nodeID string) ([]map[string]interface{}, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `
		SELECT n.id, n.name, n.type, n.content
		FROM graph_nodes n
		JOIN graph_edges e ON (e.source_id = ? AND e.target_id = n.id) 
		                   OR (e.target_id = ? AND e.source_id = n.id)
		WHERE n.id != ?
		LIMIT 10
	`

	rows, err := s.db.Query(query, nodeID, nodeID, nodeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var id, name, nodeType, content string
		if err := rows.Scan(&id, &name, &nodeType, &content); err == nil {
			results = append(results, map[string]interface{}{
				"id":      id,
				"name":    name,
				"type":    nodeType,
				"content": content,
			})
		}
	}
	return results, nil
}

// ClearGraph limpa permanentemente as tabelas de nós e arestas.
func (s *DuckDBStore) ClearGraph() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`DELETE FROM graph_nodes`)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`DELETE FROM graph_edges`)
	return err
}
