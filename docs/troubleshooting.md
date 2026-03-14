# AIStack Troubleshooting Guide

## Quick Diagnostics

```bash
aistack doctor          # Full system check
aistack status          # Service status
aistack logs            # All service logs
aistack logs ollama -f  # Follow Ollama logs
aistack report          # Generate support bundle
```

---

## Common Issues

### Docker not found or not running

```bash
# Check Docker status
sudo systemctl status docker

# Start Docker
sudo systemctl start docker
sudo systemctl enable docker

# Re-run installer
sudo aistack install --yes
```

### Permission denied: docker socket

```bash
sudo usermod -aG docker $USER
newgrp docker
# Or log out and back in
```

### Port already in use

```bash
# Find what's using the port
sudo lsof -i :3000
sudo lsof -i :11434

# Option 1: Stop the conflicting service
sudo systemctl stop conflicting-service

# Option 2: Change AIStack ports in /opt/aistack/.env
WEBUI_PORT=3001
OLLAMA_PORT=11435
```

### GPU not detected / CUDA errors

```bash
# Check GPU
nvidia-smi

# Test GPU in Docker
docker run --rm --gpus all ubuntu:22.04 nvidia-smi

# Reinstall nvidia-container-toolkit
distribution=$(. /etc/os-release;echo $ID$VERSION_ID)
curl -fsSL https://nvidia.github.io/libnvidia-container/gpgkey | sudo gpg --dearmor -o /usr/share/keyrings/nvidia-container-toolkit-keyring.gpg
sudo apt-get install -y nvidia-container-toolkit
sudo nvidia-ctk runtime configure --runtime=docker
sudo systemctl restart docker
```

### Open WebUI can't connect to Ollama

```bash
# Check Ollama is running
aistack logs ollama

# Check internal DNS
docker exec aistack-openwebui curl http://ollama:11434/api/tags

# Verify env variable
grep OLLAMA_BASE_URL /opt/aistack/.env
# Should be: OLLAMA_BASE_URL=http://ollama:11434
```

### Model download fails / connection timeout

```bash
# Test Ollama registry connectivity
curl -v https://registry.ollama.ai

# Pull directly inside container
docker exec -it aistack-ollama ollama pull llama3.2:3b

# Check disk space
df -h /
aistack doctor  # Shows disk status
```

### Out of VRAM — model won't load

```bash
# Check VRAM usage
nvidia-smi

# Reduce context window in .env
# Edit /opt/aistack/.env:
OLLAMA_KV_CACHE_TYPE=q4_0    # Reduces KV cache VRAM

# Then restart
aistack down && aistack up

# Or use a smaller quantization
aistack models estimate --model qwen2.5:14b --ctx 4096
aistack models pull qwen2.5:14b  # Will suggest q4_K_S for tight VRAM
```

### Slow inference on CPU

```bash
# Check thread configuration
grep OLLAMA_NUM_THREADS /opt/aistack/.env

# Set explicitly (use physical core count, not hyperthreading)
# Edit /opt/aistack/.env:
OLLAMA_NUM_THREADS=8   # Example: 8 physical cores

# Use a smaller/faster model
aistack models recommend   # Shows "Best Fast/Small" category
aistack models pull llama3.2:3b
```

### aistack up fails with compose errors

```bash
# Check compose config validity
docker compose -f /opt/aistack/compose/docker-compose.yml config

# Check .env file
cat /opt/aistack/.env | grep -v "^#" | grep -v "^$"

# Reset to fresh .env
cp /opt/aistack/configs/env.example /opt/aistack/.env
# Then edit WEBUI_SECRET_KEY with a random value
```

### Open WebUI shows login but won't accept password

```bash
# First run: create admin account through the UI (first user = admin)
# If locked out, reset the database:
docker exec aistack-openwebui sqlite3 /app/backend/data/webui.db \
  "DELETE FROM user WHERE role='admin';"
# Then refresh and create new admin via signup
```

---

## Logs Reference

| What to check | Command |
|---------------|---------|
| All services | `aistack logs` |
| Ollama only | `aistack logs ollama` |
| Open WebUI only | `aistack logs openwebui` |
| Follow in real-time | `aistack logs -f` |
| Last N lines | `aistack logs --tail 200` |
| System install log | `cat /var/log/aistack/install.log` |

---

## Reset / Clean Install

```bash
# Stop everything
aistack down

# ⚠ DESTRUCTIVE: Remove all data and start fresh
aistack down --volumes
sudo rm -rf /opt/aistack /var/lib/aistack /var/log/aistack
sudo rm -f /usr/local/bin/aistack

# Re-install
curl -sSL https://raw.githubusercontent.com/workhubonline-soft/aistack/v0.1.1/install.sh | sudo bash
```

---

## Getting Support

1. Run `aistack doctor` and note any failures
2. Run `aistack report` to generate a support bundle
3. Open an issue on GitHub and attach the report archive

```bash
aistack report
# Output: /tmp/aistack-report-YYYYMMDD-HHMMSS.tar.gz
```
