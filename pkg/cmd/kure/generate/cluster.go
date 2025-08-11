package generate

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"gopkg.in/yaml.v3"

	"github.com/go-kure/kure/pkg/cli"
	"github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"

	// Import implementations to register workflow factories
	_ "github.com/go-kure/kure/pkg/stack/argocd"
	_ "github.com/go-kure/kure/pkg/stack/fluxcd"

	// Import generators to register them
	_ "github.com/go-kure/kure/pkg/stack/generators/appworkload"
	_ "github.com/go-kure/kure/pkg/stack/generators/fluxhelm"
)

// ClusterOptions contains options for the cluster command
type ClusterOptions struct {
	// Input options
	ConfigFile string
	InputDir   string

	// Output options
	OutputDir   string
	ManifestDir string

	// Layout options
	BundleGrouping      string
	ApplicationGrouping string
	FluxPlacement       string

	// Dependencies
	Factory   cli.Factory
	IOStreams cli.IOStreams
}

// NewClusterCommand creates the cluster subcommand
func NewClusterCommand(factory cli.Factory) *cobra.Command {
	o := &ClusterOptions{
		Factory:   factory,
		IOStreams: factory.IOStreams(),
	}

	cmd := &cobra.Command{
		Use:   "cluster [flags] CONFIG_FILE",
		Short: "Generate cluster manifests from configuration",
		Long: `Generate complete cluster manifests with GitOps configuration.

This command processes cluster configuration files and generates a complete
directory structure with Kubernetes manifests organized for GitOps workflows.

Examples:
  # Generate cluster from config file
  kure generate cluster examples/clusters/basic/cluster.yaml

  # Generate with custom output directory
  kure generate cluster --output-dir ./output cluster.yaml

  # Generate with different layout options
  kure generate cluster --bundle-grouping=nested --flux-placement=separate cluster.yaml`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o.ConfigFile = args[0]

			if err := o.Complete(); err != nil {
				return err
			}
			if err := o.Validate(); err != nil {
				return err
			}
			return o.Run()
		},
	}

	// Add flags
	o.AddFlags(cmd.Flags())

	return cmd
}

// AddFlags adds flags to the command
func (o *ClusterOptions) AddFlags(flags *pflag.FlagSet) {
	flags.StringVarP(&o.OutputDir, "output-dir", "d", "out", "output directory for generated manifests")
	flags.StringVar(&o.ManifestDir, "manifest-dir", "clusters", "manifests directory name in output")
	flags.StringVarP(&o.BundleGrouping, "bundle-grouping", "b", "flat", "bundle grouping strategy (flat|nested)")
	flags.StringVarP(&o.ApplicationGrouping, "application-grouping", "a", "flat", "application grouping strategy (flat|nested)")
	flags.StringVar(&o.FluxPlacement, "flux-placement", "integrated", "flux placement strategy (integrated|separate)")
	flags.StringVarP(&o.InputDir, "input-dir", "i", "", "input directory for loading app configs (defaults to config file directory)")
}

// Complete completes the options
func (o *ClusterOptions) Complete() error {
	globalOpts := o.Factory.GlobalOptions()

	// Set input directory default
	if o.InputDir == "" {
		o.InputDir = filepath.Dir(o.ConfigFile)
	}

	// Apply dry-run logic
	if globalOpts.DryRun && o.OutputDir == "out" {
		o.OutputDir = "/dev/stdout"
	}

	return nil
}

