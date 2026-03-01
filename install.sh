#!/usr/bin/env bash
# EK-1 Ego-Kernel — installer
# Usage:  curl -fsSL https://raw.githubusercontent.com/EgoKernel/EK-1/main/install.sh | bash
# Or run locally from a cloned repo:  bash install.sh
set -euo pipefail

# ── Config ────────────────────────────────────────────────────────────────────
EK1_DIR="${EK1_DIR:-$HOME/.ek1}"
GITHUB_RAW="https://raw.githubusercontent.com/EgoKernel/EK-1/main"
HEALTH_URL="http://localhost/health"
HEALTH_TIMEOUT=120   # seconds to wait for the stack to become healthy
CLI_NAME="ek1"

# ── Colours ───────────────────────────────────────────────────────────────────
if [ -t 1 ]; then
  BOLD='\033[1m'; DIM='\033[2m'; RED='\033[0;31m'; GREEN='\033[0;32m'
  YELLOW='\033[1;33m'; CYAN='\033[0;36m'; RESET='\033[0m'
else
  BOLD=''; DIM=''; RED=''; GREEN=''; YELLOW=''; CYAN=''; RESET=''
fi

info()    { echo -e "${CYAN}[ek1]${RESET} $*"; }
success() { echo -e "${GREEN}[ek1]${RESET} $*"; }
warn()    { echo -e "${YELLOW}[ek1]${RESET} $*"; }
error()   { echo -e "${RED}[ek1] ERROR:${RESET} $*" >&2; }
die()     { error "$*"; exit 1; }
step()    { echo -e "\n${BOLD}▶ $*${RESET}"; }

# ── Banner ────────────────────────────────────────────────────────────────────
print_banner() {
  echo -e "${BOLD}${CYAN}"
  cat <<'EOF'
  _____  _  __        __  _
 | ____|| |/ / ____  /_ || |
 |  _|  | ' / |___|  | || |
 | |___ | . \  ___   | || |___
 |_____||_|\_\|___|  |_||_____|
EOF
  echo -e "${RESET}${DIM}  Ego-Kernel — your personal AI agent${RESET}"
  echo ""
}

# ── OS detection ─────────────────────────────────────────────────────────────
detect_os() {
  OS="$(uname -s)"
  ARCH="$(uname -m)"
  case "$OS" in
    Linux*)
      if grep -qi microsoft /proc/version 2>/dev/null; then
        PLATFORM="wsl"
      else
        PLATFORM="linux"
      fi
      ;;
    Darwin*) PLATFORM="macos" ;;
    *)       die "Unsupported OS: $OS. EK-1 supports Linux, macOS, and WSL2." ;;
  esac
  info "Detected platform: ${BOLD}${PLATFORM}${RESET} (${ARCH})"
}

# ── Docker check / install ────────────────────────────────────────────────────
check_docker() {
  step "Checking Docker"

  if ! command -v docker &>/dev/null; then
    warn "Docker not found."
    install_docker
  else
    info "Docker found: $(docker --version)"
  fi

  # Ensure the daemon is running
  if ! docker info &>/dev/null 2>&1; then
    if [[ "$PLATFORM" == "macos" ]]; then
      info "Starting Docker Desktop..."
      open -a Docker
      local waited=0
      while ! docker info &>/dev/null 2>&1; do
        sleep 2; waited=$((waited + 2))
        [[ $waited -ge 60 ]] && die "Docker Desktop did not start within 60 s. Please start it manually."
      done
    elif [[ "$PLATFORM" == "linux" || "$PLATFORM" == "wsl" ]]; then
      info "Starting Docker daemon..."
      sudo systemctl start docker 2>/dev/null || sudo service docker start 2>/dev/null \
        || die "Could not start Docker daemon. Run: sudo systemctl start docker"
    fi
  fi

  # Verify docker compose (v2) is available
  if ! docker compose version &>/dev/null 2>&1; then
    die "Docker Compose v2 not found. Upgrade Docker Desktop, or install the compose plugin:\n  https://docs.docker.com/compose/install/"
  fi

  success "Docker is ready."
}

install_docker() {
  case "$PLATFORM" in
    macos)
      echo ""
      warn "Docker Desktop for Mac is required but not installed."
      echo "  → Download it from: ${BOLD}https://www.docker.com/products/docker-desktop${RESET}"
      echo "  → Install it, then re-run this script."
      exit 1
      ;;
    linux|wsl)
      echo ""
      info "Installing Docker via the official install script..."
      echo "  (This requires sudo access)"
      curl -fsSL https://get.docker.com | sudo sh
      sudo usermod -aG docker "$USER"
      warn "Docker installed. You may need to log out and back in for group permissions."
      warn "For this session, continuing with sudo..."
      DOCKER_SUDO="sudo"
      ;;
  esac
}

# ── Install directory ─────────────────────────────────────────────────────────
create_install_dir() {
  step "Setting up install directory: ${EK1_DIR}"
  mkdir -p "$EK1_DIR"
  success "Directory ready."
}

