# Changelog

All notable changes to AIStack are documented here.
Format based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).
Versioning follows [Semantic Versioning](https://semver.org/).

---

## [Unreleased]

### Added
- Initial project structure

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

[Unreleased]: https://github.com/workhubonline-soft/aistack/compare/v0.1.0...HEAD
[v0.1.0]: https://github.com/workhubonline-soft/aistack/releases/tag/v0.1.0
