package lmstudio_acp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
)

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string        `json:"model,omitempty"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	Stream      bool          `json:"stream"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type sessionState struct {
	SessionID string
	Messages  []chatMessage
}

type bridge struct {
	baseURL string
	model   string
	client  *http.Client
	writer  *bufio.Writer

	mu               sync.Mutex
	nextID           int
	pending          map[int]chan rpcMessage
	session          sessionState
	sessionCwd       string
	contextSize      int
	contextThreshold int
}

func newBridge() *bridge {
	base := strings.TrimSpace(os.Getenv("LUMAESTRO_LMSTUDIO_URL"))
	if base == "" {
		base = strings.TrimSpace(os.Getenv("LMSTUDIO_URL"))
	}
	if base == "" {
		base = "http://localhost:1234"
	}
	base = strings.TrimRight(base, "/")

	model := strings.TrimSpace(os.Getenv("LUMAESTRO_LMSTUDIO_MODEL"))
	if model == "" {
		model = strings.TrimSpace(os.Getenv("LMSTUDIO_MODEL"))
	}

	b := &bridge{
		baseURL: base,
		model:   model,
		client: &http.Client{
			Timeout: 300 * time.Second,
		},
		writer:           bufio.NewWriter(os.Stdout),
		pending:          map[int]chan rpcMessage{},
		nextID:           1000,
		contextSize:      4096,
		contextThreshold: 3276,
	}
	b.fetchContextSize()
	return b
}

func (b *bridge) write(msg rpcMessage) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if msg.JSONRPC == "" {
		msg.JSONRPC = "2.0"
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	if _, err := b.writer.Write(payload); err != nil {
		return err
	}
	if err := b.writer.WriteByte('\n'); err != nil {
		return err
	}
	return b.writer.Flush()
}

