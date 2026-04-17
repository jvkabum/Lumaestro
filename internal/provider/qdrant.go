package provider

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// QdrantClient gerencia a comunicação com o servidor remoto.
type QdrantClient struct {
	BaseURL string
	APIKey  string

	fallbackEnabled bool
	fallbackMu      sync.RWMutex
	fallbackData    map[string]*fallbackCollection
	fallbackActive  bool
	fallbackPath    string
}

type fallbackPoint struct {
	ID      uint64
	Vector  []float32
	Payload map[string]interface{}
}

type fallbackCollection struct {
	Points map[uint64]fallbackPoint
	Order  []uint64
}

type fallbackStore struct {
	Collections map[string]fallbackCollectionSnapshot `json:"collections"`
}

type fallbackCollectionSnapshot struct {
	Points []fallbackPoint `json:"points"`
}

// NewQdrantClient inicializa o cliente com a URL e a chave de autenticação (Coolify).
func NewQdrantClient(baseURL string, apiKey string) *QdrantClient {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		fmt.Println("[QDRANT] ⚠️ AVISO: URL do Qdrant está vazia! O sistema falhará ao conectar.")
	}
	envToggle := strings.TrimSpace(strings.ToLower(os.Getenv("LUMAESTRO_PARALLEL_MEMORY")))
	fallbackEnabled := true
	if envToggle == "0" || envToggle == "false" || envToggle == "off" {
		fallbackEnabled = false
	}
	fallbackPath := strings.TrimSpace(os.Getenv("LUMAESTRO_PARALLEL_MEMORY_PATH"))
	if fallbackPath == "" {
		fallbackPath = filepath.Join(".lumaestro", "parallel-memory.json")
	}

	client := &QdrantClient{
		BaseURL:         baseURL,
		APIKey:          strings.TrimSpace(apiKey),
		fallbackEnabled: fallbackEnabled,
		fallbackData:    make(map[string]*fallbackCollection),
		fallbackPath:    fallbackPath,
	}

	if err := client.loadFallbackFromDisk(); err != nil {
		fmt.Printf("[QDRANT] ⚠️ Falha ao carregar memória paralela local: %v\n", err)
	}

	return client
}

func (c *QdrantClient) loadFallbackFromDisk() error {
	if !c.fallbackEnabled || strings.TrimSpace(c.fallbackPath) == "" {
		return nil
	}
	body, err := os.ReadFile(c.fallbackPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return nil
	}

	var store fallbackStore
	if err := json.Unmarshal(body, &store); err != nil {
		return err
	}

	c.fallbackMu.Lock()
	defer c.fallbackMu.Unlock()
	for name, snapshot := range store.Collections {
		col := &fallbackCollection{Points: map[uint64]fallbackPoint{}, Order: make([]uint64, 0, len(snapshot.Points))}
		for _, point := range snapshot.Points {
			id := point.ID
			col.Points[id] = fallbackPoint{
				ID:      id,
				Vector:  cloneVector(point.Vector),
				Payload: clonePayload(point.Payload),
			}
			col.Order = append(col.Order, id)
		}
		c.fallbackData[name] = col
	}
	return nil
}

func (c *QdrantClient) snapshotFallbackStore() fallbackStore {
	store := fallbackStore{Collections: make(map[string]fallbackCollectionSnapshot, len(c.fallbackData))}
	for name, col := range c.fallbackData {
		snapshot := fallbackCollectionSnapshot{Points: make([]fallbackPoint, 0, len(col.Points))}
		seen := make(map[uint64]struct{}, len(col.Points))
		for _, id := range col.Order {
			point, ok := col.Points[id]
			if !ok {
				continue
			}
			snapshot.Points = append(snapshot.Points, fallbackPoint{
				ID:      point.ID,
				Vector:  cloneVector(point.Vector),
				Payload: clonePayload(point.Payload),
			})
			seen[id] = struct{}{}
		}
		for id, point := range col.Points {
			if _, ok := seen[id]; ok {
				continue
			}
			snapshot.Points = append(snapshot.Points, fallbackPoint{
				ID:      point.ID,
				Vector:  cloneVector(point.Vector),
				Payload: clonePayload(point.Payload),
			})
		}
		store.Collections[name] = snapshot
	}
	return store
}

