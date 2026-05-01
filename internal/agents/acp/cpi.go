package acp

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ============================================================
// 🛡️ CPI — Consciousness Protocol Isolation
// Módulo de isolamento de consciência para os agentes ACP.
// Garante que TODOS os acessos a arquivos, diretórios e comandos
// estejam restritos à ÓRBITA_ATIVA (Workspace do projeto).
// ============================================================

// CPIValidator é o validador central de caminhos e comandos do CPI.
type CPIValidator struct {
	// ActiveOrbit é o caminho absoluto normalizado da ÓRBITA_ATIVA (Workspace).
	ActiveOrbit string
	// VaultOrbit é o caminho do Obsidian Vault, permitido para o Crawler mesmo se ActiveOrbit estiver vazio.
	VaultOrbit  string
}

// NewCPIValidator cria um validador CPI com as órbitas autorizadas definidas.
func NewCPIValidator(workspace string, vaultPath string) *CPIValidator {
	orbit := strings.TrimSpace(workspace)
	vault := strings.TrimSpace(vaultPath)
	
	v := &CPIValidator{}

	// Normaliza Workspace
	if orbit != "" && orbit != "." {
		if absOrbit, err := filepath.Abs(orbit); err == nil {
			v.ActiveOrbit = filepath.Clean(absOrbit)
		}
	}

	// Normaliza Vault
	if vault != "" && vault != "." {
		if absVault, err := filepath.Abs(vault); err == nil {
			v.VaultOrbit = filepath.Clean(absVault)
		}
	}

	return v
}

// IsArmed retorna true se o CPI tem pelo menos uma zona de confiança definida.
func (c *CPIValidator) IsArmed() bool {
	return (c.ActiveOrbit != "" && c.ActiveOrbit != ".") || (c.VaultOrbit != "" && c.VaultOrbit != ".")
}

// ValidatePath verifica se um caminho está dentro de uma das órbitas autorizadas.
func (c *CPIValidator) ValidatePath(path string) (string, error) {
	// 🛡️ MODO FAIL-CLOSED: Se NENHUMA órbita estiver definida, bloqueia TUDO.
	if !c.IsArmed() {
		fmt.Println("🛡️ [CPI] Sistema desarmado — bloqueio preventivo de IO físico.")
		return "", fmt.Errorf("🛡️ CPI DESARMADO: Nenhuma órbita (Workspace ou Vault) definida. Operação bloqueada")
	}

	// Impedir navegação para home
	if strings.Contains(path, "~") {
		return "", fmt.Errorf("🛡️ CPI VIOLAÇÃO: Referência ao home directory (~) bloqueada. Use caminhos relativos ao projeto")
	}

	// 🔒 REGRA 2: Rejeitar path traversal explícito
	if strings.Contains(path, "..") {
		fmt.Printf("🛡️ [CPI ALERTA] Tentativa de Path Traversal bloqueada: %s\n", path)
		return "", fmt.Errorf("🛡️ CPI VIOLAÇÃO: Path traversal detectado (%s). Operação bloqueada", path)
	}

	// 🔒 REGRA 2: Rejeitar home shortcuts
	if strings.HasPrefix(path, "~") {
		fmt.Printf("🛡️ [CPI ALERTA] Tentativa de acesso ao Home (~) bloqueada: %s\n", path)
		return "", fmt.Errorf("🛡️ CPI VIOLAÇÃO: Referência ao home directory (~) bloqueada. Use caminhos relativos ao projeto")
	}

	// Resolver caminho absoluto
	absPath := path
	if !filepath.IsAbs(path) {
		absPath = filepath.Join(c.ActiveOrbit, path)
	}

	// 🔍 HARDCORE: Resolução de Symlinks
	// Avalia o caminho real para impedir bypass de links simbólicos para fora da órbita.
	realPath, err := filepath.EvalSymlinks(absPath)
	if err == nil {
		absPath = realPath
	}
	
	absPath = filepath.Clean(absPath)

	// 🔒 REGRA 1 (WHITELIST ABSOLUTA): O caminho DEVE estar sob uma das órbitas autorizadas (Workspace ou Vault)
	pathNorm := strings.ToLower(absPath)
	orbitNorm := strings.ToLower(c.ActiveOrbit)
	vaultNorm := strings.ToLower(c.VaultOrbit)

	inWorkspace := orbitNorm != "" && strings.HasPrefix(pathNorm, orbitNorm)
	inVault := vaultNorm != "" && strings.HasPrefix(pathNorm, vaultNorm)

	if !inWorkspace && !inVault {
		fmt.Printf("🛡️ [CPI BLOQUEIO] Caminho fora de órbita: %s\n", absPath)
		return "", fmt.Errorf("🛡️ CPI VIOLAÇÃO: Acesso fora das jurisdições autorizadas (Workspace/Vault). Alvo: %s", absPath)
	}

	// 🔒 REGRA 7: Proibir diretórios do sistema (redundância de segurança)
	forbidden := []string{
		`c:\windows`, `c:\users`, `c:\program files`, `c:\programdata`,
		`/etc`, `/usr`, `/var`, `/sys`, `/proc`, `/boot`, `/root`,
	}
	for _, f := range forbidden {
		if strings.HasPrefix(pathNorm, f) {
			return "", fmt.Errorf("🛡️ CPI VIOLAÇÃO CRÍTICA: Acesso a diretório protegido do sistema bloqueado (%s)", path)
		}
	}

	return absPath, nil
}

