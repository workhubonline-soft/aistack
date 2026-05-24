# Changelog

All notable changes to AIStack are documented here.
Format based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).
Versioning follows [Semantic Versioning](https://semver.org/).

---

## [Unreleased]

---

## [v2.1.1] — 2026-05-24

### Fixed
- Nginx container failed to start: relative volume paths (`./configs/...`) were resolved relative to the compose file directory, not the project root. Changed to `../configs/...`.
- Nginx upstream `openwebui:8080` was resolved at startup, crashing nginx if OpenWebUI restarted. Now uses Docker's embedded DNS at request time.
- `aistack` CLI required `sudo` because `/opt/aistack` was root-only. Installer now `chgrp docker` on config dirs and sets group-read permissions, making the CLI usable without `sudo` for users in the `docker` group.
- `saveState()` now writes state file with `0o660` (group-writable) so non-root invocations work.

---

## [v2.1.0] — 2026-04-11

### Security
- Default WebUI binding changed from `0.0.0.0` to `127.0.0.1` (opt-in for external access)
- Removed `chmod 644 .env` from install.sh — secrets now stay at `0600`
- Grafana password auto-generated randomly instead of hardcoded `aistack`
- OLLAMA_HOST environment variable validated to prevent URL injection

### Changed
- **Model catalog v2.0**: replaced Qwen 2.5, Llama 3.x, Gemma 2 with Qwen 3, Llama 4, Gemma 3, Devstral
- Ollama HTTP client: consistent timeouts (30s default, no timeout for streaming, 5min for generation)
- Docker images pinned: ollama:0.9, open-webui:latest, prometheus:v3.3, grafana:11.6
- Resource limits added for OpenWebUI (2G), Prometheus (512M), Grafana (256M)
- Backup uses streaming pipe instead of loading entire volume into memory

### Added
- `aistack doctor --json` — machine-readable JSON output for diagnostics
- DeepSeek R1 70B, Devstral 24B, Qwen 3 30B-A3B (MoE) to model catalog
- Context length validation (128-131072) for `models estimate`

### Fixed
- Doctor command refactored: `--json` flag now works, function split into smaller units

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

[Unreleased]: https://github.com/workhubonline-soft/aistack/compare/v2.1.0...HEAD
[v2.1.0]: https://github.com/workhubonline-soft/aistack/compare/v2.0.0...v2.1.0
[v2.0.0]: https://github.com/workhubonline-soft/aistack/compare/v0.1.0...v2.0.0
[v0.1.0]: https://github.com/workhubonline-soft/aistack/releases/tag/v0.1.0
