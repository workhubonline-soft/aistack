package cmd

import (
	"fmt"
	"math"
	"os"
	"sort"
	"strings"

	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"

	"github.com/workhubonline-soft/aistack/internal/hardware"
	"github.com/workhubonline-soft/aistack/internal/models"
	"github.com/workhubonline-soft/aistack/internal/ollama"
)

func newModelsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "models",
		Short: "Model catalog, recommendations and downloads",
	}
	cmd.AddCommand(
		newModelsListCmd(),
		newModelsRecommendCmd(),
		newModelsEstimateCmd(),
		newModelsPullCmd(),
		newModelsBenchmarkCmd(),
	)
	return cmd
}

// ── models list ───────────────────────────────────────────────────────────────
func newModelsListCmd() *cobra.Command {
	var filterFamily string
	var filterEngine string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all models in the catalog",
		RunE: func(cmd *cobra.Command, args []string) error {
			catalog, err := models.LoadCatalog(catalogPath())
			if err != nil {
				return fmt.Errorf("loading catalog: %w", err)
			}

			hw, _ := hardware.Detect()

			color.Cyan("\n  AIStack Model Catalog\n")
			table := tablewriter.NewWriter(os.Stdout)
			table.SetHeader([]string{"ID", "Name", "Family", "Params", "Best Quant", "Engine", "Min VRAM", "RAM", "Status"})
			table.SetBorder(false)
			table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
			table.SetAlignment(tablewriter.ALIGN_LEFT)

			for _, m := range catalog.Models {
				if filterFamily != "" && !strings.EqualFold(m.Family, filterFamily) {
					continue
				}
				if filterEngine != "" && !strings.EqualFold(m.Engine, filterEngine) {
					continue
				}

				compat := "  "
				if hw != nil {
					status := m.CheckCompatibility(hw)
					switch status.Level {
					case models.CompatOK:
						compat = color.GreenString("✓")
					case models.CompatWarn:
						compat = color.YellowString("⚠")
					case models.CompatFail:
						compat = color.RedString("✗")
					}
				}

				bestQuant := m.BestQuantForHardware(hw)
				minVRAM := m.EstimateVRAM(bestQuant, 4096)

				vramStr := "CPU only"
				if minVRAM > 0 {
					vramStr = fmt.Sprintf("%d GB", int(math.Ceil(float64(minVRAM)/1024)))
				}

				ramGB := int(math.Ceil(float64(m.EstimateRAM(bestQuant)) / 1024))

				table.Append([]string{
					m.ID,
					m.Name,
					m.Family,
					m.SizeLabel,
					bestQuant,
					m.Engine,
					vramStr,
					fmt.Sprintf("%d GB", ramGB),
					compat,
				})
			}
			table.Render()
			fmt.Printf("\n  Total: %d models. Legend: ✓=compatible  ⚠=marginal  ✗=insufficient resources\n\n",
				len(catalog.Models))
			return nil
		},
	}
	cmd.Flags().StringVar(&filterFamily, "family", "", "Filter by model family (llama/qwen/mistral/...)")
	cmd.Flags().StringVar(&filterEngine, "engine", "", "Filter by engine (ollama/vllm)")
	return cmd
}

