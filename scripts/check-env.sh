#!/usr/bin/env bash
# ==============================================================================
# silent-receipts-lab: Environment Diagnostic & Preflight Checker
# ==============================================================================
# Designed specifically for macOS (OCLP Ventura/Sonoma on Intel x86_64)
# Also works on Linux fallback machines.
# ==============================================================================

set -u

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
BOLD='\033[1m'
NC='\033[0m' # No Color

pass() { echo -e "  [${GREEN}✓ PASS${NC}] $1"; }
fail() { echo -e "  [${RED}✗ FAIL${NC}] $1"; }
warn() { echo -e "  [${YELLOW}! WARN${NC}] $1"; }
info() { echo -e "  [${BLUE}i INFO${NC}] $1"; }

echo -e "${BOLD}======================================================${NC}"
echo -e "${BOLD}   silent-receipts-lab: Environment Diagnostic Check  ${NC}"
echo -e "${BOLD}======================================================${NC}"
echo ""

TOTAL_ERRORS=0

# ------------------------------------------------------------------------------
# 1. Operating System & Architecture
# ------------------------------------------------------------------------------
echo -e "${BOLD}1. System & Architecture Detection${NC}"
OS_NAME=$(uname -s)
ARCH_NAME=$(uname -m)

info "Operating System: $OS_NAME"
info "CPU Architecture: $ARCH_NAME"

if [ "$OS_NAME" = "Darwin" ]; then
    MAC_VER=$(sw_vers -productVersion 2>/dev/null || echo "Unknown")
    MAC_BUILD=$(sw_vers -buildVersion 2>/dev/null || echo "Unknown")
    info "macOS Product Version: $MAC_VER (Build $MAC_BUILD)"

    MAJOR_VER=$(echo "$MAC_VER" | cut -d. -f1)
    if [ "$MAJOR_VER" -ge 13 ]; then
        pass "Running modern macOS ($MAC_VER) — ideal for current Go toolchains"
    elif [ "$MAJOR_VER" -eq 11 ] || [ "$MAJOR_VER" -eq 12 ]; then
        warn "Running macOS $MAC_VER (Big Sur / Monterey). Go works, but newer OCLP Ventura/Sonoma is preferred."
    else
        warn "macOS version $MAC_VER detected."
    fi

    if [ "$ARCH_NAME" = "x86_64" ]; then
        pass "Hardware is Intel x86_64. Ensure all manual installers use 'darwin-amd64'."
    elif [ "$ARCH_NAME" = "arm64" ]; then
        pass "Hardware is Apple Silicon (arm64). Ensure installers use 'darwin-arm64'."
    fi
elif [ "$OS_NAME" = "Linux" ]; then
    info "Running on Linux ($ARCH_NAME) — good as a fallback environment."
    pass "Linux environment detected."
else
    warn "Unrecognized operating system: $OS_NAME"
fi
echo ""

# ------------------------------------------------------------------------------
# 2. Go Toolchain Verification
# ------------------------------------------------------------------------------
echo -e "${BOLD}2. Go Toolchain Check${NC}"
GO_BIN=""

if command -v go >/dev/null 2>&1; then
    GO_BIN=$(command -v go)
    GO_VERSION_STR=$(go version 2>/dev/null)
    pass "Go binary found at: $GO_BIN"
    pass "Go version: $GO_VERSION_STR"

    # Check Go version number
    GO_VER_NUM=$(go version | awk '{print $3}' | sed 's/go//')
    info "Detected Go version string: $GO_VER_NUM"
else
    # Check default manual installation path
    if [ -x "/usr/local/go/bin/go" ]; then
        warn "Go is installed at /usr/local/go/bin/go, but NOT found in your current PATH!"
        echo -e "       ${YELLOW}Fix:${NC} Run this command to add Go to your zsh profile:"
        echo -e "       ${BOLD}echo 'export PATH=\"/usr/local/go/bin:\$PATH\"' >> ~/.zprofile && source ~/.zprofile${NC}"
        GO_BIN="/usr/local/go/bin/go"
    else
        fail "Go is not installed or not found in standard paths."
        echo -e "       ${YELLOW}Fix:${NC} Download the official macOS package from https://go.dev/dl"
        echo -e "       Choose: ${BOLD}go<version>.darwin-amd64.pkg${NC} (for Intel Mac)"
        TOTAL_ERRORS=$((TOTAL_ERRORS + 1))
    fi