# ── Config files ─────────────────────────────────────────────────────────────
download_config() {
  step "Fetching configuration files"

  # If we're running from inside the cloned repo, copy local files instead.
  SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
  if [[ -f "$SCRIPT_DIR/docker-compose.release.yml" && -f "$SCRIPT_DIR/nginx.conf" ]]; then
    info "Source repo detected — copying local config files."
    cp "$SCRIPT_DIR/docker-compose.release.yml" "$EK1_DIR/docker-compose.yml"
    cp "$SCRIPT_DIR/nginx.conf"                  "$EK1_DIR/nginx.conf"
  else
    info "Downloading from GitHub..."
    curl -fsSL "${GITHUB_RAW}/docker-compose.release.yml" -o "$EK1_DIR/docker-compose.yml"
    curl -fsSL "${GITHUB_RAW}/nginx.conf"                  -o "$EK1_DIR/nginx.conf"
  fi

  success "Config files ready."
}

# ── Secret key ────────────────────────────────────────────────────────────────
generate_secret_key() {
  if command -v openssl &>/dev/null; then
    openssl rand -hex 32
  elif command -v python3 &>/dev/null; then
    python3 -c "import secrets; print(secrets.token_hex(32))"
  else
    # Fallback: read 32 bytes from urandom, hex-encode with od
    od -vAn -N32 -tx1 /dev/urandom | tr -d ' \n'
  fi
}

# ── Write .env ────────────────────────────────────────────────────────────────
write_env() {
  step "Writing environment configuration"

  ENV_FILE="$EK1_DIR/.env"

  if [[ -f "$ENV_FILE" ]]; then
    warn ".env already exists — keeping your existing configuration."
    return
  fi

  SECRET_KEY="$(generate_secret_key)"

  cat > "$ENV_FILE" <<EOF
# EK-1 configuration — generated by installer on $(date -u +"%Y-%m-%d")

# Required: AES-256-GCM key for encrypting stored credentials.
# Auto-generated — do not share this value.
EK1_SECRET_KEY=${SECRET_KEY}

# Ollama settings (OLLAMA_HOST is overridden to the internal Docker address)
OLLAMA_HOST=http://localhost:11434
OLLAMA_MODEL=llama3.2

# How often the scheduler pulls signals and runs the brain pipeline
SYNC_INTERVAL_MINUTES=15
EOF

  chmod 600 "$ENV_FILE"
  success ".env created with a fresh secret key."
}

# ── Pull images & start ───────────────────────────────────────────────────────
start_services() {
  step "Pulling Docker images (this may take a few minutes on first run)"
  docker compose -f "$EK1_DIR/docker-compose.yml" --project-directory "$EK1_DIR" pull --quiet

  step "Starting EK-1 services"
  docker compose -f "$EK1_DIR/docker-compose.yml" --project-directory "$EK1_DIR" up -d
  success "Services started."
}

# ── Health check ──────────────────────────────────────────────────────────────
wait_for_healthy() {
  step "Waiting for EK-1 to become ready (Ollama model download included)"
  info "This can take 5–15 minutes on first run while llama3.2 downloads (~2 GB)."

  local elapsed=0
  local spinner=('⠋' '⠙' '⠹' '⠸' '⠼' '⠴' '⠦' '⠧' '⠇' '⠏')
  local i=0

  while true; do
    if curl -sf "$HEALTH_URL" &>/dev/null; then
      echo ""
      success "EK-1 is healthy!"
      return 0
    fi

    # Show a simple progress indicator every 5 s
    if (( elapsed % 5 == 0 )); then
      printf "\r  ${DIM}%s  %d s elapsed...${RESET}" "${spinner[$i]}" "$elapsed"
      i=$(( (i + 1) % ${#spinner[@]} ))
    fi

    sleep 1
    elapsed=$((elapsed + 1))

    if (( elapsed >= HEALTH_TIMEOUT )); then
      echo ""
      warn "Stack did not become healthy within ${HEALTH_TIMEOUT} s."
      warn "Ollama model may still be downloading. Check progress with:"
      echo "  ${BOLD}ek1 logs${RESET}"
      return 0   # Not a fatal error — user can check logs
    fi
  done
}

# ── CLI wrapper ───────────────────────────────────────────────────────────────
install_cli() {
  step "Installing the '${CLI_NAME}' command"

  # Choose a bin directory the user can write to without sudo
  local bin_dir
  if [[ "$PLATFORM" == "macos" ]]; then
    bin_dir="/usr/local/bin"
  else
    bin_dir="$HOME/.local/bin"
    mkdir -p "$bin_dir"
  fi

  local cli_path="${bin_dir}/${CLI_NAME}"

  cat > "$cli_path" <<CLISCRIPT
#!/usr/bin/env bash
# EK-1 Ego-Kernel — management CLI
# Auto-generated by install.sh — do not edit manually.
set -euo pipefail

EK1_DIR="${EK1_DIR}"
COMPOSE="docker compose -f \${EK1_DIR}/docker-compose.yml --project-directory \${EK1_DIR}"

case "\${1:-help}" in
  start)
    echo "Starting EK-1..."
    \$COMPOSE up -d
    echo "Open http://localhost in your browser."
    ;;
  stop)
    echo "Stopping EK-1..."
    \$COMPOSE stop
    ;;
  restart)
    echo "Restarting EK-1..."
    \$COMPOSE restart
    ;;
  status)
    \$COMPOSE ps
    ;;
  logs)
    \$COMPOSE logs -f "\${2:-}"
    ;;
  update)
    echo "Pulling latest images..."
    \$COMPOSE pull
    \$COMPOSE up -d
    echo "EK-1 updated."
    ;;
  open)
    URL="http://localhost"
    case "\$(uname -s)" in
      Darwin) open "\$URL" ;;
      Linux)
        if grep -qi microsoft /proc/version 2>/dev/null; then
          powershell.exe /c start "\$URL" 2>/dev/null || xdg-open "\$URL" 2>/dev/null || true
        else
          xdg-open "\$URL" 2>/dev/null || echo "Open \$URL in your browser."
        fi
        ;;
    esac
    ;;
  uninstall)
    echo "This will remove EK-1 containers, images, and all data."
    read -rp "Are you sure? [y/N] " confirm
    if [[ "\$confirm" =~ ^[Yy]$ ]]; then
      \$COMPOSE down -v --rmi all
      rm -rf "\${EK1_DIR}"
      echo "EK-1 has been uninstalled."
      echo "Remove the CLI manually: rm \$(command -v ek1)"
    else
      echo "Aborted."
    fi
    ;;
  help|--help|-h)
    cat <<EOF
