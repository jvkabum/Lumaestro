# 🚀 Script de Desenvolvimento Lumaestro
# Use apenas ".\dev" no seu terminal para iniciar.

Write-Host "🧠 Iniciando Motor Cognitivo Lumaestro..." -ForegroundColor Cyan

# Garante que o Go esteja disponivel no PATH da sessao atual.
if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
	$goDir = "C:\Program Files\Go\bin"
	if (Test-Path (Join-Path $goDir "go.exe")) {
		$env:Path = "$goDir;$env:Path"
	}
}

# Resolve o binario do Wails mesmo quando nao estiver no PATH.
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
	Write-Host "❌ Wails CLI nao encontrada. Instale com:" -ForegroundColor Red
	Write-Host "   C:\Program Files\Go\bin\go.exe install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0" -ForegroundColor Yellow
	exit 1
}

# Garante a execucao com as tags de estabilidade do DuckDB
& $wailsExe dev -tags "duckdb_use_lib,no_duckdb_arrow"
