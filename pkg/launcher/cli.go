package launcher

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/logger"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// CLI provides command-line interface for the launcher
type CLI struct {
	logger    logger.Logger
	loader    PackageLoader
	resolver  Resolver
	processor PatchProcessor
	validator Validator
	builder   Builder
	extension ExtensionLoader
	schema    SchemaGenerator
	outputWriter io.Writer // configurable output writer
}

// NewCLI creates a new CLI instance
func NewCLI(log logger.Logger) *CLI {
	if log == nil {
		log = logger.Default()
	}
	resolver := NewResolver(log)
	return &CLI{
		logger:       log,
		loader:       NewPackageLoader(log),
		resolver:     resolver,
		processor:    NewPatchProcessor(log, resolver),
		validator:    NewValidator(log),
		builder:      NewBuilder(log),
		extension:    NewExtensionLoader(log),
		schema:       NewSchemaGenerator(log),
		outputWriter: os.Stdout, // default to stdout
	}
}

// SetOutputWriter sets the output writer for CLI commands
func (c *CLI) SetOutputWriter(w io.Writer) {
	c.outputWriter = w
	c.builder.SetOutputWriter(w)
}

// RootCommand creates the root command for kurel
func (c *CLI) RootCommand() *cobra.Command {
	var opts LauncherOptions
	
	cmd := &cobra.Command{
		Use:   "kurel",
		Short: "Kurel package system for Kubernetes manifest generation",
		Long: `Kurel is a declarative package system for generating Kubernetes manifests.
It provides strongly-typed builders, variable substitution, patching, and validation.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	// Global flags
	cmd.PersistentFlags().BoolVarP(&opts.Verbose, "verbose", "v", false, "Enable verbose output")
	cmd.PersistentFlags().BoolVar(&opts.Debug, "debug", false, "Enable debug output")
	cmd.PersistentFlags().BoolVar(&opts.StrictMode, "strict", false, "Treat warnings as errors")
	cmd.PersistentFlags().IntVar(&opts.MaxDepth, "max-depth", 10, "Maximum variable substitution depth")
	cmd.PersistentFlags().DurationVar(&opts.Timeout, "timeout", 30*time.Second, "Operation timeout")

	// Add subcommands
	cmd.AddCommand(c.BuildCommand(&opts))
	cmd.AddCommand(c.ValidateCommand(&opts))
	cmd.AddCommand(c.InfoCommand(&opts))
	cmd.AddCommand(c.SchemaCommand(&opts))
	cmd.AddCommand(c.DebugCommand(&opts))

	return cmd
}

// BuildCommand creates the build command
func (c *CLI) BuildCommand(opts *LauncherOptions) *cobra.Command {
	var (
		buildOpts    BuildOptions
		valuesFile   string
		patches      []string
		localPath    string
		noExtensions bool
		formatStr    string
	)

	cmd := &cobra.Command{
		Use:   "build [package-path]",
		Short: "Build manifests from a kurel package",
		Long: `Build generates Kubernetes manifests from a kurel package.
It resolves variables, applies patches, and outputs the final manifests.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Set CLI output writer from command
			c.SetOutputWriter(cmd.OutOrStdout())
			
			// Determine package path
			packagePath := "."
			if len(args) > 0 {
				packagePath = args[0]
			}

			// Setup logger based on flags
			if opts.Verbose {
				opts.Logger = logger.Default()
			} else {
				opts.Logger = logger.Noop()
			}

			// Set format from string
			switch strings.ToLower(formatStr) {
			case "json":
				buildOpts.Format = FormatJSON
			default:
				buildOpts.Format = FormatYAML
			}

			ctx := context.Background()
			return c.runBuild(ctx, packagePath, valuesFile, patches, localPath, !noExtensions, buildOpts, opts)
		},
	}

	// Build-specific flags
	cmd.Flags().StringVarP(&buildOpts.OutputPath, "output", "o", "", "Output path (default: stdout)")
	cmd.Flags().StringVar(&valuesFile, "values", "", "Values file for parameter overrides")
	cmd.Flags().StringSliceVarP(&patches, "patch", "p", nil, "Enable specific patches")
	cmd.Flags().StringVar(&localPath, "local", "", "Path to local extensions")
	cmd.Flags().BoolVar(&noExtensions, "no-extensions", false, "Disable local extensions")
	
	// Output format flags
	cmd.Flags().StringVar(&formatStr, "format", "yaml", "Output format (yaml, json)")
	cmd.Flags().BoolVar(&buildOpts.PrettyPrint, "pretty", false, "Pretty print JSON output")
	cmd.Flags().BoolVar(&buildOpts.SeparateFiles, "separate", false, "Write each resource to separate file")
	cmd.Flags().BoolVar(&buildOpts.IncludeIndex, "index", false, "Include index prefix in filenames")
	
	// Filter flags
	cmd.Flags().StringVar(&buildOpts.FilterKind, "kind", "", "Filter by resource kind")
	cmd.Flags().StringVar(&buildOpts.FilterName, "name", "", "Filter by resource name")
	cmd.Flags().StringVar(&buildOpts.FilterNamespace, "namespace", "", "Filter by namespace")
	
	// Transform flags
	cmd.Flags().StringToStringVar(&buildOpts.AddLabels, "add-label", nil, "Add labels to all resources")
	cmd.Flags().StringToStringVar(&buildOpts.AddAnnotations, "add-annotation", nil, "Add annotations to all resources")

	return cmd
}

