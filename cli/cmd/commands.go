package cmd

import (
	"archive/tar"
	"compress/gzip"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

const (
	aistackDir = "/opt/aistack"
	composeDir = "/opt/aistack/compose"
	envFile    = "/opt/aistack/.env"
	stateFile  = "/var/lib/aistack/state.json"
	logDir     = "/var/log/aistack"
	backupDir  = "/var/lib/aistack/backups"
)

// ── install ───────────────────────────────────────────────────────────────────

func newInstallCmd() *cobra.Command {
	var noModelDownload bool

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install dependencies and prepare AIStack",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInstall(noModelDownload)
		},
	}
	cmd.Flags().BoolVar(&noModelDownload, "no-model-download", false, "Skip initial model download")
	return cmd
}

func runInstall(noModelDownload bool) error {
	color.Cyan("\n  AIStack Install\n")

	steps := []struct {
		name string
		fn   func() error
	}{
		{"Checking system requirements", checkSystemRequirements},
		{"Setting up directories", setupDirectories},
		{"Generating configuration", generateConfig},
		{"Pulling Docker images", pullDockerImages},
	}

	for _, step := range steps {
		s := spinner.New(spinner.CharSets[14], 80*time.Millisecond)
		s.Suffix = "  " + step.name
		s.Start()

		if err := step.fn(); err != nil {
			s.Stop()
			color.Red("  ✗ %s: %v\n", step.name, err)
			return err
		}
		s.Stop()
		color.Green("  ✓ %s\n", step.name)
	}

	if !noModelDownload {
		fmt.Println()
		color.Yellow("  💡 Tip: Pull your first model with:")
		fmt.Printf("     aistack models recommend\n")
		fmt.Printf("     aistack models pull llama3.2:3b\n\n")
	}

	saveState("installed")
	color.Green("\n  ✓ Installation complete! Run: aistack up\n\n")
	return nil
}

func checkSystemRequirements() error {
	// Docker check
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("docker not found — run install.sh first")
	}
	if err := exec.Command("docker", "compose", "version").Run(); err != nil {
		return fmt.Errorf("docker compose plugin not found")
	}
	// Disk space check (need at least 20GB)
	out, err := exec.Command("df", "-BG", "--output=avail", "/").Output()
	if err == nil {
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		if len(lines) >= 2 {
			var free int
			_, _ = fmt.Sscanf(strings.TrimSpace(lines[1]), "%dG", &free)
			if free < 20 {
				return fmt.Errorf("insufficient disk space: %dGB free, need at least 20GB", free)
			}
		}
	}
	return nil
}

func setupDirectories() error {
	dirs := []string{
		aistackDir,
		composeDir,
		logDir,
		"/var/lib/aistack",
		backupDir,
		filepath.Join(aistackDir, "configs"),
		filepath.Join(aistackDir, "models"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("creating %s: %w", d, err)
		}
	}
	return nil
}

func generateConfig() error {
	if _, err := os.Stat(envFile); err == nil {
		return nil // Already exists — don't overwrite
	}

	// Read example config
	examplePaths := []string{
		filepath.Join(aistackDir, "configs", "env.example"),
		"./configs/env.example",
	}

	var content string
	for _, p := range examplePaths {
		data, err := os.ReadFile(p)
		if err == nil {
			content = string(data)
			break
		}
	}
	if content == "" {
		return fmt.Errorf("env.example not found")
	}

	// Generate random secret key
	secret, err := generateSecret(32)
	if err != nil {
		return err
	}
	content = strings.ReplaceAll(content, "CHANGE_ME_RANDOM_SECRET_KEY_32CHARS", secret)
	content = strings.ReplaceAll(content, "AISTACK_INSTALLED_AT=", "AISTACK_INSTALLED_AT="+time.Now().Format(time.RFC3339))

	hostname, _ := os.Hostname()
	content = strings.ReplaceAll(content, "AISTACK_HOST_HOSTNAME=", "AISTACK_HOST_HOSTNAME="+hostname)

	return os.WriteFile(envFile, []byte(content), 0o600)
}

func pullDockerImages() error {
	images := []string{
		"ollama/ollama:latest",
		"ghcr.io/open-webui/open-webui:main",
	}
	for _, img := range images {
		cmd := exec.Command("docker", "pull", img)
		cmd.Stdout = nil
		cmd.Stderr = nil
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("pulling %s: %w", img, err)
		}
	}
	return nil
}

