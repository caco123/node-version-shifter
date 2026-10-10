#!/usr/bin/env bash
# ==============================================================================
# nvsh (node-version-shifter) uninstaller for Linux and macOS (Darwin)
# 100% User-Space - Zero Sudo / Administrator Privileges Required
# ==============================================================================
set -e

NVSH_DIR="${NVSH_DIR:-$HOME/.nvsh}"

FORCE=0
KEEP_VERSIONS=0

# Parse arguments
while [ $# -gt 0 ]; do
  case "$1" in
    -y|--yes|-f|--force)
      FORCE=1
      shift
      ;;
    --keep-versions)
      KEEP_VERSIONS=1
      shift
      ;;
    -h|--help)
      echo "nvsh uninstaller"
      echo ""
      echo "Usage:"
      echo "  ./uninstall.sh [options]"
      echo ""
      echo "Options:"
      echo "  -y, --yes, -f, --force   Skip confirmation prompt"
      echo "  --keep-versions          Keep downloaded Node.js versions in $NVSH_DIR/versions"
      echo "  -h, --help               Show this help message"
      exit 0
      ;;
    *)
      echo "Unknown option: $1"
      echo "Run './uninstall.sh --help' for usage."
      exit 1
      ;;
  esac
done

echo "======================================================================"
echo "  nvsh (Node Version Shifter) Uninstaller"
echo "======================================================================"
echo ""

# Confirm uninstallation if not forced and running in an interactive terminal
if [ "$FORCE" -ne 1 ]; then
  if [ -t 0 ]; then
    echo "This will remove nvsh, its shims, and configuration from:"
    echo "  $NVSH_DIR"
    if [ "$KEEP_VERSIONS" -eq 1 ]; then
      echo "  (Downloaded Node.js versions will be kept in $NVSH_DIR/versions)"
    else
      echo "  (All installed Node.js versions will also be deleted)"
    fi
    echo ""
    read -r -p "Are you sure you want to continue? [y/N] " response
    case "$response" in
      [yY][eE][sS]|[yY]|[sS][iI]|[sS])
        ;;
      *)
        echo "Uninstallation cancelled."
        exit 0
        ;;
    esac
  fi
fi

echo ""
echo "==> Cleaning up shell configuration files..."

CANDIDATE_FILES=(
  "$HOME/.bashrc"
  "$HOME/.bash_profile"
  "$HOME/.zshrc"
  "$HOME/.profile"
  "$HOME/.config/fish/config.fish"
)

clean_file() {
  local file="$1"
  if [ -f "$file" ]; then
    if grep -qE "nvsh|NVSH_DIR" "$file" 2>/dev/null; then
      local tmp_file
      tmp_file="$(mktemp "${TMPDIR:-/tmp}/nvsh_clean.XXXXXX")"
      
      # Filter out lines matching nvsh patterns
      grep -vE "(# nvsh|NVSH_DIR|\.nvsh/shims|\.nvsh/bin|node-version-shifter)" "$file" > "$tmp_file" || true
      
      cat "$tmp_file" > "$file"
      rm -f "$tmp_file"
      echo "    ✓ Cleaned $file"
    fi
  fi
}

for cf in "${CANDIDATE_FILES[@]}"; do
  clean_file "$cf"
done

# Remove nvsh installation directory
if [ -d "$NVSH_DIR" ]; then
  if [ "$KEEP_VERSIONS" -eq 1 ]; then
    echo "==> Removing nvsh binaries, shims, cache and configuration (keeping versions)..."
    rm -rf "$NVSH_DIR/bin" "$NVSH_DIR/shims" "$NVSH_DIR/cache" "$NVSH_DIR/default" "$NVSH_DIR/config.json"
    echo "    ✓ Retained $NVSH_DIR/versions"
  else
    echo "==> Removing nvsh directory ($NVSH_DIR)..."
    rm -rf "$NVSH_DIR"
    echo "    ✓ Removed $NVSH_DIR"
  fi
else
  echo "==> Directory $NVSH_DIR not found. Skipping folder removal."
fi

echo ""
echo "======================================================================"
echo "  nvsh has been successfully uninstalled!"
echo "======================================================================"
echo ""
echo "To reflect changes in your current terminal session, restart your terminal"
echo "or run:"
echo "  hash -r 2>/dev/null || rehash 2>/dev/null"
echo ""
