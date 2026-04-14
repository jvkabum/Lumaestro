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
	"time"
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

// CheckStatus verifica se um comando está disponível no PATH do sistema.
func (i *Installer) CheckStatus(name string) bool {
	// No Windows, tenta com .exe se não houver extensão
	searchName := name
	if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(name), ".exe") {
		searchName = name + ".exe"
	}

	// Prioridade total para o PATH do sistema
	_, err := exec.LookPath(searchName)
	if err == nil {
		return true
	}

	// Fallback apenas para manter retrocompatibilidade com instalações locais antigas
	cwd, _ := os.Getwd()
	localBin := filepath.Join(cwd, "node_modules", ".bin", name+".cmd")
	if _, err := os.Stat(localBin); err == nil {
		return true
	}
	
	return false
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
	// A biblioteca do Gemini CLI utiliza Application Default Credentials (ADC)
	// Verifica o ADC do Google Cloud no Windows
	appData := os.Getenv("APPDATA")
	if appData != "" {
		adcPath := filepath.Join(appData, "gcloud", "application_default_credentials.json")
		if _, err := os.Stat(adcPath); err == nil {
			return true
		}
	}

	// Verifica o ADC no Unix/Linux/macOS
	home, err := os.UserHomeDir()
	if err == nil {
		// Padrão GCloud ADC
		adcPathUnix := filepath.Join(home, ".config", "gcloud", "application_default_credentials.json")
		if _, err := os.Stat(adcPathUnix); err == nil {
			return true
		}

		// Padrão Nativo Gemini CLI (oauth_creds.json) - Muito comum no Windows/NPM
		geminiPath := filepath.Join(home, ".gemini", "oauth_creds.json")
		if _, err := os.Stat(geminiPath); err == nil {
			return true
		}
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

// InstallGemini CLI via NPM Global.
func (i *Installer) InstallGemini() error {
	i.LogChan <- "📦 Instalando Gemini CLI globalmente no sistema..."
	if runtime.GOOS == "windows" {
		return i.runStreaming("cmd", "/C", "npm install -g @google/gemini-cli@latest --force")
	}
	return i.runStreaming("npm", "install", "-g", "@google/gemini-cli@latest", "--force")
}

// InstallClaude CLI via NPM Global.
func (i *Installer) InstallClaude() error {
	i.LogChan <- "📦 Instalando Claude Code globalmente no sistema..."
	if runtime.GOOS == "windows" {
		return i.runStreaming("cmd", "/C", "npm install -g @anthropic-ai/claude-code@latest --force")
	}
	return i.runStreaming("npm", "install", "-g", "@anthropic-ai/claude-code@latest", "--force")
}

// InstallLlamaCPP instala o motor de inferência local (llama-server).
func (i *Installer) InstallLlamaCPP() error {
	i.LogChan <- "📦 Instalando motor local (llama.cpp) para RAG nativo..."
	if runtime.GOOS == "windows" {
		i.LogChan <- "⏳ Executando winget install llama.cpp... (Aceite os termos se solicitado)"
		return i.runStreaming("powershell", "-Command", "winget install llama.cpp --accept-source-agreements --accept-package-agreements")
	} else if runtime.GOOS == "darwin" {
		i.LogChan <- "⏳ Executando brew install llama.cpp..."
		return i.runStreaming("brew", "install", "llama.cpp")
	}
	return fmt.Errorf("instalação automática do llama.cpp não suportada para %s. Instale manualmente.", runtime.GOOS)
}

// SyncPath injeta caminhos comuns (Claude e NPM) no PATH do processo atual.
// Isso garante que o app encontre as ferramentas mesmo que o PATH global esteja desatualizado.
func (i *Installer) SyncPath() {
	home, _ := os.UserHomeDir()
	appData := os.Getenv("APPDATA")
	
	// Caminhos prováveis
	paths := []string{
		filepath.Join(home, ".local", "bin"),         // Claude Code
		filepath.Join(appData, "npm"),                // Gemini CLI (NPM Global)
		filepath.Join(home, "AppData", "Roaming", "npm"),   // Fallback NPM
		`C:\Program Files\llama.cpp`,                // Winget padrão (Admin)
	}

	// 🔍 Busca dinâmica pelo diretório do WinGet (Portátil)
	winGetDir := filepath.Join(home, "AppData", "Local", "Microsoft", "WinGet", "Packages")
	if entries, err := os.ReadDir(winGetDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() && strings.Contains(strings.ToLower(entry.Name()), "llamacpp") {
				paths = append(paths, filepath.Join(winGetDir, entry.Name()))
			}
		}
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

// KillOrphans encerra qualquer instância pendente de serviços (por porta e nome) para evitar conflitos.
func (i *Installer) KillOrphans() {
	if runtime.GOOS == "windows" {
		// Obtém o PID atual para evitar que o Maestro se encerre sozinho!
		currentPid := os.Getpid()
		fmt.Printf("[Installer] 🧹 Limpeza Profunda (PID Local %d): Encerrando instâncias zumbis nas portas 8001, 8085, 8086...\n", currentPid)
		
		// Script PowerShell robusto que ignora o processo atual
		script := fmt.Sprintf(`
			$currentPid = %d;
			$ports = @(8001, 8085, 8086, 8087);
			foreach ($p in $ports) {
				$conns = Get-NetTCPConnection -LocalPort $p -ErrorAction SilentlyContinue | Where-Object { $_.OwningProcess -ne $currentPid };
				if ($conns) {
					$conns | ForEach-Object { Stop-Process -Id $_.OwningProcess -Force -ErrorAction SilentlyContinue };
				}
			}
			Get-Process -Name "llama-server" -ErrorAction SilentlyContinue | Where-Object { $_.Id -ne $currentPid } | Stop-Process -Force -ErrorAction SilentlyContinue;
		`, currentPid)
		exec.Command("powershell", "-Command", script).Run()
	} else {
		exec.Command("pkill", "-9", "llama-server").Run()
	}
	// Pausa tática para o SO liberar os sockets e arquivos
	time.Sleep(2 * time.Second)
}

// InstallObsidian via Powershell.
func (i *Installer) InstallObsidian() error {
	i.LogChan <- "Baixando e instalando Obsidian..."
	script := "(New-Object Net.WebClient).DownloadFile('https://obsidian.md/download/latest/Obsidian.exe', 'ObsidianInstaller.exe'); Start-Process 'ObsidianInstaller.exe' -Wait"
	return i.runStreaming("powershell", "-Command", script)
}

// GetSetupCommand retorna o binário e os argumentos necessários para configurar a IA interativamente.
func (i *Installer) GetSetupCommand(name string) (string, []string) {
	binaryPath := name
	if _, err := exec.LookPath(name); err != nil {
		cwd, _ := os.Getwd()
		localPath := filepath.Join(cwd, "node_modules", ".bin", name+".cmd")
		if _, errS := os.Stat(localPath); errS == nil {
			binaryPath = localPath
		}
	}

	var args []string
	if name == "claude" {
		args = []string{"auth", "login"}
	} else if name == "gemini" {
		// No Gemini v0.37.0+, o comando 'login' é inválido. 
		// Rodar o binário puro inicia o REPL e oferece as opções de autenticação (OAuth vs API Key).
		args = []string{}
	}

	return binaryPath, args
}

// SetupTool abre um terminal externo para configurar a CLI (fluxo de login interativo) - Legado/Fallback.
func (i *Installer) SetupTool(name string) error {
	binaryPath, args := i.GetSetupCommand(name)
	
	finalCmd := fmt.Sprintf("& '%s' %s", binaryPath, strings.Join(args, " "))
	if name == "gemini" {
		finalCmd = fmt.Sprintf("$env:NO_BROWSER='true'; & '%s' login", binaryPath)
	}

	// Abre uma nova janela do PowerShell no Windows com o comando correto
	return exec.Command("cmd", "/c", "start", "powershell", "-NoExit", "-Command", finalCmd).Run()
}
