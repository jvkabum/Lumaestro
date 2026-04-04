# 🚀 Script de Desenvolvimento Lumaestro
# Use apenas ".\dev" no seu terminal para iniciar.

Write-Host "🧠 Iniciando Motor Cognitivo Lumaestro..." -ForegroundColor Cyan

# Garante a execuÃ§Ã£o com as tags de estabilidade do DuckDB
wails dev -tags "duckdb_use_lib,no_duckdb_arrow"