// ValidateCommand creates the validate command
func (c *CLI) ValidateCommand(opts *LauncherOptions) *cobra.Command {
	var (
		valuesFile string
		schemaFile string
		outputJSON bool
	)

	cmd := &cobra.Command{
		Use:   "validate [package-path]",
		Short: "Validate a kurel package",
		Long:  `Validate checks a kurel package for errors and warnings.`,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Set CLI output writer from command
			c.SetOutputWriter(cmd.OutOrStdout())
			
			packagePath := "."
			if len(args) > 0 {
				packagePath = args[0]
			}

			if opts.Verbose {
				opts.Logger = logger.Default()
			} else {
				opts.Logger = logger.Noop()
			}

			ctx := context.Background()
			return c.runValidate(ctx, packagePath, valuesFile, schemaFile, outputJSON, opts)
		},
	}

	cmd.Flags().StringVar(&valuesFile, "values", "", "Values file for validation")
	cmd.Flags().StringVar(&schemaFile, "schema", "", "Custom schema file")
	cmd.Flags().BoolVar(&outputJSON, "json", false, "Output validation results as JSON")

	return cmd
}

// InfoCommand creates the info command
func (c *CLI) InfoCommand(opts *LauncherOptions) *cobra.Command {
	var (
		outputFormat string
		showAll      bool
	)

	cmd := &cobra.Command{
		Use:   "info [package-path]",
		Short: "Display information about a kurel package",
		Long:  `Info displays metadata, parameters, resources, and patches in a package.`,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Set CLI output writer from command
			c.SetOutputWriter(cmd.OutOrStdout())
			
			packagePath := "."
			if len(args) > 0 {
				packagePath = args[0]
			}

			if opts.Verbose {
				opts.Logger = logger.Default()
			} else {
				opts.Logger = logger.Noop()
			}

			ctx := context.Background()
			return c.runInfo(ctx, packagePath, outputFormat, showAll, opts)
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text, yaml, json)")
	cmd.Flags().BoolVar(&showAll, "all", false, "Show all details including resource content")

	return cmd
}

