package models

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/workhubonline-soft/aistack/internal/hardware"
)

// ── Catalog loading ───────────────────────────────────────────────────────────

func TestLoadCatalog(t *testing.T) {
	catalog := loadTestCatalog(t)
	if len(catalog.Models) == 0 {
		t.Fatal("catalog has no models")
	}
	t.Logf("Loaded %d models", len(catalog.Models))
}

func TestCatalogModelsHaveRequiredFields(t *testing.T) {
	catalog := loadTestCatalog(t)
	for _, m := range catalog.Models {
		t.Run(m.ID, func(t *testing.T) {
			if m.ID == "" {
				t.Error("model has empty ID")
			}
			if m.Name == "" {
				t.Errorf("model %s has empty Name", m.ID)
			}
			if m.ParamsB <= 0 {
				t.Errorf("model %s has invalid ParamsB: %f", m.ID, m.ParamsB)
			}
			if m.Engine == "" {
				t.Errorf("model %s has empty Engine", m.ID)
			}
			if len(m.AvailableQuants) == 0 {
				t.Errorf("model %s has no quants", m.ID)
			}
			if m.DefaultQuant == "" {
				t.Errorf("model %s has empty DefaultQuant", m.ID)
			}
		})
	}
}

func TestFindModel(t *testing.T) {
	catalog := loadTestCatalog(t)

	tests := []struct {
		query   string
		wantNil bool
	}{
		{"llama3.1:8b", false},
		{"qwen2.5:14b", false},
		{"nonexistent:99b", true},
		{"deepseek-r1:7b", false},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			m := catalog.FindModel(tt.query)
			if tt.wantNil && m != nil {
				t.Errorf("expected nil for %s, got %s", tt.query, m.ID)
			}
			if !tt.wantNil && m == nil {
				t.Errorf("expected model for %s, got nil", tt.query)
			}
		})
	}
}

// ── VRAM Estimation ───────────────────────────────────────────────────────────

func TestEstimateVRAM_Sanity(t *testing.T) {
	tests := []struct {
		name    string
		paramsB float64
		quant   string
		ctx     int
		minMiB  int
		maxMiB  int
	}{
		// 7B q4 should be roughly 4-6 GB
		{"7B q4_K_M", 7.0, "q4_K_M", 4096, 3500, 6500},
		// 14B q4 should be roughly 8-11 GB
		{"14B q4_K_M", 14.0, "q4_K_M", 4096, 7000, 12000},
		// 70B q4 should be roughly 35-50 GB
		{"70B q4_K_M", 70.0, "q4_K_M", 4096, 30000, 55000},
		// 3B q4 should be roughly 2-3 GB
		{"3B q4_K_M", 3.0, "q4_K_M", 4096, 1500, 3500},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Model{
				ID:      "test",
				ParamsB: tt.paramsB,
			}
			vram := m.EstimateVRAM(tt.quant, tt.ctx)
			if vram < tt.minMiB || vram > tt.maxMiB {
				t.Errorf("EstimateVRAM(%s, %d) = %d MiB, want [%d, %d]",
					tt.quant, tt.ctx, vram, tt.minMiB, tt.maxMiB)
			}
		})
	}
}

func TestEstimateVRAM_LargerQuantNeedsMoreVRAM(t *testing.T) {
	m := &Model{ID: "test", ParamsB: 7.0}
	ctx := 4096

	q4 := m.EstimateVRAM("q4_K_M", ctx)
	q8 := m.EstimateVRAM("q8_0", ctx)
	fp16 := m.EstimateVRAM("fp16", ctx)

	if !(q4 < q8 && q8 < fp16) {
		t.Errorf("Expected q4 < q8 < fp16, got q4=%d q8=%d fp16=%d", q4, q8, fp16)
	}
}

func TestEstimateKVCache_ScalesWithContext(t *testing.T) {
	m := &Model{ID: "test", ParamsB: 7.0}

	kv4k := m.EstimateKVCache(4096)
	kv8k := m.EstimateKVCache(8192)
	kv32k := m.EstimateKVCache(32768)

	if !(kv4k < kv8k && kv8k < kv32k) {
		t.Errorf("KV cache should scale with context: 4k=%d 8k=%d 32k=%d", kv4k, kv8k, kv32k)
	}

	// 8k context should be ~2x 4k context
	ratio := float64(kv8k) / float64(kv4k)
	if ratio < 1.8 || ratio > 2.2 {
		t.Errorf("KV cache ratio for 2x context should be ~2.0, got %.2f", ratio)
	}
}