// ── up ────────────────────────────────────────────────────────────────────────

func newUpCmd() *cobra.Command {
	var enableMonitoring bool
	var enableNginx bool

	cmd := &cobra.Command{
		Use:   "up",
		Short: "Start AIStack services",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUp(enableMonitoring, enableNginx)
		},
	}
	cmd.Flags().BoolVar(&enableMonitoring, "monitoring", false, "Enable Prometheus + Grafana")
	cmd.Flags().BoolVar(&enableNginx, "nginx", false, "Enable Nginx reverse proxy")
	return cmd
}

func runUp(monitoring, nginx bool) error {
	color.Cyan("\n  Starting AIStack...\n")

	composeFiles := buildComposeArgs()
	profiles := []string{}
	if monitoring {
		profiles = append(profiles, "monitoring")
	}
	if nginx {
		profiles = append(profiles, "nginx")
	}

	composeArgs := make([]string, len(composeFiles))
	copy(composeArgs, composeFiles)
	composeArgs = append(composeArgs, "up", "-d", "--remove-orphans")
	if len(profiles) > 0 {
		composeArgs = append([]string{"--profile", strings.Join(profiles, ",")}, composeArgs...)
	}
	args := composeArgs

	cmd := exec.Command("docker", append([]string{"compose"}, args...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = composeDir

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker compose up failed: %w", err)
	}

	saveState("running")
	fmt.Println()
	printAccessInfo()
	return nil
}

// ── down ──────────────────────────────────────────────────────────────────────

func newDownCmd() *cobra.Command {
	var removeVolumes bool

	cmd := &cobra.Command{
		Use:   "down",
		Short: "Stop AIStack services",
		RunE: func(cmd *cobra.Command, args []string) error {
			color.Cyan("\n  Stopping AIStack...\n")
			composeFiles := buildComposeArgs()
			cmdArgs := make([]string, len(composeFiles))
			copy(cmdArgs, composeFiles)
			cmdArgs = append(cmdArgs, "down")
			if removeVolumes {
				cmdArgs = append(cmdArgs, "-v")
				color.Yellow("  ⚠ Removing all volumes (data will be lost!)\n")
			}
			c := exec.Command("docker", append([]string{"compose"}, cmdArgs...)...)
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			c.Dir = composeDir
			if err := c.Run(); err != nil {
				return err
			}
			saveState("stopped")
			color.Green("\n  ✓ AIStack stopped\n\n")
			return nil
		},
	}
	cmd.Flags().BoolVar(&removeVolumes, "volumes", false, "Remove volumes (destroys all data!)")
	return cmd
}

// ── status ────────────────────────────────────────────────────────────────────

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show status of AIStack services",
		RunE: func(cmd *cobra.Command, args []string) error {
			color.Cyan("\n  AIStack Service Status\n")
			composeFiles := buildComposeArgs()
			c := exec.Command("docker", append(append([]string{"compose"}, composeFiles...), "ps", "--format", "table")...)
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			c.Dir = composeDir
			if err := c.Run(); err != nil {
				return err
			}
			fmt.Println()
			printAccessInfo()
			return nil
		},
	}
}

// ── logs ──────────────────────────────────────────────────────────────────────

func newLogsCmd() *cobra.Command {
	var follow bool
	var tail int

	cmd := &cobra.Command{
		Use:   "logs [service]",
		Short: "View service logs",
		RunE: func(cmd *cobra.Command, args []string) error {
			composeFiles := buildComposeArgs()
			cmdArgs := make([]string, len(composeFiles))
			copy(cmdArgs, composeFiles)
			cmdArgs = append(cmdArgs, "logs")
			if follow {
				cmdArgs = append(cmdArgs, "-f")
			}
			if tail > 0 {
				cmdArgs = append(cmdArgs, "--tail", fmt.Sprintf("%d", tail))
			}
			cmdArgs = append(cmdArgs, args...)

			c := exec.Command("docker", append([]string{"compose"}, cmdArgs...)...)
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			c.Dir = composeDir
			return c.Run()
		},
	}
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow log output")
	cmd.Flags().IntVar(&tail, "tail", 100, "Number of lines to show")
	return cmd
}