// SchemaCommand creates the schema command
func (c *CLI) SchemaCommand(opts *LauncherOptions) *cobra.Command {
	var (
		outputPath   string
		includeK8s   bool
		prettyPrint  bool
	)

	cmd := &cobra.Command{
		Use:   "schema [package-path]",
		Short: "Generate JSON schema for a kurel package",
		Long:  `Schema generates a JSON schema that can be used for validation and IDE support.`,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Set CLI output writer from command
			c.SetOutputWriter(cmd.OutOrStdout())
			
			packagePath := "."
			if len(args) > 0 {
				packagePath = args[0]
			}

			if opts.Verbose {
				opts.Logger = logger.Default()
			} else {
				opts.Logger = logger.Noop()
			}

			ctx := context.Background()
			return c.runSchema(ctx, packagePath, outputPath, includeK8s, prettyPrint, opts)
		},
	}

	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "Output path (default: stdout)")
	cmd.Flags().BoolVar(&includeK8s, "k8s", false, "Include Kubernetes resource schemas")
	cmd.Flags().BoolVar(&prettyPrint, "pretty", true, "Pretty print JSON output")

	return cmd
}

// DebugCommand creates the debug command with subcommands
func (c *CLI) DebugCommand(opts *LauncherOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "debug",
		Short: "Debug commands for troubleshooting",
		Long:  `Debug provides various troubleshooting commands for packages.`,
	}

	// Add debug subcommands
	cmd.AddCommand(c.debugVariablesCommand(opts))
	cmd.AddCommand(c.debugPatchesCommand(opts))
	cmd.AddCommand(c.debugResourcesCommand(opts))

	return cmd
}

// debugVariablesCommand shows variable resolution details
func (c *CLI) debugVariablesCommand(opts *LauncherOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "variables [package-path]",
		Short: "Show variable resolution graph",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Set CLI output writer from command
			c.SetOutputWriter(cmd.OutOrStdout())
			
			packagePath := "."
			if len(args) > 0 {
				packagePath = args[0]
			}

			opts.Logger = logger.Default() // Always verbose for debug
			ctx := context.Background()
			
			// Load package
			def, err := c.loader.LoadDefinition(ctx, packagePath, opts)
			if err != nil {
				return errors.Wrap(err, "failed to load package")
			}

			// Show variable graph
			resolver := c.resolver.(*variableResolver)
			graph := resolver.DebugVariableGraph(def.Parameters)
			fmt.Fprintln(c.outputWriter, graph)
			
			return nil
		},
	}
}

// debugPatchesCommand shows patch dependencies
func (c *CLI) debugPatchesCommand(opts *LauncherOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "patches [package-path]",
		Short: "Show patch dependency graph",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Set CLI output writer from command
			c.SetOutputWriter(cmd.OutOrStdout())
			
			packagePath := "."
			if len(args) > 0 {
				packagePath = args[0]
			}

			opts.Logger = logger.Default()
			ctx := context.Background()
			
			// Load package
			def, err := c.loader.LoadDefinition(ctx, packagePath, opts)
			if err != nil {
				return errors.Wrap(err, "failed to load package")
			}

			// Show patch graph
			processor := c.processor.(*patchProcessor)
			graph := processor.DebugPatchGraph(def.Patches)
			fmt.Fprintln(c.outputWriter, graph)
			
			return nil
		},
	}
}

// debugResourcesCommand shows resource details
func (c *CLI) debugResourcesCommand(opts *LauncherOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "resources [package-path]",
		Short: "Show detailed resource information",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Set CLI output writer from command
			c.SetOutputWriter(cmd.OutOrStdout())
			
			packagePath := "."
			if len(args) > 0 {
				packagePath = args[0]
			}

			opts.Logger = logger.Default()
			ctx := context.Background()
			
			// Load package
			def, err := c.loader.LoadDefinition(ctx, packagePath, opts)
			if err != nil {
				return errors.Wrap(err, "failed to load package")
			}

			// Show resource details
			fmt.Fprintf(c.outputWriter, "Package: %s\n", def.Metadata.Name)
			fmt.Fprintf(c.outputWriter, "Resources: %d\n\n", len(def.Resources))
			
			for i, resource := range def.Resources {
				fmt.Fprintf(c.outputWriter, "[%d] %s/%s\n", i+1, resource.Kind, resource.GetName())
				fmt.Fprintf(c.outputWriter, "    Namespace: %s\n", resource.GetNamespace())
				if len(resource.Metadata.Labels) > 0 {
					fmt.Fprintf(c.outputWriter, "    Labels:\n")
					for k, v := range resource.Metadata.Labels {
						fmt.Fprintf(c.outputWriter, "      %s: %s\n", k, v)
					}
				}
				fmt.Fprintln(c.outputWriter)
			}
			
			return nil
		},
	}
}

