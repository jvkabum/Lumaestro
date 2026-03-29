//go:build windows

package agents

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"Lumaestro/internal/config"

	"github.com/charmbracelet/x/conpty"
)

// ConPTYSession encapsula uma sessão ConPTY real no Windows.
// O processo filho acredita que está num terminal real (cores, ANSI, spinners).
type ConPTYSession struct {
	Pty     *conpty.ConPty
	Process *os.Process
	mu      sync.Mutex
	closed  bool
}

// StartConPTY cria um pseudo-terminal ConPTY real e spawna o comando dentro dele.
func StartConPTY(command string, args []string, cols, rows int) (*ConPTYSession, error) {
	if cols <= 0 {
		cols = 120
	}
	if rows <= 0 {
		rows = 40
	}

	// Cria o ConPTY com dimensões iniciais
	cpty, err := conpty.New(cols, rows, 0)
	if err != nil {
		return nil, fmt.Errorf("falha ao criar ConPTY: %w", err)
	}

	// Resolve o caminho do executável (Prioriza Local do Lumaestro)
	var cmdPath string
	cwd, _ := os.Getwd()
	
	// Tenta binário local (.cmd para Windows)
	localBin := filepath.Join(cwd, "node_modules", ".bin", command+".cmd")
	if _, err := os.Stat(localBin); err == nil {
		cmdPath = localBin
	} else {
		// Fallback para o PATH do sistema
		var errLook error
		cmdPath, errLook = exec.LookPath(command)
		if errLook != nil {
			cpty.Close()
			return nil, fmt.Errorf("executável '%s' não encontrado localmente nem no PATH: %w", command, errLook)
		}
	}

	// Resolve o caminho do executável (Prioriza Local do Lumaestro)
	cwd, _ = os.Getwd()
	localBinDir := filepath.Join(cwd, "node_modules", ".bin")

	// Injeta variáveis de ambiente e garante que o PATH inclua o NODE e o .bin local
	env := os.Environ()
	pathUpdated := false
	for i, v := range env {
		if strings.HasPrefix(strings.ToUpper(v), "PATH=") {
			// Adiciona o diretório .bin local no início do PATH
			env[i] = "PATH=" + localBinDir + ";" + v[5:]
			pathUpdated = true
			break
		}
	}
	if !pathUpdated {
		env = append(env, "PATH="+localBinDir)
	}

	cfg, errConfig := config.Load()
	if errConfig == nil && cfg != nil {
		if cfg.GeminiAPIKey != "" && cfg.UseGeminiAPIKey {
			env = append(env, "GEMINI_API_KEY="+cfg.GeminiAPIKey)
		}
		if cfg.ClaudeAPIKey != "" && cfg.UseClaudeAPIKey {
			env = append(env, "ANTHROPIC_API_KEY="+cfg.ClaudeAPIKey)
		}
	}

	// Adiciona flags de compatibilidade para Node 22 (ESM)
	env = append(env, "NODE_OPTIONS=--no-warnings")

	// Estratégia Robusta: Usar o CMD oficial do sistema
	spawnCmd := os.Getenv("COMSPEC")
	if spawnCmd == "" {
		spawnCmd = "C:\\Windows\\System32\\cmd.exe"
	}
	
	// Prepara o comando final. O /c executa o script e encerra, mas como o Gemini/Claude 
	// são interativos, o processo se manterá aberto dentro do PTY.
	spawnArgs := []string{spawnCmd, "/c", cmdPath}
	
	if len(args) > 0 {
		spawnArgs = append(spawnArgs, args...)
	}

	// Spawna o processo dentro do ConPTY
	pid, _, err := cpty.Spawn(
		spawnCmd,
		spawnArgs,
		&syscall.ProcAttr{
			Dir: cwd, // DIRETÓRIO RAIZ: Fundamental para o Node resolver as dependências
			Env: env,
		},
	)
	if err != nil {
		cpty.Close()
		return nil, fmt.Errorf("falha ao spawnar '%s' com o comando '%s': %w", spawnCmd, cmdPath, err)
	}

	// Obtém o handle do processo pelo PID
	proc, err := os.FindProcess(pid)
	if err != nil {
		cpty.Close()
		return nil, fmt.Errorf("falha ao encontrar processo PID %d: %w", pid, err)
	}

	return &ConPTYSession{
		Pty:     cpty,
		Process: proc,
	}, nil
}

// Read lê bytes do output do PTY (o que o processo escreve no terminal).
func (s *ConPTYSession) Read(p []byte) (int, error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return 0, io.EOF
	}
	s.mu.Unlock()
	return s.Pty.Read(p)
}

// Write escreve bytes no PTY (simula digitação do teclado).
func (s *ConPTYSession) Write(p []byte) (int, error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return 0, io.ErrClosedPipe
	}
	s.mu.Unlock()
	return s.Pty.Write(p)
}

// Resize altera as dimensões do PTY (quando o xterm.js redimensiona).
func (s *ConPTYSession) Resize(cols, rows int) error {
	if cols <= 0 || rows <= 0 {
		return nil
	}
	return s.Pty.Resize(cols, rows)
}

// Close encerra o ConPTY e mata o processo.
func (s *ConPTYSession) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}
	s.closed = true

	// Fecha o PTY (isso sinaliza EOF pro processo)
	if s.Pty != nil {
		s.Pty.Close()
	}

	// Mata o processo se ainda estiver rodando
	if s.Process != nil {
		s.Process.Kill()
	}

	return nil
}

// Wait espera o processo terminar e retorna o status.
func (s *ConPTYSession) Wait() (*os.ProcessState, error) {
	if s.Process == nil {
		return nil, fmt.Errorf("processo não existe")
	}
	return s.Process.Wait()
}
