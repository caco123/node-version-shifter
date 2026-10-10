# ==============================================================================
# nvsh (node-version-shifter) uninstaller for Windows (PowerShell)
# 100% User-Space - Zero Administrator Privileges Required
# ==============================================================================
[CmdletBinding()]
param(
    [switch]$Force,
    [switch]$KeepVersions,
    [string]$CustomNvshDir = ""
)

$ErrorActionPreference = "Stop"

$NvshDir = if ($CustomNvshDir) {
    $CustomNvshDir
} elseif ($env:NVSH_DIR) {
    $env:NVSH_DIR
} else {
    Join-Path $env:USERPROFILE ".nvsh"
}

$BinDir = Join-Path $NvshDir "bin"
$ShimsDir = Join-Path $NvshDir "shims"
$VersionsDir = Join-Path $NvshDir "versions"

Write-Host "======================================================================" -ForegroundColor Cyan
Write-Host "  nvsh (Node Version Shifter) Uninstaller for Windows" -ForegroundColor Cyan
Write-Host "======================================================================" -ForegroundColor Cyan
Write-Host ""

# Prompt for confirmation if -Force is not passed
if (-not $Force) {
    Write-Host "This will remove nvsh, its shims and configuration from:" -ForegroundColor Yellow
    Write-Host "  $NvshDir" -ForegroundColor White
    if ($KeepVersions) {
        Write-Host "  (Downloaded Node.js versions will be kept in $VersionsDir)" -ForegroundColor Yellow
    } else {
        Write-Host "  (All installed Node.js versions will also be deleted)" -ForegroundColor Yellow
    }
    Write-Host ""
    $Confirmation = Read-Host "Are you sure you want to continue? [y/N]"
    if ($Confirmation -notmatch '^[yYsS]') {
        Write-Host "Uninstallation cancelled." -ForegroundColor Yellow
        exit 0
    }
}

Write-Host ""
Write-Host "==> Cleaning User PATH environment variable..." -ForegroundColor Cyan

# 1. Clean User PATH in Windows Registry (no admin required)
$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($UserPath) {
    $Parts = $UserPath -split ';' | Where-Object {
        $trimmed = $_.Trim()
        $trimmed -ne "" -and
        $trimmed -notlike "*\.nvsh*" -and
        $trimmed -notlike "*node-version-shifter*"
    }
    $CleanPath = ($Parts -join ';')
    if ($CleanPath -ne $UserPath) {
        [Environment]::SetEnvironmentVariable("Path", $CleanPath, "User")
        Write-Host "    [OK] Removed nvsh directories from User PATH." -ForegroundColor Green
    } else {
        Write-Host "    [OK] User PATH had no nvsh entries." -ForegroundColor Gray
    }
}

# 2. Remove NVSH_DIR and NODE_HOME User environment variables if set
if ([Environment]::GetEnvironmentVariable("NVSH_DIR", "User")) {
    [Environment]::SetEnvironmentVariable("NVSH_DIR", $null, "User")
    Write-Host "    [OK] Removed NVSH_DIR environment variable." -ForegroundColor Green
}
if ([Environment]::GetEnvironmentVariable("NODE_HOME", "User")) {
    [Environment]::SetEnvironmentVariable("NODE_HOME", $null, "User")
    Write-Host "    [OK] Removed NODE_HOME environment variable." -ForegroundColor Green
}

# 3. Clean PowerShell profile files if present
$ProfileCandidates = @(
    $PROFILE,
    $PROFILE.CurrentUserCurrentHost,
    $PROFILE.CurrentUserAllHosts
) | Where-Object { $_ -and (Test-Path $_) } | Select-Object -Unique

foreach ($prof in $ProfileCandidates) {
    try {
        $lines = Get-Content -Path $prof
        if ($lines -match "nvsh|NVSH_DIR|NODE_HOME") {
            $filtered = $lines | Where-Object {
                $_ -notmatch "nvsh|NVSH_DIR|NODE_HOME|\.nvsh"
            }
            Set-Content -Path $prof -Value $filtered
            Write-Host "    [OK] Cleaned PowerShell profile: $prof" -ForegroundColor Green
        }
    } catch {
        Write-Host "    [!] Could not clean profile $prof" -ForegroundColor Yellow
    }
}

# 4. Remove installation directory
if (Test-Path $NvshDir) {
    if ($KeepVersions) {
        Write-Host "==> Removing nvsh binaries, shims and configuration (keeping versions)..." -ForegroundColor Cyan
        @("bin", "shims", "cache", "default", "config.json") | ForEach-Object {
            $target = Join-Path $NvshDir $_
            if (Test-Path $target) {
                Remove-Item -Path $target -Recurse -Force -ErrorAction SilentlyContinue
            }
        }
        Write-Host "    [OK] Retained $VersionsDir" -ForegroundColor Green
    } else {
        Write-Host "==> Removing $NvshDir..." -ForegroundColor Cyan
        Remove-Item -Path $NvshDir -Recurse -Force -ErrorAction SilentlyContinue
        Write-Host "    [OK] Removed $NvshDir" -ForegroundColor Green
    }
} else {
    Write-Host "==> Directory $NvshDir not found. Skipping folder removal." -ForegroundColor Gray
}

# 5. Clean current session PATH and environment variables
if ($env:PATH) {
    $SessionParts = $env:PATH -split ';' | Where-Object {
        $trimmed = $_.Trim()
        $trimmed -ne "" -and
        $trimmed -notlike "*\.nvsh*" -and
        $trimmed -notlike "*node-version-shifter*"
    }
    $env:PATH = ($SessionParts -join ';')
}
if ($env:NVSH_DIR) {
    Remove-Item env:\NVSH_DIR -ErrorAction SilentlyContinue
}
if ($env:NODE_HOME) {
    Remove-Item env:\NODE_HOME -ErrorAction SilentlyContinue
}

Write-Host ""
Write-Host "======================================================================" -ForegroundColor Green
Write-Host "  nvsh has been successfully uninstalled!" -ForegroundColor Green
Write-Host "======================================================================" -ForegroundColor Green
Write-Host ""
Write-Host "User environment PATH and profiles have been cleaned." -ForegroundColor Green
Write-Host "Restart any open terminal windows to complete the process." -ForegroundColor Green
Write-Host ""
