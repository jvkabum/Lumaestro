package tools

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Installer gerencia a instalação de ferramentas externas com streaming de logs.
type Installer struct {
	LogChan chan string
}

// NewInstaller inicializa o instalador com um canal de feedback.
func NewInstaller() *Installer {
	return &Installer{
		LogChan: make(chan string, 100),
	}
}

// CheckStatus verifica se um comando está disponível localmente (node_modules/.bin) ou no PATH.
func (i *Installer) CheckStatus(name string) bool {
	// Primeiro, tenta achar o binário local do projeto (Windows .cmd)
	cwd, _ := os.Getwd()
	localBin := filepath.Join(cwd, "node_modules", ".bin", name+".cmd")
	if _, err := os.Stat(localBin); err == nil {
		return true
	}
	
	// Fallback para o PATH do sistema
	_, err := exec.LookPath(name)
	return err == nil
}

// CheckClaudeAuth verifica silenciosamente se já existe uma sessão de login ativa do Claude (OAuth) no sistema.
func (i *Installer) CheckClaudeAuth() bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	configPath := filepath.Join(home, ".claude.json")
	if _, err := os.Stat(configPath); err == nil {
		return true
	}
	return false
}

// CheckGeminiAuth verifica silenciosamente se existe uma sessão configurada do Gemini no sistema.
func (i *Installer) CheckGeminiAuth() bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	// O Gemini CLI pode usar ~/.gemini/settings.json
	configPath := filepath.Join(home, ".gemini", "settings.json")
	if _, err := os.Stat(configPath); err == nil {
		return true
	}
	return false
}

// runStreaming abre um processo e envia a saída linha por linha para o canal.
func (i *Installer) runStreaming(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		return err
	}

	// Scanner para capturar a saída
	multi := io.MultiReader(stdout, stderr)
	scanner := bufio.NewScanner(multi)

	go func() {
		for scanner.Scan() {
			i.LogChan <- scanner.Text()
		}
	}()

	return cmd.Wait()
}

// InstallGemini CLI via NPM Local (Tudo pelo Lumaestro).
func (i *Installer) InstallGemini() error {
	i.LogChan <- "📦 Instalando Gemini CLI localmente no Lumaestro..."
	if runtime.GOOS == "windows" {
		return i.runStreaming("cmd", "/C", "npm install @google/gemini-cli@latest --force")
	}
	return i.runStreaming("npm", "install", "@google/gemini-cli@latest", "--force")
}

// InstallClaude CLI via NPM Local (Tudo pelo Lumaestro).
func (i *Installer) InstallClaude() error {
	i.LogChan <- "📦 Instalando Claude Code localmente no Lumaestro..."
	if runtime.GOOS == "windows" {
		return i.runStreaming("cmd", "/C", "npm install @anthropic-ai/claude-code@latest --force")
	}
	return i.runStreaming("npm", "install", "@anthropic-ai/claude-code@latest", "--force")
}

// SyncPath injeta caminhos comuns (Claude e NPM) no PATH do processo atual.
// Isso garante que o app encontre as ferramentas mesmo que o PATH global esteja desatualizado.
func (i *Installer) SyncPath() {
	home, _ := os.UserHomeDir()
	appData := os.Getenv("APPDATA")
	
	// Caminhos prováveis
	paths := []string{
		filepath.Join(home, ".local", "bin"),       // Claude Code
		filepath.Join(appData, "npm"),              // Gemini CLI (NPM Global)
		filepath.Join(home, "AppData", "Roaming", "npm"), // Fallback NPM
	}

	currentPath := os.Getenv("PATH")
	newPaths := []string{}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			if !strings.Contains(currentPath, p) {
				newPaths = append(newPaths, p)
			}
		}
	}

	if len(newPaths) > 0 {
		sep := string(os.PathListSeparator)
		os.Setenv("PATH", currentPath+sep+strings.Join(newPaths, sep))
		i.LogChan <- fmt.Sprintf("✅ Ambiente Sincronizado: %d novos caminhos injetados.", len(newPaths))
	}
}

// FixClaudePath injeta o caminho do Claude .local/bin no PATH do usuário Windows e limpa o ambiente.
func (i *Installer) FixClaudePath() error {
	home, _ := os.UserHomeDir()
	binPath := filepath.Join(home, ".local", "bin")

	// Script PowerShell Robusto: Define, persiste e envia broadcast para o sistema
	script := fmt.Sprintf(`
		$p = '%s';
		$v = [System.Environment]::GetEnvironmentVariable('PATH', 'User');
		if ($v -notlike '*'+$p+'*') {
			$newPath = $v.TrimEnd(';') + ';' + $p;
			[System.Environment]::SetEnvironmentVariable('PATH', $newPath, 'User');
			
			# Broadcast de mudança para o Windows (ajuda novos processos a verem sem logoff)
			$signature = '[DllImport("user32.dll", SetLastError = true, CharSet = CharSet.Auto)]
			public static extern IntPtr SendMessageTimeout(IntPtr hWnd, uint Msg, IntPtr wParam, string lParam, uint fuFlags, uint uTimeout, out IntPtr lpdwResult);';
			$type = Add-Type -MemberDefinition $signature -Name "Win32" -Namespace "Env" -PassThru;
			$result = [IntPtr]::Zero;
			$type::SendMessageTimeout(0xFFFF, 0x001A, [IntPtr]::Zero, "Environment", 0x0002, 1000, [out]$result);
			
			Write-Host 'FIXED';
		} else {
			Write-Host 'EXISTS';
		}
	`, binPath)

	cmd := exec.Command("powershell", "-Command", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return err
	}

	result := strings.TrimSpace(string(out))
	if result == "FIXED" || result == "EXISTS" {
		i.SyncPath() // Aplica imediatamente no processo atual
		if result == "FIXED" {
			i.LogChan <- "🔧 Herança Maestro: O caminho " + binPath + " agora é parte definitiva da sua alma digital."
		}
	}
	
	return nil
}

// InstallObsidian via Powershell.
func (i *Installer) InstallObsidian() error {
	i.LogChan <- "Baixando e instalando Obsidian..."
	script := "(New-Object Net.WebClient).DownloadFile('https://obsidian.md/download/latest/Obsidian.exe', 'ObsidianInstaller.exe'); Start-Process 'ObsidianInstaller.exe' -Wait"
	return i.runStreaming("powershell", "-Command", script)
}

// SetupTool abre um terminal externo para configurar a CLI (fluxo de login interativo)
func (i *Installer) SetupTool(name string) error {
	// Resolve o caminho do binário local para o terminal externo
	cwd, _ := os.Getwd()
	binaryPath := filepath.Join(cwd, "node_modules", ".bin", name+".cmd")
	
	// Se o binário local não existir, tenta o global como fallback
	if _, err := os.Stat(binaryPath); err != nil {
		binaryPath = name 
	}

	finalCmd := fmt.Sprintf("& '%s'", binaryPath)
	if name == "claude" {
		finalCmd = fmt.Sprintf("& '%s' auth login", binaryPath)
	} else if name == "gemini" {
		// Força o modo sem navegador para evitar erros de permissão no OAuth
		finalCmd = fmt.Sprintf("$env:NO_BROWSER='true'; & '%s'", binaryPath)
	}

	// Abre uma nova janela do PowerShell no Windows com o comando correto
	return exec.Command("cmd", "/c", "start", "powershell", "-NoExit", "-Command", finalCmd).Run()
}
