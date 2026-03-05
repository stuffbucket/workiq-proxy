#!/bin/sh
# workiq-proxy installer — https://stuffbucket.github.io/workiq-proxy/
#
# Usage:
#   curl -fsSL https://stuffbucket.github.io/workiq-proxy/install.sh | sh
#
# What it does:
#   1. Detects (or installs) Node.js 18+
#   2. Installs @stuffbucket/workiq-proxy globally via npm
#   3. Detects existing @microsoft/workiq installations
#
# Environment variables:
#   WORKIQ_SKIP_NODE_INSTALL=1  — skip automatic Node.js installation
#   WORKIQ_NPM_FLAGS="..."     — extra flags passed to npm install

set -e

# ── Helpers ──────────────────────────────────────────────────────────

BOLD=''
DIM=''
GREEN=''
YELLOW=''
RED=''
RESET=''

if [ -t 1 ]; then
  BOLD='\033[1m'
  DIM='\033[2m'
  GREEN='\033[32m'
  YELLOW='\033[33m'
  RED='\033[31m'
  RESET='\033[0m'
fi

info()  { printf "${GREEN}▸${RESET} %s\n" "$*"; }
warn()  { printf "${YELLOW}▸${RESET} %s\n" "$*"; }
fail()  { printf "${RED}✗${RESET} %s\n" "$*" >&2; exit 1; }

command_exists() { command -v "$1" >/dev/null 2>&1; }

# ── Detect OS and package manager ────────────────────────────────────

detect_os() {
  OS="$(uname -s)"
  case "$OS" in
    Darwin) ;;
    Linux)  ;;
    MINGW*|MSYS*|CYGWIN*|Windows_NT)
      printf "\n"
      warn "It looks like you're running on Windows."
      info "Use PowerShell instead:"
      printf "\n"
      printf "  %sirm https://stuffbucket.github.io/workiq-proxy/install.ps1 | iex%s\n" "$BOLD" "$RESET"
      printf "\n"
      exit 1
      ;;
    *)      fail "Unsupported OS: $OS. Install Node.js manually, then run: npm install -g @stuffbucket/workiq-proxy" ;;
  esac
}

# ── Node.js ──────────────────────────────────────────────────────────

MIN_NODE_MAJOR=18

check_node_version() {
  if ! command_exists node; then
    return 1
  fi
  NODE_VERSION="$(node --version 2>/dev/null)" || return 1
  NODE_MAJOR="$(printf '%s' "$NODE_VERSION" | sed 's/^v//' | cut -d. -f1)"
  if [ "$NODE_MAJOR" -ge "$MIN_NODE_MAJOR" ] 2>/dev/null; then
    return 0
  fi
  return 1
}

install_node() {
  if [ "${WORKIQ_SKIP_NODE_INSTALL:-}" = "1" ]; then
    fail "Node.js $MIN_NODE_MAJOR+ is required but WORKIQ_SKIP_NODE_INSTALL is set. Install Node.js manually."
  fi

  info "Node.js $MIN_NODE_MAJOR+ not found — installing..."

  # macOS: prefer Homebrew
  if [ "$OS" = "Darwin" ]; then
    if command_exists brew; then
      info "Installing Node.js via Homebrew..."
      brew install node
    else
      fail "Homebrew not found. Install Node.js from https://nodejs.org or install Homebrew first."
    fi
    return
  fi

  # Linux: try common package managers
  if command_exists apt-get; then
    info "Installing Node.js via apt..."
    sudo apt-get update -qq
    sudo apt-get install -y -qq nodejs npm
  elif command_exists dnf; then
    info "Installing Node.js via dnf..."
    sudo dnf install -y nodejs npm
  elif command_exists yum; then
    info "Installing Node.js via yum..."
    sudo yum install -y nodejs npm
  elif command_exists pacman; then
    info "Installing Node.js via pacman..."
    sudo pacman -S --noconfirm nodejs npm
  elif command_exists apk; then
    info "Installing Node.js via apk..."
    sudo apk add --no-cache nodejs npm
  else
    fail "No supported package manager found. Install Node.js from https://nodejs.org"
  fi
}

ensure_node() {
  if check_node_version; then
    info "Node.js $(node --version) found"
    return
  fi

  # Node exists but is too old?
  if command_exists node; then
    warn "Node.js $(node --version) is installed but $MIN_NODE_MAJOR+ is required"
  fi

  install_node

  # Verify installation
  if ! check_node_version; then
    fail "Node.js installation failed or version is still below $MIN_NODE_MAJOR. Install manually from https://nodejs.org"
  fi
  info "Node.js $(node --version) installed"
}

# ── npm ──────────────────────────────────────────────────────────────

ensure_npm() {
  if ! command_exists npm; then
    fail "npm not found. It should have been installed with Node.js. Install it manually or reinstall Node."
  fi
}

# ── Check for existing @microsoft/workiq ─────────────────────────────

check_workiq() {
  if npm ls -g @microsoft/workiq --depth=0 >/dev/null 2>&1; then
    info "@microsoft/workiq is already installed globally"
    warn "workiq-proxy includes @microsoft/workiq as a dependency — the global copy will work alongside it"
  fi
}

# ── Install workiq-proxy ─────────────────────────────────────────────

install_workiq_proxy() {
  if command_exists workiq-proxy; then
    CURRENT="$(workiq-proxy version 2>/dev/null | head -1 || true)"
    if [ -n "$CURRENT" ]; then
      info "workiq-proxy is already installed: $CURRENT"
      info "Upgrading to latest..."
    fi
  fi

  info "Installing @stuffbucket/workiq-proxy..."
  # shellcheck disable=SC2086
  npm install -g @stuffbucket/workiq-proxy ${WORKIQ_NPM_FLAGS:-}
  info "workiq-proxy installed: $(workiq-proxy version 2>/dev/null | head -1 || echo 'unknown version')"
}

# ── Main ─────────────────────────────────────────────────────────────

main() {
  printf "\n"
  printf "%sworkiq-proxy installer%s\n" "$BOLD" "$RESET"
  printf "%shttps://stuffbucket.github.io/workiq-proxy/%s\n" "$DIM" "$RESET"
  printf "\n"

  detect_os
  ensure_node
  ensure_npm
  check_workiq
  install_workiq_proxy

  printf "\n"
  info "Done! Next steps:"
  printf "\n"
  printf "  %sworkiq-proxy accept-eula%s    Accept the Work IQ EULA\n" "$BOLD" "$RESET"
  printf "  %sworkiq-proxy ask%s             Ask a question interactively\n" "$BOLD" "$RESET"
  printf "  %sworkiq-proxy install claude%s  Register with Claude Code\n" "$BOLD" "$RESET"
  printf "  %sworkiq-proxy install copilot%s Register with VS Code Copilot\n" "$BOLD" "$RESET"
  printf "\n"
  printf "  Docs: %shttps://stuffbucket.github.io/workiq-proxy/%s\n" "$DIM" "$RESET"
  printf "\n"
}

main