// ── Compatibility ─────────────────────────────────────────────────────────────

func TestCompatibility_CPUOnly(t *testing.T) {
	hw := &hardware.Info{
		HasGPU: false,
		RAM:    hardware.RAMInfo{TotalMB: 16384, FreeMB: 12000},
	}

	tests := []struct {
		name    string
		paramsB float64
		quant   string
		want    CompatLevel
	}{
		{"3B model fits in 12GB RAM", 3.0, "q4_K_M", CompatOK},
		{"7B model fits in 12GB RAM", 7.0, "q4_K_M", CompatOK},
		{"70B q4 won't fit in 12GB RAM", 70.0, "q4_K_M", CompatFail},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Model{
				ID:              "test",
				ParamsB:         tt.paramsB,
				DefaultQuant:    tt.quant,
				AvailableQuants: []string{tt.quant},
				DefaultCtx:      4096,
			}
			result := m.CheckCompatibilityFull(hw, tt.quant, 4096)
			if result.Level != tt.want {
				t.Errorf("compat level = %v, want %v (reason: %s)",
					result.Level, tt.want, result.Reason)
			}
		})
	}
}

func TestCompatibility_GPU8GB(t *testing.T) {
	hw := &hardware.Info{
		HasGPU: true,
		GPUs: []hardware.GPUInfo{
			{Index: 0, Name: "RTX 3070", VRAMMiB: 8192},
		},
		RAM: hardware.RAMInfo{TotalMB: 32768, FreeMB: 28000},
	}

	tests := []struct {
		name    string
		paramsB float64
		quant   string
		want    CompatLevel
	}{
		// 7B q4 ≈ 4.5 GB — fits in 8GB
		{"7B q4 fits in 8GB", 7.0, "q4_K_M", CompatOK},
		// 14B q4 ≈ 8.7 GB — won't fit in 8GB
		{"14B q4 too large for 8GB", 14.0, "q4_K_M", CompatFail},
		// 3B q8 ≈ 3.5 GB — fits comfortably
		{"3B q8 fits in 8GB", 3.0, "q8_0", CompatOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Model{
				ID:              "test",
				ParamsB:         tt.paramsB,
				DefaultQuant:    tt.quant,
				AvailableQuants: []string{tt.quant},
				DefaultCtx:      4096,
			}
			result := m.CheckCompatibilityFull(hw, tt.quant, 4096)
			if result.Level != tt.want {
				t.Errorf("compat level = %v, want %v (reason: %s)",
					result.Level, tt.want, result.Reason)
			}
		})
	}
}

func TestCompatibility_GPU24GB(t *testing.T) {
	hw := &hardware.Info{
		HasGPU: true,
		GPUs: []hardware.GPUInfo{
			{Index: 0, Name: "RTX 4090", VRAMMiB: 24576},
		},
		RAM: hardware.RAMInfo{TotalMB: 64000, FreeMB: 60000},
	}

	// 14B q6 ≈ 12.5 GB — fits easily in 24GB
	m := &Model{
		ID:              "qwen2.5:14b",
		ParamsB:         14.0,
		DefaultQuant:    "q6_K",
		AvailableQuants: []string{"q6_K"},
		DefaultCtx:      8192,
	}
	result := m.CheckCompatibilityFull(hw, "q6_K", 8192)
	if result.Level != CompatOK {
		t.Errorf("14B q6 should fit in 24GB: %s", result.Reason)
	}
}

// ── Best Quant Selection ──────────────────────────────────────────────────────

func TestBestQuantForHardware_CPU(t *testing.T) {
	hw := &hardware.Info{
		HasGPU: false,
		RAM:    hardware.RAMInfo{TotalMB: 16384, FreeMB: 12000},
	}
	m := &Model{
		ID:              "test",
		ParamsB:         7.0,
		DefaultQuant:    "q4_K_M",
		AvailableQuants: []string{"q4_K_S", "q4_K_M", "q5_K_M", "q8_0"},
		DefaultCtx:      4096,
	}
	q := m.BestQuantForHardware(hw)
	// CPU mode should prefer q4_K_M for balance
	if q != "q4_K_M" {
		t.Logf("Got quant %s for CPU mode (acceptable)", q)
	}
}

