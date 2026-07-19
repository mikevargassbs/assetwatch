# Runs the backend and, if the embedded frontend is stale or missing,
# rebuilds it first - so scripts\dev.ps1 is the one command that gets you
# both frontend and backend from ./cmd/api.
$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
$web = Join-Path $root "web"
$dist = Join-Path $web "dist"

function Get-NewestMTime($paths) {
    $files = Get-ChildItem -Path $paths -Recurse -File -ErrorAction SilentlyContinue
    if (-not $files) { return $null }
    return ($files | Measure-Object -Property LastWriteTimeUtc -Maximum).Maximum
}

$distAssets = Join-Path $dist "assets"
$distBuilt = Get-NewestMTime @($distAssets)

$sourcePaths = @(
    (Join-Path $web "src"),
    (Join-Path $web "index.html"),
    (Join-Path $web "package.json"),
    (Join-Path $web "vite.config.ts")
)
$sourceChanged = Get-NewestMTime $sourcePaths

$needsBuild = (-not $distBuilt) -or ($sourceChanged -and $sourceChanged -gt $distBuilt)

if ($needsBuild) {
    Write-Host "Frontend build is missing or stale - running npm run build..."
    if (-not (Test-Path (Join-Path $web "node_modules"))) {
        & npm --prefix $web install
    }
    & npm --prefix $web run build
} else {
    Write-Host "Frontend build is up to date, skipping npm build."
}

go run ./cmd/api

