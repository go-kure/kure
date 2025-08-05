package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/go-kure/kure/pkg/cmd/generate"
	"github.com/go-kure/kure/pkg/cmd/options"
)

const (
	// KureVersion is injected during build
	KureVersion = "dev"
)

// NewKureCommand creates the root command for kure CLI
func NewKureCommand() *cobra.Command {
	globalOpts := options.NewGlobalOptions()

	cmd := &cobra.Command{
		Use:   "kure",
		Short: "A Go library for programmatically building Kubernetes resources",
		Long: `Kure is a Go library for programmatically building Kubernetes resources used by GitOps tools.

The library emphasizes strongly-typed object construction over templating engines,
supporting both Flux and ArgoCD workflows for GitOps-native resource management.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return globalOpts.Complete()
		},
	}

	// Add global flags
	globalOpts.AddFlags(cmd.PersistentFlags())

	// Initialize configuration
	initConfig(globalOpts)

	// Add subcommands
	cmd.AddCommand(
		newGenerateCommand(globalOpts),
		newValidateCommand(globalOpts),
		newConfigCommand(globalOpts),
		newCompletionCommand(),
		newVersionCommand(),
	)

	return cmd
}

// Execute runs the root command
func Execute() {
	cmd := NewKureCommand()
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// initConfig initializes Viper configuration
func initConfig(globalOpts *options.GlobalOptions) {
	if globalOpts.ConfigFile != "" {
		viper.SetConfigFile(globalOpts.ConfigFile)
	} else {
		// Search for config in home directory
		home, err := os.UserHomeDir()
		if err != nil {
			return
		}

		// Search config in home directory with name ".kure" (without extension)
		viper.AddConfigPath(home)
		viper.AddConfigPath(".")
		viper.SetConfigName(".kure")
		viper.SetConfigType("yaml")
	}

	// Environment variable prefix
	viper.SetEnvPrefix("KURE")
	viper.AutomaticEnv()

	// Read config file if found
	if err := viper.ReadInConfig(); err == nil && globalOpts.Verbose {
		fmt.Fprintf(os.Stderr, "Using config file: %s\n", viper.ConfigFileUsed())
	}
}

func newGenerateCommand(globalOpts *options.GlobalOptions) *cobra.Command {
	return generate.NewGenerateCommand(globalOpts)
}

func newValidateCommand(globalOpts *options.GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate configuration files",
		Long:  "Validate Kure configuration files for syntax and consistency",
	}
}

func newConfigCommand(globalOpts *options.GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Manage kure configuration",
		Long:  "View and modify kure configuration settings",
	}
}

func newCompletionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate completion script",
		Long: `Generate the autocompletion script for kure for the specified shell.
See each sub-command's help for details on how to use the generated script.`,
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		Run: func(cmd *cobra.Command, args []string) {
			switch args[0] {
			case "bash":
				cmd.Root().GenBashCompletion(os.Stdout)
			case "zsh":
				cmd.Root().GenZshCompletion(os.Stdout)
			case "fish":
				cmd.Root().GenFishCompletion(os.Stdout, true)
			case "powershell":
				cmd.Root().GenPowerShellCompletion(os.Stdout)
			}
		},
	}
}

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Long:  "Print the version number of kure",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("kure version %s\n", KureVersion)
		},
	}
}