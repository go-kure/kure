package kure

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/go-kure/kure/pkg/cmd/kure/generate"
	"github.com/go-kure/kure/pkg/cmd/shared"
	"github.com/go-kure/kure/pkg/cmd/shared/options"
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
	shared.InitConfig("kure", globalOpts)

	// Add subcommands
	cmd.AddCommand(
		newGenerateCommand(globalOpts),
		NewPatchCommand(globalOpts),
		newValidateCommand(globalOpts),
		newConfigCommand(globalOpts),
		shared.NewCompletionCommand(),
		shared.NewVersionCommand("kure"),
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
