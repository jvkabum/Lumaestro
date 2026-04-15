package agents

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"Lumaestro/internal/config"
	"Lumaestro/internal/prompts"
)

// ExecutionLog representa uma linha de saída do agente.
type ExecutionLog struct {
	Source  string `json:"source"`
	Content string `json:"content"`
	Type    string `json:"type,omitempty"` // "thought", "message", "system"
}

// Executor gerencia a execução de processos CLI.
type Executor struct {
	LogChan        chan ExecutionLog
	ActiveSessions map[string]*CLISession
	mu             sync.Mutex

	// Canal para output bruto do terminal (bytes tagged por agente)
	TerminalOutput chan TerminalData

	// Modo Autônomo (--approval-mode=yolo)
	AutonomousMode bool
}

// CLISession representa uma sessão interativa com uma CLI.
type CLISession struct {
	ID             string
	AgentName      string
	Cmd            *exec.Cmd
	Stdin          io.WriteCloser
	Cancel         context.CancelFunc
	IsOneShotProxy bool

	// ConPTY — Terminal real do Windows
	Pty *ConPTYSession
}

// TerminalData representa bytes brutos de um terminal com identificação do agente.
type TerminalData struct {
	Agent string
	Data  []byte
}

// NewExecutor inicializa o executor.
func NewExecutor() *Executor {
	return &Executor{
		LogChan:        make(chan ExecutionLog, 100),
		ActiveSessions: make(map[string]*CLISession),
		TerminalOutput: make(chan TerminalData, 256),
	}
}

// StartSession inicia uma sessão interativa com terminal ConPTY real.
func (e *Executor) StartSession(ctx context.Context, agent string, sessionID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Encerra sessão anterior se existir
	if s, ok := e.ActiveSessions[sessionID]; ok {
		if s.Pty != nil {
			s.Pty.Close()
		}
		if s.Cancel != nil {
			s.Cancel()
		}
		delete(e.ActiveSessions, sessionID)
	}

	_, cancel := context.WithCancel(ctx)

	// Preparação de ambiente (Auto-Onboarding)
	e.ensureOnboarding(agent)

	// Tenta iniciar ConPTY real — se falhar, usa One-Shot Proxy como fallback
	args := []string{}
	if agent == "gemini" {
		args = append(args, "-r")
		if e.AutonomousMode {
			args = append(args, "--approval-mode=yolo")
		}
	}
	fmt.Printf("[Maestro] Iniciando ConPTY para %s...\n", agent)
	pty, err := StartConPTY(agent, args, 120, 40)
	if err != nil {
		fmt.Printf("[Maestro] ❌ Falha no ConPTY: %v\n", err)
		// Fallback: One-Shot Proxy (modo antigo)
		e.LogChan <- ExecutionLog{
			Source:  "SYSTEM",
			Content: fmt.Sprintf("⚠️ ConPTY falhou (%v). Usando One-Shot Proxy.", err),
		}

		session := &CLISession{
			ID:             sessionID,
			AgentName:      agent,
			Cancel:         cancel,
			IsOneShotProxy: true,
		}
		e.ActiveSessions[sessionID] = session
		return nil
	}

	// Modo Terminal Real — o processo CLI está vivo dentro do ConPTY
	session := &CLISession{
		ID:             sessionID,
		AgentName:      agent,
		Cancel:         cancel,
		Pty:            pty,
		IsOneShotProxy: false,
	}
	e.ActiveSessions[sessionID] = session

	// Goroutine que lê o PTY continuamente e emite bytes brutos
	go e.readPTY(session)

	return nil
}

// StartCustomSession inicia uma sessão ConPTY com comando e argumentos específicos (útil para login).
func (e *Executor) StartCustomSession(ctx context.Context, agent string, binary string, args []string, sessionID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if s, ok := e.ActiveSessions[sessionID]; ok {
		if s.Pty != nil { s.Pty.Close() }
		if s.Cancel != nil { s.Cancel() }
		delete(e.ActiveSessions, sessionID)
	}

	_, cancel := context.WithCancel(ctx)

	fmt.Printf("[Maestro] Iniciando ConPTY Custom (%s) para %s...\n", binary, agent)
	// StartConPTY assume que o primeiro argumento é o binário e os outros são args
	pty, err := StartConPTY(binary, args, 120, 40)
	if err != nil {
		cancel()
		return fmt.Errorf("falha ao iniciar terminal custom: %v", err)
	}

	session := &CLISession{
		ID:             sessionID,
		AgentName:      agent,
		Cancel:         cancel,
		Pty:            pty,
		IsOneShotProxy: false,
	}
	e.ActiveSessions[sessionID] = session

	go e.readPTY(session)
	return nil
}

