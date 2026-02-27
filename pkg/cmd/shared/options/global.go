package options

import (
	"os"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/go-kure/kure/pkg/errors"
)

// GlobalOptions contains global flags and configuration
type GlobalOptions struct {
	// Configuration
	ConfigFile string
	Verbose    bool
	Debug      bool
	Strict     bool

	// Output options
	Output     string
	OutputFile string
	NoHeaders  bool
	ShowLabels bool
	Wide       bool

	// Common flags
	DryRun    bool
	Namespace string
}

// NewGlobalOptions creates a new GlobalOptions with defaults
func NewGlobalOptions() *GlobalOptions {
	return &GlobalOptions{
		Output:    "yaml",
		Verbose:   false,
		Debug:     false,
		DryRun:    false,
		Namespace: "",
	}
}

// AddFlags adds global flags to the provided FlagSet
func (o *GlobalOptions) AddFlags(flags *pflag.FlagSet) {
	// Configuration flags
	flags.StringVarP(&o.ConfigFile, "config", "c", o.ConfigFile, "config file (default is $HOME/.kure.yaml)")
	flags.BoolVarP(&o.Verbose, "verbose", "v", o.Verbose, "verbose output")
	flags.BoolVar(&o.Debug, "debug", o.Debug, "debug output")
	flags.BoolVar(&o.Strict, "strict", o.Strict, "treat warnings as errors")

	// Output flags
	flags.StringVarP(&o.Output, "output", "o", o.Output, "output format (yaml|json|table|wide|name)")
	flags.StringVarP(&o.OutputFile, "output-file", "f", o.OutputFile, "write output to file instead of stdout")
	flags.BoolVar(&o.NoHeaders, "no-headers", o.NoHeaders, "don't print headers (for table output)")
	flags.BoolVar(&o.ShowLabels, "show-labels", o.ShowLabels, "show resource labels in table output")
	flags.BoolVar(&o.Wide, "wide", o.Wide, "use wide output format")

	// Common flags
	flags.BoolVar(&o.DryRun, "dry-run", o.DryRun, "print generated resources without writing to files")
	flags.StringVarP(&o.Namespace, "namespace", "n", o.Namespace, "target namespace for operations")
}

// Complete completes the global options by reading from configuration
func (o *GlobalOptions) Complete() error {
	// Override with viper values if available
	if viper.IsSet("verbose") {
		o.Verbose = viper.GetBool("verbose")
	}
	if viper.IsSet("debug") {
		o.Debug = viper.GetBool("debug")
	}
	if viper.IsSet("output") {
		o.Output = viper.GetString("output")
	}
	if viper.IsSet("namespace") {
		o.Namespace = viper.GetString("namespace")
	}

	// Set debug logging if requested
	if o.Debug {
		_ = os.Setenv("KURE_DEBUG", "1")
		o.Verbose = true
	}

	return o.Validate()
}

// Validate validates the global options
func (o *GlobalOptions) Validate() error {
	// Validate output format
	validOutputs := []string{"yaml", "json", "table", "wide", "name"}
	valid := false
	for _, format := range validOutputs {
		if o.Output == format {
			valid = true
			break
		}
	}
	if !valid {
		return errors.NewValidationError("output", o.Output, "GlobalOptions", validOutputs)
	}

	// Adjust wide flag based on output format
	if o.Output == "wide" {
		o.Wide = true
		o.Output = "table"
	}

	return nil
}

// IsTableOutput returns true if output format requires table formatting
func (o *GlobalOptions) IsTableOutput() bool {
	return o.Output == "table" || o.Output == "wide" || o.Output == "name"
}

// IsJSONOutput returns true if output format is JSON
func (o *GlobalOptions) IsJSONOutput() bool {
	return o.Output == "json"
}

// IsYAMLOutput returns true if output format is YAML
func (o *GlobalOptions) IsYAMLOutput() bool {
	return o.Output == "yaml"
}
