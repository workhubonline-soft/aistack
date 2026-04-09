# Changelog

All notable changes to AIStack are documented here.
Format based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).
Versioning follows [Semantic Versioning](https://semver.org/).

---

## [Unreleased]

---

## [v2.0.0] — 2026-04-09

### Changed
- **Ollama integration**: replaced `docker exec` calls with native HTTP API client (`net/http`)
- **install.sh**: removed unnecessary system dependencies (`wget`, `git`, `jq`, `gnupg`, `lsb-release`, `apt-transport-https`), now only requires `curl` and `ca-certificates`
- **install.sh**: use `VERSION_CODENAME` from `/etc/os-release` instead of `lsb_release`
- **Docker Compose profiles**: fixed multi-profile flag passing (was comma-joined, now separate `--profile` flags)

### Added
- `aistack models benchmark` — measures tokens/sec via Ollama API
- Working Prometheus configuration (`configs/prometheus/prometheus.yml`)
- Working Grafana provisioning: datasource, dashboard provider, pre-built "AIStack Overview" dashboard
- Ollama HTTP client package (`cli/internal/ollama/client.go`)

### Fixed
- Monitoring stack (Prometheus + Grafana) now starts correctly with `--monitoring` flag
- README references to non-existent `docs/troubleshooting.md` link removed
- Version URLs updated across all files

---

## [v0.1.0] — 2025-01-01

### Added
- `aistack install` — installs Docker, nvidia-container-toolkit, prepares directories
- `aistack up / down / status / logs / update` — full service lifecycle management
- `aistack doctor` — system diagnostics (OS, CPU, RAM, disk, GPU, Docker, ports, network)
- `aistack models list` — catalog with compatibility status per model
- `aistack models recommend` — hardware-aware top-pick recommendations
- `aistack models estimate` — per-model VRAM/RAM estimate with quant comparison table
- `aistack models pull` — download models via Ollama with disk check
- `aistack backup` — tar.gz backup of volumes and configuration
- `aistack report` — diagnostic report bundle for support
- Docker Compose stack: Ollama + Open WebUI + optional Nginx/Prometheus/Grafana
- CPU and NVIDIA GPU profiles (low-vram / 8gb / 16gb / 24gb)
- Model catalog: 20+ models (Llama 3.1/3.2, Qwen 2.5, DeepSeek R1, Gemma 2, Phi 4, Mistral)
- Auto hardware detection: CPU cores, RAM, VRAM per GPU, driver version
- Auto profile selection based on minimum VRAM
- Bootstrap `install.sh` for one-command deployment
- Idempotent installs (safe to re-run)
- Non-interactive mode: `--yes --profile --no-model-download`

[Unreleased]: https://github.com/workhubonline-soft/aistack/compare/v2.0.0...HEAD
[v2.0.0]: https://github.com/workhubonline-soft/aistack/compare/v0.1.0...v2.0.0
[v0.1.0]: https://github.com/workhubonline-soft/aistack/releases/tag/v0.1.0
