# AIStack — Self-Hosted AI Server Installer

> One command to turn a clean Ubuntu server into a fully operational AI stack.

[![Ubuntu](https://img.shields.io/badge/Ubuntu-22.04%20%7C%2024.04-orange)](https://ubuntu.com)
[![Docker](https://img.shields.io/badge/Docker-required-blue)](https://docker.com)
[![License](https://img.shields.io/badge/License-MIT-green)](LICENSE)

## What You Get

| Service | URL | Description |
|---------|-----|-------------|
| **Open WebUI** | `http://your-server:3000` | ChatGPT-like interface |
| **Ollama** | `http://localhost:11434` | LLM inference engine |
| **Nginx** *(optional)* | `http://your-server:80` | Reverse proxy |
| **Prometheus + Grafana** *(optional)* | `:9090` / `:3001` | Monitoring |

## Quick Start

### One-liner install
```bash
curl -sSL https://install.example.com/aistack | sudo bash
```

### From source
```bash
git clone https://github.com/workhubonline-soft/aistack
cd aistack
sudo ./install.sh
```

### Non-interactive (CI/automation)
```bash
sudo ./install.sh --yes --profile cpu
sudo ./install.sh --yes --profile nvidia-24gb --no-model-download
```

## CLI Reference

```
aistack <command> [flags]

Commands:
  install     Install dependencies and prepare AIStack
  up          Start all services
  down        Stop all services
  status      Show service status
  logs        View service logs (aistack logs ollama -f)
  update      Pull latest images and restart
  backup      Backup volumes and configuration
  report      Generate diagnostic report (for support)
  doctor      Run system diagnostics
  models      Model catalog and advisor
  version     Show version

Global flags:
  --yes, -y       Auto-confirm all prompts
  --profile       Override auto-detected profile
  --verbose, -v   Verbose output
```

## Model Advisor

```bash
# See all available models with compatibility status
aistack models list

# Get recommendations for YOUR hardware
aistack models recommend

# Estimate resources for a specific model
aistack models estimate --model qwen2.5:14b --ctx 8192
aistack models estimate --model llama3.1:70b --ctx 4096 --quant q4_K_M

# Download a model
aistack models pull qwen2.5:7b
aistack models pull llama3.2:3b
```

### Example Recommendations by Hardware

**CPU only (16GB RAM)**
- ✓ Llama 3.2 3B (q4_K_M) — best general chat
- ✓ Qwen 2.5 3B (q4_K_M) — best multilingual
- ✓ Phi 3.5 Mini (q4_K_M) — best coding

**NVIDIA 8GB VRAM**
- ✓ Llama 3.1 8B (q4_K_M) — 5.4 GB VRAM
- ✓ Qwen 2.5 7B (q4_K_M) — 4.8 GB VRAM
- ✓ Qwen 2.5 Coder 7B — best coding
- ⚠ Mistral 7B (q5_K_M) — tight fit

**NVIDIA 24GB VRAM**
- ✓ Qwen 2.5 14B (q6_K) — best quality/speed
- ✓ DeepSeek R1 14B — best reasoning
- ✓ Gemma 2 27B (q4_K_M) — near-70B quality
- ✓ Llama 3.1 70B (q2_K) — maximum quality (tight)

## Supported Platforms

| OS | Arch | CPU | NVIDIA GPU |
|----|------|-----|------------|
| Ubuntu 22.04 LTS | x86_64 | ✓ | ✓ |
| Ubuntu 24.04 LTS | x86_64 | ✓ | ✓ |

## Profiles

AIStack auto-detects your hardware and selects a profile:

| Profile | Trigger | Ollama config |
|---------|---------|---------------|
| `cpu` | No GPU detected | CPU threads, RAM limit |
| `nvidia-low-vram` | GPU < 8GB VRAM | GPU passthrough |
| `nvidia-8gb` | 8–15GB VRAM | GPU + flash attention |
| `nvidia-16gb` | 16–23GB VRAM | GPU + larger context |
| `nvidia-24gb` | 24GB+ VRAM | GPU + KV cache quant |

Override: `aistack up --profile nvidia-24gb`

## Doctor

Run pre-install or post-install diagnostics:

```bash
aistack doctor
```

Checks:
- ✓ OS version and architecture
- ✓ CPU, RAM, disk space
- ✓ Docker Engine + Compose plugin
- ✓ Docker permissions
- ✓ NVIDIA GPU + driver (if present)
- ✓ nvidia-container-toolkit
- ✓ Port availability (3000, 11434, 8080, 80, 443)
- ✓ DNS resolution + Docker Hub connectivity
- ✓ Ollama registry connectivity

## Backup & Restore

```bash
# Backup (configs + all volumes)
aistack backup
aistack backup --output /mnt/backups/aistack-$(date +%Y%m%d).tar.gz

# Restore (manual)
tar -xzf aistack-backup-20250101.tar.gz
docker run --rm -v aistack-openwebui-data:/data -v $(pwd):/restore \
  alpine tar -xzf /restore/volumes/openwebui-data.tar.gz -C /
```

## Directory Structure

```
/opt/aistack/
  compose/          Docker Compose files
  configs/          Nginx, Prometheus, Grafana configs
  models/           Model catalog (catalog.yaml)
  .env              Your configuration (auto-generated)

/var/lib/aistack/
  state.json        Installer state
  backups/          Backup archives

/var/log/aistack/   Logs
/usr/local/bin/aistack  CLI binary
```

## Configuration

Edit `/opt/aistack/.env` to customize:

```bash
# Example: enable larger context window
OLLAMA_KV_CACHE_TYPE=q4_0    # Save VRAM on context cache
OLLAMA_MAX_LOADED_MODELS=2   # Keep 2 models in VRAM
WEBUI_NAME=My AI Server      # Custom name in UI
```

Then restart: `aistack down && aistack up`

## Optional Services

```bash
# Start with Nginx reverse proxy
aistack up --nginx

# Start with Prometheus + Grafana monitoring
aistack up --monitoring

# Both
aistack up --nginx --monitoring
```

## Troubleshooting

See [docs/troubleshooting.md](docs/troubleshooting.md)

```bash
# Generate support report
aistack report
# → /tmp/aistack-report-20250101-120000.tar.gz
```

## Contributing

PRs welcome! See [CONTRIBUTING.md](CONTRIBUTING.md).

Key areas:
- `models/catalog.yaml` — add new models
- `cli/internal/models/catalog.go` — improve VRAM estimates
- `cli/cmd/` — new CLI commands

## License

MIT License — see [LICENSE](LICENSE)