fi
echo ""

# ------------------------------------------------------------------------------
# 3. Shell Profile & PATH Configuration
# ------------------------------------------------------------------------------
echo -e "${BOLD}3. Shell & PATH Configuration${NC}"
info "Current Shell: ${SHELL:-unknown}"

if [ "$OS_NAME" = "Darwin" ]; then
    if echo "$PATH" | grep -q "/usr/local/go/bin"; then
        pass "/usr/local/go/bin is present in PATH"
    else
        warn "/usr/local/go/bin is missing from the active PATH"
        echo -e "       Add to ~/.zprofile for permanent access across all Terminal windows:"
        echo -e "       ${BOLD}echo 'export PATH=\"/usr/local/go/bin:\$PATH\"' >> ~/.zprofile${NC}"
    fi

    # Check ~/.zprofile and ~/.zshrc
    if [ -f "$HOME/.zprofile" ] && grep -q "/usr/local/go/bin" "$HOME/.zprofile" 2>/dev/null; then
        pass "Found Go PATH entry in ~/.zprofile"
    else
        info "~/.zprofile does not yet reference /usr/local/go/bin"
    fi
fi
echo ""

# ------------------------------------------------------------------------------
# 4. Pure-Go SQLite vs CGO Status
# ------------------------------------------------------------------------------
echo -e "${BOLD}4. CGO & Pure-Go SQLite Status${NC}"
if [ -n "$GO_BIN" ]; then
    CURRENT_CGO=$("$GO_BIN" env CGO_ENABLED 2>/dev/null || echo "unknown")
    info "Go CGO_ENABLED setting: $CURRENT_CGO"

    info "This project uses modernc.org/sqlite (pure Go)."
    info "Benefit on OCLP: You do NOT need Apple Xcode Command Line Tools (CLT) or gcc!"
    info "You can build with CGO_ENABLED=0 without issues."
    pass "Pure-Go SQLite configuration selected for zero-CGO builds."
fi
echo ""

# ------------------------------------------------------------------------------
# 5. Repository & Session Storage Safety
# ------------------------------------------------------------------------------
echo -e "${BOLD}5. Repository & Session Safety${NC}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$SCRIPT_DIR"

if [ -f ".gitignore" ]; then
    if grep -q "store/" .gitignore && grep -q "\*.db" .gitignore; then
        pass ".gitignore properly protects 'store/' and '*.db' session data."
    else
        warn ".gitignore may not be ignoring store/ or *.db files!"
    fi
else
    fail ".gitignore file not found!"
    TOTAL_ERRORS=$((TOTAL_ERRORS + 1))
fi

if [ -d "store" ]; then
    pass "Session storage directory 'store/' exists."
else
    info "Session directory 'store/' will be created automatically on first run."
fi
echo ""

# ------------------------------------------------------------------------------
# 6. Summary & Next Steps
# ------------------------------------------------------------------------------
echo -e "${BOLD}======================================================${NC}"
if [ $TOTAL_ERRORS -eq 0 ]; then
    echo -e "${GREEN}${BOLD}   Preflight Check Passed! Ready to proceed.          ${NC}"
    echo -e "${BOLD}======================================================${NC}"
    echo ""
    echo "Next steps:"
    echo "  1. Test minimal client:  go run ./cmd/minimal"
    echo "  2. Receipt lab tool:     go run ./cmd/receipts"
    echo "  3. Read documentation:   cat docs/01-environment-setup.md"
else
    echo -e "${RED}${BOLD}   Preflight Check completed with $TOTAL_ERRORS issue(s).       ${NC}"
    echo -e "${BOLD}======================================================${NC}"
    echo ""
    echo "Review the [FAIL] notes above and follow the steps in docs/01-environment-setup.md."
fi
exit $TOTAL_ERRORS