func (c *QdrantClient) persistFallbackToDisk() {
	if !c.fallbackEnabled || strings.TrimSpace(c.fallbackPath) == "" {
		return
	}

	c.fallbackMu.RLock()
	store := c.snapshotFallbackStore()
	c.fallbackMu.RUnlock()

	body, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		fmt.Printf("[QDRANT] ⚠️ Falha ao serializar memória paralela local: %v\n", err)
		return
	}

	dir := filepath.Dir(c.fallbackPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fmt.Printf("[QDRANT] ⚠️ Falha ao criar diretório da memória paralela local: %v\n", err)
			return
		}
	}

	if err := os.WriteFile(c.fallbackPath, body, 0o600); err != nil {
		fmt.Printf("[QDRANT] ⚠️ Falha ao persistir memória paralela local: %v\n", err)
	}
}

func clonePayload(src map[string]interface{}) map[string]interface{} {
	if src == nil {
		return map[string]interface{}{}
	}
	dst := make(map[string]interface{}, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func cloneVector(src []float32) []float32 {
	if src == nil {
		return nil
	}
	dst := make([]float32, len(src))
	copy(dst, src)
	return dst
}

func cosineSimilarity(a []float32, b []float32) float64 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := 0; i < len(a); i++ {
		av := float64(a[i])
		bv := float64(b[i])
		dot += av * bv
		na += av * av
		nb += bv * bv
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

func (c *QdrantClient) ensureFallbackCollection(name string) *fallbackCollection {
	col, ok := c.fallbackData[name]
	if !ok {
		col = &fallbackCollection{Points: map[uint64]fallbackPoint{}, Order: []uint64{}}
		c.fallbackData[name] = col
	}
	return col
}

func (c *QdrantClient) activateFallback(reason error) {
	if !c.fallbackEnabled {
		return
	}
	if reason == nil {
		return
	}
	c.fallbackMu.Lock()
	defer c.fallbackMu.Unlock()
	if c.fallbackActive {
		return
	}
	c.fallbackActive = true
	fmt.Printf("[QDRANT] ⚠️ Conexão indisponível. Ativando memória paralela local: %v\n", reason)
}

func (c *QdrantClient) useFallback(err error) bool {
	if !c.fallbackEnabled {
		return false
	}
	if err != nil {
		// 🛡️ Ignora 404 (Not Found) para ativação do fallback global.
		// Erro 404 é um erro lógico (coleção não existe), não um erro de rede/servidor.
		if strings.Contains(err.Error(), "Status 404") || strings.Contains(err.Error(), "Not found") {
			return false
		}
		c.activateFallback(err)
		return true
	}
	c.fallbackMu.RLock()
	active := c.fallbackActive
	c.fallbackMu.RUnlock()
	return active
}

func (c *QdrantClient) fallbackSetPayload(collection string, id uint64, payload map[string]interface{}) error {
	c.fallbackMu.Lock()
	col := c.ensureFallbackCollection(collection)
	p, ok := col.Points[id]
	if !ok {
		c.fallbackMu.Unlock()
		return fmt.Errorf("item %d não encontrado na coleção %s", id, collection)
	}
	for k, v := range payload {
		p.Payload[k] = v
	}
	col.Points[id] = p
	c.fallbackMu.Unlock()
	c.persistFallbackToDisk()
	return nil
}

func (c *QdrantClient) fallbackUpsertPoint(collection string, id uint64, vector []float32, payload map[string]interface{}) error {
	c.fallbackMu.Lock()
	col := c.ensureFallbackCollection(collection)
	_, exists := col.Points[id]
	col.Points[id] = fallbackPoint{ID: id, Vector: cloneVector(vector), Payload: clonePayload(payload)}
	if !exists {
		col.Order = append(col.Order, id)
	}
	c.fallbackMu.Unlock()
	c.persistFallbackToDisk()
	return nil
}

func (c *QdrantClient) fallbackSearch(collection string, vector []float32, limit int) ([]map[string]interface{}, error) {
	c.fallbackMu.RLock()
	defer c.fallbackMu.RUnlock()
	col, ok := c.fallbackData[collection]
	if !ok || len(col.Order) == 0 || limit <= 0 {
		return []map[string]interface{}{}, nil
	}

	if vector == nil {
		out := make([]map[string]interface{}, 0, limit)
		for i := len(col.Order) - 1; i >= 0 && len(out) < limit; i-- {
			id := col.Order[i]
			if p, ok := col.Points[id]; ok {
				out = append(out, clonePayload(p.Payload))
			}
		}
		return out, nil
	}

	type scored struct {
		Payload map[string]interface{}
		Score   float64
	}
	scoredList := make([]scored, 0, len(col.Points))
	for _, id := range col.Order {
		p, ok := col.Points[id]
		if !ok {
			continue
		}
		score := cosineSimilarity(vector, p.Vector)
		scoredList = append(scoredList, scored{Payload: clonePayload(p.Payload), Score: score})
	}

	for i := 0; i < len(scoredList); i++ {
		for j := i + 1; j < len(scoredList); j++ {
			if scoredList[j].Score > scoredList[i].Score {
				scoredList[i], scoredList[j] = scoredList[j], scoredList[i]
			}
		}
	}

	if limit > len(scoredList) {
		limit = len(scoredList)
	}
	out := make([]map[string]interface{}, 0, limit)
	for i := 0; i < limit; i++ {
		out = append(out, scoredList[i].Payload)
	}
	return out, nil
}

func (c *QdrantClient) fallbackSearchByField(collection string, key string, value string) (map[string]interface{}, error) {
	c.fallbackMu.RLock()
	defer c.fallbackMu.RUnlock()
	col, ok := c.fallbackData[collection]
	if !ok {
		return nil, fmt.Errorf("item '%s' não encontrado em %s", value, key)
	}
	for _, id := range col.Order {
		p, ok := col.Points[id]
		if !ok {
			continue
		}
		if v, ok := p.Payload[key]; ok && fmt.Sprintf("%v", v) == value {
			return clonePayload(p.Payload), nil
		}
	}
	return nil, fmt.Errorf("item '%s' não encontrado em %s", value, key)
}

func (c *QdrantClient) fallbackSearchWithScores(collection string, vector []float32, limit int) ([]map[string]interface{}, error) {
	results, err := c.fallbackSearch(collection, vector, limit)
	if err != nil {
		return nil, err
	}
	for i := range results {
		if _, ok := results[i]["_score"]; !ok {
			results[i]["_score"] = 0.0
		}
	}
	return results, nil
}

func (c *QdrantClient) fallbackCheckCollectionExists(name string) (bool, error) {
	c.fallbackMu.RLock()
	defer c.fallbackMu.RUnlock()
	_, ok := c.fallbackData[name]
	return ok, nil
}

func (c *QdrantClient) fallbackCreateCollection(name string, _ int) error {
	c.fallbackMu.Lock()
	c.ensureFallbackCollection(name)
	c.fallbackMu.Unlock()
	c.persistFallbackToDisk()
	return nil
}

func (c *QdrantClient) fallbackGetPoints(collection string, ids []uint64) ([]map[string]interface{}, error) {
	c.fallbackMu.RLock()
	defer c.fallbackMu.RUnlock()
	col, ok := c.fallbackData[collection]
	if !ok {
		return []map[string]interface{}{}, nil
	}
	out := make([]map[string]interface{}, 0, len(ids))
	for _, id := range ids {
		if p, ok := col.Points[id]; ok {
			out = append(out, clonePayload(p.Payload))
		}
	}
	return out, nil
}

func (c *QdrantClient) fallbackCountPoints(collection string) (int, error) {
	c.fallbackMu.RLock()
	defer c.fallbackMu.RUnlock()
	col, ok := c.fallbackData[collection]
	if !ok {
		return 0, nil
	}
	return len(col.Points), nil
}

func (c *QdrantClient) fallbackDeleteCollection(name string) error {
	c.fallbackMu.Lock()
	delete(c.fallbackData, name)
	c.fallbackMu.Unlock()
	c.persistFallbackToDisk()
	return nil
}

// checkResponse valida se o status da resposta é de sucesso (2xx).
func (c *QdrantClient) checkResponse(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	body, _ := io.ReadAll(resp.Body)
	errMsg := strings.TrimSpace(string(body))
	if errMsg == "" {
		errMsg = resp.Status
	}

	// 🤫 Silêncio Estratégico: 404 (Not Found) é esperado se as coleções ainda não foram criadas.
	// Não poluímos o terminal com logs vermelhos para este caso específico.
	if resp.StatusCode != http.StatusNotFound {
		fmt.Printf("[DEBUG-QDRANT] ❌ ERRO DO SERVIDOR (Status %d): %s\n", resp.StatusCode, errMsg)
	}

	return fmt.Errorf("falha no servidor Qdrant (Status %d): %s", resp.StatusCode, errMsg)
}

// SetPayload atualiza metadados (payload) de um ponto existente sem sobrescrever o vetor.
func (c *QdrantClient) SetPayload(collection string, id uint64, payload map[string]interface{}) error {
	url := fmt.Sprintf("%s/collections/%s/points/payload?wait=true", c.BaseURL, collection)

	data := map[string]interface{}{
		"payload": payload,
		"points":  []uint64{id},
	}

	body, _ := json.Marshal(data)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		if c.useFallback(err) {
			return c.fallbackSetPayload(collection, id, payload)
		}
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		// fmt.Printf("[DEBUG-QDRANT] 🏹 Enviando Request: %s | Triple-Auth Ativo | KeyPrefix: %s...\n", url, c.APIKey[:4])
		// Redundância Tripla: Alguns proxies exigem minúsculo, outros capitalizado, outros Bearer.
		req.Header["api-key"] = []string{c.APIKey}
		req.Header.Set("Api-Key", c.APIKey)
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		if c.useFallback(err) {
			return c.fallbackSetPayload(collection, id, payload)
		}
		return err
	}
	defer resp.Body.Close()

	if err := c.checkResponse(resp); err != nil {
		if c.useFallback(err) {
			return c.fallbackSetPayload(collection, id, payload)
		}
		return err
	}
	return nil
}

// UpsertPoint envia uma nota (vetor + metadados) para o Qdrant.
func (c *QdrantClient) UpsertPoint(collection string, id uint64, vector []float32, payload map[string]interface{}) error {
	url := fmt.Sprintf("%s/collections/%s/points?wait=true", c.BaseURL, collection)

	point := map[string]interface{}{
		"points": []map[string]interface{}{
			{
				"id":      id,
				"vector":  vector,
				"payload": payload,
			},
		},
	}

	body, _ := json.Marshal(point)
	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(body))
	if err != nil {
		if c.useFallback(err) {
			return c.fallbackUpsertPoint(collection, id, vector, payload)
		}
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		req.Header["api-key"] = []string{c.APIKey}
		req.Header.Set("Api-Key", c.APIKey)
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		if c.useFallback(err) {
			return c.fallbackUpsertPoint(collection, id, vector, payload)
		}
		return err
	}
	defer resp.Body.Close()

	if err := c.checkResponse(resp); err != nil {
		if c.useFallback(err) {
			return c.fallbackUpsertPoint(collection, id, vector, payload)
		}
		return err
	}
	return nil
}

// Search busca os pontos mais próximos de um vetor OU lista pontos recentes (se vector for nil).
func (c *QdrantClient) Search(collection string, vector []float32, limit int) ([]map[string]interface{}, error) {
	var url string
	var query map[string]interface{}

	if vector == nil {
		// Modo Scroll: Lista os pontos mais recentes/relevantes sem busca vetorial.
		url = fmt.Sprintf("%s/collections/%s/points/scroll", c.BaseURL, collection)
		query = map[string]interface{}{
			"limit":        limit,
			"with_payload": true,
		}
	} else {
		// Modo Search: Busca vetorial clássica.
		url = fmt.Sprintf("%s/collections/%s/points/search", c.BaseURL, collection)
		query = map[string]interface{}{
			"vector":       vector,
			"limit":        limit,
			"with_payload": true,
		}
	}

	body, _ := json.Marshal(query)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		if c.useFallback(err) {
			return c.fallbackSearch(collection, vector, limit)
		}
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		// fmt.Printf("[DEBUG-QDRANT] 🏹 Enviando Request (%s): %s | Triple-Auth Ativo\n", collection, url)
		req.Header["api-key"] = []string{c.APIKey}
		req.Header.Set("Api-Key", c.APIKey)
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		if c.useFallback(err) {
			return c.fallbackSearch(collection, vector, limit)
		}
		return nil, err
	}
	defer resp.Body.Close()

	if err := c.checkResponse(resp); err != nil {
		if c.useFallback(err) {
			return c.fallbackSearch(collection, vector, limit)
		}
		return nil, err
	}

	var result struct {
		Result interface{} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		if c.useFallback(err) {
			return c.fallbackSearch(collection, vector, limit)
		}
		return nil, err
	}

	outputs := make([]map[string]interface{}, 0)

	// Trata a diferença de retorno entre Search (Slice) e Scroll (Map com Points)
	if vector == nil {
		scrollRes, _ := result.Result.(map[string]interface{})
		points, _ := scrollRes["points"].([]interface{})
		for _, p := range points {
			pointMap, _ := p.(map[string]interface{})
			if payload, ok := pointMap["payload"].(map[string]interface{}); ok {
				outputs = append(outputs, payload)
			}
		}
	} else {
		searchRes, _ := result.Result.([]interface{})
		for _, r := range searchRes {
			resMap, _ := r.(map[string]interface{})
			if payload, ok := resMap["payload"].(map[string]interface{}); ok {
				outputs = append(outputs, payload)
			}
		}
	}

	return outputs, nil
}

