# Contributing to AIStack

## Development Setup

```bash
git clone https://github.com/workhubonline-soft/aistack
cd aistack

# Install Go 1.22+
# https://go.dev/dl/

# Download dependencies
make deps

# Build and run locally
make dev ARGS="doctor"
make dev ARGS="models list"
make dev ARGS="models recommend"
```

## Project Structure

```
aistack/
  cli/
    main.go                      # Entry point
    cmd/
      root.go                    # Root cobra command + flags
      doctor.go                  # aistack doctor
      models.go                  # aistack models *
      commands.go                # install, up, down, status, logs, backup, report
    internal/
      hardware/detect.go         # CPU/RAM/GPU detection
      models/catalog.go          # Model catalog + VRAM estimation
      models/catalog_test.go     # Tests
      hardware/detect_test.go    # Tests
  models/catalog.yaml            # Model database (edit to add models)
  compose/                       # Docker Compose files
  configs/                       # Nginx, env template
  .github/workflows/             # CI/CD pipelines
```

## Adding a Model to the Catalog

Edit `models/catalog.yaml`:

```yaml
- id: mymodel:7b
  name: "MyModel 7B"
  family: mymodel
  size: "7B"
  params_b: 7.0
  engine: ollama
  ollama_tag: "mymodel:7b"
  description: "Description of the model and its strengths."
  tags: [chat, coding]           # chat | coding | fast | long-context | multilingual | embedding
  quants: [q4_K_M, q5_K_M, q8_0]
  default_quant: q4_K_M
  max_ctx: 32768
  default_ctx: 4096
```

Then validate: `make validate-catalog`

## Running Tests

```bash
make test               # All tests
make test-coverage      # With HTML coverage report
make test-short         # Skip integration tests
```

## Linting

```bash
make lint    # golangci-lint (installs automatically)
make fmt     # gofmt + goimports
make vet     # go vet
```

## Release Process

Releases are fully automated via GitHub Actions.

### Steps to release:

1. Update `CHANGELOG.md` — add section for new version
2. Bump version tag:
   ```bash
   git tag v0.2.0
   git push origin v0.2.0
   ```
3. GitHub Actions automatically:
   - Runs full CI (lint + test)
   - Builds binaries for linux/amd64 and linux/arm64
   - Creates GitHub Release with binaries + checksums
   - Builds and pushes Docker image to GHCR
   - Updates `install.sh` with new version pointer

### Version naming:
- `v0.1.0` — stable release
- `v0.2.0-rc1` — release candidate (marked as pre-release)
- `v0.2.0-beta1` — beta (marked as pre-release)

### Manual release (if needed):
```bash
make release            # Build all + checksums
# Then create GitHub release manually and upload dist/ files
```

## Commit Style

```
feat: add GPU memory fraction configuration
fix: correct VRAM calculation for MoE models
docs: update troubleshooting for port conflicts
chore: update dependencies
test: add VRAM estimation edge cases
```

## Pull Request Checklist

- [ ] `make test` passes
- [ ] `make lint` passes
- [ ] New features have tests
- [ ] New models added to `catalog.yaml` with all required fields
- [ ] `CHANGELOG.md` updated under `[Unreleased]`