// ── update ────────────────────────────────────────────────────────────────────

func newUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Update Docker images and restart services",
		RunE: func(cmd *cobra.Command, args []string) error {
			color.Cyan("\n  Updating AIStack...\n")

			// Pull new images
			s := spinner.New(spinner.CharSets[14], 80*time.Millisecond)
			s.Suffix = "  Pulling latest images"
			s.Start()
			composeFiles := buildComposeArgs()
			pullArgs := make([]string, 0, len(composeFiles)+1)
			pullArgs = append(pullArgs, composeFiles...)
			pullArgs = append(pullArgs, "pull")
			c := exec.Command("docker", append([]string{"compose"}, pullArgs...)...)
			c.Dir = composeDir
			if err := c.Run(); err != nil {
				s.Stop()
				return err
			}
			s.Stop()
			color.Green("  ✓ Images updated\n")

			// Restart
			return runUp(false, false)
		},
	}
}

// ── backup ────────────────────────────────────────────────────────────────────

func newBackupCmd() *cobra.Command {
	var outputPath string

	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Backup AIStack volumes and configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBackup(outputPath)
		},
	}
	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "Output path for backup archive")
	return cmd
}

func runBackup(outputPath string) error {
	color.Cyan("\n  AIStack Backup\n")

	timestamp := time.Now().Format("20060102-150405")
	if outputPath == "" {
		outputPath = filepath.Join(backupDir, fmt.Sprintf("aistack-backup-%s.tar.gz", timestamp))
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return err
	}

	color.Yellow("  ⚠ Services will continue running during backup.\n")

	steps := []struct {
		name string
		fn   func(tw *tar.Writer) error
	}{
		{"Backing up configuration", backupConfigs},
		{"Backing up Open WebUI data", backupOpenWebUI},
	}

	f, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("creating backup file: %w", err)
	}
	defer func() { _ = f.Close() }()

	gw := gzip.NewWriter(f)
	defer func() { _ = gw.Close() }()
	tw := tar.NewWriter(gw)
	defer func() { _ = tw.Close() }()

	for _, step := range steps {
		s := spinner.New(spinner.CharSets[14], 80*time.Millisecond)
		s.Suffix = "  " + step.name
		s.Start()
		if err := step.fn(tw); err != nil {
			s.Stop()
			color.Yellow("  ⚠ %s: %v (continuing)\n", step.name, err)
		} else {
			s.Stop()
			color.Green("  ✓ %s\n", step.name)
		}
	}

	stat, _ := os.Stat(outputPath)
	size := ""
	if stat != nil {
		size = fmt.Sprintf(" (%.1f MB)", float64(stat.Size())/1024/1024)
	}

	color.Green("\n  ✓ Backup complete: %s%s\n\n", outputPath, size)
	return nil
}

func backupConfigs(tw *tar.Writer) error {
	return addToTar(tw, envFile, "config/.env")
}

func backupOpenWebUI(tw *tar.Writer) error {
	// Export Open WebUI volume via docker
	out, err := exec.Command("docker", "run", "--rm",
		"-v", "aistack-openwebui-data:/data",
		"alpine", "tar", "-czf", "-", "/data",
	).Output()
	if err != nil {
		return fmt.Errorf("docker volume export: %w", err)
	}
	return addBytesToTar(tw, out, "volumes/openwebui-data.tar.gz")
}

// ── report ────────────────────────────────────────────────────────────────────

func newReportCmd() *cobra.Command {
	var outputPath string

	cmd := &cobra.Command{
		Use:   "report",
		Short: "Generate diagnostic report for support",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReport(outputPath)
		},
	}
	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "Output path for report archive")
	return cmd
}

func runReport(outputPath string) error {
	color.Cyan("\n  Generating Diagnostic Report...\n")

	timestamp := time.Now().Format("20060102-150405")
	if outputPath == "" {
		outputPath = fmt.Sprintf("/tmp/aistack-report-%s.tar.gz", timestamp)
	}

	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	gw := gzip.NewWriter(f)
	defer func() { _ = gw.Close() }()
	tw := tar.NewWriter(gw)
	defer func() { _ = tw.Close() }()

	// Collect: doctor output, versions, compose config, logs
	collectReportData(tw)

	color.Green("\n  ✓ Report saved: %s\n", outputPath)
	fmt.Printf("  Share this file when opening a support issue.\n\n")
	return nil
}

