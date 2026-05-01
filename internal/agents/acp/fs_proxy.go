package acp

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"Lumaestro/internal/config"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// FSProxy gerencia o acesso ao sistema de arquivos para os agentes.
type FSProxy struct {
	CPI *CPIValidator
	Ctx context.Context
}

func NewFSProxy(cpi *CPIValidator) *FSProxy {
	return &FSProxy{
		CPI: cpi,
	}
}

// IsCommandAllowed verifica se o comando está na lista de binários autorizados.
func (p *FSProxy) IsCommandAllowed(command string) bool {
	base := strings.ToLower(filepath.Base(command))
	base = strings.TrimSuffix(base, ".exe")

	allowed := map[string]bool{
		"npm":   true,
		"git":   true,
		"ls":    true,
		"dir":   true,
		"pwd":   true,
		"mkdir": true,
		"rm":    true,
		"cp":    true,
		"mv":    true,
		"cat":   true,
		"type":  true,
		"echo":  true,
		"pnpm":  true,
		"yarn":  true,
		"wails": true,
		"cmd":   true,
	}

	return allowed[base]
}

func (p *FSProxy) getSecurityConfig() config.SecurityConfig {
	cfg, _ := config.Load()
	return cfg.Security
}

func (p *FSProxy) ReadFile(path string) (string, error) {
	sc := p.getSecurityConfig()
	if !sc.AllowRead {
		return "", fmt.Errorf("🔒 BLOQUEADO: Leitura de arquivos não autorizada")
	}
	
	validPath, err := p.CPI.ValidatePath(path)
	if err != nil {
		p.emitSecurityAlert("TENTATIVA DE LEITURA BLOQUEADA", path)
		return "", err
	}
	
	data, err := os.ReadFile(validPath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (p *FSProxy) WriteFile(path string, content string) error {
	sc := p.getSecurityConfig()
	
	validPath, err := p.CPI.ValidatePath(path)
	if err != nil {
		p.emitSecurityAlert("TENTATIVA DE ESCRITA BLOQUEADA", path)
		return err
	}

	fileExists := false
	if _, err := os.Stat(validPath); err == nil {
		fileExists = true
	}
	
	if fileExists && !sc.AllowWrite {
		return fmt.Errorf("🔒 BLOQUEADO: Sobrescrita de arquivos desativada")
	}
	
	if !fileExists && !sc.AllowCreate {
		return fmt.Errorf("🔒 BLOQUEADO: Criação de arquivos desativada")
	}

	if err := os.MkdirAll(filepath.Dir(validPath), 0755); err != nil {
		return err
	}

	return os.WriteFile(validPath, []byte(content), 0644)
}

func (p *FSProxy) DeleteFile(path string) error {
	validPath, err := p.CPI.ValidatePath(path)
	if err != nil {
		p.emitSecurityAlert("TENTATIVA DE DELEÇÃO BLOQUEADA", path)
		return err
	}
	return os.Remove(validPath)
}

func (p *FSProxy) MoveFile(oldPath, newPath string) error {
	vOld, err := p.CPI.ValidatePath(oldPath)
	if err != nil {
		return err
	}
	vNew, err := p.CPI.ValidatePath(newPath)
	if err != nil {
		return err
	}
	return os.Rename(vOld, vNew)
}

func (p *FSProxy) RunCommand(command string, args []string) (string, error) {
	sc := p.getSecurityConfig()
	if !sc.AllowRunCommands {
		return "", fmt.Errorf("🔒 BLOQUEADO: Execução de comandos desativada")
	}
	
	if !p.IsCommandAllowed(command) {
		p.emitSecurityAlert("COMANDO NÃO AUTORIZADO", command)
		return "", fmt.Errorf("🛡️ CPI BLOQUEIO: Comando '%s' não autorizado", command)
	}

	if err := p.CPI.ValidateCommand(command, args); err != nil {
		p.emitSecurityAlert("ARGUMENTOS MALICIOSOS BLOQUEADOS", command)
		return "", err
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = p.CPI.GetOrbitCWD()

	minimalEnv := []string{
		"PATH=" + p.GetSanitizedPath(),
		"TEMP=" + os.Getenv("TEMP"),
		"TMP=" + os.Getenv("TMP"),
		"SystemRoot=" + os.Getenv("SystemRoot"),
	}
	cmd.Env = minimalEnv

	output, err := cmd.CombinedOutput()
	return string(output), err
}

func (p *FSProxy) GetSanitizedPath() string {
	originalPath := os.Getenv("PATH")
	sep := string(os.PathListSeparator)
	parts := strings.Split(originalPath, sep)
	var trustedParts []string
	trustedPatterns := []string{"system32", "windows", "bin", "program files", "usr/local", "usr/bin", "go/bin", "nodejs", "npm", "git"}
	for _, part := range parts {
		lowerPart := strings.ToLower(part)
		if strings.Contains(lowerPart, "users") || strings.Contains(lowerPart, "desktop") || strings.Contains(lowerPart, "downloads") {
			continue
		}
		isTrusted := false
		for _, pattern := range trustedPatterns {
			if strings.Contains(lowerPart, pattern) {
				isTrusted = true
				break
			}
		}
		if isTrusted {
			trustedParts = append(trustedParts, part)
		}
	}
	return strings.Join(trustedParts, sep)
}

func (p *FSProxy) emitSecurityAlert(action string, target string) {
	if p.Ctx == nil {
		return
	}
	runtime.EventsEmit(p.Ctx, "agent:status", map[string]string{
		"agent":  "system",
		"action": fmt.Sprintf("🛡️ CPI: %s [%s]", action, target),
		"kind":   "error",
	})
}
