package lightning

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// ProxyServer é o serviço Go que intercepta chamadas de LLM.
type ProxyServer struct {
	Store  *DuckDBStore
	Port   string
	Server *http.Server
}

// NewProxyServer cria uma nova instância do interceptor.
func NewProxyServer(store *DuckDBStore, port string) *ProxyServer {
	return &ProxyServer{
		Store: store,
		Port:  port,
	}
}

// Start inicia o servidor de interceptação em uma goroutine.
func (p *ProxyServer) Start() {
	mux := http.NewServeMux()
	
	// Rota compatível com OpenAI Chat Completions
	mux.HandleFunc("/v1/chat/completions", p.handleChatCompletions)
	
	// Rota genérica para capturar qualquer coisa
	mux.HandleFunc("/", p.handleCatchAll)

	p.Server = &http.Server{
		Addr:    ":" + p.Port,
		Handler: mux,
	}

	go func() {
		log.Printf("[⚡ Lightning Proxy] Ouvindo na porta %s...\n", p.Port)
		if err := p.Server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[❌ Lightning Proxy] Erro: %v\n", err)
		}
	}()
}

// handleChatCompletions intercepta, loga e encaminha a chamada.
func (p *ProxyServer) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	rolloutID := r.Header.Get("x-rollout-id")
	if rolloutID == "" {
		rolloutID = "default-" + uuid.NewString()
	}
	attemptID := r.Header.Get("x-attempt-id")
	if attemptID == "" {
		attemptID = "att-" + uuid.NewString()
	}

	body, _ := io.ReadAll(r.Body)
	startTime := GetNowTimestamp()

	// 🕵️ Log inicial no DuckDB (Início do Span)
	span := Span{
		RolloutID:  rolloutID,
		AttemptID:  attemptID,
		SequenceID: 0, // Incrementar conforme real
		TraceID:    uuid.NewString(),
		SpanID:     uuid.NewString(),
		Name:       "llm_call",
		StartTime:  startTime,
		Status:     TraceStatus{StatusCode: "UNSET"},
		Attributes: map[string]interface{}{
			"request_body": string(body),
			"method":       r.Method,
			"path":         r.URL.Path,
		},
	}

	// 🚀 Forward Real para o Provedor
	// Por padrão, se não houver URL, usamos o Mapper interno ou Gemini
	targetURL := "https://generativelanguage.googleapis.com/v1beta/openai/chat/completions"
	if r.URL.Query().Get("provider") == "openai" {
		targetURL = "https://api.openai.com/v1/chat/completions"
	}

	req, _ := http.NewRequest("POST", targetURL, bytes.NewBuffer(body))
	req.Header = r.Header.Clone()
	
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	
	if err != nil {
		span.Status.StatusCode = "ERROR"
		errStr := err.Error()
		span.Status.Description = &errStr
		p.Store.InsertSpan(span)
		http.Error(w, "Erro ao encaminhar para o LLM: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	endTime := GetNowTimestamp()
	span.EndTime = &endTime
	span.Status.StatusCode = "OK"

	// 🕵️ EXTRAÇÃO DE USO (TOKENS)
	var result struct {
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	json.Unmarshal(respBody, &result)
	span.PromptTokens = result.Usage.PromptTokens
	span.CompletionTokens = result.Usage.CompletionTokens

	// Salva o rastro completo
	p.Store.InsertSpan(span)

	// Devolve a resposta original para o agente
	for k, v := range resp.Header {
		w.Header()[k] = v
	}
	w.WriteHeader(resp.StatusCode)
	w.Write(respBody)
}

func (p *ProxyServer) handleCatchAll(w http.ResponseWriter, r *http.Request) {
	log.Printf("[⚡ Lightning Proxy] Chamada não mapeada: %s %s\n", r.Method, r.URL.Path)
	w.WriteHeader(http.StatusNotFound)
}

// Stop desliga o proxy.
func (p *ProxyServer) Stop() {
	if p.Server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		p.Server.Shutdown(ctx)
	}
}
