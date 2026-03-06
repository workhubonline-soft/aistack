package hardware

import (
	"testing"
)

func TestMinGPUVRAM_NoGPU(t *testing.T) {
	hw := &Info{HasGPU: false}
	if hw.MinGPUVRAM() != 0 {
		t.Errorf("MinGPUVRAM() with no GPU should be 0, got %d", hw.MinGPUVRAM())
	}
}

func TestMinGPUVRAM_SingleGPU(t *testing.T) {
	hw := &Info{
		HasGPU: true,
		GPUs:   []GPUInfo{{Index: 0, VRAMMiB: 8192}},
	}
	if hw.MinGPUVRAM() != 8192 {
		t.Errorf("MinGPUVRAM() = %d, want 8192", hw.MinGPUVRAM())
	}
}

func TestMinGPUVRAM_MultipleGPUs(t *testing.T) {
	hw := &Info{
		HasGPU: true,
		GPUs: []GPUInfo{
			{Index: 0, VRAMMiB: 24576},
			{Index: 1, VRAMMiB: 8192}, // weakest link
		},
	}
	if hw.MinGPUVRAM() != 8192 {
		t.Errorf("MinGPUVRAM() should return minimum: %d, want 8192", hw.MinGPUVRAM())
	}
}

func TestTotalGPUVRAM(t *testing.T) {
	hw := &Info{
		HasGPU: true,
		GPUs: []GPUInfo{
			{Index: 0, VRAMMiB: 24576},
			{Index: 1, VRAMMiB: 24576},
		},
	}
	if hw.TotalGPUVRAM() != 49152 {
		t.Errorf("TotalGPUVRAM() = %d, want 49152", hw.TotalGPUVRAM())
	}
}

func TestSelectProfile_NoGPU(t *testing.T) {
	hw := &Info{HasGPU: false}
	hw.selectProfile()
	if hw.Profile != "cpu" {
		t.Errorf("profile = %s, want cpu", hw.Profile)
	}
}

func TestSelectProfile_GPU_Tiers(t *testing.T) {
	tests := []struct {
		vram    int
		profile string
	}{
		{4096, "nvidia-low-vram"},
		{8192, "nvidia-8gb"},
		{16384, "nvidia-16gb"},
		{24576, "nvidia-24gb"},
		{40960, "nvidia-24gb"},
	}

	for _, tt := range tests {
		hw := &Info{
			HasGPU: true,
			GPUs:   []GPUInfo{{Index: 0, VRAMMiB: tt.vram}},
		}
		hw.selectProfile()
		if hw.Profile != tt.profile {
			t.Errorf("VRAM=%d: profile=%s, want %s", tt.vram, hw.Profile, tt.profile)
		}
	}
}
