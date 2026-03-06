package cmd

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	yes     bool
	profile string
	verbose bool
)

var rootCmd = &cobra.Command{
	Use:   "aistack",
	Short: "Self-hosted AI Stack installer and manager",
	Long: color.CyanString(`
   █████╗ ██╗    ███████╗████████╗ █████╗  ██████╗██╗  ██╗
  ██╔══██╗██║    ██╔════╝╚══██╔══╝██╔══██╗██╔════╝██║ ██╔╝
  ███████║██║    ███████╗   ██║   ███████║██║     █████╔╝
  ██╔══██║██║    ╚════██║   ██║   ██╔══██║██║     ██╔═██╗
  ██║  ██║██║    ███████║   ██║   ██║  ██║╚██████╗██║  ██╗
  ╚═╝  ╚═╝╚═╝    ╚══════╝   ╚═╝   ╚═╝  ╚═╝ ╚═════╝╚═╝  ╚═╝`) + `

Self-hosted AI Stack — Ollama + Open WebUI on Ubuntu
`,
	SilenceUsage: true,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: /opt/aistack/config.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&yes, "yes", "y", false, "Auto-confirm all prompts")
	rootCmd.PersistentFlags().StringVar(&profile, "profile", "", "Installation profile (cpu|nvidia-8gb|nvidia-16gb|nvidia-24gb)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")

	// Register all subcommands
	rootCmd.AddCommand(
		newDoctorCmd(),
		newInstallCmd(),
		newUpCmd(),
		newDownCmd(),
		newStatusCmd(),
		newLogsCmd(),
		newUpdateCmd(),
		newBackupCmd(),
		newReportCmd(),
		newModelsCmd(),
		newVersionCmd(),
	)
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.AddConfigPath("/opt/aistack")
		viper.AddConfigPath(".")
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}

	viper.SetEnvPrefix("AISTACK")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil && verbose {
		fmt.Fprintln(os.Stderr, "Using config:", viper.ConfigFileUsed())
	}
}
