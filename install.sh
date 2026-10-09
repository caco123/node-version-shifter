#!/usr/bin/env bash
# ==============================================================================
# nvsh (node-version-shifter) installer for Linux and macOS (Darwin)
# 100% User-Space - Zero Sudo / Administrator Privileges Required
# Shim Mode - No Symlinks
# ==============================================================================
set -e

NVSH_DIR="${NVSH_DIR:-$HOME/.nvsh}"
BIN_DIR="$NVSH_DIR/bin"
SHIMS_DIR="$NVSH_DIR/shims"
VERSIONS_DIR="$NVSH_DIR/versions"
CACHE_DIR="$NVSH_DIR/cache"

echo "==> Installing nvsh to $NVSH_DIR (User-Space, No Admin Privileges)..."

mkdir -p "$BIN_DIR" "$SHIMS_DIR" "$VERSIONS_DIR" "$CACHE_DIR"

# Detect OS and Architecture
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$ARCH" in
  x86_64|amd64)
    GOARCH="amd64"
    ;;
  arm64|aarch64)
    GOARCH="arm64"
    ;;
  *)
    echo "Error: Unsupported architecture $ARCH"
    exit 1
    ;;
esac

TARGET_BIN="$BIN_DIR/nvsh"

# If running within the source repository and Go is installed, build locally
if command -v go >/dev/null 2>&1 && [ -f "./cmd/nvsh/main.go" ]; then
  echo "==> Compiling nvsh from source using Go..."
  go build -ldflags="-s -w" -o "$TARGET_BIN" ./cmd/nvsh
elif [ -f "./nvsh" ]; then
  echo "==> Using local nvsh binary..."
  cp "./nvsh" "$TARGET_BIN"
  chmod 755 "$TARGET_BIN"
else
  # Binary release fallback
  echo "==> Please build nvsh using 'go build -o nvsh ./cmd/nvsh' or place the precompiled binary in $TARGET_BIN"
  exit 1
fi

chmod 755 "$TARGET_BIN"

# Execute internal setup to populate shims and configure shell profile
"$TARGET_BIN" setup

echo ""
echo "======================================================================"
echo "  nvsh (Node Version Shifter) installed successfully!"
echo "  Mode: Shim Mode (NO symlinks)"
echo "======================================================================"
echo ""
echo "To get started in your current terminal session, run:"
echo "  export PATH=\"$SHIMS_DIR:$BIN_DIR:\$PATH\""
echo ""
echo "Then install Node.js:"
echo "  nvsh install lts"
echo "  node -v"
echo ""