// SearchByField busca uma nota por campo exato no payload (navegação de grafo).
func (c *QdrantClient) SearchByField(collection string, key string, value string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/collections/%s/points/scroll", c.BaseURL, collection)

	query := map[string]interface{}{
		"filter": map[string]interface{}{
			"must": []map[string]interface{}{
				{
					"key":   key,
					"match": map[string]interface{}{"value": value},
				},
			},
		},
		"limit":        1,
		"with_payload": true,
	}

	body, _ := json.Marshal(query)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		if c.useFallback(err) {
			return c.fallbackSearchByField(collection, key, value)
		}
		return nil, fmt.Errorf("erro ao criar requisição: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		// fmt.Printf("[DEBUG-QDRANT] 🏹 Enviando Scroll: %s | Triple-Auth Ativo\n", url)
		req.Header["api-key"] = []string{c.APIKey}
		req.Header.Set("Api-Key", c.APIKey)
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		if c.useFallback(err) {
			return c.fallbackSearchByField(collection, key, value)
		}
		return nil, fmt.Errorf("falha de conexão com Qdrant: %w", err)
	}
	defer resp.Body.Close()

	if err := c.checkResponse(resp); err != nil {
		if c.useFallback(err) {
			return c.fallbackSearchByField(collection, key, value)
		}
		return nil, err
	}

	var result struct {
		Result struct {
			Points []struct {
				Payload map[string]interface{} `json:"payload"`
			} `json:"points"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		if c.useFallback(err) {
			return c.fallbackSearchByField(collection, key, value)
		}
		return nil, fmt.Errorf("erro ao decodificar resposta do Qdrant: %w", err)
	}

	if len(result.Result.Points) == 0 {
		return nil, fmt.Errorf("item '%s' não encontrado em %s", value, key)
	}

	return result.Result.Points[0].Payload, nil
}

// SearchWithScores busca vetorial que retorna também o score de similaridade.
func (c *QdrantClient) SearchWithScores(collection string, vector []float32, limit int) ([]map[string]interface{}, error) {
	url := fmt.Sprintf("%s/collections/%s/points/search", c.BaseURL, collection)

	query := map[string]interface{}{
		"vector":       vector,
		"limit":        limit,
		"with_payload": true,
	}

	body, _ := json.Marshal(query)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		if c.useFallback(err) {
			return c.fallbackSearchWithScores(collection, vector, limit)
		}
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		// fmt.Printf("[DEBUG-QDRANT] 🏹 Buscando com Scores: %s | Triple-Auth Ativo\n", url)
		req.Header["api-key"] = []string{c.APIKey}
		req.Header.Set("Api-Key", c.APIKey)
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		if c.useFallback(err) {
			return c.fallbackSearchWithScores(collection, vector, limit)
		}
		return nil, err
	}
	defer resp.Body.Close()

	if err := c.checkResponse(resp); err != nil {
		if c.useFallback(err) {
			return c.fallbackSearchWithScores(collection, vector, limit)
		}
		return nil, err
	}

	var result struct {
		Result []struct {
			Score   float64                `json:"score"`
			Payload map[string]interface{} `json:"payload"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		if c.useFallback(err) {
			return c.fallbackSearchWithScores(collection, vector, limit)
		}
		return nil, err
	}

	outputs := make([]map[string]interface{}, 0)
	for _, r := range result.Result {
		entry := r.Payload
		entry["_score"] = r.Score
		outputs = append(outputs, entry)
	}

	return outputs, nil
}

// CheckCollectionExists verifica se uma coleção já existe no Qdrant.
func (c *QdrantClient) CheckCollectionExists(name string) (bool, error) {
	url := fmt.Sprintf("%s/collections/%s", c.BaseURL, name)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		if c.useFallback(err) {
			return c.fallbackCheckCollectionExists(name)
		}
		return false, err
	}
	if c.APIKey != "" {
		// fmt.Printf("[DEBUG-QDRANT] 🏹 Verificando Coleção: %s | Triple-Auth Ativo\n", url)
		req.Header["api-key"] = []string{c.APIKey}
		req.Header.Set("Api-Key", c.APIKey)
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		if c.useFallback(err) {
			return c.fallbackCheckCollectionExists(name)
		}
		return false, err
	}
	defer resp.Body.Close()

	// 200 = Existe, 404 = Não Existe
	return resp.StatusCode == http.StatusOK, nil
}