func collectReportData(tw *tar.Writer) {
	// Doctor output
	if out, err := exec.Command("aistack", "doctor").CombinedOutput(); err == nil {
		_ = addBytesToTar(tw, out, "report/doctor.txt")
	}

	// Docker version
	if out, err := exec.Command("docker", "version").Output(); err == nil {
		_ = addBytesToTar(tw, out, "report/docker-version.txt")
	}

	// Compose config
	composeFiles := buildComposeArgs()
	configArgs := make([]string, 0, len(composeFiles)+2)
	configArgs = append(configArgs, "compose")
	configArgs = append(configArgs, composeFiles...)
	configArgs = append(configArgs, "config")
	if out, err := exec.Command("docker", configArgs...).Output(); err == nil {
		_ = addBytesToTar(tw, out, "report/compose-config.yml")
	}

	// Service status
	cf2 := buildComposeArgs()
	statusArgs := make([]string, 0, len(cf2)+2)
	statusArgs = append(statusArgs, "compose")
	statusArgs = append(statusArgs, cf2...)
	statusArgs = append(statusArgs, "ps")
	if out, err := exec.Command("docker", statusArgs...).Output(); err == nil {
		_ = addBytesToTar(tw, out, "report/services-status.txt")
	}

	// Recent logs
	cf3 := buildComposeArgs()
	logsArgs := make([]string, 0, len(cf3)+3)
	logsArgs = append(logsArgs, "compose")
	logsArgs = append(logsArgs, cf3...)
	logsArgs = append(logsArgs, "logs", "--tail=100")
	if out, err := exec.Command("docker", logsArgs...).Output(); err == nil {
		_ = addBytesToTar(tw, out, "report/recent-logs.txt")
	}

	// nvidia-smi
	if out, err := exec.Command("nvidia-smi").Output(); err == nil {
		_ = addBytesToTar(tw, out, "report/nvidia-smi.txt")
	}
}

// ── version ───────────────────────────────────────────────────────────────────

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show AIStack version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("AIStack %s\n", version)
		},
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

var version = "dev"

func buildComposeArgs() []string {
	profile := detectCurrentProfile()
	base := []string{
		"-f", filepath.Join(composeDir, "docker-compose.yml"),
	}
	switch {
	case strings.HasPrefix(profile, "nvidia"):
		base = append(base, "-f", filepath.Join(composeDir, "docker-compose.nvidia.yml"))
	default:
		base = append(base, "-f", filepath.Join(composeDir, "docker-compose.cpu.yml"))
	}
	if envFile != "" {
		base = append(base, "--env-file", envFile)
	}
	return base
}

func detectCurrentProfile() string {
	// Read from env file
	data, err := os.ReadFile(envFile)
	if err != nil {
		return "cpu"
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "AISTACK_PROFILE=") {
			return strings.TrimPrefix(line, "AISTACK_PROFILE=")
		}
	}
	return "cpu"
}

func printAccessInfo() {
	color.Cyan("  Access URLs:")
	fmt.Printf("    Open WebUI:  %s\n", color.CyanString("http://localhost:3000"))
	fmt.Printf("    Ollama API:  %s\n", color.CyanString("http://localhost:11434"))
	fmt.Println()
}

func saveState(status string) {
	_ = os.MkdirAll("/var/lib/aistack", 0o755)
	content := fmt.Sprintf(`{"status":%q,"updated_at":%q}`,
		status, time.Now().Format(time.RFC3339))
	_ = os.WriteFile(stateFile, []byte(content), 0o644)
}

func generateSecret(length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b)[:length], nil
}

func addToTar(tw *tar.Writer, srcPath, destPath string) error {
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}
	return addBytesToTar(tw, data, destPath)
}

func addBytesToTar(tw *tar.Writer, data []byte, name string) error {
	hdr := &tar.Header{
		Name:    name,
		Mode:    0o644,
		Size:    int64(len(data)),
		ModTime: time.Now(),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	_, err := tw.Write(data)
	return err
}

// suppress unused import
