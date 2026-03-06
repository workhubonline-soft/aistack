package hardware

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
)

// Info holds all detected hardware information
type Info struct {
	OS      OSInfo
	CPU     CPUInfo
	RAM     RAMInfo
	Disk    DiskInfo
	GPUs    []GPUInfo
	HasGPU  bool
	Profile string // auto-selected profile
}

type OSInfo struct {
	ID         string
	VersionID  string
	PrettyName string
	KernelVer  string
	Arch       string
}

type CPUInfo struct {
	Model   string
	Cores   int
	Threads int
}

type RAMInfo struct {
	TotalMB uint64
	FreeMB  uint64
}

type DiskInfo struct {
	Path    string
	TotalGB uint64
	FreeGB  uint64
}

type GPUInfo struct {
	Index       int
	Name        string
	VRAMMiB     int
	DriverVer   string
	CUDAVer     string
	Temperature int
}

// Detect collects all hardware information
func Detect() (*Info, error) {
	info := &Info{}

	if err := info.detectOS(); err != nil {
		return nil, fmt.Errorf("OS detection: %w", err)
	}
	if err := info.detectCPU(); err != nil {
		return nil, fmt.Errorf("CPU detection: %w", err)
	}
	if err := info.detectRAM(); err != nil {
		return nil, fmt.Errorf("RAM detection: %w", err)
	}
	if err := info.detectDisk(); err != nil {
		return nil, fmt.Errorf("Disk detection: %w", err)
	}
	// GPU detection is best-effort
	info.detectGPUs()

	info.selectProfile()
	return info, nil
}

func (h *Info) detectOS() error {
	f, err := os.Open("/etc/os-release")
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := parts[0]
		val := strings.Trim(parts[1], `"`)
		switch key {
		case "ID":
			h.OS.ID = val
		case "VERSION_ID":
			h.OS.VersionID = val
		case "PRETTY_NAME":
			h.OS.PrettyName = val
		}
	}

	// Kernel version
	if out, err := exec.Command("uname", "-r").Output(); err == nil {
		h.OS.KernelVer = strings.TrimSpace(string(out))
	}

	// Arch
	if out, err := exec.Command("uname", "-m").Output(); err == nil {
		h.OS.Arch = strings.TrimSpace(string(out))
	}

	return nil
}

func (h *Info) detectCPU() error {
	info, err := cpu.Info()
	if err != nil {
		return err
	}
	if len(info) > 0 {
		h.CPU.Model = info[0].ModelName
	}

	physical, _ := cpu.Counts(false)
	logical, _ := cpu.Counts(true)
	h.CPU.Cores = physical
	h.CPU.Threads = logical
	return nil
}

func (h *Info) detectRAM() error {
	vm, err := mem.VirtualMemory()
	if err != nil {
		return err
	}
	h.RAM.TotalMB = vm.Total / 1024 / 1024
	h.RAM.FreeMB = vm.Available / 1024 / 1024
	return nil
}

func (h *Info) detectDisk() error {
	stat, err := disk.Usage("/")
	if err != nil {
		return err
	}
	h.Disk.Path = "/"
	h.Disk.TotalGB = stat.Total / 1024 / 1024 / 1024
	h.Disk.FreeGB = stat.Free / 1024 / 1024 / 1024
	return nil
}

func (h *Info) detectGPUs() {
	nvidiaSmi, err := exec.LookPath("nvidia-smi")
	if err != nil {
		return // No GPU, not an error
	}

	query := "index,name,memory.total,driver_version,compute_cap,temperature.gpu"
	out, err := exec.Command(nvidiaSmi,
		"--query-gpu="+query,
		"--format=csv,noheader,nounits",
	).Output()
	if err != nil {
		return
	}

	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.Split(line, ", ")
		if len(parts) < 6 {
			continue
		}

		gpu := GPUInfo{}
		gpu.Index, _ = strconv.Atoi(strings.TrimSpace(parts[0]))
		gpu.Name = strings.TrimSpace(parts[1])
		gpu.VRAMMiB, _ = strconv.Atoi(strings.TrimSpace(parts[2]))
		gpu.DriverVer = strings.TrimSpace(parts[3])
		gpu.CUDAVer = strings.TrimSpace(parts[4])
		gpu.Temperature, _ = strconv.Atoi(strings.TrimSpace(parts[5]))

		h.GPUs = append(h.GPUs, gpu)
	}

	h.HasGPU = len(h.GPUs) > 0
}

func (h *Info) selectProfile() {
	if !h.HasGPU {
		h.Profile = "cpu"
		return
	}

	// Use minimum VRAM among all GPUs (weakest link)
	minVRAM := h.GPUs[0].VRAMMiB
	for _, g := range h.GPUs {
		if g.VRAMMiB < minVRAM {
			minVRAM = g.VRAMMiB
		}
	}

	switch {
	case minVRAM < 8192:
		h.Profile = "nvidia-low-vram"
	case minVRAM < 16384:
		h.Profile = "nvidia-8gb"
	case minVRAM < 24576:
		h.Profile = "nvidia-16gb"
	default:
		h.Profile = "nvidia-24gb"
	}
}

// MinGPUVRAM returns the minimum VRAM across all GPUs in MiB
func (h *Info) MinGPUVRAM() int {
	if !h.HasGPU {
		return 0
	}
	min := h.GPUs[0].VRAMMiB
	for _, g := range h.GPUs {
		if g.VRAMMiB < min {
			min = g.VRAMMiB
		}
	}
	return min
}

// TotalGPUVRAM returns total VRAM across all GPUs
func (h *Info) TotalGPUVRAM() int {
	total := 0
	for _, g := range h.GPUs {
		total += g.VRAMMiB
	}
	return total
}
