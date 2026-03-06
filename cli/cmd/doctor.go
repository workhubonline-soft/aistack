package cmd

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"github.com/workhubonline-soft/aistack/internal/hardware"
)

type CheckResult struct {
	Name    string
	Status  string // ok | warn | fail | info
	Message string
	Fix     string
}

var doctorPortList = []int{3000, 11434, 8080, 80, 443}

func newDoctorCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Run system diagnostics and compatibility checks",
		Long:  "Checks OS, Docker, GPU, disk space, ports and network connectivity",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDoctor(jsonOutput)
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output results as JSON")
	return cmd
}

func runDoctor(jsonOutput bool) error {
	bold := color.New(color.Bold)
	cyan := color.New(color.FgCyan, color.Bold)

	cyan.Println("\n╔══════════════════════════════════════════╗")
	cyan.Println("║         AIStack System Doctor            ║")
	cyan.Println("╚══════════════════════════════════════════╝")
	fmt.Printf("  Checked: %s\n\n", time.Now().Format("2006-01-02 15:04:05"))

	var results []CheckResult
	var hasFailures bool

	// ── Hardware Detection ──────────────────────────────────────────────────
	bold.Println("  Hardware")
	hw, err := hardware.Detect()
	if err != nil {
		results = append(results, CheckResult{
			Name: "Hardware Detection", Status: "warn",
			Message: fmt.Sprintf("Partial detection: %v", err),
		})
	} else {
		results = append(results, CheckResult{
			Name:    "OS",
			Status:  osCheckStatus(hw),
			Message: fmt.Sprintf("%s (kernel: %s, arch: %s)", hw.OS.PrettyName, hw.OS.KernelVer, hw.OS.Arch),
		})
		results = append(results, CheckResult{
			Name:    "CPU",
			Status:  "ok",
			Message: fmt.Sprintf("%s — %d cores / %d threads", hw.CPU.Model, hw.CPU.Cores, hw.CPU.Threads),
		})
		ramStatus := "ok"
		if hw.RAM.TotalMB < 8192 {
			ramStatus = "warn"
		}
		results = append(results, CheckResult{
			Name:    "RAM",
			Status:  ramStatus,
			Message: fmt.Sprintf("Total: %d MB / Free: %d MB", hw.RAM.TotalMB, hw.RAM.FreeMB),
			Fix:     ifStr(ramStatus == "warn", "Minimum 8GB RAM recommended for CPU inference"),
		})
		diskStatus := "ok"
		if hw.Disk.FreeGB < 20 {
			diskStatus = "fail"
		} else if hw.Disk.FreeGB < 50 {
			diskStatus = "warn"
		}
		results = append(results, CheckResult{
			Name:    "Disk Space",
			Status:  diskStatus,
			Message: fmt.Sprintf("Free: %d GB / Total: %d GB", hw.Disk.FreeGB, hw.Disk.TotalGB),
			Fix:     ifStr(diskStatus != "ok", "Free at least 50GB for models and container images"),
		})
	}

	// ── Docker ──────────────────────────────────────────────────────────────
	bold.Println("\n  Docker")
	results = append(results, checkDocker()...)

	// ── GPU ─────────────────────────────────────────────────────────────────
	bold.Println("\n  NVIDIA GPU")
	if hw != nil && hw.HasGPU {
		for _, gpu := range hw.GPUs {
			results = append(results, CheckResult{
				Name:    fmt.Sprintf("GPU %d", gpu.Index),
				Status:  "ok",
				Message: fmt.Sprintf("%s — %d MiB VRAM (driver: %s, CUDA: %s)", gpu.Name, gpu.VRAMMiB, gpu.DriverVer, gpu.CUDAVer),
			})
		}
		results = append(results, checkNvidiaContainerToolkit())
	} else {
		results = append(results, CheckResult{
			Name:    "NVIDIA GPU",
			Status:  "info",
			Message: "No NVIDIA GPU detected — CPU-only mode",
		})
	}

	// ── Ports ───────────────────────────────────────────────────────────────
	bold.Println("\n  Ports")
	for _, port := range doctorPortList {
		results = append(results, checkPort(port))
	}

	// ── Network ─────────────────────────────────────────────────────────────
	bold.Println("\n  Network")
	results = append(results, checkNetwork()...)

	// ── Render results ──────────────────────────────────────────────────────
	fmt.Println()
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Check", "Status", "Details"})
	table.SetBorder(false)
	table.SetColumnSeparator("  ")
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetColWidth(50)

	for _, r := range results {
		icon, colorCode := statusIcon(r.Status)
		statusStr := color.New(colorCode).Sprintf("%s %s", icon, strings.ToUpper(r.Status))
		table.Append([]string{r.Name, statusStr, r.Message})
		if r.Status == "fail" {
			hasFailures = true
		}
	}
	table.Render()

	// ── Fixes ───────────────────────────────────────────────────────────────
	var fixes []CheckResult
	for _, r := range results {
		if r.Fix != "" {
			fixes = append(fixes, r)
		}
	}
	if len(fixes) > 0 {
		fmt.Println()
		color.Yellow("  ⚠ Recommendations:")
		for _, r := range fixes {
			fmt.Printf("    • %s: %s\n", r.Name, r.Fix)
		}
	}

	// ── Summary ─────────────────────────────────────────────────────────────
	fmt.Println()
	if hasFailures {
		color.Red("  ✗ System has issues that need to be fixed before installation.")
		color.Red("    Run: aistack install to attempt automatic fixes.")
		return fmt.Errorf("doctor found critical issues")
	}
	color.Green("  ✓ System looks good! Ready to run: aistack install")
	fmt.Println()
	return nil
}

