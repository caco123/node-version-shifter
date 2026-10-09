# ==============================================================================
# nvsh (node-version-shifter) installer for Windows (PowerShell)
# 100% User-Space - Zero Administrator Privileges Required
# Shim Mode - No Symlinks
# ==============================================================================
$ErrorActionPreference = "Stop"

$NvshDir = if ($env:NVSH_DIR) { $env:NVSH_DIR } else { Join-Path $env:USERPROFILE ".nvsh" }
$BinDir = Join-Path $NvshDir "bin"
$ShimsDir = Join-Path $NvshDir "shims"
$VersionsDir = Join-Path $NvshDir "versions"
$CacheDir = Join-Path $NvshDir "cache"

Write-Host "==> Installing nvsh to $NvshDir (User-Space, No Admin Privileges)..." -ForegroundColor Cyan

# Create directories
New-Item -ItemType Directory -Force -Path $BinDir | Out-Null
New-Item -ItemType Directory -Force -Path $ShimsDir | Out-Null
New-Item -ItemType Directory -Force -Path $VersionsDir | Out-Null
New-Item -ItemType Directory -Force -Path $CacheDir | Out-Null

$TargetBin = Join-Path $BinDir "nvsh.exe"

# If Go is installed and running from source repo
if ((Get-Command "go" -ErrorAction SilentlyContinue) -and (Test-Path ".\cmd\nvsh\main.go")) {
    Write-Host "==> Compiling nvsh from source using Go..." -ForegroundColor Yellow
    go build -ldflags="-s -w" -o $TargetBin .\cmd\nvsh
} elseif (Test-Path ".\nvsh.exe") {
    Copy-Item -Path ".\nvsh.exe" -Destination $TargetBin -Force
} else {
    Write-Host "Error: nvsh.exe not found and Go compiler not available." -ForegroundColor Red
    exit 1
}

# Run internal setup to populate shims and configure shell profile
& $TargetBin setup

# Add to User PATH if not already present (No admin rights required)
$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
$PathsToAdd = @($ShimsDir, $BinDir)
$Modified = $false

foreach ($p in $PathsToAdd) {
    if ($UserPath -notlike "*$p*") {
        $UserPath = "$p;$UserPath"
        $Modified = $true
    }
}

if ($Modified) {
    [Environment]::SetEnvironmentVariable("Path", $UserPath, "User")
    Write-Host "==> Added nvsh shims and bin to User PATH." -ForegroundColor Green
}

# Update current session PATH
$env:PATH = "$ShimsDir;$BinDir;$env:PATH"

Write-Host ""
Write-Host "======================================================================" -ForegroundColor Green
Write-Host "  nvsh (Node Version Shifter) installed successfully!" -ForegroundColor Green
Write-Host "  Mode: Shim Mode (NO symlinks required)" -ForegroundColor Green
Write-Host "======================================================================" -ForegroundColor Green
Write-Host ""
Write-Host "To verify installation in your current PowerShell session:"
Write-Host "  nvsh --version"
Write-Host ""
Write-Host "Then install your first Node.js version:"
Write-Host "  nvsh install lts"
Write-Host "  node -v"
Write-Host ""
