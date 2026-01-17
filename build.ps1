# Build script for qobuz-dl-go
# Usage: .\build.ps1 [-Release] [-All]

param(
    [switch]$Release,  # Build release version with version number
    [switch]$All,      # Build for all platforms
    [switch]$Help
)

if ($Help) {
    Write-Host @"
qobuz-dl-go Build Script
========================
Usage: .\build.ps1 [options]

Options:
  -Release    Build release version (uses VERSION file, e.g., v0.1.0)
  -All        Build for Windows, Linux, and macOS
  -Help       Show this help message

Examples:
  .\build.ps1              # Dev build with timestamp
  .\build.ps1 -Release     # Release build with version from VERSION file
  .\build.ps1 -All         # Build for all platforms
  .\build.ps1 -Release -All # Release build for all platforms
"@
    exit 0
}

$ErrorActionPreference = "Stop"

# Ensure release directory exists
$releaseDir = "release"
if (-not (Test-Path $releaseDir)) {
    New-Item -ItemType Directory -Path $releaseDir | Out-Null
}

# Get build info
$timestamp = Get-Date -Format "yyyyMMdd-HHmm"
$baseName = "qobuz-dl-go"

# Read version from VERSION file
$versionFile = "VERSION"
if (Test-Path $versionFile) {
    $version = (Get-Content $versionFile -Raw).Trim()
} else {
    $version = "dev"
}

# Get git commit hash (short)
$gitCommit = "unknown"
try {
    $gitCommit = (git rev-parse --short HEAD 2>$null)
    if (-not $gitCommit) { $gitCommit = "unknown" }
} catch {
    $gitCommit = "unknown"
}

# Determine version tag for filename
if ($Release) {
    $versionTag = "v$version"
    $buildVersion = $version
} else {
    $versionTag = $timestamp
    $buildVersion = "dev-$timestamp"
}

# Build ldflags for version injection
$ldflags = "-s -w " +
    "-X qobuz-dl-go/internal/version.Version=$buildVersion " +
    "-X qobuz-dl-go/internal/version.BuildTime=$timestamp " +
    "-X qobuz-dl-go/internal/version.GitCommit=$gitCommit"

function Build-Platform {
    param(
        [string]$GOOS,
        [string]$GOARCH,
        [string]$Ext
    )
    
    $outputName = "$releaseDir/${baseName}-${versionTag}-${GOOS}-${GOARCH}${Ext}"
    
    Write-Host "Building for $GOOS/$GOARCH -> $outputName" -ForegroundColor Cyan
    
    $env:GOOS = $GOOS
    $env:GOARCH = $GOARCH
    $env:CGO_ENABLED = "0"
    
    go build -ldflags="$ldflags" -o $outputName ./cmd/qobuz-dl
    
    if ($LASTEXITCODE -eq 0) {
        Write-Host "  OK" -ForegroundColor Green
    } else {
        Write-Host "  FAILED" -ForegroundColor Red
        exit 1
    }
}

Write-Host ""
Write-Host "=== qobuz-dl-go Build ===" -ForegroundColor Yellow
Write-Host "Version: $buildVersion" -ForegroundColor Gray
Write-Host "Commit:  $gitCommit" -ForegroundColor Gray
Write-Host ""

if ($All) {
    # Build for multiple platforms
    Build-Platform -GOOS "windows" -GOARCH "amd64" -Ext ".exe"
    Build-Platform -GOOS "linux" -GOARCH "amd64" -Ext ""
    Build-Platform -GOOS "darwin" -GOARCH "amd64" -Ext ""
    Build-Platform -GOOS "darwin" -GOARCH "arm64" -Ext ""
} else {
    # Build for current platform (Windows)
    $outputName = "$releaseDir/${baseName}-${versionTag}-windows-amd64.exe"
    
    Write-Host "Building -> $outputName" -ForegroundColor Cyan
    
    # Clear cross-compile env vars to use native platform
    Remove-Item Env:GOOS -ErrorAction SilentlyContinue
    Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
    $env:CGO_ENABLED = "0"
    
    go build -ldflags="$ldflags" -o $outputName ./cmd/qobuz-dl
    
    if ($LASTEXITCODE -eq 0) {
        Write-Host "  OK" -ForegroundColor Green
        Write-Host ""
        Write-Host "Output: $outputName" -ForegroundColor Green
    } else {
        Write-Host "  FAILED" -ForegroundColor Red
        exit 1
    }
}

Write-Host ""
Write-Host "Build complete!" -ForegroundColor Yellow