// readPTY lê continuamente o output do ConPTY e envia bytes brutos
// para o canal TerminalOutput. O xterm.js renderiza as sequências ANSI.
func (e *Executor) readPTY(s *CLISession) {
	buf := make([]byte, 4096)
	for {
		n, err := s.Pty.Read(buf)
		if n > 0 {
			// Copia os bytes para não perder com a reutilização do buffer
			data := make([]byte, n)
			copy(data, buf[:n])

			// Envia bytes brutos para o frontend (xterm.js renderiza tudo)
			select {
			case e.TerminalOutput <- TerminalData{Agent: s.AgentName, Data: data}:
			default:
				// Canal cheio — descarta para não bloquear
			}
		}
		if err != nil {
			fmt.Printf("[Maestro] Terminal %s encerrado com erro/fechamento: %v\n", s.AgentName, err)
			// PTY fechou — processo terminou
			e.LogChan <- ExecutionLog{
				Source:  "SYSTEM",
				Content: fmt.Sprintf("Sessão %s (%s) encerrada.", s.ID, s.AgentName),
			}

			e.mu.Lock()
			delete(e.ActiveSessions, s.ID)
			e.mu.Unlock()

			// Sinaliza fim da sessão com Data nil
			select {
			case e.TerminalOutput <- TerminalData{Agent: s.AgentName, Data: nil}:
			default:
			}
			return
		}
	}
}

// SendInput envia texto para uma sessão ativa.
func (e *Executor) SendInput(sessionID string, input string) error {
	e.mu.Lock()
	session, ok := e.ActiveSessions[sessionID]
	e.mu.Unlock()

	if !ok {
		return fmt.Errorf("sessão '%s' não encontrada", sessionID)
	}

	// Modo Terminal Real — escreve bytes brutos no PTY (como digitar no teclado)
	if session.Pty != nil && !session.IsOneShotProxy {
		// No Windows PTY, precisa adicionar o \r para simular a tecla Enter
		_, err := session.Pty.Write([]byte(input + "\r"))
		return err
	}

	// Fallback: One-Shot Proxy (modo antigo)
	if session.IsOneShotProxy {
		go func() {
			var cmd *exec.Cmd
			cwd, _ := os.Getwd()
			binaryName := session.AgentName
			
			// Prioridade total para o GLOBAL agora que usamos -g
			if globalPath, errGL := exec.LookPath(binaryName); errGL == nil {
				binaryName = globalPath
			} else {
				// Fallback para binário local legado
				localBin := filepath.Join(cwd, "node_modules", ".bin", binaryName+".cmd")
				if _, err := os.Stat(localBin); err == nil {
					binaryName = localBin
				}
			}

			if session.AgentName == "claude" {
				cmd = exec.CommandContext(context.Background(), binaryName, "-p", input)
			} else {
				cmd = exec.CommandContext(context.Background(), binaryName, "-p", input)
			}
			cmd.Dir = filepath.Dir(binaryName) // DIRETÓRIO DE TRABALHO DO EXECUTÁVEL COMPLETO

			// Injeção de Variáveis
			cmd.Env = os.Environ()
			cfg, errConfig := config.Load()
			if errConfig == nil && cfg != nil {
				if cfg.GetActiveGeminiKey() != "" && cfg.UseGeminiAPIKey {
					cmd.Env = append(cmd.Env, "GEMINI_API_KEY="+cfg.GetActiveGeminiKey())
				}
				if cfg.ClaudeAPIKey != "" && cfg.UseClaudeAPIKey {
					cmd.Env = append(cmd.Env, "ANTHROPIC_API_KEY="+cfg.ClaudeAPIKey)
				}
			}

			stdout, _ := cmd.StdoutPipe()
			stderr, _ := cmd.StderrPipe()

			if err := cmd.Start(); err != nil {
				e.LogChan <- ExecutionLog{
					Source:  "ERROR",
					Content: fmt.Sprintf("❌ Falha de proxy para %s: %v", session.AgentName, err),
				}
				return
			}

			e.monitorSession(session, stdout, stderr, session.AgentName)
			cmd.Wait()
		}()
		return nil
	}

	_, err := io.WriteString(session.Stdin, input+"\n")
	return err
}

// SendRawInput envia bytes brutos para o PTY (para teclas especiais como Ctrl+C).
func (e *Executor) SendRawInput(sessionID string, data []byte) error {
	e.mu.Lock()
	session, ok := e.ActiveSessions[sessionID]
	e.mu.Unlock()

	if !ok {
		return fmt.Errorf("sessão '%s' não encontrada", sessionID)
	}

	if session.Pty != nil {
		_, err := session.Pty.Write(data)
		return err
	}

	return fmt.Errorf("sessão não tem PTY ativo")
}

// ResizePTY redimensiona o terminal ConPTY.
func (e *Executor) ResizePTY(sessionID string, cols, rows int) error {
	e.mu.Lock()
	session, ok := e.ActiveSessions[sessionID]
	e.mu.Unlock()

	if !ok {
		return nil // Silenciosa — resize pode acontecer durante transição
	}

	if session.Pty != nil {
		return session.Pty.Resize(cols, rows)
	}
	return nil
}

// StopSession encerra uma sessão ativa.
func (e *Executor) StopSession(sessionID string) error {
	e.mu.Lock()
	session, ok := e.ActiveSessions[sessionID]
	e.mu.Unlock()

	if ok {
		// Fecha o ConPTY se existir
		if session.Pty != nil {
			session.Pty.Close()
		}
		if session.Cancel != nil {
			session.Cancel()
		}

		e.mu.Lock()
		delete(e.ActiveSessions, sessionID)
		e.mu.Unlock()
		return nil
	}
	return fmt.Errorf("sessão '%s' não encontrada", sessionID)
}

