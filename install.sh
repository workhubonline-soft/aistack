#!/usr/bin/env bash
# ==============================================================================
# AIStack Installer — Bootstrap Script
# Supports: Ubuntu 22.04 LTS, 24.04 LTS (x86_64)
# Usage:
#   curl -sSL https://install.example.com/aistack | bash
#   ./install.sh [--yes] [--profile cpu|gpu] [--no-model-download]
# ==============================================================================

set -euo pipefail

# ── Constants ─────────────────────────────────────────────────────────────────
AISTACK_VERSION="${AISTACK_VERSION:-0.1.0}"
AISTACK_DIR="${AISTACK_DIR:-/opt/aistack}"
AISTACK_BIN="/usr/local/bin/aistack"
AISTACK_LOG_DIR="/var/log/aistack"
AISTACK_STATE_DIR="/var/lib/aistack"
GITHUB_REPO="your-org/aistack"
BINARY_URL="https://github.com/${GITHUB_REPO}/releases/download/${AISTACK_VERSION}/aistack-linux-amd64"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

# ── Flags ─────────────────────────────────────────────────────────────────────
AUTO_YES=false
PROFILE=""
NO_MODEL_DOWNLOAD=false

while [[ $# -gt 0 ]]; do
  case $1 in
    --yes)               AUTO_YES=true ;;
    --profile=*)         PROFILE="${1#*=}" ;;
    --profile)           shift; PROFILE="${1:-}" ;;
    --no-model-download) NO_MODEL_DOWNLOAD=true ;;
  esac
  shift
done

# ── Helpers ───────────────────────────────────────────────────────────────────
log()     { printf "${GREEN}[✓]${NC} %s\n" "$*"; }
info()    { printf "${BLUE}[→]${NC} %s\n" "$*"; }
warn()    { printf "${YELLOW}[!]${NC} %s\n" "$*"; }
error()   { printf "${RED}[✗]${NC} %s\n" "$*" >&2; }
header()  { printf "\n${BOLD}${CYAN}%s${NC}\n" "$*"; printf '%s\n' "────────────────────────────────────────"; }
die()     { error "$*"; exit 1; }

confirm() {
  if $AUTO_YES; then return 0; fi
  read -rp "$(printf "${YELLOW}[?]${NC} %s [y/N]: " "$*")" answer
  [[ "$answer" =~ ^[Yy]$ ]]
}

require_root() {
  if [[ $EUID -ne 0 ]]; then
    die "This script must be run as root. Try: sudo $0 $*"
  fi
}

# ── Banner ────────────────────────────────────────────────────────────────────
print_banner() {
  printf "${CYAN}"
  cat << 'EOF'
   █████╗ ██╗    ███████╗████████╗ █████╗  ██████╗██╗  ██╗
  ██╔══██╗██║    ██╔════╝╚══██╔══╝██╔══██╗██╔════╝██║ ██╔╝
  ███████║██║    ███████╗   ██║   ███████║██║     █████╔╝
  ██╔══██║██║    ╚════██║   ██║   ██╔══██║██║     ██╔═██╗
  ██║  ██║██║    ███████║   ██║   ██║  ██║╚██████╗██║  ██╗
  ╚═╝  ╚═╝╚═╝    ╚══════╝   ╚═╝   ╚═╝  ╚═╝ ╚═════╝╚═╝  ╚═╝
EOF
  printf "${NC}\n"
  printf "  ${BOLD}Self-hosted AI Stack Installer${NC} — v%s\n" "${AISTACK_VERSION}"
  printf "  Ubuntu 22.04 / 24.04 · CPU & NVIDIA GPU\n\n"
}

# ── OS Check ──────────────────────────────────────────────────────────────────
check_os() {
  header "Checking Operating System"

  if [[ ! -f /etc/os-release ]]; then
    die "Cannot detect OS. /etc/os-release not found."
  fi

  # shellcheck source=/dev/null
  . /etc/os-release
  info "Detected: ${PRETTY_NAME}"

  if [[ "$ID" != "ubuntu" ]]; then
    die "Only Ubuntu is supported (detected: $ID)"
  fi

  case "$VERSION_ID" in
    22.04|24.04) log "Ubuntu ${VERSION_ID} — supported" ;;
    *)
      warn "Ubuntu ${VERSION_ID} is not officially supported."
      confirm "Continue anyway?" || die "Aborted by user."
      ;;
  esac

  # Architecture check
  ARCH=$(uname -m)
  if [[ "$ARCH" != "x86_64" ]]; then
    die "Only x86_64 is supported (detected: $ARCH)"
  fi
  log "Architecture: ${ARCH}"
}

