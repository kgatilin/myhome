#!/usr/bin/env bash
set -euo pipefail

# bootstrap.sh — Bootstrap myhome on a clean machine.
# Usage: ./bootstrap.sh [env]
#   env: environment name (default: full)

ENV="${1:-full}"
MYHOME_REPO="${MYHOME_REPO:-git@github.com:kgatilin/myhome.git}"
MYHOME_SRC="${HOME}/dev/tools/myhome"
MYHOME_BIN="${HOME}/go/bin/myhome"

info() { printf "==> %s\n" "$1"; }

# Step 1: Install mise if not present
if command -v mise &>/dev/null; then
    info "mise already installed"
else
    info "Installing mise..."
    curl -fsSL https://mise.jdx.dev/install.sh | sh
fi

# Add mise to PATH for this session
if [[ -f "${HOME}/.local/bin/mise" ]]; then
    export PATH="${HOME}/.local/bin:${PATH}"
fi

# Activate mise in current shell
eval "$(mise activate bash 2>/dev/null || true)"

# Step 2: Install Go via mise
if mise which go &>/dev/null; then
    info "Go already installed via mise"
else
    info "Installing Go via mise..."
    mise use --global go@latest
    mise install
fi

# Ensure Go is on PATH
eval "$(mise env 2>/dev/null || true)"

# Step 3: Clone myhome source if not present
if [[ -d "${MYHOME_SRC}/.git" ]]; then
    info "myhome source already cloned at ${MYHOME_SRC}"
else
    info "Cloning myhome source..."
    mkdir -p "$(dirname "${MYHOME_SRC}")"
    git clone "${MYHOME_REPO}" "${MYHOME_SRC}"
fi

# Step 4: Build myhome
info "Building myhome..."
mkdir -p "$(dirname "${MYHOME_BIN}")"
(cd "${MYHOME_SRC}" && go build -o "${MYHOME_BIN}" ./cmd/myhome)

# Ensure go/bin is on PATH
export PATH="${HOME}/go/bin:${PATH}"

# Step 5: Run myhome init
info "Running myhome init --env ${ENV}..."
myhome init --env "${ENV}"

info "Bootstrap complete."