// Validate validates the options
func (o *ClusterOptions) Validate() error {
	// Validate config file exists
	if _, err := os.Stat(o.ConfigFile); os.IsNotExist(err) {
		return errors.NewFileError("read", o.ConfigFile, "file does not exist", errors.ErrFileNotFound)
	}

	// Validate grouping options
	validGroupings := []string{"flat", "nested"}
	if !contains(validGroupings, o.BundleGrouping) {
		return errors.NewValidationError("bundle-grouping", o.BundleGrouping, "Options", validGroupings)
	}
	if !contains(validGroupings, o.ApplicationGrouping) {
		return errors.NewValidationError("application-grouping", o.ApplicationGrouping, "Options", validGroupings)
	}

	// Validate flux placement
	validPlacements := []string{"integrated", "separate"}
	if !contains(validPlacements, o.FluxPlacement) {
		return errors.NewValidationError("flux-placement", o.FluxPlacement, "Options", validPlacements)
	}

	return nil
}

// Run executes the cluster command
func (o *ClusterOptions) Run() error {
	globalOpts := o.Factory.GlobalOptions()

	if globalOpts.Verbose {
		fmt.Fprintf(o.IOStreams.ErrOut, "Processing cluster config: %s\n", o.ConfigFile)
	}

	// Load cluster configuration
	cluster, err := o.loadClusterConfig()
	if err != nil {
		return errors.Wrapf(err, "failed to load cluster config")
	}

	// Load applications from input directory
	if err := o.loadClusterApps(cluster); err != nil {
		return errors.Wrapf(err, "failed to load cluster apps")
	}

	// Generate layout
	rules := o.buildLayoutRules(cluster)
	ml, err := o.generateLayout(cluster, rules)
	if err != nil {
		return errors.Wrapf(err, "failed to generate layout")
	}

	// Write output
	if err := o.writeOutput(ml); err != nil {
		return errors.Wrapf(err, "failed to write output")
	}

	if globalOpts.Verbose {
		fmt.Fprintf(o.IOStreams.ErrOut, "Generated cluster manifests: %s\n", o.OutputDir)
	}

	return nil
}

