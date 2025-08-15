package kurel

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/go-kure/kure/pkg/cmd/shared"
	"github.com/go-kure/kure/pkg/cmd/shared/options"
	"github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/launcher"
	"github.com/go-kure/kure/pkg/logger"
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
	var (
		outputPath   string
		valuesFile   string
		patches      []string
		outputFormat string
		filterKind   string
		filterName   string
		addLabels    map[string]string
	)

	cmd := &cobra.Command{
		Use:   "build <package>",
		Short: "Build Kubernetes manifests from kurel package",
		Long: `Build generates Kubernetes manifests from a kurel package.

The build command processes the package structure, applies patches based on
configuration, and outputs phase-organized manifests ready for GitOps deployment.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Setup logger
			log := logger.Default()
			if globalOpts.Verbose {
				log = logger.Default()
			} else {
				log = logger.Noop()
			}

			// Create launcher components
			loader := launcher.NewPackageLoader(log)
			resolver := launcher.NewResolver(log)
			processor := launcher.NewPatchProcessor(log, resolver)
			builder := launcher.NewBuilder(log)
			extension := launcher.NewExtensionLoader(log)

			// Create launcher options
			opts := launcher.DefaultOptions()
			opts.Logger = log
			opts.Verbose = globalOpts.Verbose

			// Load package
			ctx := context.Background()
			packagePath := args[0]

			def, err := loader.LoadDefinition(ctx, packagePath, opts)
			if err != nil {
				return errors.Wrap(err, "failed to load package")
			}

			// Load values if provided
			userValues := make(launcher.ParameterMap)
			if valuesFile != "" {
				data, err := os.ReadFile(valuesFile)
				if err != nil {
					return errors.NewFileError("read", valuesFile, "failed to read values file", err)
				}
				if err := yaml.Unmarshal(data, &userValues); err != nil {
					return errors.NewParseError(valuesFile, "invalid YAML", 0, 0, err)
				}
			}

			// Apply extensions
			def, err = extension.LoadWithExtensions(ctx, def, "", opts)
			if err != nil {
				return errors.Wrap(err, "failed to load extensions")
			}

			// Resolve patch dependencies
			enabledPatches, err := processor.ResolveDependencies(ctx, def.Patches, def.Parameters)
			if err != nil {
				return errors.Wrap(err, "failed to resolve patch dependencies")
			}

			// Create package instance
			instance := &launcher.PackageInstance{
				Definition:     def,
				UserValues:     userValues,
				EnabledPatches: enabledPatches,
			}

			// Configure build options
			buildOpts := launcher.BuildOptions{
				FilterKind: filterKind,
				FilterName: filterName,
				AddLabels:  addLabels,
			}

			// Determine output
			if outputPath != "" {
				if info, err := os.Stat(outputPath); err == nil && info.IsDir() {
					buildOpts.Output = launcher.OutputDirectory
				} else {
					buildOpts.Output = launcher.OutputFile
				}
				buildOpts.OutputPath = outputPath
			} else {
				buildOpts.Output = launcher.OutputStdout
			}

			// Set format
			switch strings.ToLower(outputFormat) {
			case "json":
				buildOpts.Format = launcher.FormatJSON
			default:
				buildOpts.Format = launcher.FormatYAML
			}

			// Build manifests
			return builder.Build(ctx, instance, buildOpts, opts)
		},
	}

	// Add flags
	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "Output path (default: stdout)")
	cmd.Flags().StringVar(&valuesFile, "values", "", "Values file for parameter overrides")
	cmd.Flags().StringSliceVarP(&patches, "patch", "p", nil, "Enable specific patches")
	cmd.Flags().StringVar(&outputFormat, "format", "yaml", "Output format (yaml, json)")
	cmd.Flags().StringVar(&filterKind, "kind", "", "Filter by resource kind")
	cmd.Flags().StringVar(&filterName, "name", "", "Filter by resource name")
	cmd.Flags().StringToStringVar(&addLabels, "add-label", nil, "Add labels to all resources")

	return cmd
}

func newValidateCommand(globalOpts *options.GlobalOptions) *cobra.Command {
	var (
		valuesFile string
		schemaFile string
		outputJSON bool
	)

	cmd := &cobra.Command{
		Use:   "validate <package>",
		Short: "Validate kurel package structure and configuration",
		Long: `Validate checks the kurel package for structural correctness,
parameter validation, and patch consistency.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Setup logger
			log := logger.Default()
			if !globalOpts.Verbose {
				log = logger.Noop()
			}

			// Create launcher components
			loader := launcher.NewPackageLoader(log)
			validator := launcher.NewValidator(log)

			// Create launcher options
			opts := launcher.DefaultOptions()
			opts.Logger = log
			opts.Verbose = globalOpts.Verbose
			opts.StrictMode = globalOpts.Strict

			// Load package
			ctx := context.Background()
			packagePath := args[0]

			def, err := loader.LoadDefinition(ctx, packagePath, opts)
			if err != nil {
				return errors.Wrap(err, "failed to load package")
			}

			// Load values if provided
			if valuesFile != "" {
				var userValues launcher.ParameterMap
				data, err := os.ReadFile(valuesFile)
				if err != nil {
					return errors.NewFileError("read", valuesFile, "failed to read values file", err)
				}
				if err := yaml.Unmarshal(data, &userValues); err != nil {
					return errors.NewParseError(valuesFile, "invalid YAML", 0, 0, err)
				}
				// Merge user values into parameters for validation
				for k, v := range userValues {
					def.Parameters[k] = v
				}
			}

			// Validate package
			result, err := validator.ValidatePackage(ctx, def)
			if err != nil {
				return errors.Wrap(err, "validation failed")
			}

			// Output results
			if outputJSON {
				encoder := json.NewEncoder(os.Stdout)
				encoder.SetIndent("", "  ")
				return encoder.Encode(result)
			}

			// Text output
			if len(result.Errors) > 0 {
				fmt.Printf("Errors (%d):\n", len(result.Errors))
				for _, e := range result.Errors {
					fmt.Printf("  ✗ %s\n", e.Error())
				}
				fmt.Println()
			}

			if len(result.Warnings) > 0 {
				fmt.Printf("Warnings (%d):\n", len(result.Warnings))
				for _, w := range result.Warnings {
					fmt.Printf("  ⚠ %s\n", w.String())
				}
				fmt.Println()
			}

			if result.IsValid() {
				fmt.Println("✓ Package is valid")
			} else {
				fmt.Println("✗ Package has errors")
				if opts.StrictMode {
					return errors.New("validation failed")
				}
			}

			return nil
		},
	}

	// Add flags
	cmd.Flags().StringVar(&valuesFile, "values", "", "Values file for validation")
	cmd.Flags().StringVar(&schemaFile, "schema", "", "Custom schema file")
	cmd.Flags().BoolVar(&outputJSON, "json", false, "Output validation results as JSON")

	return cmd
}

