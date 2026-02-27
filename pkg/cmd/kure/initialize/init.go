package initialize

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/go-kure/kure/pkg/cli"
	"github.com/go-kure/kure/pkg/cmd/shared/options"
	"github.com/go-kure/kure/pkg/errors"
)

// dnsNameRegex validates RFC 1123 DNS label names.
var dnsNameRegex = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

// InitOptions contains options for the init command.
type InitOptions struct {
	ProjectName string
	OutputDir   string
	GitOpsType  string

	Factory   cli.Factory
	IOStreams cli.IOStreams
}

// NewInitCommand creates the init subcommand.
func NewInitCommand(globalOpts *options.GlobalOptions) *cobra.Command {
	factory := cli.NewFactory(globalOpts)
	o := &InitOptions{
		Factory:   factory,
		IOStreams: factory.IOStreams(),
	}

	cmd := &cobra.Command{
		Use:   "init [PROJECT_NAME] [flags]",
		Short: "Scaffold a new kure project",
		Long: `Scaffold a new kure project with cluster configuration and example application templates.

Creates a directory structure ready for use with "kure generate cluster":
  cluster.yaml    - Cluster configuration
  apps/           - Application definitions
  infra/          - Infrastructure definitions

Examples:
  # Scaffold in current directory using directory name as project name
  kure init

  # Scaffold with explicit project name
  kure init my-cluster

  # Scaffold for ArgoCD
  kure init my-cluster --gitops argocd

  # Scaffold in a specific directory
  kure init my-cluster --dir /path/to/project`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				o.ProjectName = args[0]
			}

			if err := o.Complete(); err != nil {
				return err
			}
			if err := o.Validate(); err != nil {
				return err
			}
			return o.Run()
		},
	}

	o.AddFlags(cmd.Flags())

	return cmd
}

// AddFlags adds flags to the command.
func (o *InitOptions) AddFlags(flags *pflag.FlagSet) {
	flags.StringVar(&o.OutputDir, "dir", ".", "target directory")
	flags.StringVar(&o.GitOpsType, "gitops", "flux", "gitops tool: flux or argocd")
}

// Complete fills in defaults for unset fields.
func (o *InitOptions) Complete() error {
	// Resolve output directory to absolute path
	absDir, err := filepath.Abs(o.OutputDir)
	if err != nil {
		return errors.Wrapf(err, "failed to resolve output directory")
	}
	o.OutputDir = absDir

	// Default project name to directory basename
	if o.ProjectName == "" {
		o.ProjectName = filepath.Base(o.OutputDir)
	}

	return nil
}

// Validate checks that all options are valid.
func (o *InitOptions) Validate() error {
	if !dnsNameRegex.MatchString(o.ProjectName) {
		return errors.NewValidationError(
			"project-name", o.ProjectName, "init",
			[]string{"must be a valid DNS label: lowercase alphanumeric and hyphens, e.g. my-cluster"},
		)
	}

	validGitOps := []string{"flux", "argocd"}
	if !contains(validGitOps, o.GitOpsType) {
		return errors.NewValidationError("gitops", o.GitOpsType, "init", validGitOps)
	}

	// Refuse to overwrite existing cluster.yaml
	clusterFile := filepath.Join(o.OutputDir, "cluster.yaml")
	if _, err := os.Stat(clusterFile); err == nil {
		return errors.NewFileError("init", clusterFile, "file already exists; remove it first to re-initialize", nil)
	}

	return nil
}

// Run executes the init command.
func (o *InitOptions) Run() error {
	// Create subdirectories
	for _, dir := range []string{"apps", "infra"} {
		dirPath := filepath.Join(o.OutputDir, dir)
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			return errors.Wrapf(err, "failed to create directory %s", dir)
		}
	}

	// Write cluster.yaml
	clusterContent := fmt.Sprintf(clusterTemplate, o.ProjectName, o.GitOpsType)
	clusterFile := filepath.Join(o.OutputDir, "cluster.yaml")
	if err := os.WriteFile(clusterFile, []byte(clusterContent), 0644); err != nil {
		return errors.Wrapf(err, "failed to write cluster.yaml")
	}

	// Write apps/example.yaml
	exampleContent := appExampleTemplate
	exampleFile := filepath.Join(o.OutputDir, "apps", "example.yaml")
	if err := os.WriteFile(exampleFile, []byte(exampleContent), 0644); err != nil {
		return errors.Wrapf(err, "failed to write apps/example.yaml")
	}

	// Print summary
	_, _ = fmt.Fprintf(o.IOStreams.ErrOut, "Initialized kure project %q in %s\n", o.ProjectName, o.OutputDir)
	_, _ = fmt.Fprintf(o.IOStreams.ErrOut, "  created cluster.yaml\n")
	_, _ = fmt.Fprintf(o.IOStreams.ErrOut, "  created apps/example.yaml\n")
	_, _ = fmt.Fprintf(o.IOStreams.ErrOut, "  created apps/\n")
	_, _ = fmt.Fprintf(o.IOStreams.ErrOut, "  created infra/\n")
	_, _ = fmt.Fprintf(o.IOStreams.ErrOut, "\nNext steps:\n")
	_, _ = fmt.Fprintf(o.IOStreams.ErrOut, "  Edit apps/example.yaml or add more app configs under apps/\n")
	_, _ = fmt.Fprintf(o.IOStreams.ErrOut, "  Run: kure generate cluster cluster.yaml\n")

	return nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// Templates — simple runtime format consumed by "kure generate cluster".

const clusterTemplate = `name: %s
gitops:
  type: %s
  bootstrap:
    enabled: true
    sourceURL: "oci://ghcr.io/example/cluster-manifests"
    sourceRef: "v1.0.0"
node:
  name: flux-system
  children:
    - name: apps
    - name: infra
`

const appExampleTemplate = `apiVersion: generators.gokure.dev/v1alpha1
kind: AppWorkload
metadata:
  name: example
  namespace: apps
spec:
  workload: Deployment
  replicas: 1
  containers:
    - name: app
      image: nginx:1.27-alpine
      ports:
        - name: http
          containerPort: 80
      resources:
        requests:
          memory: 64Mi
          cpu: 50m
        limits:
          memory: 128Mi
          cpu: 100m
`