// ── models recommend ──────────────────────────────────────────────────────────
func newModelsRecommendCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "recommend",
		Short: "Get model recommendations for your hardware",
		RunE: func(cmd *cobra.Command, args []string) error {
			catalog, err := models.LoadCatalog(catalogPath())
			if err != nil {
				return fmt.Errorf("loading catalog: %w", err)
			}
			hw, err := hardware.Detect()
			if err != nil {
				return fmt.Errorf("detecting hardware: %w", err)
			}

			color.Cyan("\n  Model Recommendations for Your Hardware\n")

			// Print hardware summary
			if hw.HasGPU {
				fmt.Printf("  Hardware: %s — %d MiB VRAM\n", hw.GPUs[0].Name, hw.MinGPUVRAM())
			} else {
				fmt.Printf("  Hardware: CPU only — %d MB RAM\n", hw.RAM.TotalMB)
			}
			fmt.Println()

			categories := []struct {
				Name string
				Tag  string
				Desc string
			}{
				{"Best General Chat", "chat", "Balanced quality and speed for everyday use"},
				{"Best Coding", "coding", "Optimized for code generation and review"},
				{"Best Fast/Small", "fast", "Fastest response, lowest resource usage"},
				{"Best Long Context", "long-context", "Maximum context window for documents"},
				{"Best Multilingual", "multilingual", "Strong non-English language support"},
			}

			for _, cat := range categories {
				recs := catalog.GetRecommendations(hw, cat.Tag, 3)
				if len(recs) == 0 {
					continue
				}

				color.Yellow("  ▶ %s\n", cat.Name)
				fmt.Printf("    %s\n\n", cat.Desc)

				for i, rec := range recs {
					compat := rec.CheckCompatibility(hw)
					icon := compatIcon(compat.Level)

					fmt.Printf("    %d. %s %s — %s (%s)\n",
						i+1, icon, color.New(color.Bold).Sprint(rec.Name),
						rec.SizeLabel, rec.BestQuantForHardware(hw))
					fmt.Printf("       %s\n", rec.Description)
					fmt.Printf("       %s Pull: %s\n\n",
						color.CyanString("→"),
						color.CyanString("aistack models pull %s", rec.OllamaID()))
				}
			}
			return nil
		},
	}
}

// ── models estimate ───────────────────────────────────────────────────────────
func newModelsEstimateCmd() *cobra.Command {
	var modelID string
	var ctx int
	var quant string

	cmd := &cobra.Command{
		Use:   "estimate",
		Short: "Estimate resource usage for a specific model",
		Example: `  aistack models estimate --model qwen2.5:14b --ctx 8192
  aistack models estimate --model llama3.1:8b --ctx 4096 --quant q4_K_M`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if modelID == "" && len(args) > 0 {
				modelID = args[0]
			}
			if modelID == "" {
				return fmt.Errorf("specify model with --model or as argument")
			}

			catalog, err := models.LoadCatalog(catalogPath())
			if err != nil {
				return fmt.Errorf("loading catalog: %w", err)
			}
			hw, err := hardware.Detect()
			if err != nil {
				return fmt.Errorf("detecting hardware: %w", err)
			}

			m := catalog.FindModel(modelID)
			if m == nil {
				return fmt.Errorf("model '%s' not found in catalog. Run: aistack models list", modelID)
			}

			if quant == "" {
				quant = m.BestQuantForHardware(hw)
			}

			color.Cyan("\n  Resource Estimate: %s\n", m.Name)
			fmt.Printf("  Quantization: %s  |  Context: %d tokens\n\n", quant, ctx)

			// Calculate estimates
			vramMiB := m.EstimateVRAM(quant, ctx)
			kvCacheMiB := m.EstimateKVCache(ctx)
			ramMiB := m.EstimateRAM(quant)

			printEstimateRow("Model weights (VRAM)", vramMiB, hw.MinGPUVRAM())
			printEstimateRow("KV Cache (VRAM)", kvCacheMiB, 0)
			totalVRAM := vramMiB + kvCacheMiB
			printEstimateRow("Total VRAM required", totalVRAM, hw.MinGPUVRAM())
			fmt.Println()
			printEstimateRow("System RAM required", ramMiB, int(hw.RAM.FreeMB))
			fmt.Println()

			// Compatibility verdict
			compat := m.CheckCompatibilityFull(hw, quant, ctx)
			printVerdict(compat)

			// Show quant alternatives
			fmt.Println()
			color.Yellow("  Quantization alternatives:")
			quants := m.AvailableQuants
			sort.Slice(quants, func(i, j int) bool {
				return m.EstimateVRAM(quants[i], ctx) < m.EstimateVRAM(quants[j], ctx)
			})
			table := tablewriter.NewWriter(os.Stdout)
			table.SetHeader([]string{"Quant", "VRAM", "Quality", "Compatible"})
			table.SetBorder(false)
			for _, q := range quants {
				v := m.EstimateVRAM(q, ctx)
				var ok string
				switch {
				case hw.MinGPUVRAM() > 0 && v <= hw.MinGPUVRAM():
					ok = color.GreenString("✓")
				case hw.MinGPUVRAM() == 0:
					ok = color.CyanString("CPU")
				default:
					ok = color.RedString("✗")
				}
				table.Append([]string{q, fmt.Sprintf("%d MiB", v), qualityLabel(q), ok})
			}
			table.Render()
			fmt.Println()
			return nil
		},
	}
	cmd.Flags().StringVarP(&modelID, "model", "m", "", "Model ID (e.g. qwen2.5:14b)")
	cmd.Flags().IntVar(&ctx, "ctx", 4096, "Context length in tokens")
	cmd.Flags().StringVar(&quant, "quant", "", "Quantization (default: auto-select)")
	return cmd
}

