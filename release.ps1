param(
    [string]$Tag = "v1.0.0",
    [switch]$SkipBuild,
    [switch]$NoPush
)

powershell -File .\scripts\release.ps1 -Tag $Tag -SkipBuild:$SkipBuild -NoPush:$NoPush