# ── Dependencies ──────────────────────────────────────────────────────────────
install_base_deps() {
  header "Installing Base Dependencies"

  local deps=(curl wget git jq ca-certificates gnupg lsb-release apt-transport-https)
  local missing=()

  for dep in "${deps[@]}"; do
    if ! command -v "$dep" &>/dev/null && ! dpkg -l "$dep" &>/dev/null; then
      missing+=("$dep")
    fi
  done

  if [[ ${#missing[@]} -eq 0 ]]; then
    log "All base dependencies already installed"
    return 0
  fi

  info "Installing: ${missing[*]}"
  apt-get update -qq
  apt-get install -y -qq "${missing[@]}"
  log "Base dependencies installed"
}

# ── Docker ────────────────────────────────────────────────────────────────────
install_docker() {
  header "Docker Engine"

  if command -v docker &>/dev/null; then
    local docker_ver
    docker_ver=$(docker version --format '{{.Server.Version}}' 2>/dev/null || echo "unknown")
    log "Docker already installed: ${docker_ver}"

    # Check compose plugin
    if docker compose version &>/dev/null; then
      log "Docker Compose plugin: available"
    else
      warn "Docker Compose plugin not found — installing"
      _install_compose_plugin
    fi
    return 0
  fi

  info "Docker not found — installing via official script"
  confirm "Install Docker Engine?" || die "Docker is required. Aborted."

  # Install using official Docker apt repo
  install -m 0755 -d /etc/apt/keyrings
  curl -fsSL https://download.docker.com/linux/ubuntu/gpg | \
    gpg --dearmor -o /etc/apt/keyrings/docker.gpg
  chmod a+r /etc/apt/keyrings/docker.gpg

  echo "deb [arch=$(dpkg --print-architecture) \
    signed-by=/etc/apt/keyrings/docker.gpg] \
    https://download.docker.com/linux/ubuntu \
    $(lsb_release -cs) stable" | \
    tee /etc/apt/sources.list.d/docker.list > /dev/null

  apt-get update -qq
  apt-get install -y docker-ce docker-ce-cli containerd.io \
    docker-buildx-plugin docker-compose-plugin

  systemctl enable --now docker
  log "Docker installed and started"
}

_install_compose_plugin() {
  apt-get update -qq
  apt-get install -y docker-compose-plugin
  log "Docker Compose plugin installed"
}

add_user_to_docker_group() {
  local target_user="${SUDO_USER:-$USER}"
  if [[ -n "$target_user" && "$target_user" != "root" ]]; then
    if ! groups "$target_user" | grep -q docker; then
      usermod -aG docker "$target_user"
      warn "User '$target_user' added to docker group. Re-login required for non-sudo usage."
    fi
  fi
}

# ── NVIDIA ────────────────────────────────────────────────────────────────────
detect_nvidia() {
  GPU_PRESENT=false
  GPU_VRAM=0
  GPU_MODEL=""

  if command -v nvidia-smi &>/dev/null; then
    if nvidia-smi &>/dev/null 2>&1; then
      GPU_PRESENT=true
      GPU_MODEL=$(nvidia-smi --query-gpu=name --format=csv,noheader | head -1)
      GPU_VRAM=$(nvidia-smi --query-gpu=memory.total --format=csv,noheader | \
        head -1 | tr -d ' MiB')
      log "NVIDIA GPU detected: ${GPU_MODEL} (${GPU_VRAM} MiB VRAM)"
    fi
  fi

  if $GPU_PRESENT; then
    info "GPU VRAM: ${GPU_VRAM} MiB"
    if [[ "$GPU_VRAM" -lt 8192 ]]; then
      GPU_TIER="low"
    elif [[ "$GPU_VRAM" -lt 16384 ]]; then
      GPU_TIER="8gb"
    elif [[ "$GPU_VRAM" -lt 24576 ]]; then
      GPU_TIER="16gb"
    else
      GPU_TIER="24gb"
    fi
    log "GPU tier: ${GPU_TIER}"
  fi
}

install_nvidia_container_toolkit() {
  header "NVIDIA Container Toolkit"

  if docker run --rm --gpus all ubuntu:22.04 nvidia-smi &>/dev/null 2>&1; then
    log "nvidia-container-toolkit already working"
    return 0
  fi

  info "Installing nvidia-container-toolkit"
  confirm "Install NVIDIA Container Toolkit?" || { warn "Skipping GPU support"; return 0; }

  curl -fsSL https://nvidia.github.io/libnvidia-container/gpgkey | \
    gpg --dearmor -o /usr/share/keyrings/nvidia-container-toolkit-keyring.gpg

  curl -s -L https://nvidia.github.io/libnvidia-container/stable/deb/nvidia-container-toolkit.list | \
    sed 's#deb https://#deb [signed-by=/usr/share/keyrings/nvidia-container-toolkit-keyring.gpg] https://#g' | \
    tee /etc/apt/sources.list.d/nvidia-container-toolkit.list

  apt-get update -qq
  apt-get install -y nvidia-container-toolkit
  nvidia-ctk runtime configure --runtime=docker
  systemctl restart docker

  # Verify
  if docker run --rm --gpus all ubuntu:22.04 nvidia-smi &>/dev/null 2>&1; then
    log "GPU container test: PASSED"
  else
    warn "GPU container test failed. Check your NVIDIA driver installation."
  fi
}

# ── Profile Selection ─────────────────────────────────────────────────────────
select_profile() {
  header "Selecting Installation Profile"

  if [[ -n "$PROFILE" ]]; then
    info "Profile override: ${PROFILE}"
    return 0
  fi

  detect_nvidia

  if $GPU_PRESENT; then
    case "$GPU_TIER" in
      low)   PROFILE="nvidia-low-vram"  ;;
      8gb)   PROFILE="nvidia-8gb"       ;;
      16gb)  PROFILE="nvidia-16gb"      ;;
      24gb)  PROFILE="nvidia-24gb"      ;;
    esac
    log "Auto-selected GPU profile: ${PROFILE}"
  else
    PROFILE="cpu"
    log "No GPU detected — using CPU profile"
  fi
}

