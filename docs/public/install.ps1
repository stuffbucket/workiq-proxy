# workiq-proxy installer for Windows
# https://stuffbucket.github.io/workiq-proxy/
#
# Usage:
#   irm https://stuffbucket.github.io/workiq-proxy/install.ps1 | iex
#
# What it does:
#   1. Detects (or installs) Node.js 18+
#   2. Installs @stuffbucket/workiq-proxy globally via npm
#   3. Detects existing @microsoft/workiq installations
#
# Environment variables:
#   WORKIQ_SKIP_NODE_INSTALL=1  — skip automatic Node.js installation
#   WORKIQ_NPM_FLAGS="..."     — extra flags passed to npm install

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

# ── Helpers ──────────────────────────────────────────────────────────

function Write-Info  { param([string]$Message) Write-Host "▸ $Message" -ForegroundColor Green }
function Write-Warn  { param([string]$Message) Write-Host "▸ $Message" -ForegroundColor Yellow }
function Write-Fail  { param([string]$Message) Write-Host "✗ $Message" -ForegroundColor Red; exit 1 }

function Test-Command { param([string]$Name) return [bool](Get-Command $Name -ErrorAction SilentlyContinue) }

# ── Node.js ──────────────────────────────────────────────────────────

$MinNodeMajor = 18

function Test-NodeVersion {
    if (-not (Test-Command 'node')) { return $false }
    try {
        $version = & node --version 2>$null
        $major = [int]($version -replace '^v' -split '\.')[0]
        return $major -ge $MinNodeMajor
    } catch {
        return $false
    }
}

function Install-Node {
    if ($env:WORKIQ_SKIP_NODE_INSTALL -eq '1') {
        Write-Fail "Node.js $MinNodeMajor+ is required but WORKIQ_SKIP_NODE_INSTALL is set. Install Node.js manually."
    }

    Write-Info "Node.js $MinNodeMajor+ not found — installing..."

    if (Test-Command 'winget') {
        Write-Info 'Installing Node.js via winget...'
        & winget install --id OpenJS.NodeJS.LTS --accept-source-agreements --accept-package-agreements
        # Refresh PATH so node/npm are available in this session
        $env:Path = [System.Environment]::GetEnvironmentVariable('Path', 'Machine') + ';' + [System.Environment]::GetEnvironmentVariable('Path', 'User')
    } elseif (Test-Command 'choco') {
        Write-Info 'Installing Node.js via Chocolatey...'
        & choco install nodejs-lts -y
        $env:Path = [System.Environment]::GetEnvironmentVariable('Path', 'Machine') + ';' + [System.Environment]::GetEnvironmentVariable('Path', 'User')
    } elseif (Test-Command 'scoop') {
        Write-Info 'Installing Node.js via Scoop...'
        & scoop install nodejs-lts
    } else {
        Write-Fail 'No supported package manager found (winget, choco, scoop). Install Node.js from https://nodejs.org'
    }
}

function Confirm-Node {
    if (Test-NodeVersion) {
        Write-Info "Node.js $(& node --version) found"
        return
    }

    if (Test-Command 'node') {
        Write-Warn "Node.js $(& node --version) is installed but $MinNodeMajor+ is required"
    }

    Install-Node

    if (-not (Test-NodeVersion)) {
        Write-Fail "Node.js installation failed or version is still below $MinNodeMajor. Install manually from https://nodejs.org"
    }
    Write-Info "Node.js $(& node --version) installed"
}

# ── npm ──────────────────────────────────────────────────────────────

function Confirm-Npm {
    if (-not (Test-Command 'npm')) {
        Write-Fail 'npm not found. It should have been installed with Node.js. Install it manually or reinstall Node.'
    }
}

# ── Check for existing @microsoft/workiq ─────────────────────────────

function Test-WorkIQ {
    try {
        $null = & npm ls -g '@microsoft/workiq' --depth=0 2>$null
        if ($LASTEXITCODE -eq 0) {
            Write-Info '@microsoft/workiq is already installed globally'
            Write-Warn 'workiq-proxy includes @microsoft/workiq as a dependency — the global copy will work alongside it'
        }
    } catch {
        # not installed — that's fine
    }
}

# ── Install workiq-proxy ─────────────────────────────────────────────

function Install-WorkIQProxy {
    if (Test-Command 'workiq-proxy') {
        try {
            $current = (& workiq-proxy version 2>$null | Select-Object -First 1)
            if ($current) {
                Write-Info "workiq-proxy is already installed: $current"
                Write-Info 'Upgrading to latest...'
            }
        } catch {}
    }

    Write-Info 'Installing @stuffbucket/workiq-proxy...'
    $npmFlags = if ($env:WORKIQ_NPM_FLAGS) { $env:WORKIQ_NPM_FLAGS } else { '' }
    $installCmd = "npm install -g @stuffbucket/workiq-proxy $npmFlags".Trim()
    Invoke-Expression $installCmd

    try {
        $installed = (& workiq-proxy version 2>$null | Select-Object -First 1)
        Write-Info "workiq-proxy installed: $($installed ?? 'unknown version')"
    } catch {
        Write-Info 'workiq-proxy installed: unknown version'
    }
}

# ── Main ─────────────────────────────────────────────────────────────

Write-Host ''
Write-Host 'workiq-proxy installer' -ForegroundColor White -NoNewline
Write-Host ''
Write-Host 'https://stuffbucket.github.io/workiq-proxy/' -ForegroundColor DarkGray
Write-Host ''

Confirm-Node
Confirm-Npm
Test-WorkIQ
Install-WorkIQProxy

Write-Host ''
Write-Info 'Done! Next steps:'
Write-Host ''
Write-Host '  workiq-proxy accept-eula    ' -ForegroundColor White -NoNewline; Write-Host 'Accept the Work IQ EULA'
Write-Host '  workiq-proxy ask            ' -ForegroundColor White -NoNewline; Write-Host 'Ask a question interactively'
Write-Host '  workiq-proxy install claude  ' -ForegroundColor White -NoNewline; Write-Host 'Register with Claude Code'
Write-Host '  workiq-proxy install copilot ' -ForegroundColor White -NoNewline; Write-Host 'Register with VS Code Copilot'
Write-Host ''
Write-Host '  Docs: ' -NoNewline; Write-Host 'https://stuffbucket.github.io/workiq-proxy/' -ForegroundColor DarkGray
Write-Host ''