// Implementation methods

func (c *CLI) runBuild(ctx context.Context, packagePath, valuesFile string, patches []string, localPath string, useExtensions bool, buildOpts BuildOptions, opts *LauncherOptions) error {
	// Load package
	c.logger.Info("Loading package from %s", packagePath)
	def, err := c.loader.LoadDefinition(ctx, packagePath, opts)
	if err != nil {
		return errors.Wrap(err, "failed to load package")
	}

	// Validate package before building
	result, err := c.validator.ValidatePackage(ctx, def)
	if err != nil {
		return errors.Wrap(err, "validation failed")
	}
	if !result.IsValid() {
		return errors.Errorf("package has validation errors: %d errors found", len(result.Errors))
	}

	// Load values file if provided
	userValues := make(ParameterMap)
	if valuesFile != "" {
		data, err := os.ReadFile(valuesFile)
		if err != nil {
			return errors.NewFileError("read", valuesFile, "failed to read values file", err)
		}
		if err := yaml.Unmarshal(data, &userValues); err != nil {
			return errors.NewParseError(valuesFile, "invalid YAML", 0, 0, err)
		}
	}

	// Apply extensions if enabled
	if useExtensions {
		def, err = c.extension.LoadWithExtensions(ctx, def, localPath, opts)
		if err != nil {
			return errors.Wrap(err, "failed to load extensions")
		}
	}

	// Filter patches if specified
	enabledPatches := def.Patches
	if len(patches) > 0 {
		enabledPatches = c.filterPatches(def.Patches, patches)
	}

	// Resolve patch dependencies
	enabledPatches, err = c.processor.ResolveDependencies(ctx, enabledPatches, def.Parameters)
	if err != nil {
		return errors.Wrap(err, "failed to resolve patch dependencies")
	}

	// Create package instance
	instance := &PackageInstance{
		Definition:     def,
		UserValues:     userValues,
		EnabledPatches: enabledPatches,
		LocalPath:      localPath,
	}

	// Determine output destination
	if buildOpts.OutputPath != "" {
		if info, err := os.Stat(buildOpts.OutputPath); err == nil && info.IsDir() {
			buildOpts.Output = OutputDirectory
		} else {
			buildOpts.Output = OutputFile
		}
	} else {
		buildOpts.Output = OutputStdout
	}

	// Format is already set from the command flags

	// Build manifests
	return c.builder.Build(ctx, instance, buildOpts, opts)
}

func (c *CLI) runValidate(ctx context.Context, packagePath, valuesFile, schemaFile string, outputJSON bool, opts *LauncherOptions) error {
	// Load package
	def, err := c.loader.LoadDefinition(ctx, packagePath, opts)
	if err != nil {
		return errors.Wrap(err, "failed to load package")
	}

	// Validate package
	result, err := c.validator.ValidatePackage(ctx, def)
	if err != nil {
		return errors.Wrap(err, "validation failed")
	}

	// Output results
	if outputJSON {
		encoder := json.NewEncoder(c.outputWriter)
		encoder.SetIndent("", "  ")
		return encoder.Encode(result)
	}

	// Text output
	if len(result.Errors) > 0 {
		fmt.Fprintf(c.outputWriter, "Errors (%d):\n", len(result.Errors))
		for _, e := range result.Errors {
			fmt.Fprintf(c.outputWriter, "  ✗ %s\n", e.Error())
		}
		fmt.Fprintln(c.outputWriter)
	}

	if len(result.Warnings) > 0 {
		fmt.Fprintf(c.outputWriter, "Warnings (%d):\n", len(result.Warnings))
		for _, w := range result.Warnings {
			fmt.Fprintf(c.outputWriter, "  ⚠ %s\n", w.String())
		}
		fmt.Fprintln(c.outputWriter)
	}

	if result.IsValid() {
		fmt.Fprintln(c.outputWriter, "✓ Package is valid")
	} else {
		fmt.Fprintln(c.outputWriter, "✗ Package has errors")
		if opts.StrictMode {
			return errors.New("validation failed")
		}
	}

	return nil
}

