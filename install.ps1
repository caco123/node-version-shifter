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

$ScriptDir = if ($PSScriptRoot) { $PSScriptRoot } else { (Get-Location).Path }
$LocalCandidates = @(
    (Join-Path $ScriptDir "nvsh.exe"),
    ".\nvsh.exe",
    (Join-Path $ScriptDir "dist\nvsh-windows-amd64.exe"),
    ".\dist\nvsh-windows-amd64.exe"
)

$FoundLocal = $null
foreach ($cand in $LocalCandidates) {
    if (Test-Path $cand) {
        $FoundLocal = (Resolve-Path $cand).Path
        break
    }
}

# If Go is installed and running from source repo
if ((Get-Command "go" -ErrorAction SilentlyContinue) -and (Test-Path (Join-Path $ScriptDir "cmd\nvsh\main.go"))) {
    Write-Host "==> Compiling nvsh from source using Go..." -ForegroundColor Yellow
    go build -ldflags="-s -w" -o $TargetBin (Join-Path $ScriptDir "cmd\nvsh")
} elseif ($FoundLocal) {
    Write-Host "==> Using local binary: $FoundLocal" -ForegroundColor Yellow
    Copy-Item -Path $FoundLocal -Destination $TargetBin -Force
} else {
    Write-Host "==> Local nvsh.exe not found and Go compiler not available." -ForegroundColor Yellow
    Write-Host "==> Downloading precompiled nvsh binary from GitHub..." -ForegroundColor Cyan

    $Arch = if ([System.Environment]::Is64BitOperatingSystem) {
        if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "amd64" }
    } else { "386" }

    $Repo = "caco123/node-version-shifter"
    $DownloadUrls = @(
        "https://github.com/$Repo/releases/latest/download/nvsh-windows-$Arch.exe",
        "https://raw.githubusercontent.com/$Repo/master/nvsh.exe"
    )

    $Downloaded = $false
    try {
        [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
    } catch {}

    foreach ($url in $DownloadUrls) {
        try {
            Write-Host "==> Attempting download from: $url" -ForegroundColor Gray
            Invoke-WebRequest -Uri $url -OutFile $TargetBin -UseBasicParsing -TimeoutSec 30
            if ((Test-Path $TargetBin) -and ((Get-Item $TargetBin).Length -gt 100000)) {
                $Downloaded = $true
                Write-Host "==> Download completed successfully." -ForegroundColor Green
                break
            }
        } catch {
            # Try next URL
        }
    }

    if (-not $Downloaded) {
        Write-Host "Error: nvsh.exe could not be downloaded and Go compiler is not available." -ForegroundColor Red
        Write-Host "Please place nvsh.exe in the repository root or install Go." -ForegroundColor Red
        exit 1
    }
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
