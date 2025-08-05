package generate

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/cli"
	"github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/generators"
	kio "github.com/go-kure/kure/pkg/io"
)

// AppOptions contains options for the app command
type AppOptions struct {
	// Input options
	ConfigFiles []string
	InputDir    string
	
	// Output options
	OutputDir  string
	OutputFile string
	
	// Dependencies
	Factory   cli.Factory
	IOStreams cli.IOStreams
}

// NewAppCommand creates the app subcommand
func NewAppCommand(factory cli.Factory) *cobra.Command {
	o := &AppOptions{
		Factory:   factory,
		IOStreams: factory.IOStreams(),
	}

	cmd := &cobra.Command{
		Use:   "app [flags] CONFIG_FILE...",
		Short: "Generate application workload manifests",
		Long: `Generate application workload manifests from configuration files.

This command processes application workload configuration files and generates
Kubernetes manifests for deployments, services, and other application resources.

Examples:
  # Generate from single config file
  kure generate app app-config.yaml

  # Generate from multiple config files
  kure generate app app1.yaml app2.yaml app3.yaml

  # Generate from directory
  kure generate app --input-dir ./apps

  # Generate to specific output file
  kure generate app --output-file manifests.yaml app-config.yaml`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			o.ConfigFiles = args
			
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
func (o *AppOptions) AddFlags(flags *pflag.FlagSet) {
	flags.StringVar(&o.InputDir, "input-dir", "", "input directory containing app config files")
	flags.StringVarP(&o.OutputDir, "output-dir", "d", "out/apps", "output directory for generated manifests")
	flags.StringVarP(&o.OutputFile, "output-file", "f", "", "output file for generated manifests (stdout if not specified)")
}

// Complete completes the options
func (o *AppOptions) Complete() error {
	globalOpts := o.Factory.GlobalOptions()
	
	// If input directory is specified, scan for config files
	if o.InputDir != "" {
		files, err := o.scanInputDirectory()
		if err != nil {
			return errors.Wrapf(err, "failed to scan input directory")
		}
		o.ConfigFiles = append(o.ConfigFiles, files...)
	}
	
	// Apply dry-run logic
	if globalOpts.DryRun && o.OutputFile == "" {
		o.OutputFile = "/dev/stdout"
	}
	
	return nil
}

// Validate validates the options
func (o *AppOptions) Validate() error {
	if len(o.ConfigFiles) == 0 {
		return errors.NewValidationError("config-files", "empty", "Required", []string{"at least one config file or input directory"})
	}
	
	// Validate all config files exist
	for _, file := range o.ConfigFiles {
		if _, err := os.Stat(file); os.IsNotExist(err) {
			return errors.NewFileError("read", file, "file does not exist", errors.ErrFileNotFound)
		}
	}
	
	return nil
}

// Run executes the app command
func (o *AppOptions) Run() error {
	globalOpts := o.Factory.GlobalOptions()
	
	if globalOpts.Verbose {
		fmt.Fprintf(o.IOStreams.ErrOut, "Processing %d app config files\n", len(o.ConfigFiles))
	}
	
	// Load all applications
	apps, err := o.loadApplications()
	if err != nil {
		return errors.Wrapf(err, "failed to load applications")
	}
	
	if len(apps) == 0 {
		fmt.Fprintf(o.IOStreams.ErrOut, "No applications found in config files\n")
		return nil
	}
	
	// Generate manifests
	resources, err := o.generateManifests(apps)
	if err != nil {
		return errors.Wrapf(err, "failed to generate manifests")
	}
	
	// Write output
	if err := o.writeOutput(resources); err != nil {
		return errors.Wrapf(err, "failed to write output")
	}
	
	if globalOpts.Verbose {
		fmt.Fprintf(o.IOStreams.ErrOut, "Generated %d resources for %d applications\n", len(resources), len(apps))
	}
	
	return nil
}

// scanInputDirectory scans the input directory for config files
func (o *AppOptions) scanInputDirectory() ([]string, error) {
	var files []string
	
	err := filepath.Walk(o.InputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		if info.IsDir() {
			return nil
		}
		
		ext := filepath.Ext(info.Name())
		if ext == ".yaml" || ext == ".yml" {
			files = append(files, path)
		}
		
		return nil
	})
	
	return files, err
}

// loadApplications loads all applications from config files
func (o *AppOptions) loadApplications() ([]*stack.Application, error) {
	var apps []*stack.Application
	
	for _, configFile := range o.ConfigFiles {
		fileApps, err := o.loadApplicationsFromFile(configFile)
		if err != nil {
			return nil, errors.NewFileError("read", configFile, "failed to load applications", err)
		}
		apps = append(apps, fileApps...)
	}
	
	return apps, nil
}

// loadApplicationsFromFile loads applications from a single config file
func (o *AppOptions) loadApplicationsFromFile(configFile string) ([]*stack.Application, error) {
	file, err := os.Open(configFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var apps []*stack.Application
	dec := yaml.NewDecoder(file)
	
	for {
		var cfg generators.AppWorkloadConfig
		if err := dec.Decode(&cfg); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		
		app := stack.NewApplication(cfg.Name, cfg.Namespace, &cfg)
		apps = append(apps, app)
	}
	
	return apps, nil
}

// generateManifests generates Kubernetes manifests from applications
func (o *AppOptions) generateManifests(apps []*stack.Application) ([]runtime.Object, error) {
	var allResources []runtime.Object
	
	for _, app := range apps {
		// Create a bundle for the application
		bundle, err := stack.NewBundle(app.Name, []*stack.Application{app}, nil)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to create bundle for app %s", app.Name)
		}
		
		// Generate resources
		resources, err := bundle.Generate()
		if err != nil {
			return nil, errors.Wrapf(err, "failed to generate resources for app %s", app.Name)
		}
		
		// Convert to runtime.Object if needed
		for _, resource := range resources {
			// resource is *client.Object, need to dereference it
			if *resource != nil {
				if runtimeObj, ok := (*resource).(runtime.Object); ok {
					allResources = append(allResources, runtimeObj)
				}
			}
		}
	}
	
	return allResources, nil
}

// writeOutput writes the generated resources to output
func (o *AppOptions) writeOutput(resources []runtime.Object) error {
	globalOpts := o.Factory.GlobalOptions()
	
	// Convert to client.Object pointers
	clientObjects := make([]*client.Object, 0, len(resources))
	for _, resource := range resources {
		if clientObj, ok := resource.(client.Object); ok {
			clientObjects = append(clientObjects, &clientObj)
		}
	}
	
	// Encode resources to YAML
	output, err := kio.EncodeObjectsToYAML(clientObjects)
	if err != nil {
		return errors.Wrapf(err, "failed to encode resources")
	}
	
	// Determine output destination
	if o.OutputFile != "" {
		return o.writeToFile(output)
	}
	
	if globalOpts.DryRun {
		_, err := o.IOStreams.Out.Write(output)
		return err
	}
	
	// Write to directory structure
	return o.writeToDirectory(resources)
}

// writeToFile writes output to a single file
func (o *AppOptions) writeToFile(output []byte) error {
	if o.OutputFile == "/dev/stdout" {
		_, err := o.IOStreams.Out.Write(output)
		return err
	}
	
	// Create directory if needed
	dir := filepath.Dir(o.OutputFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	
	return os.WriteFile(o.OutputFile, output, 0644)
}

// writeToDirectory writes output to organized directory structure
func (o *AppOptions) writeToDirectory(resources []runtime.Object) error {
	// Clean output directory
	if err := os.RemoveAll(o.OutputDir); err != nil {
		return err
	}
	
	// Group resources by application
	appResources := make(map[string][]runtime.Object)
	
	for _, resource := range resources {
		appName := "unknown"
		if namedObj, ok := resource.(interface{ GetName() string }); ok {
			appName = namedObj.GetName()
		}
		appResources[appName] = append(appResources[appName], resource)
	}
	
	// Write each application's resources to separate files
	for appName, appRes := range appResources {
		appDir := filepath.Join(o.OutputDir, appName)
		if err := os.MkdirAll(appDir, 0755); err != nil {
			return err
		}
		
		// Convert to client.Object pointers
		clientObjects := make([]*client.Object, 0, len(appRes))
		for _, resource := range appRes {
			if clientObj, ok := resource.(client.Object); ok {
				clientObjects = append(clientObjects, &clientObj)
			}
		}
		
		output, err := kio.EncodeObjectsToYAML(clientObjects)
		if err != nil {
			return err
		}
		
		fileName := fmt.Sprintf("%s-generated.yaml", appName)
		filePath := filepath.Join(appDir, fileName)
		
		if err := os.WriteFile(filePath, output, 0644); err != nil {
			return err
		}
	}
	
	return nil
}