EK-1 Ego-Kernel — management CLI

Usage: ek1 <command>

Commands:
  start      Start all EK-1 services
  stop       Stop all services (data is preserved)
  restart    Restart all services
  status     Show running containers
  logs       Tail logs (optionally: ek1 logs <service>)
  update     Pull latest images and restart
  open       Open the dashboard in your browser
  uninstall  Remove EK-1 and all its data

Data directory: \${EK1_DIR}
EOF
    ;;
  *)
    echo "Unknown command: \$1"
    echo "Run 'ek1 help' for usage."
    exit 1
    ;;
esac
CLISCRIPT

  chmod +x "$cli_path"

  # Ensure the bin dir is on PATH for this session and future ones
  if [[ ":$PATH:" != *":${bin_dir}:"* ]]; then
    export PATH="${bin_dir}:$PATH"
    # Persist to shell profile
    for profile in "$HOME/.bashrc" "$HOME/.zshrc" "$HOME/.profile"; do
      if [[ -f "$profile" ]]; then
        if ! grep -q "${bin_dir}" "$profile" 2>/dev/null; then
          echo "export PATH=\"${bin_dir}:\$PATH\"" >> "$profile"
        fi
      fi
    done
    warn "${bin_dir} added to PATH. Run: source ~/.bashrc  (or restart your shell)"
  fi

  success "CLI installed at: ${cli_path}"
}

# ── Open browser ──────────────────────────────────────────────────────────────
open_browser() {
  local url="http://localhost"
  case "$PLATFORM" in
    macos)  open "$url" ;;
    wsl)    powershell.exe /c start "$url" 2>/dev/null || true ;;
    linux)  xdg-open "$url" 2>/dev/null || true ;;
  esac
}

# ── Success message ───────────────────────────────────────────────────────────
print_success() {
  echo ""
  echo -e "${GREEN}${BOLD}╔══════════════════════════════════════════╗"
  echo -e "║        EK-1 is up and running!          ║"
  echo -e "╚══════════════════════════════════════════╝${RESET}"
  echo ""
  echo -e "  ${BOLD}Dashboard:${RESET}  http://localhost"
  echo -e "  ${BOLD}API:${RESET}        http://localhost/health"
  echo -e "  ${BOLD}Data dir:${RESET}   ${EK1_DIR}"
  echo ""
  echo -e "  ${BOLD}Useful commands:${RESET}"
  echo -e "    ${CYAN}ek1 logs${RESET}      — tail all service logs"
  echo -e "    ${CYAN}ek1 stop${RESET}      — stop EK-1 (data preserved)"
  echo -e "    ${CYAN}ek1 update${RESET}    — pull latest version"
  echo -e "    ${CYAN}ek1 uninstall${RESET} — remove everything"
  echo ""
  echo -e "  ${DIM}Note: on first run Ollama downloads ~2 GB. If the dashboard"
  echo -e "  shows a loading state, run 'ek1 logs ollama-pull' to track progress.${RESET}"
  echo ""
}

# ── Main ──────────────────────────────────────────────────────────────────────
main() {
  print_banner
  detect_os
  check_docker
  create_install_dir
  download_config
  write_env
  start_services
  wait_for_healthy
  install_cli
  open_browser
  print_success
}

main "$@"
