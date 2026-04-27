# Script de Build Lumaestro (Producao)
# Compila o app com as tags de correcao e empacota as DLLs necessarias.

Write-Host "Iniciando Compilacao de Producao do Lumaestro..." -ForegroundColor Cyan
$env:PATH += ";$PSScriptRoot\..\deps\duckdb"

# 1. Executa o build do Wails com as tags de estabilidade do DuckDB
wails build -tags "duckdb_use_lib,no_duckdb_arrow"

if ($LASTEXITCODE -ne 0) {
    Write-Host "Erro na compilacao!" -ForegroundColor Red
$env:PATH += ";$PSScriptRoot\..\deps\duckdb"
    exit $LASTEXITCODE
}

# 2. Define os caminhos
$dllSource = "deps\duckdb\duckdb.dll"
$binFolder = "build\bin"
$exePath = "$binFolder\Lumaestro.exe"

# 3. Copia a DLL do DuckDB para a pasta do binario
if (Test-Path $dllSource) {
    Write-Host "Empacotando DuckDB DLL em $binFolder..." -ForegroundColor Green
$env:PATH += ";$PSScriptRoot\..\deps\duckdb"
    Copy-Item $dllSource -Destination $binFolder -Force
} else {
    Write-Host "Aviso: duckdb.dll nao encontrada em $dllSource." -ForegroundColor Yellow
$env:PATH += ";$PSScriptRoot\..\deps\duckdb"
}

Write-Host "Build concluido com sucesso!" -ForegroundColor Green
$env:PATH += ";$PSScriptRoot\..\deps\duckdb"
Write-Host "Seu executavel esta pronto em: $exePath" -ForegroundColor Cyan
$env:PATH += ";$PSScriptRoot\..\deps\duckdb"
