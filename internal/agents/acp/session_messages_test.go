package acp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCleanUserRequestPrompt(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{
			input:    "<USER_REQUEST>\n[IDIOMA] Responda em PT-BR\nOBJETIVO: ola\n</USER_REQUEST>\n<ADDITIONAL_METADATA>\nTime: 123\n</ADDITIONAL_METADATA>",
			expected: "ola",
		},
		{
			input:    "<USER_REQUEST>\n[IDIOMA] PT-BR\nOBJETIVO: oque voce acha do meu projeto?\n</USER_REQUEST>",
			expected: "oque voce acha do meu projeto?",
		},
		{
			input:    "<USER_REQUEST>como funciona a gravidade?</USER_REQUEST>",
			expected: "como funciona a gravidade?",
		},
		{
			input:    "apenas um texto direto",
			expected: "apenas um texto direto",
		},
	}

	for _, c := range cases {
		got := cleanUserRequestPrompt(c.input)
		if got != c.expected {
			t.Errorf("cleanUserRequestPrompt(%q) = %q; want %q", c.input, got, c.expected)
		}
	}
}

func TestParseAntigravityTranscript(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "transcript.jsonl")

	content := `{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","content":"<USER_REQUEST>\nOBJETIVO: teste de pergunta\n</USER_REQUEST>"}
{"step_index":1,"source":"SYSTEM","type":"SYSTEM_MESSAGE","content":"system alert"}
{"step_index":2,"source":"MODEL","type":"PLANNER_RESPONSE","content":"resposta do modelo","thinking":"pensamento interno"}
{"step_index":3,"source":"MODEL","type":"PLANNER_RESPONSE","content":"","thinking":"apenas pensamento sem texto"}
`
	if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
		t.Fatalf("falha ao criar arquivo de teste: %v", err)
	}

	msgs, err := parseAntigravityTranscript(logPath)
	if err != nil {
		t.Fatalf("parseAntigravityTranscript retornou erro: %v", err)
	}

	if len(msgs) != 2 {
		t.Fatalf("esperava 2 mensagens (1 user, 1 assistant), obteve %d", len(msgs))
	}

	if msgs[0].Role != "user" || msgs[0].Text != "teste de pergunta" {
		t.Errorf("mensagem 0 incorreta: %+v", msgs[0])
	}

	if msgs[1].Role != "assistant" || msgs[1].Text != "resposta do modelo" || msgs[1].Thought != "pensamento interno" {
		t.Errorf("mensagem 1 incorreta: %+v", msgs[1])
	}
}

func TestGetSessionMessagesReal(t *testing.T) {
	executor := &ACPExecutor{
		Workspace: "D:\\Git Hub\\Fortress",
	}
	sessionID := "ad33dad5-0720-4f42-a42b-8ecab05a265f"
	msgs, err := executor.GetSessionMessages(sessionID)
	if err != nil {
		t.Fatalf("erro ao buscar mensagens: %v", err)
	}
	t.Logf("Encontradas %d mensagens na sessão real %s", len(msgs), sessionID)
	for i, m := range msgs {
		t.Logf("[%d] %s: %s", i, m.Role, m.Text[:min(40, len(m.Text))])
	}
	if len(msgs) == 0 {
		t.Log("Aviso: sessão real não encontrada no caminho padrão deste ambiente de teste")
	}
}
