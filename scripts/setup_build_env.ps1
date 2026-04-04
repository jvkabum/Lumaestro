# 🚀 Setup do Ambiente de Build Lumaestro com DuckDB Dinâmico
# Este script configura as variáveis de ambiente necessárias para o compilador C (GCC)
# localizar os cabeçalhos e a biblioteca do DuckDB durante o build do Wails (CGO).

$absPath = Get-Location
$depsPath = Join-Path $absPath "deps\duckdb"

Write-Host "Configurando CGO para DuckDB em: $depsPath" -ForegroundColor Cyan

# 🛠️ 1. Habilitar CGO (Necessário para go-duckdb)
$env:CGO_ENABLED = "1"

# 🔍 2. Caminho para os Headers (duckdb.h)
$env:CGO_CFLAGS = "-I$depsPath"

# 🔗 3. Caminho para a Biblioteca de Linkagem (duckdb.lib)
$env:CGO_LDFLAGS = "-L$depsPath -lduckdb"

Write-Host "Variáveis de ambiente CGO configuradas com sucesso!" -ForegroundColor Green
Write-Host "Agora você pode rodar: wails dev -tags duckdb_use_lib" -ForegroundColor Yellow