func (c *CLI) runInfo(ctx context.Context, packagePath, outputFormat string, showAll bool, opts *LauncherOptions) error {
	// Load package
	def, err := c.loader.LoadDefinition(ctx, packagePath, opts)
	if err != nil {
		return errors.Wrap(err, "failed to load package")
	}

	switch outputFormat {
	case "json":
		encoder := json.NewEncoder(c.outputWriter)
		encoder.SetIndent("", "  ")
		return encoder.Encode(def)
		
	case "yaml":
		encoder := yaml.NewEncoder(c.outputWriter)
		encoder.SetIndent(2)
		return encoder.Encode(def)
		
	default: // text
		fmt.Fprintf(c.outputWriter, "Package: %s\n", def.Metadata.Name)
		fmt.Fprintf(c.outputWriter, "Version: %s\n", def.Metadata.Version)
		if def.Metadata.Description != "" {
			fmt.Fprintf(c.outputWriter, "Description: %s\n", def.Metadata.Description)
		}
		fmt.Fprintln(c.outputWriter)

		if len(def.Parameters) > 0 {
			fmt.Fprintf(c.outputWriter, "Parameters (%d):\n", len(def.Parameters))
			for k, v := range def.Parameters {
				fmt.Fprintf(c.outputWriter, "  %s: %v\n", k, formatValue(v))
			}
			fmt.Fprintln(c.outputWriter)
		}

		fmt.Fprintf(c.outputWriter, "Resources (%d):\n", len(def.Resources))
		for _, r := range def.Resources {
			fmt.Fprintf(c.outputWriter, "  - %s/%s", r.Kind, r.GetName())
			if ns := r.GetNamespace(); ns != "" {
				fmt.Fprintf(c.outputWriter, " (namespace: %s)", ns)
			}
			fmt.Fprintln(c.outputWriter)
		}
		
		if len(def.Patches) > 0 {
			fmt.Fprintf(c.outputWriter, "\nPatches (%d):\n", len(def.Patches))
			for _, p := range def.Patches {
				fmt.Fprintf(c.outputWriter, "  - %s", p.Name)
				if p.Metadata != nil && p.Metadata.Description != "" {
					fmt.Fprintf(c.outputWriter, ": %s", p.Metadata.Description)
				}
				fmt.Fprintln(c.outputWriter)
			}
		}
	}

	return nil
}

func (c *CLI) runSchema(ctx context.Context, packagePath, outputPath string, includeK8s, prettyPrint bool, opts *LauncherOptions) error {
	// Generate schema
	schema, err := c.schema.GeneratePackageSchema(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to generate schema")
	}

	// Export to JSON
	data, err := c.schema.ExportSchema(schema)
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

	_, err = c.outputWriter.Write(data)
	return err
}

func (c *CLI) filterPatches(patches []Patch, names []string) []Patch {
	nameSet := make(map[string]bool)
	for _, name := range names {
		nameSet[name] = true
	}

	var filtered []Patch
	for _, patch := range patches {
		if nameSet[patch.Name] {
			filtered = append(filtered, patch)
		}
	}
	return filtered
}

func formatValue(v interface{}) string {
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