// ── models pull ───────────────────────────────────────────────────────────────
func newModelsPullCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pull <model-id>",
		Short: "Download a model via Ollama",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			modelID := args[0]

			catalog, err := models.LoadCatalog(catalogPath())
			if err != nil {
				return fmt.Errorf("loading catalog: %w", err)
			}
			hw, _ := hardware.Detect()

			m := catalog.FindModel(modelID)
			ollamaID := modelID
			if m != nil {
				ollamaID = m.OllamaID()

				if hw != nil {
					compat := m.CheckCompatibility(hw)
					switch compat.Level {
					case models.CompatFail:
						color.Red("\n  ✗ Warning: %s\n", compat.Reason)
						if !yes {
							return fmt.Errorf("model may not run on your hardware. Use --yes to force")
						}
					case models.CompatWarn:
						color.Yellow("\n  ⚠ %s\n", compat.Reason)
					}
				}

				// Disk space check
				diskFreeGB := hw.Disk.FreeGB
				modelSizeGB := m.EstimateDiskGB(m.BestQuantForHardware(hw))
				if diskFreeGB < uint64(modelSizeGB+2) {
					return fmt.Errorf("not enough disk space: need ~%dGB, have %dGB free",
						modelSizeGB+2, diskFreeGB)
				}
			}

			color.Cyan("\n  Pulling model: %s\n", ollamaID)
			fmt.Printf("  This may take a while depending on your connection speed.\n\n")

			client := ollama.NewClient(ollamaBaseURL())
			if err := client.Ping(); err != nil {
				return fmt.Errorf("cannot reach Ollama: %w\n  Is AIStack running? Try: aistack up", err)
			}

			var lastStatus string
			return client.Pull(ollamaID, func(p ollama.PullProgress) {
				if p.Total > 0 {
					pct := float64(p.Completed) / float64(p.Total) * 100
					fmt.Printf("\r  %s  %.1f%%", p.Status, pct)
				} else if p.Status != lastStatus {
					if lastStatus != "" {
						fmt.Println()
					}
					fmt.Printf("  %s", p.Status)
					lastStatus = p.Status
				}
			})
		},
	}
}

