package models

import (
	"fmt"
	"math"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/workhubonline-soft/aistack/internal/hardware"
)

// ── Compatibility levels ───────────────────────────────────────────────────────

type CompatLevel int

const (
	CompatOK   CompatLevel = iota // Fits comfortably
	CompatWarn                    // Tight fit, may be slow
	CompatFail                    // Will not run
)

type CompatResult struct {
	Level      CompatLevel
	Reason     string
	Suggestion string
}

// ── Model data structures ──────────────────────────────────────────────────────

type Catalog struct {
	Version string  `yaml:"version"`
	Models  []Model `yaml:"models"`
}

type Model struct {
	ID              string   `yaml:"id"`
	Name            string   `yaml:"name"`
	Family          string   `yaml:"family"`
	SizeLabel       string   `yaml:"size"` // "7B", "14B", etc.
	ParamsB         float64  `yaml:"params_b"`
	Engine          string   `yaml:"engine"` // ollama | vllm
	OllamaTag       string   `yaml:"ollama_tag"`
	Description     string   `yaml:"description"`
	Tags            []string `yaml:"tags"` // chat, coding, fast, long-context, multilingual
	AvailableQuants []string `yaml:"quants"`
	DefaultQuant    string   `yaml:"default_quant"`
	// Resource overrides (optional, auto-calculated if missing)
	VRAMOverrideMiB int `yaml:"vram_mib,omitempty"`
	RAMOverrideMiB  int `yaml:"ram_mib,omitempty"`
	// Context
	MaxCtx     int `yaml:"max_ctx"`
	DefaultCtx int `yaml:"default_ctx"`
}

// ── Catalog loading ────────────────────────────────────────────────────────────

func LoadCatalog(path string) (*Catalog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading catalog %s: %w", path, err)
	}
	var catalog Catalog
	if err := yaml.Unmarshal(data, &catalog); err != nil {
		return nil, fmt.Errorf("parsing catalog: %w", err)
	}
	return &catalog, nil
}

func (c *Catalog) FindModel(id string) *Model {
	id = strings.ToLower(id)
	for i := range c.Models {
		m := &c.Models[i]
		if strings.ToLower(m.ID) == id ||
			strings.ToLower(m.OllamaTag) == id ||
			strings.Contains(strings.ToLower(m.Name), id) {
			return m
		}
	}
	return nil
}

func (c *Catalog) GetRecommendations(hw *hardware.Info, tag string, limit int) []Model {
	candidates := make([]Model, 0, len(c.Models))
	for _, m := range c.Models {
		if !m.HasTag(tag) {
			continue
		}
		compat := m.CheckCompatibility(hw)
		if compat.Level == CompatFail {
			continue
		}
		candidates = append(candidates, m)
	}

	// Sort: OK before Warn, then by params desc (larger = better quality)
	for i := 0; i < len(candidates)-1; i++ {
		for j := i + 1; j < len(candidates); j++ {
			ci := candidates[i].CheckCompatibility(hw)
			cj := candidates[j].CheckCompatibility(hw)
			if ci.Level > cj.Level ||
				(ci.Level == cj.Level && candidates[i].ParamsB < candidates[j].ParamsB) {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			}
		}
	}

	if limit > 0 && len(candidates) > limit {
		return candidates[:limit]
	}
	return candidates
}

// ── Model methods ──────────────────────────────────────────────────────────────

func (m *Model) OllamaID() string {
	if m.OllamaTag != "" {
		return m.OllamaTag
	}
	return m.ID
}

func (m *Model) HasTag(tag string) bool {
	for _, t := range m.Tags {
		if strings.EqualFold(t, tag) {
			return true
		}
	}
	return false
}