# ── AIStack CLI Binary ────────────────────────────────────────────────────────
install_aistack_binary() {
  header "Installing AIStack CLI"

  if [[ -f "$AISTACK_BIN" ]]; then
    local current_ver
    current_ver=$("$AISTACK_BIN" version 2>/dev/null || echo "unknown")
    if [[ "$current_ver" == "$AISTACK_VERSION" ]]; then
      log "AIStack CLI already up-to-date: ${current_ver}"
      return 0
    fi
    info "Updating AIStack CLI: ${current_ver} → ${AISTACK_VERSION}"
  fi

  info "Downloading AIStack CLI binary..."
  curl -fsSL "$BINARY_URL" -o /tmp/aistack-new
  chmod +x /tmp/aistack-new
  mv /tmp/aistack-new "$AISTACK_BIN"
  log "AIStack CLI installed: $AISTACK_BIN"
}

# ── Deploy AIStack files ──────────────────────────────────────────────────────
deploy_aistack() {
  header "Deploying AIStack"

  # Create directories
  mkdir -p "$AISTACK_DIR"
  mkdir -p "$AISTACK_LOG_DIR"
  mkdir -p "$AISTACK_STATE_DIR"

  # If running from git clone, copy compose/configs
  SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
  if [[ -d "${SCRIPT_DIR}/compose" ]]; then
    cp -r "${SCRIPT_DIR}/compose" "$AISTACK_DIR/"
    cp -r "${SCRIPT_DIR}/configs" "$AISTACK_DIR/"
    cp -r "${SCRIPT_DIR}/models" "$AISTACK_DIR/"
    log "Files deployed from local clone"
  else
    # Running via curl — download files from GitHub
    info "Downloading AIStack configuration files..."
    local tmp_dir
    tmp_dir=$(mktemp -d)
    curl -sSL "https://api.github.com/repos/${GITHUB_REPO}/tarball/v${AISTACK_VERSION}"       -o "${tmp_dir}/aistack.tar.gz"
    tar -xzf "${tmp_dir}/aistack.tar.gz" -C "${tmp_dir}" --strip-components=1
    cp -r "${tmp_dir}/compose" "$AISTACK_DIR/"
    cp -r "${tmp_dir}/configs" "$AISTACK_DIR/"
    cp -r "${tmp_dir}/models" "$AISTACK_DIR/"
    rm -rf "${tmp_dir}"
    log "Files downloaded from GitHub"
  fi

  log "AIStack directory: ${AISTACK_DIR}"
}

# ── Run ───────────────────────────────────────────────────────────────────────
run_aistack() {
  header "Starting AIStack"

  if [[ ! -f "$AISTACK_BIN" ]]; then
    warn "AIStack binary not found — running from source"
    # In development: use go run or pre-built binary from repo
    return 0
  fi

  local flags="--profile ${PROFILE}"
  if $AUTO_YES; then flags="$flags --yes"; fi
  if $NO_MODEL_DOWNLOAD; then flags="$flags --no-model-download"; fi

  "$AISTACK_BIN" install $flags
  "$AISTACK_BIN" up
}

# ── Summary ───────────────────────────────────────────────────────────────────
print_summary() {
  header "Installation Complete"
  printf "\n"
  printf "  ${GREEN}${BOLD}AIStack is running!${NC}\n"
  printf "\n"
  printf "  ${BOLD}Services:${NC}\n"
  printf "    • Open WebUI:  ${CYAN}http://localhost:3000${NC}\n"
  printf "    • Ollama API:  ${CYAN}http://localhost:11434${NC}\n"
  printf "\n"
  printf "  ${BOLD}Useful commands:${NC}\n"
  printf "    aistack status          — check services\n"
  printf "    aistack doctor          — run diagnostics\n"
  printf "    aistack models list     — available models\n"
  printf "    aistack models recommend — recommendations for your hardware\n"
  printf "    aistack logs            — view logs\n"
  printf "\n"
  printf "  ${BOLD}Logs:${NC} %s/\n" "${AISTACK_LOG_DIR}"
  printf "\n"
}

# ── Main ──────────────────────────────────────────────────────────────────────
main() {
  print_banner
  require_root

  check_os
  install_base_deps
  install_docker
  add_user_to_docker_group
  select_profile

  if [[ "$PROFILE" != "cpu" ]]; then
    install_nvidia_container_toolkit
  fi

  deploy_aistack
  run_aistack
  print_summary
}

main "$@"