// ── models benchmark ──────────────────────────────────────────────────────────
func newModelsBenchmarkCmd() *cobra.Command {
	var modelID string

	cmd := &cobra.Command{
		Use:   "benchmark [model]",
		Short: "Quick performance benchmark (tokens/sec)",
		Example: `  aistack models benchmark llama3.2:3b
  aistack models benchmark --model qwen2.5:7b`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if modelID == "" && len(args) > 0 {
				modelID = args[0]
			}

			client := ollama.NewClient(ollamaBaseURL())
			if err := client.Ping(); err != nil {
				return fmt.Errorf("cannot reach Ollama: %w\n  Is AIStack running? Try: aistack up", err)
			}

			// If no model specified, pick the first loaded one
			if modelID == "" {
				loaded, err := client.List()
				if err != nil {
					return fmt.Errorf("listing models: %w", err)
				}
				if len(loaded) == 0 {
					return fmt.Errorf("no models loaded. Pull one first: aistack models pull llama3.2:3b")
				}
				modelID = loaded[0].Name
			}

			color.Cyan("\n  Benchmark: %s\n", modelID)
			fmt.Println("  Sending test prompt...")
			fmt.Println()

			const testPrompt = "Explain what a binary search tree is in exactly three sentences."
			resp, err := client.Generate(modelID, testPrompt)
			if err != nil {
				return fmt.Errorf("benchmark failed: %w", err)
			}

			totalSec := float64(resp.TotalDuration) / 1e9
			evalSec := float64(resp.EvalDuration) / 1e9
			promptSec := float64(resp.PromptEvalDuration) / 1e9

			tokPerSec := 0.0
			if evalSec > 0 {
				tokPerSec = float64(resp.EvalCount) / evalSec
			}
			promptTokPerSec := 0.0
			if promptSec > 0 {
				promptTokPerSec = float64(resp.PromptEvalCount) / promptSec
			}

			table := tablewriter.NewWriter(os.Stdout)
			table.SetHeader([]string{"Metric", "Value"})
			table.SetBorder(false)
			table.SetAlignment(tablewriter.ALIGN_LEFT)
			table.Append([]string{"Model", modelID})
			table.Append([]string{"Prompt tokens", fmt.Sprintf("%d", resp.PromptEvalCount)})
			table.Append([]string{"Generated tokens", fmt.Sprintf("%d", resp.EvalCount)})
			table.Append([]string{"Prompt eval", fmt.Sprintf("%.1f tok/s", promptTokPerSec)})
			table.Append([]string{"Generation speed", fmt.Sprintf("%.1f tok/s", tokPerSec)})
			table.Append([]string{"Total time", fmt.Sprintf("%.2fs", totalSec)})
			table.Render()
			fmt.Println()

			return nil
		},
	}
	cmd.Flags().StringVarP(&modelID, "model", "m", "", "Model to benchmark (default: first loaded)")
	return cmd
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func catalogPath() string {
	paths := []string{
		"/opt/aistack/models/catalog.yaml",
		"./models/catalog.yaml",
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "./models/catalog.yaml"
}

func compatIcon(level models.CompatLevel) string {
	switch level {
	case models.CompatOK:
		return color.GreenString("✓")
	case models.CompatWarn:
		return color.YellowString("⚠")
	default:
		return color.RedString("✗")
	}
}

func printEstimateRow(label string, valueMiB, availMiB int) {
	valueStr := fmt.Sprintf("%d MiB (%.1f GB)", valueMiB, float64(valueMiB)/1024)
	if availMiB > 0 {
		if valueMiB <= availMiB {
			fmt.Printf("  %-35s %s  %s\n", label, valueStr,
				color.GreenString("(%.0f%% of available)", 100*float64(valueMiB)/float64(availMiB)))
		} else {
			fmt.Printf("  %-35s %s  %s\n", label, valueStr,
				color.RedString("(exceeds available by %d MiB)", valueMiB-availMiB))
		}
	} else {
		fmt.Printf("  %-35s %s\n", label, valueStr)
	}
}

func printVerdict(compat models.CompatResult) {
	switch compat.Level {
	case models.CompatOK:
		color.Green("  ✓ COMPATIBLE — %s\n", compat.Reason)
	case models.CompatWarn:
		color.Yellow("  ⚠ MARGINAL — %s\n", compat.Reason)
		if compat.Suggestion != "" {
			fmt.Printf("    Suggestion: %s\n", compat.Suggestion)
		}
	case models.CompatFail:
		color.Red("  ✗ NOT COMPATIBLE — %s\n", compat.Reason)
		if compat.Suggestion != "" {
			fmt.Printf("    Suggestion: %s\n", compat.Suggestion)
		}
	}
}

func ollamaBaseURL() string {
	if v := os.Getenv("OLLAMA_HOST"); v != "" {
		return "http://" + v
	}
	return "http://localhost:11434"
}

func qualityLabel(quant string) string {
	q := strings.ToLower(quant)
	switch {
	case strings.Contains(q, "q2"):
		return "Low"
	case strings.Contains(q, "q3"):
		return "Medium-Low"
	case strings.Contains(q, "q4"):
		return "Good"
	case strings.Contains(q, "q5"):
		return "Very Good"
	case strings.Contains(q, "q6"):
		return "Excellent"
	case strings.Contains(q, "q8"):
		return "Near-lossless"
	case strings.Contains(q, "fp16"):
		return "Full precision"
	default:
		return "Variable"
	}
}