func checkDocker() []CheckResult {
	var results []CheckResult

	// Docker binary
	path, err := exec.LookPath("docker")
	if err != nil {
		results = append(results, CheckResult{
			Name:    "Docker Engine",
			Status:  "fail",
			Message: "docker not found",
			Fix:     "Run: aistack install --yes to install Docker automatically",
		})
		return results
	}

	// Docker version
	out, err := exec.Command("docker", "version", "--format", "{{.Server.Version}}").Output()
	if err != nil {
		results = append(results, CheckResult{
			Name:    "Docker Engine",
			Status:  "fail",
			Message: fmt.Sprintf("docker found at %s but daemon not running: %v", path, err),
			Fix:     "Run: sudo systemctl start docker",
		})
		return results
	}
	ver := strings.TrimSpace(string(out))
	results = append(results, CheckResult{
		Name:    "Docker Engine",
		Status:  "ok",
		Message: fmt.Sprintf("v%s at %s", ver, path),
	})

	// Compose plugin
	out2, err := exec.Command("docker", "compose", "version", "--short").Output()
	if err != nil {
		results = append(results, CheckResult{
			Name:    "Docker Compose Plugin",
			Status:  "fail",
			Message: "docker compose plugin not found",
			Fix:     "Run: sudo apt install docker-compose-plugin",
		})
	} else {
		results = append(results, CheckResult{
			Name:    "Docker Compose Plugin",
			Status:  "ok",
			Message: fmt.Sprintf("v%s", strings.TrimSpace(string(out2))),
		})
	}

	// Docker socket permissions
	if _, err := exec.Command("docker", "ps").Output(); err != nil {
		results = append(results, CheckResult{
			Name:    "Docker Permissions",
			Status:  "warn",
			Message: "Current user cannot run docker without sudo",
			Fix:     "Run: sudo usermod -aG docker $USER && newgrp docker",
		})
	} else {
		results = append(results, CheckResult{
			Name:    "Docker Permissions",
			Status:  "ok",
			Message: "User has docker access",
		})
	}

	return results
}

func checkNvidiaContainerToolkit() CheckResult {
	out, err := exec.Command("docker", "run", "--rm", "--gpus", "all",
		"ubuntu:22.04", "nvidia-smi", "--query-gpu=name", "--format=csv,noheader",
	).Output()
	if err != nil {
		return CheckResult{
			Name:    "nvidia-container-toolkit",
			Status:  "fail",
			Message: "GPU passthrough to containers not working",
			Fix:     "Run: aistack install to set up nvidia-container-toolkit",
		}
	}
	gpuName := strings.TrimSpace(string(out))
	return CheckResult{
		Name:    "nvidia-container-toolkit",
		Status:  "ok",
		Message: fmt.Sprintf("GPU passthrough OK: %s", gpuName),
	}
}

func checkPort(port int) CheckResult {
	addr := fmt.Sprintf(":%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return CheckResult{
			Name:    fmt.Sprintf("Port %d", port),
			Status:  "warn",
			Message: fmt.Sprintf("Port %d is already in use", port),
			Fix:     fmt.Sprintf("Check what's using port %d: sudo lsof -i :%d", port, port),
		}
	}
	ln.Close()
	return CheckResult{
		Name:    fmt.Sprintf("Port %d", port),
		Status:  "ok",
		Message: "Available",
	}
}

func checkNetwork() []CheckResult {
	var results []CheckResult

	// DNS
	_, err := net.LookupHost("registry-1.docker.io")
	if err != nil {
		results = append(results, CheckResult{
			Name:    "DNS Resolution",
			Status:  "fail",
			Message: "Cannot resolve registry-1.docker.io",
			Fix:     "Check /etc/resolv.conf and network connectivity",
		})
	} else {
		results = append(results, CheckResult{
			Name:    "DNS Resolution",
			Status:  "ok",
			Message: "registry-1.docker.io resolved",
		})
	}

	// Docker Hub connectivity
	conn, err := net.DialTimeout("tcp", "registry-1.docker.io:443", 5*time.Second)
	if err != nil {
		results = append(results, CheckResult{
			Name:    "Docker Hub",
			Status:  "fail",
			Message: "Cannot reach Docker Hub",
			Fix:     "Check firewall rules: ufw allow out 443/tcp",
		})
	} else {
		conn.Close()
		results = append(results, CheckResult{
			Name:    "Docker Hub",
			Status:  "ok",
			Message: "registry-1.docker.io:443 reachable",
		})
	}

	// Ollama registry
	conn2, err := net.DialTimeout("tcp", "registry.ollama.ai:443", 5*time.Second)
	if err != nil {
		results = append(results, CheckResult{
			Name:    "Ollama Registry",
			Status:  "warn",
			Message: "Cannot reach registry.ollama.ai",
			Fix:     "Model downloads may fail. Check firewall.",
		})
	} else {
		conn2.Close()
		results = append(results, CheckResult{
			Name:    "Ollama Registry",
			Status:  "ok",
			Message: "registry.ollama.ai:443 reachable",
		})
	}

	return results
}

func osCheckStatus(hw *hardware.Info) string {
	if hw.OS.ID != "ubuntu" {
		return "fail"
	}
	switch hw.OS.VersionID {
	case "22.04", "24.04":
		return "ok"
	default:
		return "warn"
	}
}

func statusIcon(status string) (string, color.Attribute) {
	switch status {
	case "ok":
		return "✓", color.FgGreen
	case "warn":
		return "⚠", color.FgYellow
	case "fail":
		return "✗", color.FgRed
	default:
		return "ℹ", color.FgCyan
	}
}

func ifStr(cond bool, s string) string {
	if cond {
		return s
	}
	return ""
}