func newInfoCommand(globalOpts *options.GlobalOptions) *cobra.Command {
	var (
		outputFormat string
		showAll      bool
	)

	cmd := &cobra.Command{
		Use:   "info <package>",
		Short: "Show package information",
		Long: `Info displays detailed information about a kurel package including
metadata, available patches, configurable parameters, and deployment phases.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Setup logger
			log := logger.Default()
			if !globalOpts.Verbose {
				log = logger.Noop()
			}

			// Create launcher components
			loader := launcher.NewPackageLoader(log)

			// Create launcher options
			opts := launcher.DefaultOptions()
			opts.Logger = log
			opts.Verbose = globalOpts.Verbose

			// Load package
			ctx := context.Background()
			packagePath := args[0]

			def, err := loader.LoadDefinition(ctx, packagePath, opts)
			if err != nil {
				return errors.Wrap(err, "failed to load package")
			}

			// Format output
			switch outputFormat {
			case "json":
				encoder := json.NewEncoder(os.Stdout)
				encoder.SetIndent("", "  ")
				return encoder.Encode(def)

			case "yaml":
				encoder := yaml.NewEncoder(os.Stdout)
				encoder.SetIndent(2)
				return encoder.Encode(def)

			default: // text
				fmt.Printf("Package: %s\n", def.Metadata.Name)
				fmt.Printf("Version: %s\n", def.Metadata.Version)
				if def.Metadata.Description != "" {
					fmt.Printf("Description: %s\n", def.Metadata.Description)
				}
				fmt.Println()

				if len(def.Parameters) > 0 {
					fmt.Printf("Parameters (%d):\n", len(def.Parameters))
					for k, v := range def.Parameters {
						fmt.Printf("  %s: %v\n", k, formatParameterValue(v))
					}
					fmt.Println()
				}

				fmt.Printf("Resources (%d):\n", len(def.Resources))
				for _, r := range def.Resources {
					fmt.Printf("  - %s/%s", r.Kind, r.GetName())
					if ns := r.GetNamespace(); ns != "" {
						fmt.Printf(" (namespace: %s)", ns)
					}
					fmt.Println()
				}

				if len(def.Patches) > 0 {
					fmt.Printf("\nPatches (%d):\n", len(def.Patches))
					for _, p := range def.Patches {
						fmt.Printf("  - %s", p.Name)
						if p.Metadata != nil && p.Metadata.Description != "" {
							fmt.Printf(": %s", p.Metadata.Description)
						}
						if p.Metadata != nil && p.Metadata.Enabled != "" {
							fmt.Printf(" [enabled: %s]", p.Metadata.Enabled)
						}
						fmt.Println()
					}
				}
			}

			return nil
		},
	}

	// Add flags
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text, yaml, json)")
	cmd.Flags().BoolVar(&showAll, "all", false, "Show all details including resource content")

	return cmd
}

func newSchemaCommand(globalOpts *options.GlobalOptions) *cobra.Command {
	schemaCmd := &cobra.Command{
		Use:   "schema",
		Short: "Schema generation and validation commands",
		Long:  "Manage JSON schemas for kurel package validation",
	}

	// Add generate subcommand
	generateCmd := &cobra.Command{
		Use:   "generate <package>",
		Short: "Generate JSON schema for package parameters",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var (
				outputPath  string
				includeK8s  bool
				prettyPrint bool
			)

			// Get flags from command
			outputPath, _ = cmd.Flags().GetString("output")
			includeK8s, _ = cmd.Flags().GetBool("k8s")
			prettyPrint, _ = cmd.Flags().GetBool("pretty")

			// Setup logger
			log := logger.Default()
			if !globalOpts.Verbose {
				log = logger.Noop()
			}

			// Create launcher components
			schemaGen := launcher.NewSchemaGenerator(log)

			// Generate schema
			ctx := context.Background()
			schema, err := schemaGen.GeneratePackageSchemaWithOptions(ctx, &launcher.SchemaOptions{
				IncludeK8s: includeK8s,
			})
			if err != nil {
				return errors.Wrap(err, "failed to generate schema")
			}

			// Export to JSON
			data, err := schemaGen.ExportSchema(schema)
			if err != nil {
				return errors.Wrap(err, "failed to export schema")
			}

			// Pretty print if requested
			if prettyPrint {
				var obj interface{}
				if err := json.Unmarshal(data, &obj); err == nil {
					if pretty, err := json.MarshalIndent(obj, "", "  "); err == nil {
						data = pretty
					}
				}
			}

			// Write output
			if outputPath != "" {
				dir := filepath.Dir(outputPath)
				if err := os.MkdirAll(dir, 0755); err != nil {
					return errors.Wrap(err, "failed to create output directory")
				}
				return os.WriteFile(outputPath, data, 0644)
			}

			_, err = os.Stdout.Write(data)
			return err
		},
	}

	// Add flags to generate command
	generateCmd.Flags().StringP("output", "o", "", "Output path (default: stdout)")
	generateCmd.Flags().Bool("k8s", false, "Include Kubernetes resource schemas")
	generateCmd.Flags().Bool("pretty", true, "Pretty print JSON output")

	schemaCmd.AddCommand(generateCmd)

	return schemaCmd
}

// formatParameterValue formats a parameter value for display
func formatParameterValue(v interface{}) string {
	switch val := v.(type) {
	case string:
		if strings.Contains(val, "\n") {
			return "(multiline)"
		}
		return val
	case map[string]interface{}:
		return fmt.Sprintf("(map with %d keys)", len(val))
	case []interface{}:
		return fmt.Sprintf("(array with %d items)", len(val))
	default:
		return fmt.Sprintf("%v", val)
	}
}

func newConfigCommand(globalOpts *options.GlobalOptions) *cobra.Command {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Manage kurel configuration",
		Long:  "View and modify kurel configuration settings",
	}

	// Add view subcommand
	configCmd.AddCommand(&cobra.Command{
		Use:   "view",
		Short: "View current configuration",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Configuration:")
			fmt.Printf("  Verbose: %v\n", globalOpts.Verbose)
			fmt.Printf("  Debug: %v\n", globalOpts.Debug)
			fmt.Printf("  Strict: %v\n", globalOpts.Strict)
			fmt.Printf("  Config File: %s\n", globalOpts.ConfigFile)
		},
	})

	// Add init subcommand
	configCmd.AddCommand(&cobra.Command{
		Use:   "init",
		Short: "Initialize configuration file",
		RunE: func(cmd *cobra.Command, args []string) error {
			configPath := filepath.Join(".kurel", "config.yaml")
			if globalOpts.ConfigFile != "" {
				configPath = globalOpts.ConfigFile
			}

			dir := filepath.Dir(configPath)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return errors.Wrap(err, "failed to create config directory")
			}

			defaultConfig := `# Kurel Configuration
# For more information, see: https://github.com/go-kure/kure

# Global settings
verbose: false
debug: false
strict: false

# Launcher settings
launcher:
  maxDepth: 10
  timeout: 30s
  cacheEnabled: true
  cacheTTL: 1h

# Extension settings
extensions:
  enabled: true
  searchPaths:
    - .
    - ~/.kurel/extensions
`
			if err := os.WriteFile(configPath, []byte(defaultConfig), 0644); err != nil {
				return errors.Wrap(err, "failed to write config file")
			}

			fmt.Printf("Configuration initialized at %s\n", configPath)
			return nil
		},
	})

	return configCmd
}
