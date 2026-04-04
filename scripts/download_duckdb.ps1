# Script de Download Oficial DuckDB v1.1.3 (Estavel)
$url = "https://github.com/duckdb/duckdb/releases/download/v1.1.3/libduckdb-windows-amd64.zip"
$zipPath = "libduckdb.zip"
$destFolder = "deps\duckdb"

Write-Host "Baixando DuckDB v1.1.3 oficial..."
Invoke-WebRequest -Uri $url -OutFile $zipPath

if (!(Test-Path $destFolder)) {
    New-Item -ItemType Directory -Path $destFolder
}

Write-Host "Extraindo arquivos..."
Expand-Archive -Path $zipPath -DestinationPath $destFolder -Force

Write-Host "Atualizando DLL na raiz do projeto..."
Copy-Item "$destFolder\duckdb.dll" ".\duckdb.dll" -Force

Remove-Item $zipPath
Write-Host "DuckDB v1.1.3 instalado com sucesso!"