// ValidateCommand verifica se um comando é seguro para execução dentro da ÓRBITA_ATIVA.
// Retorna erro se o comando tentar escapar do sandbox.
func (c *CPIValidator) ValidateCommand(command string, args []string) error {
	if !c.IsArmed() {
		return fmt.Errorf("🛡️ CPI DESARMADO: Execução de comandos bloqueada sem órbita ativa")
	}

	fullCmd := strings.ToLower(strings.TrimSpace(command))
	fullArgs := strings.ToLower(strings.Join(args, " "))
	combined := fullCmd + " " + fullArgs

	// 🔒 REGRA 7: Comandos globais proibidos (HARD STOP)
	blockedCommands := []string{
		"format", "diskpart", "shutdown", "reboot", "poweroff",
		"reg delete", "reg add", "sfc", "dism", "bcdedit",
		"net user", "net localgroup", "wmic",
	}
	for _, bc := range blockedCommands {
		if strings.Contains(combined, bc) {
			return fmt.Errorf("🛡️ CPI BLOQUEIO: Comando do sistema (%s) proibido pelo protocolo de isolamento", bc)
		}
	}

	// 🔒 REGRA 3: Bloquear encadeamento e injeção (Command Injection)
	forbiddenChars := []string{"&&", ";", "|", ">", "<", "`", "$(", "||"}
	for _, char := range forbiddenChars {
		if strings.Contains(combined, char) {
			return fmt.Errorf("🛡️ CPI BLOQUEIO: Caractere de encadeamento/redirecionamento proibido detectado (%s)", char)
		}
	}

	// 🔒 REGRA 4: Bloquear Flags Perigosas (Eval/Escape)
	forbiddenFlags := []string{"-e", "--eval", "/c", "-command", "-c", "--code"}
	for i, arg := range args {
		lowerArg := strings.ToLower(arg)
		for _, flag := range forbiddenFlags {
			if lowerArg == flag || strings.HasPrefix(lowerArg, flag+"=") {
				// 🛡️ EXCEÇÃO HERMÉTICA: cmd /c dir [path]
				if strings.ToLower(command) == "cmd" && lowerArg == "/c" && len(args) == 2 && strings.ToLower(args[i+1]) == "dir" {
					continue 
				}
				// 🛡️ EXCEÇÃO HERMÉTICA: cmd /c dir [path] (versão 3 args)
				if strings.ToLower(command) == "cmd" && lowerArg == "/c" && len(args) == 3 && strings.ToLower(args[i+1]) == "dir" {
					continue
				}
				
				return fmt.Errorf("🛡️ CPI BLOQUEIO: Flag de execução perigosa detectada (%s)", arg)
			}
		}
	}

	// 🔒 REGRA 5: Bloquear Execução Indireta em Argumentos (Sniper Semântico)
	indirectExecPatterns := []string{"child_process", "spawn", "exec", "powershell", "bash", "sh", "eval(", "process.env"}
	for _, arg := range args {
		lowerArg := strings.ToLower(arg)
		for _, pattern := range indirectExecPatterns {
			if strings.Contains(lowerArg, pattern) {
				return fmt.Errorf("🛡️ CPI BLOQUEIO: Padrão de execução indireta detectado no argumento (%s)", pattern)
			}
		}
	}

	// 🔒 REGRA 6: Restrição de Scripts (npm/node)
	if strings.ToLower(command) == "npm" {
		isInstall := false
		for _, arg := range args {
			if strings.ToLower(arg) == "install" || strings.ToLower(arg) == "i" {
				isInstall = true
				break
			}
		}
		if !isInstall {
			return fmt.Errorf("🛡️ CPI BLOQUEIO: 'npm' permitido apenas para instalação. Scripts (run/exec) bloqueados")
		}
	}

	// 🔒 REGRA 7: Validar o próprio binário do comando
	if filepath.IsAbs(command) {
		_, err := c.ValidatePath(command)
		if err != nil {
			return fmt.Errorf("🛡️ CPI BLOQUEIO: Binário do comando fora da área permitida")
		}
	}

	// 🔒 REGRA 2: Detectar caminhos e escapes nos argumentos (Deep Inspection)
	forbiddenPrefixes := []string{"c:", "d:", "e:", "\\\\", "/etc", "/usr", "/var", "/root", "/sys", "/proc"}
	
	for _, arg := range args {
		cleanArg := strings.ToLower(strings.TrimSpace(arg))
		if cleanArg == "" {
			continue
		}

		// Detectar qualquer menção a diretórios do sistema em qualquer lugar do argumento
		for _, fp := range forbiddenPrefixes {
			if strings.Contains(cleanArg, fp) {
				// Se contém um prefixo proibido, validamos se ele está dentro da órbita
				_, err := c.ValidatePath(arg)
				if err != nil {
					return fmt.Errorf("🛡️ CPI BLOQUEIO: Argumento suspeito de escape detectado (%s)", arg)
				}
			}
		}

		// Detectar path traversal em argumentos
		if strings.Contains(cleanArg, "..") || strings.Contains(cleanArg, "~") {
			return fmt.Errorf("🛡️ CPI VIOLAÇÃO: Tentativa de navegação/traversal detectada (%s)", cleanArg)
		}
	}

	return nil
}

// GetOrbitCWD retorna o diretório de trabalho que deve ser usado para execução de comandos.
// Sempre retorna a ÓRBITA_ATIVA para garantir isolamento.
func (c *CPIValidator) GetOrbitCWD() string {
	if c.IsArmed() {
		return c.ActiveOrbit
	}
	cwd, _ := os.Getwd()
	return cwd
}

// String retorna uma representação textual do estado do CPI.
func (c *CPIValidator) String() string {
	if c.IsArmed() {
		return fmt.Sprintf("🛡️ CPI [ATIVA] Órbita: %s", c.ActiveOrbit)
	}
	return "🛡️ CPI [DESARMADA] — Todas as operações de arquivo bloqueadas"
}