// EstimateVRAM returns estimated VRAM in MiB for model weights
// Formula: params_b * bits_per_param / 8 * 1024 + overhead
func (m *Model) EstimateVRAM(quant string, ctxLen int) int {
	if m.VRAMOverrideMiB > 0 {
		return m.VRAMOverrideMiB
	}

	bitsPerParam := quantBits(quant)
	// Model weights in MiB
	weightsMiB := int(m.ParamsB * float64(bitsPerParam) / 8 * 1024)
	// Add KV cache
	kvMiB := m.EstimateKVCache(ctxLen)
	// Add ~5% overhead for CUDA context, buffers, etc.
	overhead := int(float64(weightsMiB) * 0.05)

	return weightsMiB + kvMiB + overhead
}

// EstimateKVCache returns KV cache size in MiB
// Simplified: 2 * num_layers * ctx * hidden_dim * 2bytes / 1024^2
// Using rough approximation based on params
func (m *Model) EstimateKVCache(ctxLen int) int {
	// Approximate: ~0.5 MiB per 1B params per 1K context at fp16
	kvPerBPerKCtx := 0.5
	return int(m.ParamsB * kvPerBPerKCtx * float64(ctxLen) / 1000)
}

// EstimateRAM returns system RAM required in MiB (for CPU offloading or CPU-only)
func (m *Model) EstimateRAM(quant string) int {
	if m.RAMOverrideMiB > 0 {
		return m.RAMOverrideMiB
	}
	bitsPerParam := quantBits(quant)
	// RAM = weights + 20% buffer for runtime
	weightsMiB := int(m.ParamsB * float64(bitsPerParam) / 8 * 1024)
	return int(float64(weightsMiB) * 1.2)
}

// EstimateDiskGB returns approximate disk space needed for download
func (m *Model) EstimateDiskGB(quant string) int {
	bitsPerParam := quantBits(quant)
	gb := m.ParamsB * float64(bitsPerParam) / 8
	return int(math.Ceil(gb))
}

// BestQuantForHardware selects the best quantization for given hardware
func (m *Model) BestQuantForHardware(hw *hardware.Info) string {
	if len(m.AvailableQuants) == 0 {
		return m.DefaultQuant
	}

	if hw == nil || !hw.HasGPU {
		// CPU mode: prefer Q4 for balance of speed and quality
		for _, q := range []string{"q4_K_M", "q4_0", "q5_K_M", "q4_K_S"} {
			if m.hasQuant(q) {
				return q
			}
		}
		return m.DefaultQuant
	}

	vramMiB := hw.MinGPUVRAM()
	ctxLen := m.DefaultCtx
	if ctxLen == 0 {
		ctxLen = 4096
	}

	// Try quants from best to worst, pick the best that fits
	priority := []string{"q8_0", "q6_K", "q5_K_M", "q4_K_M", "q4_K_S", "q4_0", "q3_K_M", "q2_K"}
	for _, q := range priority {
		if !m.hasQuant(q) {
			continue
		}
		needed := m.EstimateVRAM(q, ctxLen)
		if needed <= int(float64(vramMiB)*0.9) { // 10% safety margin
			return q
		}
	}

	// Nothing fits — return smallest
	if len(m.AvailableQuants) > 0 {
		return m.AvailableQuants[len(m.AvailableQuants)-1]
	}
	return m.DefaultQuant
}

func (m *Model) hasQuant(q string) bool {
	for _, aq := range m.AvailableQuants {
		if strings.EqualFold(aq, q) {
			return true
		}
	}
	return false
}

// CheckCompatibility checks if model runs on hardware with default quant
func (m *Model) CheckCompatibility(hw *hardware.Info) CompatResult {
	if hw == nil {
		return CompatResult{Level: CompatOK, Reason: "hardware info unavailable"}
	}
	quant := m.BestQuantForHardware(hw)
	ctxLen := m.DefaultCtx
	if ctxLen == 0 {
		ctxLen = 4096
	}
	return m.CheckCompatibilityFull(hw, quant, ctxLen)
}

