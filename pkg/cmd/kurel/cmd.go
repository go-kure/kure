package kurel

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/go-kure/kure/pkg/cmd/shared"
	"github.com/go-kure/kure/pkg/cmd/shared/options"
)

// NewKurelCommand creates the root command for kurel CLI
func NewKurelCommand() *cobra.Command {
	globalOpts := options.NewGlobalOptions()

	cmd := &cobra.Command{
		Use:   "kurel",
		Short: "Kurel - Kubernetes Resources Launcher",
		Long: `Kurel is a CLI tool for launching and managing Kubernetes resources.
It extends the Kure library with deployment and resource management capabilities.

Kurel uses a package-based approach to create reusable, customizable Kubernetes 
applications without the complexity of templating engines.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return globalOpts.Complete()
		},
	}

	// Add global flags
	globalOpts.AddFlags(cmd.PersistentFlags())

	// Initialize configuration
	shared.InitConfig("kurel", globalOpts)

	// Add subcommands
	cmd.AddCommand(
		newBuildCommand(globalOpts),
		newValidateCommand(globalOpts),
		newInfoCommand(globalOpts),
		newSchemaCommand(globalOpts),
		newConfigCommand(globalOpts),
		shared.NewCompletionCommand(),
		shared.NewVersionCommand("kurel"),
	)

	return cmd
}

// Execute runs the root command
func Execute() {
	cmd := NewKurelCommand()
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func newBuildCommand(globalOpts *options.GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "build <package>",
		Short: "Build Kubernetes manifests from kurel package",
		Long: `Build generates Kubernetes manifests from a kurel package.

The build command processes the package structure, applies patches based on
configuration, and outputs phase-organized manifests ready for GitOps deployment.`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Building package: %s\n", args[0])
			// TODO: Implement build logic using pkg/launcher
		},
	}
}

func newValidateCommand(globalOpts *options.GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "validate <package>",
		Short: "Validate kurel package structure and configuration",
		Long: `Validate checks the kurel package for structural correctness,
parameter validation, and patch consistency.`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Validating package: %s\n", args[0])
			// TODO: Implement validation logic using pkg/launcher
		},
	}
}

func newInfoCommand(globalOpts *options.GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "info <package>",
		Short: "Show package information",
		Long: `Info displays detailed information about a kurel package including
metadata, available patches, configurable parameters, and deployment phases.`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Package info: %s\n", args[0])
			// TODO: Implement info logic using pkg/launcher
		},
	}
}

func newSchemaCommand(globalOpts *options.GlobalOptions) *cobra.Command {
	schemaCmd := &cobra.Command{
		Use:   "schema",
		Short: "Schema generation and validation commands",
		Long:  "Manage JSON schemas for kurel package validation",
	}

	schemaCmd.AddCommand(
		&cobra.Command{
			Use:   "generate <package>",
			Short: "Generate JSON schema for package parameters",
			Args:  cobra.ExactArgs(1),
			Run: func(cmd *cobra.Command, args []string) {
				fmt.Printf("Generating schema for package: %s\n", args[0])
				// TODO: Implement schema generation using pkg/launcher
			},
		},
	)

	return schemaCmd
}

func newConfigCommand(globalOpts *options.GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Manage kurel configuration",
		Long:  "View and modify kurel configuration settings",
	}
}
