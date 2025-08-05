package generate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"gopkg.in/yaml.v3"

	"github.com/go-kure/kure/pkg/cli"
	"github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"
	"github.com/go-kure/kure/pkg/stack/argocd"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
)

// BootstrapOptions contains options for the bootstrap command
type BootstrapOptions struct {
	// Input options
	ConfigFile string
	
	// Output options
	OutputDir   string
	ManifestDir string
	
	// Bootstrap options
	GitOpsType string
	FluxMode   string
	
	// Dependencies
	Factory   cli.Factory
	IOStreams cli.IOStreams
}

// NewBootstrapCommand creates the bootstrap subcommand
func NewBootstrapCommand(factory cli.Factory) *cobra.Command {
	o := &BootstrapOptions{
		Factory:   factory,
		IOStreams: factory.IOStreams(),
	}

	cmd := &cobra.Command{
		Use:   "bootstrap [flags] CONFIG_FILE",
		Short: "Generate bootstrap configurations for GitOps tools",
		Long: `Generate bootstrap configurations for GitOps tools like Flux or ArgoCD.

This command processes bootstrap configuration files and generates the necessary
manifests to bootstrap a GitOps workflow in a Kubernetes cluster.

Examples:
  # Generate Flux bootstrap configuration
  kure generate bootstrap examples/bootstrap/flux-operator.yaml

  # Generate ArgoCD bootstrap configuration  
  kure generate bootstrap examples/bootstrap/argocd.yaml

  # Generate with custom output directory
  kure generate bootstrap --output-dir ./bootstrap cluster.yaml

  # Generate with specific GitOps type
  kure generate bootstrap --gitops-type=flux --flux-mode=operator bootstrap.yaml`,
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
func (o *BootstrapOptions) AddFlags(flags *pflag.FlagSet) {
	flags.StringVarP(&o.OutputDir, "output-dir", "d", "out/bootstrap", "output directory for generated manifests")
	flags.StringVarP(&o.ManifestDir, "manifest-dir", "m", "", "manifests directory name in output")
	flags.StringVarP(&o.GitOpsType, "gitops-type", "g", "", "GitOps tool type (flux|argocd) - auto-detected if not specified")
	flags.StringVar(&o.FluxMode, "flux-mode", "", "Flux installation mode (operator|toolkit) - auto-detected if not specified")
}

// Complete completes the options
func (o *BootstrapOptions) Complete() error {
	globalOpts := o.Factory.GlobalOptions()
	
	// Apply dry-run logic
	if globalOpts.DryRun && o.OutputDir == "out/bootstrap" {
		o.OutputDir = "/dev/stdout"
	}
	
	return nil
}

// Validate validates the options
func (o *BootstrapOptions) Validate() error {
	// Validate config file exists
	if _, err := os.Stat(o.ConfigFile); os.IsNotExist(err) {
		return errors.NewFileError("read", o.ConfigFile, "file does not exist", errors.ErrFileNotFound)
	}
	
	// Validate GitOps type if specified
	if o.GitOpsType != "" {
		validTypes := []string{"flux", "argocd"}
		if !contains(validTypes, o.GitOpsType) {
			return errors.NewValidationError("gitops-type", o.GitOpsType, "Options", validTypes)
		}
	}
	
	// Validate Flux mode if specified
	if o.FluxMode != "" {
		validModes := []string{"operator", "toolkit"}
		if !contains(validModes, o.FluxMode) {
			return errors.NewValidationError("flux-mode", o.FluxMode, "Options", validModes)
		}
	}
	
	return nil
}

// Run executes the bootstrap command
func (o *BootstrapOptions) Run() error {
	globalOpts := o.Factory.GlobalOptions()
	
	if globalOpts.Verbose {
		fmt.Fprintf(o.IOStreams.ErrOut, "Processing bootstrap config: %s\n", o.ConfigFile)
	}
	
	// Load cluster configuration
	cluster, err := o.loadClusterConfig()
	if err != nil {
		return errors.Wrapf(err, "failed to load cluster config")
	}
	
	// Detect GitOps type and mode if not specified
	if err := o.detectGitOpsSettings(cluster); err != nil {
		return errors.Wrapf(err, "failed to detect GitOps settings")
	}
	
	// Generate bootstrap manifests
	ml, err := o.generateBootstrap(cluster)
	if err != nil {
		return errors.Wrapf(err, "failed to generate bootstrap manifests")
	}
	
	// Write output
	if err := o.writeOutput(ml, cluster); err != nil {
		return errors.Wrapf(err, "failed to write output")
	}
	
	if globalOpts.Verbose {
		fmt.Fprintf(o.IOStreams.ErrOut, "Generated bootstrap manifests: %s\n", o.OutputDir)
		if cluster.GitOps != nil && cluster.GitOps.Bootstrap != nil {
			fmt.Fprintf(o.IOStreams.ErrOut, "GitOps type: %s\n", o.GitOpsType)
			if o.GitOpsType == "flux" {
				fmt.Fprintf(o.IOStreams.ErrOut, "Flux mode: %s\n", cluster.GitOps.Bootstrap.FluxMode)
			}
		}
	}
	
	return nil
}

// loadClusterConfig loads and parses the cluster configuration file
func (o *BootstrapOptions) loadClusterConfig() (*stack.Cluster, error) {
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

	// Ensure basic structure for bootstrap
	if cluster.Node == nil {
		cluster.Node = &stack.Node{Name: "flux-system"}
	}
	if cluster.Node.Bundle == nil {
		cluster.Node.Bundle = &stack.Bundle{Name: "infrastructure"}
	}

	return &cluster, nil
}

// detectGitOpsSettings detects GitOps type and mode from configuration
func (o *BootstrapOptions) detectGitOpsSettings(cluster *stack.Cluster) error {
	// Override with command line flags if specified
	if o.GitOpsType != "" {
		return nil
	}
	
	// Auto-detect from cluster configuration
	if cluster.GitOps != nil {
		o.GitOpsType = cluster.GitOps.Type
		
		if cluster.GitOps.Bootstrap != nil {
			if o.FluxMode == "" {
				o.FluxMode = cluster.GitOps.Bootstrap.FluxMode
			}
		}
	}
	
	// Default to flux if not specified
	if o.GitOpsType == "" {
		o.GitOpsType = "flux"
	}
	
	// Default flux mode to operator if not specified
	if o.GitOpsType == "flux" && o.FluxMode == "" {
		o.FluxMode = "operator"
	}
	
	return nil
}

// generateBootstrap generates bootstrap manifests based on GitOps type
func (o *BootstrapOptions) generateBootstrap(cluster *stack.Cluster) (*layout.ManifestLayout, error) {
	rules := layout.DefaultLayoutRules()
	rules.FluxPlacement = layout.FluxSeparate
	
	switch o.GitOpsType {
	case "argocd":
		return o.generateArgoCDBootstrap(cluster, rules)
	case "flux":
		return o.generateFluxBootstrap(cluster, rules)
	default:
		return nil, errors.NewValidationError("gitops-type", o.GitOpsType, "Supported types", []string{"flux", "argocd"})
	}
}

// generateArgoCDBootstrap generates ArgoCD bootstrap manifests
func (o *BootstrapOptions) generateArgoCDBootstrap(cluster *stack.Cluster, rules layout.LayoutRules) (*layout.ManifestLayout, error) {
	wf := argocd.Engine()
	
	// Generate bootstrap resources directly
	bootstrapObjs, err := wf.GenerateBootstrap(cluster.GitOps.Bootstrap, cluster.Node)
	if err != nil {
		return nil, err
	}
	
	// Create a basic manifest layout for ArgoCD
	ml := &layout.ManifestLayout{
		Name:      cluster.Node.Name,
		Namespace: cluster.Name,
		Resources: bootstrapObjs,
	}
	
	return ml, nil
}

// generateFluxBootstrap generates Flux bootstrap manifests
func (o *BootstrapOptions) generateFluxBootstrap(cluster *stack.Cluster, rules layout.LayoutRules) (*layout.ManifestLayout, error) {
	wf := fluxstack.Engine()
	return wf.CreateLayoutWithResources(cluster, rules)
}

// writeOutput writes the generated manifests to output
func (o *BootstrapOptions) writeOutput(ml *layout.ManifestLayout, cluster *stack.Cluster) error {
	globalOpts := o.Factory.GlobalOptions()
	
	if globalOpts.DryRun {
		return o.printToStdout(ml)
	}

	// Determine output directory structure
	configBaseName := strings.TrimSuffix(filepath.Base(o.ConfigFile), filepath.Ext(o.ConfigFile))
	outputDir := filepath.Join(o.OutputDir, configBaseName)
	
	// Clean and create output directory
	if err := os.RemoveAll(outputDir); err != nil {
		return err
	}

	// Set manifest directory from config or use default
	manifestDir := o.ManifestDir
	if manifestDir == "" {
		manifestDir = ""
	}

	// Write manifests
	cfg := layout.Config{ManifestsDir: manifestDir}
	return layout.WriteManifest(outputDir, cfg, ml)
}

// printToStdout prints the manifests to stdout for dry-run
func (o *BootstrapOptions) printToStdout(ml *layout.ManifestLayout) error {
	fmt.Fprintf(o.IOStreams.Out, "# Generated bootstrap manifests for: %s\n", ml.Name)
	fmt.Fprintf(o.IOStreams.Out, "# GitOps type: %s\n", o.GitOpsType)
	if o.GitOpsType == "flux" {
		fmt.Fprintf(o.IOStreams.Out, "# Flux mode: %s\n", o.FluxMode)
	}
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