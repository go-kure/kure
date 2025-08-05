package generate

import (
	"github.com/spf13/cobra"

	"github.com/go-kure/kure/pkg/cmd/options"
	"github.com/go-kure/kure/pkg/cli"
)

// NewGenerateCommand creates the generate command and its subcommands
func NewGenerateCommand(globalOpts *options.GlobalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate Kubernetes manifests",
		Long: `Generate Kubernetes manifests from configuration files using Kure builders.

The generate command supports multiple subcommands for different types of resources:
- cluster: Generate complete cluster manifests with GitOps configuration
- app: Generate application workload manifests
- bootstrap: Generate bootstrap configurations for GitOps tools
- patch: Apply patches to existing manifests`,
		Aliases: []string{"gen"},
	}

	// Create factory for dependency injection
	factory := cli.NewFactory(globalOpts)

	// Add subcommands
	cmd.AddCommand(
		NewClusterCommand(factory),
		NewAppCommand(factory),
		NewBootstrapCommand(factory),
		NewPatchCommand(factory),
	)

	return cmd
}