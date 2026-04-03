package lightning

import (
	"encoding/json"
	"fmt"
)

// OpenAIRequest é o formato padrão que o Proxy recebe.
type OpenAIRequest struct {
	Model    string `json:"model"`
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
	Stream bool `json:"stream"`
}

// MapToGemini converte o formato OpenAI para o formato Google AI (Gemini).
func MapToGemini(req OpenAIRequest, apiKey string) ([]byte, string, error) {
	// Estrutura simplificada do Gemini: {"contents": [{"parts":[{"text": "..."}]}]}
	type Part struct {
		Text string `json:"text"`
	}
	type Content struct {
		Role  string `json:"role,omitempty"`
		Parts []Part `json:"parts"`
	}
	type GeminiRequest struct {
		Contents []Content `json:"contents"`
	}

	geminiReq := GeminiRequest{
		Contents: make([]Content, 0, len(req.Messages)),
	}

	for _, m := range req.Messages {
		role := m.Role
		if role == "assistant" {
			role = "model"
		} else if role == "system" {
			// Nota: No Gemini 1.5+, System Instruction é um campo aparte, 
			// mas por compatibilidade aqui tratamos como 'user' se necessário ou simplificado.
			role = "user"
		}

		geminiReq.Contents = append(geminiReq.Contents, Content{
			Role: role,
			Parts: []Part{
				{Text: m.Content},
			},
		})
	}

	raw, err := json.Marshal(geminiReq)
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", req.Model, apiKey)
	
	return raw, url, err
}
