package acp

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"Lumaestro/internal/config"
)

// FSProxy gerencia o acesso ao sistema de arquivos para os agentes.
type FSProxy struct{}

func NewFSProxy() *FSProxy {
	return &FSProxy{}
}

func (p *FSProxy) getSecurityConfig() config.SecurityConfig {
	cfg, _ := config.Load()
	return cfg.Security
}

func (p *FSProxy) isPathAllowed(path string) bool {
	sc := p.getSecurityConfig()
	if sc.FullMachineAccess {
		return true
	}
	
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	
	cwd, _ := os.Getwd()
	return strings.HasPrefix(absPath, cwd)
}

func (p *FSProxy) ReadFile(path string) (string, error) {
	sc := p.getSecurityConfig()
	if !sc.AllowRead {
		return "", fmt.Errorf("🔒 BLOQUEADO: Leitura de arquivos não autorizada. Ative em Configurações > Segurança")
	}
	
	if !p.isPathAllowed(path) {
		return "", fmt.Errorf("🔒 BLOQUEADO: Acesso fora do diretório do projeto não permitido.")
	}
	
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (p *FSProxy) WriteFile(path string, content string) error {
	sc := p.getSecurityConfig()
	
	fileExists := false
	if _, err := os.Stat(path); err == nil {
		fileExists = true
	}
	
	if fileExists && !sc.AllowWrite {
		return fmt.Errorf("🔒 BLOQUEADO: Sobrescrita de arquivos desativada. Ative em Configurações > Segurança")
	}
	
	if !fileExists && !sc.AllowCreate {
		return fmt.Errorf("🔒 BLOQUEADO: Criação de arquivos desativada. Ative em Configurações > Segurança")
	}

	if !p.isPathAllowed(path) {
		return fmt.Errorf("🔒 BLOQUEADO: Acesso fora do diretório do projeto não permitido.")
	}
	
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	
	return os.WriteFile(path, []byte(content), 0644)
}

func (p *FSProxy) DeleteFile(path string) error {
	sc := p.getSecurityConfig()
	if !sc.AllowDelete {
		return fmt.Errorf("🔒 BLOQUEADO: Deleção de arquivos desativada.")
	}
	
	if !p.isPathAllowed(path) {
		return fmt.Errorf("🔒 BLOQUEADO: Acesso fora do diretório do projeto não permitido.")
	}
	
	return os.Remove(path)
}

func (p *FSProxy) MoveFile(oldPath, newPath string) error {
	sc := p.getSecurityConfig()
	if !sc.AllowMove {
		return fmt.Errorf("🔒 BLOQUEADO: Renomear/Mover arquivos desativado.")
	}
	
	if !p.isPathAllowed(oldPath) || !p.isPathAllowed(newPath) {
		return fmt.Errorf("🔒 BLOQUEADO: Acesso restrito ao diretório do projeto.")
	}
	
	return os.Rename(oldPath, newPath)
}

func (p *FSProxy) RunCommand(command string, args []string) (string, error) {
	sc := p.getSecurityConfig()
	if !sc.AllowRunCommands {
		return "", fmt.Errorf("🔒 BLOQUEADO: Execução de comandos desativada. Ative em Configurações > Segurança")
	}
	
	cmd := exec.Command(command, args...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}