// IsTerminalSession verifica se a sessão ativa usa ConPTY real.
func (e *Executor) IsTerminalSession(sessionID string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	session, ok := e.ActiveSessions[sessionID]
	if !ok {
		return false
	}
	return session.Pty != nil && !session.IsOneShotProxy
}

// monitorSession lê stdout e stderr e envia para o canal de logs (modo One-Shot Proxy).
func (e *Executor) monitorSession(s *CLISession, stdout, stderr io.ReadCloser, agent string) {
	reader := io.MultiReader(stdout, stderr)
	scanner := bufio.NewScanner(reader)

	for scanner.Scan() {
		line := scanner.Text()
		e.LogChan <- ExecutionLog{
			Source:  strings.ToUpper(agent),
			Content: line,
		}
	}

	if !s.IsOneShotProxy {
		e.mu.Lock()
		delete(e.ActiveSessions, s.ID)
		e.mu.Unlock()
	}
}

// ExecuteCLI roda o binário do agente para uma tarefa única (como Scan do Obsidian).
// Este método NÃO usa ConPTY — é one-shot puro via StdoutPipe.
func (e *Executor) ExecuteCLI(ctx context.Context, agent string, contextData string, query string) (string, error) {
	fullPrompt := prompts.GetOneShotRAGPrompt(contextData, query)

	var cmd *exec.Cmd
	cwd, _ := os.Getwd()
	binaryName := agent
	
	// Prioridade total para o GLOBAL via PATH
	if globalPath, errGL := exec.LookPath(binaryName); errGL == nil {
		binaryName = globalPath
	} else {
		// Fallback para binário local legado
		localBin := filepath.Join(cwd, "node_modules", ".bin", binaryName+".cmd")
		if _, err := os.Stat(localBin); err == nil {
			binaryName = localBin
		}
	}

	if agent == "claude" {
		cmd = exec.CommandContext(ctx, binaryName, "-p", fullPrompt)
	} else {
		cmd = exec.CommandContext(ctx, binaryName, "-p", fullPrompt)
	}
	cmd.Dir = cwd // DIRETÓRIO RAIZ

	// Injeção de Variáveis de Ambiente (CRÍTICO: Sem isso o CLI morre ou trava)
	cmd.Env = os.Environ()
	cfg, errConfig := config.Load()
	if errConfig == nil && cfg != nil {
		if cfg.GetActiveGeminiKey() != "" && cfg.UseGeminiAPIKey {
			cmd.Env = append(cmd.Env, "GEMINI_API_KEY="+cfg.GetActiveGeminiKey())
		}
		if cfg.ClaudeAPIKey != "" && cfg.UseClaudeAPIKey {
			cmd.Env = append(cmd.Env, "ANTHROPIC_API_KEY="+cfg.ClaudeAPIKey)
		}
	}
	// Silencia avisos do Node para um log de chat mais limpo
	cmd.Env = append(cmd.Env, "NODE_OPTIONS=--no-warnings")

	// Redireciona stdin para o vazio para evitar que o CLI trave esperando input do teclado no Windows
	cmd.Stdin = strings.NewReader("")

	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		return "", err
	}

	var finalOutput strings.Builder

	readStream := func(r io.Reader, source string) {
		buf := make([]byte, 1024)
		for {
			n, err := r.Read(buf)
			if n > 0 {
				data := string(buf[:n])
				finalOutput.WriteString(data)
				// Emite cada pedaço de texto em tempo real para o Vue
				e.LogChan <- ExecutionLog{
					Source:  source,
					Content: data,
				}
			}
			if err != nil {
				break
			}
		}
	}

	go readStream(stdout, strings.ToUpper(agent))
	go readStream(stderr, "ERROR")

	err := cmd.Wait()
	if err != nil {
		if strings.Contains(err.Error(), "executable file not found") || strings.Contains(err.Error(), "file does not exist") {
			return "", fmt.Errorf("ferramenta '%s' não encontrada no sistema", agent)
		}
	}
	return finalOutput.String(), err
}
// ensureOnboarding garante que os arquivos de configuração necessários existam
// para evitar que as CLIs travem em perguntas interativas no primeiro boot.
func (e *Executor) ensureOnboarding(agent string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}

	if agent == "claude" {
		configPath := filepath.Join(home, ".claude.json")
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			fmt.Println("[Maestro] ✨ Configurando Onboarding inicial do Claude...")
			content := `{"hasCompletedOnboarding": true}`
			_ = os.WriteFile(configPath, []byte(content), 0644)
		}
	} else if agent == "gemini" {
		configPath := filepath.Join(home, ".gemini.json")
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			fmt.Println("[Maestro] ✨ Configurando Onboarding inicial do Gemini...")
			content := `{"hasCompletedOnboarding": true}`
			_ = os.WriteFile(configPath, []byte(content), 0644)
		}
	}
}
