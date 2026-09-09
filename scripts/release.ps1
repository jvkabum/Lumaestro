param(
    [string]$Tag = "v1.0.0",
    [switch]$SkipBuild,
    [switch]$NoPush
)

$ErrorActionPreference = "Stop"

Write-Host "==========================================================" -ForegroundColor Cyan
Write-Host " 🚀 LUMAESTRO RELEASE PIPELINE - Tag: $Tag" -ForegroundColor Yellow
Write-Host "==========================================================" -ForegroundColor Cyan

# 1. Compilação de Produção
if (-not $SkipBuild) {
    Write-Host "[1/5] 🔨 Compilando binário de produção com Wails & DuckDB..." -ForegroundColor Cyan
    powershell -File .\scripts\build.ps1
    if ($LASTEXITCODE -ne 0) {
        Write-Error "Falha na compilação de produção!"
        exit 1
    }
} else {
    Write-Host "[1/5] ⏩ Build ignorado conforme flag -SkipBuild" -ForegroundColor DarkGray
}

$binDir = "build\bin"
$exePath = "$binDir\Lumaestro.exe"
$dllPath = "$binDir\duckdb.dll"

if (-not (Test-Path $exePath)) {
    Write-Error "Executável $exePath não encontrado!"
    exit 1
}
if (-not (Test-Path $dllPath)) {
    Write-Error "Biblioteca $dllPath não encontrada!"
    exit 1
}

# 2. Empacotamento
Write-Host "[2/5] 📦 Preparando pacote de distribuição (.zip)..." -ForegroundColor Cyan
$stagingDir = "build\release_staging"
if (Test-Path $stagingDir) { Remove-Item $stagingDir -Recurse -Force }
New-Item -ItemType Directory -Path $stagingDir -Force | Out-Null

Copy-Item $exePath -Destination $stagingDir -Force
Copy-Item $dllPath -Destination $stagingDir -Force
if (Test-Path "README.md") { Copy-Item "README.md" -Destination $stagingDir -Force }
if (Test-Path "LICENSE.md") { Copy-Item "LICENSE.md" -Destination $stagingDir -Force }

$zipName = "Lumaestro-$Tag-windows-amd64.zip"
$zipPath = "$binDir\$zipName"
if (Test-Path $zipPath) { Remove-Item $zipPath -Force }

Compress-Archive -Path "$stagingDir\*" -DestinationPath $zipPath -Force
Remove-Item $stagingDir -Recurse -Force

$zipSizeMb = [math]::Round(((Get-Item $zipPath).Length / 1MB), 2)
Write-Host "✅ Pacote gerado com sucesso: $zipPath ($zipSizeMb MB)" -ForegroundColor Green

# 3. Git Tagging
Write-Host "[3/5] 🏷️ Configurando Git Tag: $Tag..." -ForegroundColor Cyan
$existingTag = git tag -l $Tag
if ($existingTag) {
    Write-Host "Aviso: Tag '$Tag' já existe localmente." -ForegroundColor Yellow
} else {
    git tag -a $Tag -m "Release $Tag: The Cosmic Engine & Antigravity Orchestrator"
    Write-Host "✅ Tag '$Tag' criada no repositório local." -ForegroundColor Green
}

# 4. Envio para o GitHub Remoto
if (-not $NoPush) {
    Write-Host "[4/5] 🌐 Enviando Tag para o GitHub (origin)..." -ForegroundColor Cyan
    git push origin $Tag
    Write-Host "✅ Tag enviada com sucesso para origin!" -ForegroundColor Green
} else {
    Write-Host "[4/5] ⏩ Envio da tag ignorado conforme flag -NoPush" -ForegroundColor DarkGray
}

# 5. Conclusão e Link para Publicação
$releaseUrl = "https://github.com/jvkabum/Lumaestro/releases/new?tag=$Tag"
Write-Host "==========================================================" -ForegroundColor Cyan
Write-Host " 🎉 RELEASE PRONTA PARA PUBLICAÇÃO NO GITHUB!" -ForegroundColor Green
Write-Host "==========================================================" -ForegroundColor Cyan
Write-Host "📁 Arquivo ZIP para anexar na Release:" -ForegroundColor White
Write-Host "   $((Get-Item $zipPath).FullName)" -ForegroundColor Yellow
Write-Host ""
Write-Host "🌐 Link para criar a Release no GitHub:" -ForegroundColor White
Write-Host "   $releaseUrl" -ForegroundColor Cyan
Write-Host ""
Write-Host "As notas completas estão salvas em: docs\releases\$Tag.md" -ForegroundColor White
Write-Host "==========================================================" -ForegroundColor Cyan