// loadClusterConfig loads and parses the cluster configuration file
func (o *ClusterOptions) loadClusterConfig() (*stack.Cluster, error) {
	file, err := os.Open(o.ConfigFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	dec := yaml.NewDecoder(file)
	var cluster stack.Cluster
	if err := dec.Decode(&cluster); err != nil {
		return nil, err
	}

	return &cluster, nil
}

// loadClusterApps loads application configurations for the cluster
func (o *ClusterOptions) loadClusterApps(cluster *stack.Cluster) error {
	if cluster.Node == nil {
		return errors.NewValidationError("cluster.node", "nil", "Expected", []string{"non-nil cluster node"})
	}

	// Create root bundle
	rootBundle, err := stack.NewBundle(cluster.Node.Name, nil, nil)
	if err != nil {
		return err
	}
	cluster.Node.Bundle = rootBundle

	// Process child nodes
	for _, child := range cluster.Node.Children {
		child.SetParent(cluster.Node)
		childBundle, err := stack.NewBundle(child.Name, nil, nil)
		if err != nil {
			return err
		}
		child.Bundle = childBundle
		childBundle.SetParent(rootBundle)

		// Load apps for this node
		if err := o.loadNodeApps(child); err != nil {
			return errors.Wrapf(err, "failed to load apps for node %s", child.Name)
		}
	}

	return nil
}

// loadNodeApps loads application configs for a specific node
func (o *ClusterOptions) loadNodeApps(node *stack.Node) error {
	nodeDir := filepath.Join(o.InputDir, node.Name)
	entries, err := os.ReadDir(nodeDir)
	if err != nil {
		// Directory might not exist for some nodes
		return nil
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		ext := filepath.Ext(entry.Name())
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		appConfigPath := filepath.Join(nodeDir, entry.Name())
		if err := o.loadAppConfig(node, appConfigPath); err != nil {
			return errors.NewFileError("read", appConfigPath, "failed to load app config", err)
		}
	}

	return nil
}

// loadAppConfig loads a single application configuration
func (o *ClusterOptions) loadAppConfig(node *stack.Node, configPath string) error {
	file, err := os.Open(configPath)
	if err != nil {
		return err
	}
	defer file.Close()

	dec := yaml.NewDecoder(file)
	for {
		var wrapper stack.ApplicationWrapper
		if err := dec.Decode(&wrapper); err != nil {
			if err.Error() == "EOF" {
				break
			}
			return err
		}

		app := wrapper.ToApplication()
		bundle, err := stack.NewBundle(wrapper.Metadata.Name, []*stack.Application{app}, nil)
		if err != nil {
			return err
		}
		bundle.SetParent(node.Bundle)

		childNode := &stack.Node{Name: wrapper.Metadata.Name, Bundle: bundle}
		childNode.SetParent(node)
		node.Children = append(node.Children, childNode)
	}

	return nil
}

// buildLayoutRules creates layout rules from options
func (o *ClusterOptions) buildLayoutRules(cluster *stack.Cluster) layout.LayoutRules {
	rules := layout.DefaultLayoutRules()
	rules.ClusterName = cluster.Name

	// Set grouping strategies
	switch o.BundleGrouping {
	case "nested":
		rules.BundleGrouping = layout.GroupByName
	default:
		rules.BundleGrouping = layout.GroupFlat
	}

	switch o.ApplicationGrouping {
	case "nested":
		rules.ApplicationGrouping = layout.GroupByName
	default:
		rules.ApplicationGrouping = layout.GroupFlat
	}

	// Set flux placement
	switch o.FluxPlacement {
	case "separate":
		rules.FluxPlacement = layout.FluxSeparate
	default:
		rules.FluxPlacement = layout.FluxIntegrated
	}

	return rules
}

// generateLayout generates the manifest layout
func (o *ClusterOptions) generateLayout(cluster *stack.Cluster, rules layout.LayoutRules) (*layout.ManifestLayout, error) {
	// Determine GitOps provider from cluster config
	provider := "flux" // default
	if cluster.GitOps != nil && cluster.GitOps.Type != "" {
		provider = cluster.GitOps.Type
	}

	// Create workflow using the interface
	wf, err := stack.NewWorkflow(provider)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create workflow for provider %s", provider)
	}

	result, err := wf.CreateLayoutWithResources(cluster, rules)
	if err != nil {
		return nil, err
	}

	ml, ok := result.(*layout.ManifestLayout)
	if !ok {
		return nil, errors.NewValidationError("result", "interface{}", "Expected", []string{"*layout.ManifestLayout"})
	}

	return ml, nil
}

// writeOutput writes the generated manifests to output
func (o *ClusterOptions) writeOutput(ml *layout.ManifestLayout) error {
	globalOpts := o.Factory.GlobalOptions()

	if globalOpts.DryRun {
		// For dry-run, print to stdout
		return o.printToStdout(ml)
	}

	// Clean and create output directory
	if err := os.RemoveAll(o.OutputDir); err != nil {
		return err
	}

	// Write manifests
	cfg := layout.Config{ManifestsDir: o.ManifestDir}
	return layout.WriteManifest(o.OutputDir, cfg, ml)
}

// printToStdout prints the manifests to stdout for dry-run
func (o *ClusterOptions) printToStdout(ml *layout.ManifestLayout) error {
	// This is a simplified version - in a real implementation,
	// you'd want to serialize all the resources in the layout
	fmt.Fprintf(o.IOStreams.Out, "# Generated cluster manifests for: %s\n", ml.Name)
	fmt.Fprintf(o.IOStreams.Out, "# Namespace: %s\n", ml.Namespace)
	fmt.Fprintf(o.IOStreams.Out, "# Resources: %d\n", len(ml.Resources))

	// Print basic info about resources
	for _, resource := range ml.Resources {
		if namedObj, ok := resource.(interface {
			GetKind() string
			GetName() string
			GetNamespace() string
		}); ok {
			fmt.Fprintf(o.IOStreams.Out, "# - %s/%s (%s)\n",
				namedObj.GetKind(), namedObj.GetName(), namedObj.GetNamespace())
		}
	}

	return nil
}

// Helper functions
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
