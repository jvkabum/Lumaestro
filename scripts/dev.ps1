# Lumaestro Development Swarm Orchestrator
Write-Host "ðŸ§  Iniciando Motor Cognitivo Lumaestro..." -ForegroundColor Cyan

# 1. Preparar Ambiente (Injetar DLLs nativas no PATH)
$env:PATH += ";$PSScriptRoot\..\deps\duckdb"

# Garante que o Go esteja disponivel no PATH da sessao atual.
if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    $goDir = "C:\Program Files\Go\bin"
    if (Test-Path (Join-Path $goDir "go.exe")) {
        $env:Path = "$goDir;$env:Path"
    }
}

# 2. Sincronizacao
go mod tidy

# 3. Resolve o binario do Wails mesmo quando nao estiver no PATH.
$wailsExe = $null
$wailsCmd = Get-Command wails -ErrorAction SilentlyContinue
if ($wailsCmd) {
    $wailsExe = $wailsCmd.Source
} else {
    $goExe = "C:\Program Files\Go\bin\go.exe"
    if (Test-Path $goExe) {
        $gopath = & $goExe env GOPATH
        if ($LASTEXITCODE -eq 0 -and $gopath) {
            $candidate = Join-Path $gopath "bin\wails.exe"
            if (Test-Path $candidate) {
                $wailsExe = $candidate
            }
        }
    }
}

if (-not $wailsExe) {
    Write-Host "X Wails CLI nao encontrada. Instale com: go install github.com/wailsapp/wails/v2/cmd/wails@latest" -ForegroundColor Red
    exit 1
}

# 4. Execucao com as tags de estabilidade do DuckDB
& $wailsExe dev -debounce 500 -v 2 -tags "duckdb_use_lib,no_duckdb_arrow"