// CreateCollection cria uma nova coleção com suporte a vetores densos (Gemini).
func (c *QdrantClient) CreateCollection(name string, dimension int) error {
	url := fmt.Sprintf("%s/collections/%s", c.BaseURL, name)

	config := map[string]interface{}{
		"vectors": map[string]interface{}{
			"size":     dimension,
			"distance": "Cosine",
		},
	}

	body, _ := json.Marshal(config)
	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(body))
	if err != nil {
		if c.useFallback(err) {
			return c.fallbackCreateCollection(name, dimension)
		}
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		// fmt.Printf("[DEBUG-QDRANT] 🏹 Criando Coleção: %s | Triple-Auth Ativo\n", url)
		req.Header["api-key"] = []string{c.APIKey}
		req.Header.Set("Api-Key", c.APIKey)
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		if c.useFallback(err) {
			return c.fallbackCreateCollection(name, dimension)
		}
		return err
	}
	defer resp.Body.Close()

	if err := c.checkResponse(resp); err != nil {
		if c.useFallback(err) {
			return c.fallbackCreateCollection(name, dimension)
		}
		return fmt.Errorf("falha ao criar coleção: %w", err)
	}

	return nil
}

// GetPoints recupera múltiplos pontos específicos por ID.
func (c *QdrantClient) GetPoints(collection string, ids []uint64) ([]map[string]interface{}, error) {
	url := fmt.Sprintf("%s/collections/%s/points", c.BaseURL, collection)

	query := map[string]interface{}{
		"ids":          ids,
		"with_payload": true,
	}

	body, _ := json.Marshal(query)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		if c.useFallback(err) {
			return c.fallbackGetPoints(collection, ids)
		}
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		// fmt.Printf("[DEBUG-QDRANT] 🏹 Buscando Pontos: %s | Triple-Auth Ativo\n", url)
		req.Header["api-key"] = []string{c.APIKey}
		req.Header.Set("Api-Key", c.APIKey)
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		if c.useFallback(err) {
			return c.fallbackGetPoints(collection, ids)
		}
		return nil, err
	}
	defer resp.Body.Close()

	if err := c.checkResponse(resp); err != nil {
		if c.useFallback(err) {
			return c.fallbackGetPoints(collection, ids)
		}
		return nil, err
	}

	var result struct {
		Result []struct {
			Payload map[string]interface{} `json:"payload"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		if c.useFallback(err) {
			return c.fallbackGetPoints(collection, ids)
		}
		return nil, err
	}

	outputs := make([]map[string]interface{}, 0)
	for _, r := range result.Result {
		outputs = append(outputs, r.Payload)
	}

	return outputs, nil
}

// CountPoints retorna o número total de pontos em uma coleção.
func (c *QdrantClient) CountPoints(collection string) (int, error) {
	url := fmt.Sprintf("%s/collections/%s", c.BaseURL, collection)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		if c.useFallback(err) {
			return c.fallbackCountPoints(collection)
		}
		return 0, err
	}
	if c.APIKey != "" {
		req.Header["api-key"] = []string{c.APIKey}
		req.Header.Set("Api-Key", c.APIKey)
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		if c.useFallback(err) {
			return c.fallbackCountPoints(collection)
		}
		return 0, err
	}
	defer resp.Body.Close()

	if err := c.checkResponse(resp); err != nil {
		if c.useFallback(err) {
			return c.fallbackCountPoints(collection)
		}
		return 0, fmt.Errorf("erro ao obter estatísticas da coleção %s: %w", collection, err)
	}

	var result struct {
		Result struct {
			PointsCount  int `json:"points_count"`
			VectorsCount int `json:"vectors_count"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		if c.useFallback(err) {
			return c.fallbackCountPoints(collection)
		}
		return 0, err
	}

	return result.Result.PointsCount, nil
}

// DeleteCollection exclui permanentemente uma coleção do Qdrant.
func (c *QdrantClient) DeleteCollection(name string) error {
	url := fmt.Sprintf("%s/collections/%s", c.BaseURL, name)
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		if c.useFallback(err) {
			return c.fallbackDeleteCollection(name)
		}
		return err
	}
	if c.APIKey != "" {
		req.Header["api-key"] = []string{c.APIKey}
		req.Header.Set("Api-Key", c.APIKey)
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		if c.useFallback(err) {
			return c.fallbackDeleteCollection(name)
		}
		return err
	}
	defer resp.Body.Close()

	if err := c.checkResponse(resp); err != nil {
		if c.useFallback(err) {
			return c.fallbackDeleteCollection(name)
		}
		return fmt.Errorf("falha ao excluir coleção %s: %w", name, err)
	}

	return nil
}