func (b *bridge) fetchContextSize() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.baseURL+"/v1/models", nil)
	if err != nil {
		return
	}
	req.Header.Set("Accept", "application/json")

	resp, err := b.client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[CONTEXT] Não conseguiu obter tamanho do contexto: %v. Usando padrão 4096.\n", err)
		return
	}
	defer resp.Body.Close()

	var result struct {
		Data []struct {
			ID            string        `json:"id"`
			Object        string        `json:"object"`
			owned_by      string        `json:"owned_by"`
			permission    []interface{} `json:"permission"`
			ContextLength *int          `json:"context_length,omitempty"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return
	}

	for _, m := range result.Data {
		if (b.model == "" || m.ID == b.model) && m.ContextLength != nil && *m.ContextLength > 0 {
			b.contextSize = *m.ContextLength
			b.contextThreshold = (b.contextSize * 80) / 100
			return
		}
	}
}

func (b *bridge) countTokens(text string) int {
	words := len(strings.Fields(text))
	return (words * 4) / 3
}

func (b *bridge) estimateMessagesSize(msgs []chatMessage) int {
	total := 0
	for _, m := range msgs {
		total += b.countTokens(m.Role + ": " + m.Content)
		total += 10
	}
	return total
}

func (b *bridge) summarizeHistory(ctx context.Context, messages []chatMessage) (string, error) {
	if len(messages) < 3 {
		return "", nil
	}

	toSummarize := messages[:len(messages)-1]
	summaryPrompt := "Resuma a seguinte conversa em 1-2 frases, mantendo os pontos principais e decisões:\n\n"

	for _, msg := range toSummarize {
		role := "Usuário"
		if msg.Role == "assistant" {
			role = "Assistente"
		}
		summaryPrompt += role + ": " + msg.Content + "\n"
	}

	resp, err := b.callLMStudio(ctx, []chatMessage{
		{Role: "system", Content: "Você é um assistente que cria resumos concisos de conversas. Resuma mantendo contexto."},
		{Role: "user", Content: summaryPrompt},
	})
	if err != nil {
		return "", err
	}
	return resp, nil
}

func (b *bridge) compressContextIfNeeded(ctx context.Context) error {
	size := b.estimateMessagesSize(b.session.Messages)

	if size < b.contextThreshold {
		return nil
	}

	fmt.Fprintf(os.Stderr, "[CONTEXT] Usando %d/%d tokens (%.1f%%). Compactando...\n",
		size, b.contextSize, float64(size)*100/float64(b.contextSize))

	summary, err := b.summarizeHistory(ctx, b.session.Messages)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[CONTEXT] Erro ao sumarizar: %v\n", err)
		if len(b.session.Messages) > 6 {
			b.session.Messages = b.session.Messages[len(b.session.Messages)-6:]
		}
		return nil
	}

	if strings.TrimSpace(summary) != "" {
		b.notifySessionUpdate("agent_thought_chunk", "Contexto comprimido: histórico sumarizado para continuar sem travamentos.")
		b.session.Messages = []chatMessage{
			{Role: "system", Content: "Resumo da conversa anterior: " + summary},
		}
		if len(b.session.Messages) > 0 {
			b.session.Messages = append(b.session.Messages, b.session.Messages[len(b.session.Messages)-1])
		}
	} else if len(b.session.Messages) > 6 {
		b.session.Messages = b.session.Messages[len(b.session.Messages)-6:]
	}

	newSize := b.estimateMessagesSize(b.session.Messages)
	fmt.Fprintf(os.Stderr, "[CONTEXT] Novo tamanho: %d tokens (%.1f%%)\n",
		newSize, float64(newSize)*100/float64(b.contextSize))

	return nil
}

func (b *bridge) notifySessionUpdate(kind string, text string) {
	_ = b.write(rpcMessage{
		Method: "session/update",
		Params: mustMarshal(map[string]interface{}{
			"sessionId": b.session.SessionID,
			"update": map[string]interface{}{
				"sessionUpdate": kind,
				"content": map[string]string{
					"type": "text",
					"text": text,
				},
			},
		}),
	})
}

func (b *bridge) notifyToolCall(method string, reason string) {
	label := "Executando ferramenta: " + strings.TrimSpace(method)
	if strings.TrimSpace(reason) != "" {
		label += " — " + strings.TrimSpace(reason)
	}
	_ = b.write(rpcMessage{
		Method: "session/update",
		Params: mustMarshal(map[string]interface{}{
			"sessionId": b.session.SessionID,
			"update": map[string]interface{}{
				"sessionUpdate": "tool_call",
				"text":          label,
				"content": map[string]string{
					"type": "text",
					"text": label,
				},
			},
		}),
	})
}

func (b *bridge) sendToolRequest(ctx context.Context, method string, params map[string]interface{}) (json.RawMessage, error) {
	b.mu.Lock()
	id := b.nextID
	b.nextID++
	ch := make(chan rpcMessage, 1)
	b.pending[id] = ch
	b.mu.Unlock()

	err := b.write(rpcMessage{
		ID:     id,
		Method: method,
		Params: mustMarshal(params),
	})
	if err != nil {
		b.mu.Lock()
		delete(b.pending, id)
		b.mu.Unlock()
		return nil, err
	}

	select {
	case resp := <-ch:
		if resp.Error != nil {
			return nil, fmt.Errorf("tool error [%d]: %s", resp.Error.Code, resp.Error.Message)
		}
		raw, _ := json.Marshal(resp.Result)
		return raw, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(60 * time.Second):
		return nil, fmt.Errorf("timeout aguardando resposta da ferramenta")
	}
}

func (b *bridge) dispatchResponse(msg rpcMessage) {
	idFloat, ok := msg.ID.(float64)
	if !ok {
		return
	}
	id := int(idFloat)

	b.mu.Lock()
	ch, exists := b.pending[id]
	if exists {
		delete(b.pending, id)
	}
	b.mu.Unlock()

	if exists {
		ch <- msg
	}
}

func (b *bridge) callLMStudio(ctx context.Context, msgs []chatMessage) (string, error) {
	reqBodyRaw, err := json.Marshal(chatRequest{
		Model:       b.model,
		Messages:    msgs,
		Temperature: 0.3,
		Stream:      false,
	})
	if err != nil {
		return "", err
	}

	maxRetries := 3
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			wait := time.Duration((1 << uint(attempt-1))) * time.Second
			fmt.Fprintf(os.Stderr, "[RETRY] LM Studio indisponível. Tentando novamente em %v (tentativa %d/%d)...\n", wait, attempt+1, maxRetries)
			select {
			case <-time.After(wait):
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}

		ctxWithTimeout, cancel := context.WithTimeout(ctx, 180*time.Second)
		req, err := http.NewRequestWithContext(ctxWithTimeout, http.MethodPost, b.baseURL+"/v1/chat/completions", bytes.NewReader(reqBodyRaw))

		if err != nil {
			cancel()
			lastErr = err
			continue
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := b.client.Do(req)
		cancel()

		if err != nil {
			lastErr = fmt.Errorf("conexão falhou: %v", err)
			if strings.Contains(err.Error(), "context deadline exceeded") {
				lastErr = fmt.Errorf("timeout: LM Studio em %s demorando muito (modelo processando). Verifique se o modelo está sobrecarregado ou se a rede está lenta", b.baseURL)
			}
			if strings.Contains(err.Error(), "context canceled") {
				lastErr = fmt.Errorf("requisição cancelada: conexão com LM Studio foi abortada. Verifique a rede", b.baseURL)
			}
			continue
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			if resp.StatusCode >= 500 {
				lastErr = fmt.Errorf("LM Studio error %d (servidor). Tente novamente", resp.StatusCode)
				continue
			}
			lastErr = fmt.Errorf("erro %d: %s", resp.StatusCode, string(body))
			return "", lastErr
		}

		var cr chatResponse
		if err := json.Unmarshal(body, &cr); err != nil {
			lastErr = fmt.Errorf("resposta invalida: %v", err)
			continue
		}
		if cr.Error != nil {
			lastErr = fmt.Errorf("erro do modelo: %s", cr.Error.Message)
			if strings.Contains(strings.ToLower(cr.Error.Message), "overload") {
				fmt.Fprintf(os.Stderr, "[LM STUDIO] Modelo sobrecarregado. Aguardando...\n")
			}
			continue
		}
		if len(cr.Choices) == 0 {
			lastErr = fmt.Errorf("resposta vazia")
			continue
		}

		return strings.TrimSpace(cr.Choices[0].Message.Content), nil
	}

	if lastErr != nil {
		return "", fmt.Errorf("❌ LM Studio inacessível após %d tentativas: %v", maxRetries, lastErr)
	}
	return "", fmt.Errorf("❌ Falha ao conectar com LM Studio em %s", b.baseURL)
}

type toolDirective struct {
	Type   string                 `json:"type"`
	Final  string                 `json:"final"`
	Tool   *toolCall              `json:"tool"`
	Reason string                 `json:"reason"`
	Data   map[string]interface{} `json:"data"`
}

type toolCall struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params"`
}

func parseDirective(content string) (*toolDirective, bool) {
	clean := strings.TrimSpace(content)
	if strings.HasPrefix(clean, "```") {
		clean = strings.TrimPrefix(clean, "```")
		clean = strings.TrimSpace(clean)
		if strings.HasPrefix(strings.ToLower(clean), "json") {
			clean = strings.TrimSpace(clean[4:])
		}
		clean = strings.TrimSuffix(clean, "```")
		clean = strings.TrimSpace(clean)
	}

	// Tenta extrair o primeiro objeto JSON caso haja texto extra.
	if !strings.HasPrefix(clean, "{") || !strings.HasSuffix(clean, "}") {
		re := regexp.MustCompile(`\{(?s:.*)\}`)
		if m := re.FindString(clean); m != "" {
			clean = m
		}
	}

	var d toolDirective
	if err := json.Unmarshal([]byte(clean), &d); err != nil {
		return nil, false
	}
	return &d, true
}

func looksLikeRootListingRequest(prompt string) bool {
	p := strings.ToLower(prompt)
	markers := []string{
		"pasta raiz", "raiz do projeto", "listar arquivos", "lista de arquivos", "ver arquivos", "root folder", "project root", "list files",
	}
	for _, m := range markers {
		if strings.Contains(p, m) {
			return true
		}
	}
	return false
}

func (b *bridge) solveWithTools(ctx context.Context, userPrompt string) (string, error) {
	if err := b.compressContextIfNeeded(ctx); err != nil {
		return "", err
	}

	osDirective := "System OS: Linux."
	if runtime.GOOS == "windows" {
		osDirective = "System OS: Windows. Use PowerShell/cmd semantics and Windows-compatible paths."
	} else if runtime.GOOS == "darwin" {
		osDirective = "System OS: macOS. Use POSIX/zsh semantics and macOS-compatible commands."
	}

	baseInstruction := "You are an ACP-compatible coding agent inside Lumaestro. " +
		"You MUST use tools whenever information depends on filesystem, shell, or project files. " +
		"Autonomous mode is active: do not ask user for confirmation before executing safe allowed actions. " +
		osDirective + " " +
		"Never say you cannot access files before attempting a tool call. " +
		"Available methods: read_file, write_file, delete_file, move_file, run_command, Lumaestro/delegate_task, Lumaestro/complete_task, Lumaestro/request_approval. " +
		"For folder listing in Windows, prefer run_command with command=cmd and args=[\"/C\",\"dir\",\"/b\"]. " +
		"Respond ONLY strict JSON. " +
		"If a tool is needed, respond as: {\"type\":\"tool_call\",\"tool\":{\"method\":\"read_file\",\"params\":{...}},\"reason\":\"...\"}. " +
		"If no tool is needed, respond as: {\"type\":\"final\",\"final\":\"...\"}. " +
		"Never return markdown in directive mode."

	work := append([]chatMessage{}, b.session.Messages...)
	work = append(work, chatMessage{Role: "system", Content: baseInstruction})
	work = append(work, chatMessage{Role: "user", Content: userPrompt})
	b.notifySessionUpdate("agent_thought_chunk", "Analisando a solicitação...")

	if looksLikeRootListingRequest(userPrompt) {
		b.notifyToolCall("run_command", "Listar arquivos da pasta raiz")
		b.notifySessionUpdate("agent_thought_chunk", "Listando arquivos da pasta raiz via ferramenta...")
		toolRes, toolErr := b.sendToolRequest(ctx, "run_command", map[string]interface{}{
			"command": "cmd",
			"args":    []string{"/C", "dir", "/b"},
		})
		if toolErr == nil {
			b.notifySessionUpdate("agent_thought_chunk", "Arquivos da raiz coletados com sucesso.")
			return "Arquivos na pasta raiz do projeto (ferramenta):\n" + string(toolRes), nil
		}
		work = append(work, chatMessage{Role: "assistant", Content: "{\"type\":\"tool_call\",\"tool\":{\"method\":\"run_command\",\"params\":{\"command\":\"cmd\",\"args\":[\"/C\",\"dir\",\"/b\"]}},\"reason\":\"Listar arquivos da raiz\"}"})
		work = append(work, chatMessage{Role: "user", Content: "Tool error: " + toolErr.Error()})
	}

	for step := 0; step < 4; step++ {
		resp, err := b.callLMStudio(ctx, work)
		if err != nil {
			return "", err
		}

		d, ok := parseDirective(resp)
		if !ok || d == nil {
			// Tenta corrigir uma vez quando o modelo devolve texto livre.
			if step == 0 {
				work = append(work, chatMessage{Role: "assistant", Content: resp})
				work = append(work, chatMessage{Role: "user", Content: "Your last answer was not valid JSON directive. Respond again using strict JSON only."})
				continue
			}
			return resp, nil
		}

		if strings.EqualFold(d.Type, "final") {
			if strings.TrimSpace(d.Final) == "" {
				return resp, nil
			}
			return d.Final, nil
		}

		if strings.EqualFold(d.Type, "tool_call") && d.Tool != nil && strings.TrimSpace(d.Tool.Method) != "" {
			b.notifyToolCall(d.Tool.Method, d.Reason)
			b.notifySessionUpdate("agent_thought_chunk", "Usando ferramenta: "+d.Tool.Method)
			toolRes, toolErr := b.sendToolRequest(ctx, d.Tool.Method, d.Tool.Params)
			if toolErr != nil {
				b.notifySessionUpdate("agent_thought_chunk", "Falha na ferramenta: "+toolErr.Error())
				work = append(work, chatMessage{Role: "assistant", Content: resp})
				work = append(work, chatMessage{Role: "user", Content: "Tool error: " + toolErr.Error()})
				continue
			}
			b.notifySessionUpdate("agent_thought_chunk", "Ferramenta concluída: "+d.Tool.Method)
			work = append(work, chatMessage{Role: "assistant", Content: resp})
			work = append(work, chatMessage{Role: "user", Content: "Tool result for " + d.Tool.Method + ": " + string(toolRes)})
			continue
		}

		return resp, nil
	}

	resp, err := b.callLMStudio(ctx, append(b.session.Messages, chatMessage{Role: "user", Content: userPrompt}))
	if err != nil {
		return "", err
	}
	return resp, nil
}

func (b *bridge) handleRequest(msg rpcMessage) {
	method := strings.ToLower(strings.TrimSpace(msg.Method))

	sendResult := func(res interface{}) {
		_ = b.write(rpcMessage{ID: msg.ID, Result: res})
	}
	sendErr := func(code int, text string) {
		_ = b.write(rpcMessage{ID: msg.ID, Error: &rpcError{Code: code, Message: text}})
	}

	switch method {
	case "initialize":
		sendResult(map[string]interface{}{
			"protocolVersion": 1,
			"serverInfo": map[string]string{
				"name":    "lmstudio-acp-bridge",
				"version": "0.1.0",
			},
		})
	case "authenticate":
		sendResult(map[string]interface{}{"authenticated": true})
	case "session/new":
		var p struct {
			Cwd string `json:"cwd"`
		}
		_ = json.Unmarshal(msg.Params, &p)
		if p.Cwd != "" {
			b.sessionCwd = p.Cwd
		}
		b.session = sessionState{
			SessionID: fmt.Sprintf("lmstudio-%d", time.Now().UnixNano()),
			Messages:  []chatMessage{},
		}
		sendResult(map[string]interface{}{"sessionId": b.session.SessionID})
	case "session/load":
		var p struct {
			SessionID string `json:"sessionId"`
			Cwd       string `json:"cwd"`
		}
		_ = json.Unmarshal(msg.Params, &p)
		if p.Cwd != "" {
			b.sessionCwd = p.Cwd
		}
		if p.SessionID == "" {
			p.SessionID = fmt.Sprintf("lmstudio-%d", time.Now().UnixNano())
		}
		b.session.SessionID = p.SessionID
		sendResult(map[string]interface{}{"sessionId": b.session.SessionID})
	case "session/set_mode":
		sendResult(map[string]interface{}{"ok": true})
	case "session/prompt":
		if b.session.SessionID == "" {
			b.session.SessionID = fmt.Sprintf("lmstudio-%d", time.Now().UnixNano())
		}

		var p struct {
			SessionID string `json:"sessionId"`
			Prompt    []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"prompt"`
		}
		_ = json.Unmarshal(msg.Params, &p)

		parts := make([]string, 0, len(p.Prompt))
		for _, item := range p.Prompt {
			if strings.EqualFold(item.Type, "text") && strings.TrimSpace(item.Text) != "" {
				parts = append(parts, item.Text)
			}
		}
		userText := strings.TrimSpace(strings.Join(parts, "\n\n"))
		if userText == "" {
			userText = "Continue."
		}

		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
		defer cancel()

		answer, err := b.solveWithTools(ctx, userText)
		if err != nil {
			sendErr(-32000, err.Error())
			return
		}

		b.session.Messages = append(b.session.Messages,
			chatMessage{Role: "user", Content: userText},
			chatMessage{Role: "assistant", Content: answer},
		)

		if strings.TrimSpace(answer) == "" {
			answer = "(sem conteudo)"
		}
		b.notifySessionUpdate("agent_thought_chunk", "Resposta final pronta. Publicando no chat...")
		b.notifySessionUpdate("agent_message_chunk", answer)
		b.notifySessionUpdate("agent_turn_complete", "")

		sendResult(map[string]interface{}{"ok": true})
	default:
		sendErr(-32601, "method not found")
	}
}

func mustMarshal(v interface{}) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func main() {
	b := newBridge()
	incoming := make(chan rpcMessage, 64)

	go func() {
		reader := bufio.NewScanner(os.Stdin)
		for reader.Scan() {
			line := strings.TrimSpace(reader.Text())
			if line == "" {
				continue
			}

			var msg rpcMessage
			if err := json.Unmarshal([]byte(line), &msg); err != nil {
				_ = b.write(rpcMessage{Error: &rpcError{Code: -32700, Message: "invalid json"}})
				continue
			}

			if msg.Method == "" && msg.ID != nil {
				b.dispatchResponse(msg)
				continue
			}

			incoming <- msg
		}
		close(incoming)
	}()

	for msg := range incoming {
		b.handleRequest(msg)
	}
}
