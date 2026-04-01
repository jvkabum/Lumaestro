package provider

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// QdrantClient gerencia a comunicação com o servidor remoto.
type QdrantClient struct {
	BaseURL string
	APIKey  string
}

// NewQdrantClient inicializa o cliente com a URL e a chave de autenticação (Coolify).
func NewQdrantClient(baseURL string, apiKey string) *QdrantClient {
	return &QdrantClient{BaseURL: baseURL, APIKey: strings.TrimSpace(apiKey)}
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

	// Log de emergência para o terminal do desenvolvedor
	fmt.Printf("[DEBUG-QDRANT] ❌ ERRO DO SERVIDOR (Status %d): %s\n", resp.StatusCode, errMsg)

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
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		fmt.Printf("[DEBUG-QDRANT] 🏹 Enviando Request: %s | Triple-Auth Ativo | KeyPrefix: %s...\n", url, c.APIKey[:4])
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
		return err
	}
	defer resp.Body.Close()

	if err := c.checkResponse(resp); err != nil {
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
		return err
	}
	defer resp.Body.Close()

	if err := c.checkResponse(resp); err != nil {
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
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		fmt.Printf("[DEBUG-QDRANT] 🏹 Enviando Request (%s): %s | Triple-Auth Ativo\n", collection, url)
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
		return nil, err
	}
	defer resp.Body.Close()

	if err := c.checkResponse(resp); err != nil {
		return nil, err
	}

	var result struct {
		Result interface{} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
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
		return nil, fmt.Errorf("erro ao criar requisição: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		fmt.Printf("[DEBUG-QDRANT] 🏹 Enviando Scroll: %s | Triple-Auth Ativo\n", url)
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
		return nil, fmt.Errorf("falha de conexão com Qdrant: %w", err)
	}
	defer resp.Body.Close()

	if err := c.checkResponse(resp); err != nil {
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
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		fmt.Printf("[DEBUG-QDRANT] 🏹 Buscando com Scores: %s | Triple-Auth Ativo\n", url)
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
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Result []struct {
			Score   float64                `json:"score"`
			Payload map[string]interface{} `json:"payload"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
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
		return false, err
	}
	if c.APIKey != "" {
		fmt.Printf("[DEBUG-QDRANT] 🏹 Verificando Coleção: %s | Triple-Auth Ativo\n", url)
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
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		fmt.Printf("[DEBUG-QDRANT] 🏹 Criando Coleção: %s | Triple-Auth Ativo\n", url)
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
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("falha ao criar coleção: status %d", resp.StatusCode)
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
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		fmt.Printf("[DEBUG-QDRANT] 🏹 Buscando Pontos: %s | Triple-Auth Ativo\n", url)
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
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Result []struct {
			Payload map[string]interface{} `json:"payload"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	outputs := make([]map[string]interface{}, 0)
	for _, r := range result.Result {
		outputs = append(outputs, r.Payload)
	}

	return outputs, nil
}