// CheckCompatibilityFull checks with specific quant and context
func (m *Model) CheckCompatibilityFull(hw *hardware.Info, quant string, ctxLen int) CompatResult {
	if hw.HasGPU {
		return m.checkGPUCompat(hw, quant, ctxLen)
	}
	return m.checkCPUCompat(hw, quant, ctxLen)
}

func (m *Model) checkGPUCompat(hw *hardware.Info, quant string, ctxLen int) CompatResult {
	vramNeeded := m.EstimateVRAM(quant, ctxLen)
	vramAvail := hw.MinGPUVRAM()

	pct := float64(vramNeeded) / float64(vramAvail)

	switch {
	case pct <= 0.85:
		return CompatResult{
			Level:  CompatOK,
			Reason: fmt.Sprintf("%.0f%% of VRAM used (%d/%d MiB)", pct*100, vramNeeded, vramAvail),
		}
	case pct <= 1.0:
		return CompatResult{
			Level:  CompatWarn,
			Reason: fmt.Sprintf("Tight fit: %.0f%% of VRAM (%d/%d MiB). May be slow.", pct*100, vramNeeded, vramAvail),
			Suggestion: fmt.Sprintf("Try reducing context: --ctx %d, or use %s",
				ctxLen/2, m.smallerQuant(quant)),
		}
	default:
		shortage := vramNeeded - vramAvail
		return CompatResult{
			Level:  CompatFail,
			Reason: fmt.Sprintf("Needs %d MiB more VRAM than available (%d vs %d MiB)", shortage, vramNeeded, vramAvail),
			Suggestion: fmt.Sprintf("Try: smaller quant (%s), reduce context (--ctx %d), or use CPU mode",
				m.smallerQuant(quant), ctxLen/2),
		}
	}
}

func (m *Model) checkCPUCompat(hw *hardware.Info, quant string, _ int) CompatResult {
	ramNeeded := m.EstimateRAM(quant)
	ramAvail := int(hw.RAM.FreeMB)

	pct := float64(ramNeeded) / float64(ramAvail)

	switch {
	case pct <= 0.7:
		return CompatResult{
			Level:  CompatOK,
			Reason: fmt.Sprintf("%.0f%% of RAM used (%d/%d MiB)", pct*100, ramNeeded, ramAvail),
		}
	case pct <= 0.9:
		return CompatResult{
			Level:  CompatWarn,
			Reason: fmt.Sprintf("High RAM usage: %.0f%% (%d/%d MiB). System may be slow.", pct*100, ramNeeded, ramAvail),
		}
	default:
		return CompatResult{
			Level:      CompatFail,
			Reason:     fmt.Sprintf("Insufficient RAM: need %d MiB, have %d MiB free", ramNeeded, ramAvail),
			Suggestion: "Use a smaller model or add more RAM",
		}
	}
}

func (m *Model) smallerQuant(quant string) string {
	order := []string{"fp16", "q8_0", "q6_K", "q5_K_M", "q4_K_M", "q4_K_S", "q4_0", "q3_K_M", "q2_K"}
	for i, q := range order {
		if strings.EqualFold(q, quant) && i+1 < len(order) {
			return order[i+1]
		}
	}
	return "q4_K_M"
}

// ── Utils ──────────────────────────────────────────────────────────────────────

func quantBits(quant string) float64 {
	q := strings.ToLower(quant)
	switch {
	case strings.HasPrefix(q, "q2"):
		return 2.5
	case strings.HasPrefix(q, "q3"):
		return 3.5
	case strings.HasPrefix(q, "q4"):
		return 4.5
	case strings.HasPrefix(q, "q5"):
		return 5.5
	case strings.HasPrefix(q, "q6"):
		return 6.5
	case strings.HasPrefix(q, "q8"):
		return 8.5
	case q == "fp16":
		return 16
	case q == "fp32":
		return 32
	default:
		return 4.5 // default to q4
	}
}
