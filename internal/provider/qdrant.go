package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

// QdrantClient gerencia a comunicação com o servidor remoto.
type QdrantClient struct {
	BaseURL string
}

// NewQdrantClient inicializa o cliente com a URL do Coolify.
func NewQdrantClient(url string) *QdrantClient {
	return &QdrantClient{BaseURL: url}
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

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

// Search busca os pontos mais próximos de um vetor na coleção
func (c *QdrantClient) Search(collection string, vector []float32, limit int) ([]map[string]interface{}, error) {
	url := fmt.Sprintf("%s/collections/%s/points/search", c.BaseURL, collection)

	query := map[string]interface{}{
		"vector":       vector,
		"limit":        limit,
		"with_payload": true,
	}

	body, _ := json.Marshal(query)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
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

// SearchByName busca uma nota por nome exato no payload (navegação de grafo).
func (c *QdrantClient) SearchByName(collection string, name string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/collections/%s/points/scroll", c.BaseURL, collection)

	query := map[string]interface{}{
		"filter": map[string]interface{}{
			"must": []map[string]interface{}{
				{
					"key":   "name",
					"match": map[string]interface{}{"value": name},
				},
			},
		},
		"limit":        1,
		"with_payload": true,
	}

	body, _ := json.Marshal(query)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Result struct {
			Points []struct {
				Payload map[string]interface{} `json:"payload"`
			} `json:"points"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Result.Points) == 0 {
		return nil, fmt.Errorf("nota '%s' não encontrada", name)
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
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
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
