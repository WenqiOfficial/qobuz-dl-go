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

Output Structure:
  release/
    v0.1.0/
      windows-amd64/
        qobuz-dl-go.exe
      linux-amd64/
        qobuz-dl-go
      darwin-arm64/
        qobuz-dl-go
"@
    exit 0
}

$ErrorActionPreference = "Stop"

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

# Determine version tag
if ($Release) {
    $versionTag = "v$version"
    $buildVersion = $version
} else {
    $versionTag = "dev-$timestamp"
    $buildVersion = "dev-$timestamp"
}

# Build ldflags for version injection
$ldflags = "-s -w " +
    "-X github.com/WenqiOfficial/qobuz-dl-go/internal/version.Version=$buildVersion " +
    "-X github.com/WenqiOfficial/qobuz-dl-go/internal/version.BuildTime=$timestamp " +
    "-X github.com/WenqiOfficial/qobuz-dl-go/internal/version.GitCommit=$gitCommit"

function Build-Platform {
    param(
        [string]$GOOS,
        [string]$GOARCH,
        [string]$Ext
    )
    
    # Create output directory: release/v0.1.0/windows-amd64/
    $platformDir = "release/$versionTag/$GOOS-$GOARCH"
    if (-not (Test-Path $platformDir)) {
        New-Item -ItemType Directory -Path $platformDir -Force | Out-Null
    }
    
    $outputName = "$platformDir/${baseName}${Ext}"
    
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
    Build-Platform -GOOS "windows" -GOARCH "arm64" -Ext ".exe"
    Build-Platform -GOOS "linux" -GOARCH "amd64" -Ext ""
    Build-Platform -GOOS "linux" -GOARCH "arm64" -Ext ""
    Build-Platform -GOOS "darwin" -GOARCH "amd64" -Ext ""
    Build-Platform -GOOS "darwin" -GOARCH "arm64" -Ext ""
    
    Write-Host ""
    Write-Host "Output directory: release/$versionTag/" -ForegroundColor Green
} else {
    # Build for current platform (Windows)
    $platformDir = "release/$versionTag/windows-amd64"
    if (-not (Test-Path $platformDir)) {
        New-Item -ItemType Directory -Path $platformDir -Force | Out-Null
    }
    
    $outputName = "$platformDir/${baseName}.exe"
    
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