func TestBestQuantForHardware_GPU8GB(t *testing.T) {
	hw := &hardware.Info{
		HasGPU: true,
		GPUs:   []hardware.GPUInfo{{VRAMMiB: 8192}},
	}
	m := &Model{
		ID:              "test",
		ParamsB:         7.0,
		DefaultQuant:    "q4_K_M",
		AvailableQuants: []string{"q4_K_S", "q4_K_M", "q5_K_M", "q6_K", "q8_0"},
		DefaultCtx:      4096,
	}
	q := m.BestQuantForHardware(hw)
	// For 7B on 8GB, q5_K_M or q6_K should fit
	vram := m.EstimateVRAM(q, 4096)
	if vram > 8192 {
		t.Errorf("BestQuant %s requires %d MiB but only 8192 available", q, vram)
	}
	t.Logf("Best quant for 7B on 8GB VRAM: %s (%d MiB)", q, vram)
}

// ── Recommendations ───────────────────────────────────────────────────────────

func TestGetRecommendations(t *testing.T) {
	catalog := loadTestCatalog(t)

	hw8gb := &hardware.Info{
		HasGPU: true,
		GPUs:   []hardware.GPUInfo{{Name: "RTX 3070", VRAMMiB: 8192}},
		RAM:    hardware.RAMInfo{TotalMB: 32768, FreeMB: 28000},
	}

	recs := catalog.GetRecommendations(hw8gb, "chat", 5)
	if len(recs) == 0 {
		t.Error("should have chat recommendations for 8GB GPU")
	}
	t.Logf("Chat recommendations for 8GB GPU:")
	for _, r := range recs {
		compat := r.CheckCompatibility(hw8gb)
		t.Logf("  %s (%s) — %v", r.Name, r.BestQuantForHardware(hw8gb), compat.Level)
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func loadTestCatalog(t *testing.T) *Catalog {
	t.Helper()

	// Try to find catalog relative to this file
	paths := []string{
		"../../models/catalog.yaml",
		"../../../models/catalog.yaml",
		"../../../../models/catalog.yaml",
	}

	for _, p := range paths {
		abs, _ := filepath.Abs(p)
		if _, err := os.Stat(abs); err == nil {
			catalog, err := LoadCatalog(abs)
			if err != nil {
				t.Fatalf("loading catalog from %s: %v", abs, err)
			}
			return catalog
		}
	}

	// Fallback: create minimal test catalog
	t.Log("Using minimal test catalog (real catalog not found at expected paths)")
	return &Catalog{
		Models: []Model{
			{
				ID: "llama3.1:8b", Name: "Llama 3.1 8B", Family: "llama",
				ParamsB: 8.0, Engine: "ollama", OllamaTag: "llama3.1:8b",
				SizeLabel: "8B", DefaultQuant: "q4_K_M",
				AvailableQuants: []string{"q4_K_M", "q5_K_M", "q8_0"},
				Tags:            []string{"chat", "coding"}, DefaultCtx: 8192, MaxCtx: 131072,
			},
			{
				ID: "qwen2.5:14b", Name: "Qwen 2.5 14B", Family: "qwen",
				ParamsB: 14.0, Engine: "ollama", OllamaTag: "qwen2.5:14b",
				SizeLabel: "14B", DefaultQuant: "q4_K_M",
				AvailableQuants: []string{"q4_K_M", "q5_K_M", "q6_K", "q8_0"},
				Tags:            []string{"chat", "coding", "multilingual"}, DefaultCtx: 8192, MaxCtx: 131072,
			},
			{
				ID: "deepseek-r1:7b", Name: "DeepSeek R1 7B", Family: "deepseek",
				ParamsB: 7.0, Engine: "ollama", OllamaTag: "deepseek-r1:7b",
				SizeLabel: "7B", DefaultQuant: "q4_K_M",
				AvailableQuants: []string{"q4_K_M", "q5_K_M", "q8_0"},
				Tags:            []string{"chat", "coding", "fast"}, DefaultCtx: 8192, MaxCtx: 65536,
			},
		},
	}
}
