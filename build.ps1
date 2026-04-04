# 🚀 Script de Build Lumaestro (Produção)
# Compila o app com as tags de correção e empacota as DLLs necessárias.

Write-Host "🏗️ Iniciando Compilação de Produção do Lumaestro..." -ForegroundColor Cyan

# 1. Executa o build do Wails com as tags de estabilidade do DuckDB
wails build -tags "duckdb_use_lib,no_duckdb_arrow"

if ($LASTEXITCODE -ne 0) {
    Write-Host "❌ Erro na compilação!" -ForegroundColor Red
    exit $LASTEXITCODE
}

# 2. Define os caminhos
$dllSource = "deps\duckdb\duckdb.dll"
$binFolder = "build\bin"
$exePath = "$binFolder\Lumaestro.exe"

# 3. Copia a DLL do DuckDB para a pasta do binário (Essencial para rodar fora do Dev)
if (Test-Path $dllSource) {
    Write-Host "📦 Empacotando DuckDB DLL em $binFolder..." -ForegroundColor Green
    Copy-Item $dllSource -Destination $binFolder -Force
} else {
    Write-Host "⚠️ Aviso: duckdb.dll não encontrada em $dllSource. O executável pode falhar." -ForegroundColor Yellow
}

Write-Host "`n✅ Build concluído com sucesso!" -ForegroundColor Green
Write-Host "📍 Seu executável está pronto em: $exePath" -ForegroundColor Cyan
Write-Host "💡 DICA: Para rodar em outro PC, leve a pasta '$binFolder' inteira." -ForegroundColor Gray
