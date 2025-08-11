package options

import (
	"os"
	"testing"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func TestNewGlobalOptions(t *testing.T) {
	opts := NewGlobalOptions()
	if opts == nil {
		t.Fatal("expected non-nil GlobalOptions")
	}

	// Test default values
	if opts.Output != "yaml" {
		t.Errorf("expected default output to be 'yaml', got %s", opts.Output)
	}

	if opts.Verbose {
		t.Error("expected Verbose to be false by default")
	}

	if opts.Debug {
		t.Error("expected Debug to be false by default")
	}

	if opts.DryRun {
		t.Error("expected DryRun to be false by default")
	}

	if opts.Namespace != "" {
		t.Errorf("expected empty default namespace, got %s", opts.Namespace)
	}

	if opts.Strict {
		t.Error("expected Strict to be false by default")
	}

	if opts.NoHeaders {
		t.Error("expected NoHeaders to be false by default")
	}

	if opts.ShowLabels {
		t.Error("expected ShowLabels to be false by default")
	}

	if opts.Wide {
		t.Error("expected Wide to be false by default")
	}
}

func TestGlobalOptions_AddFlags(t *testing.T) {
	opts := NewGlobalOptions()
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)

	opts.AddFlags(flags)

	// Test that all main flags are added
	expectedFlags := []string{
		"config",
		"verbose",
		"debug",
		"strict",
		"output",
		"output-file",
		"no-headers",
		"show-labels",
		"wide",
		"dry-run",
		"namespace",
	}

	for _, flagName := range expectedFlags {
		if flags.Lookup(flagName) == nil {
			t.Errorf("expected flag %s to be added", flagName)
		}
	}

	// Test that short flags work
	if flags.ShorthandLookup("c") == nil {
		t.Error("expected shorthand 'c' for config flag")
	}

	if flags.ShorthandLookup("v") == nil {
		t.Error("expected shorthand 'v' for verbose flag")
	}

	if flags.ShorthandLookup("o") == nil {
		t.Error("expected shorthand 'o' for output flag")
	}

	if flags.ShorthandLookup("f") == nil {
		t.Error("expected shorthand 'f' for output-file flag")
	}

	if flags.ShorthandLookup("n") == nil {
		t.Error("expected shorthand 'n' for namespace flag")
	}
}

func TestGlobalOptions_Validate(t *testing.T) {
	tests := []struct {
		name    string
		opts    *GlobalOptions
		wantErr bool
	}{
		{
			name:    "valid yaml output",
			opts:    &GlobalOptions{Output: "yaml"},
			wantErr: false,
		},
		{
			name:    "valid json output",
			opts:    &GlobalOptions{Output: "json"},
			wantErr: false,
		},
		{
			name:    "valid table output",
			opts:    &GlobalOptions{Output: "table"},
			wantErr: false,
		},
		{
			name:    "valid wide output",
			opts:    &GlobalOptions{Output: "wide"},
			wantErr: false,
		},
		{
			name:    "valid name output",
			opts:    &GlobalOptions{Output: "name"},
			wantErr: false,
		},
		{
			name:    "invalid output format",
			opts:    &GlobalOptions{Output: "invalid"},
			wantErr: true,
		},
		{
			name:    "empty output format",
			opts:    &GlobalOptions{Output: ""},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.opts.Validate()

			if tt.wantErr && err == nil {
				t.Error("expected error but got nil")
			}

			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			// Test wide format adjustment
			if tt.opts.Output == "wide" && !tt.wantErr {
				if !tt.opts.Wide {
					t.Error("expected Wide to be true when output is 'wide'")
				}
				if tt.opts.Output != "table" {
					t.Errorf("expected output to be changed to 'table', got %s", tt.opts.Output)
				}
			}
		})
	}
}

func TestGlobalOptions_Complete(t *testing.T) {
	// Clean up viper before starting
	viper.Reset()

	tests := []struct {
		name         string
		viperValues  map[string]interface{}
		initialOpts  *GlobalOptions
		expectedOpts *GlobalOptions
		wantErr      bool
	}{
		{
			name:        "no viper values",
			viperValues: map[string]interface{}{},
			initialOpts: &GlobalOptions{
				Output:  "yaml",
				Verbose: false,
				Debug:   false,
			},
			expectedOpts: &GlobalOptions{
				Output:  "yaml",
				Verbose: false,
				Debug:   false,
			},
			wantErr: false,
		},
		{
			name: "viper overrides",
			viperValues: map[string]interface{}{
				"verbose":   true,
				"debug":     false,
				"output":    "json",
				"namespace": "test-ns",
			},
			initialOpts: &GlobalOptions{
				Output:    "yaml",
				Verbose:   false,
				Debug:     false,
				Namespace: "",
			},
			expectedOpts: &GlobalOptions{
				Output:    "json",
				Verbose:   true,
				Debug:     false,
				Namespace: "test-ns",
			},
			wantErr: false,
		},
		{
			name: "debug enables verbose",
			viperValues: map[string]interface{}{
				"debug": true,
			},
			initialOpts: &GlobalOptions{
				Output:  "yaml",
				Verbose: false,
				Debug:   false,
			},
			expectedOpts: &GlobalOptions{
				Output:  "yaml",
				Verbose: true,
				Debug:   true,
			},
			wantErr: false,
		},
		{
			name: "invalid output format from viper",
			viperValues: map[string]interface{}{
				"output": "invalid",
			},
			initialOpts: &GlobalOptions{
				Output: "yaml",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset viper
			viper.Reset()

			// Set viper values
			for key, value := range tt.viperValues {
				viper.Set(key, value)
			}

			// Save original env var
			originalDebug := os.Getenv("KURE_DEBUG")
			defer os.Setenv("KURE_DEBUG", originalDebug)

			err := tt.initialOpts.Complete()

			if tt.wantErr && err == nil {
				t.Error("expected error but got nil")
				return
			}

			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tt.wantErr {
				return
			}

			// Compare expected values
			if tt.initialOpts.Output != tt.expectedOpts.Output {
				t.Errorf("expected Output %s, got %s", tt.expectedOpts.Output, tt.initialOpts.Output)
			}

			if tt.initialOpts.Verbose != tt.expectedOpts.Verbose {
				t.Errorf("expected Verbose %v, got %v", tt.expectedOpts.Verbose, tt.initialOpts.Verbose)
			}

			if tt.initialOpts.Debug != tt.expectedOpts.Debug {
				t.Errorf("expected Debug %v, got %v", tt.expectedOpts.Debug, tt.initialOpts.Debug)
			}

			if tt.initialOpts.Namespace != tt.expectedOpts.Namespace {
				t.Errorf("expected Namespace %s, got %s", tt.expectedOpts.Namespace, tt.initialOpts.Namespace)
			}

			// Test KURE_DEBUG environment variable
			if tt.initialOpts.Debug {
				if os.Getenv("KURE_DEBUG") != "1" {
					t.Error("expected KURE_DEBUG=1 when debug is enabled")
				}
			}
		})
	}

	// Reset viper after tests
	viper.Reset()
}

func TestGlobalOptions_IsTableOutput(t *testing.T) {
	tests := []struct {
		output   string
		expected bool
	}{
		{"table", true},
		{"wide", true},
		{"name", true},
		{"yaml", false},
		{"json", false},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.output, func(t *testing.T) {
			opts := &GlobalOptions{Output: tt.output}
			result := opts.IsTableOutput()

			if result != tt.expected {
				t.Errorf("IsTableOutput() for %s = %v, expected %v", tt.output, result, tt.expected)
			}
		})
	}
}

func TestGlobalOptions_IsJSONOutput(t *testing.T) {
	tests := []struct {
		output   string
		expected bool
	}{
		{"json", true},
		{"yaml", false},
		{"table", false},
		{"wide", false},
		{"name", false},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.output, func(t *testing.T) {
			opts := &GlobalOptions{Output: tt.output}
			result := opts.IsJSONOutput()

			if result != tt.expected {
				t.Errorf("IsJSONOutput() for %s = %v, expected %v", tt.output, result, tt.expected)
			}
		})
	}
}

func TestGlobalOptions_IsYAMLOutput(t *testing.T) {
	tests := []struct {
		output   string
		expected bool
	}{
		{"yaml", true},
		{"json", false},
		{"table", false},
		{"wide", false},
		{"name", false},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.output, func(t *testing.T) {
			opts := &GlobalOptions{Output: tt.output}
			result := opts.IsYAMLOutput()

			if result != tt.expected {
				t.Errorf("IsYAMLOutput() for %s = %v, expected %v", tt.output, result, tt.expected)
			}
		})
	}
}

func TestGlobalOptions_FlagIntegration(t *testing.T) {
	opts := NewGlobalOptions()
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	opts.AddFlags(flags)

	// Test flag parsing
	args := []string{
		"--verbose",
		"--debug",
		"--output", "json",
		"--namespace", "test-ns",
		"--dry-run",
		"--strict",
		"--no-headers",
		"--show-labels",
		"--wide",
	}

	err := flags.Parse(args)
	if err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	// Verify flag values were set
	if !opts.Verbose {
		t.Error("expected Verbose to be true")
	}

	if !opts.Debug {
		t.Error("expected Debug to be true")
	}

	if opts.Output != "json" {
		t.Errorf("expected Output to be 'json', got %s", opts.Output)
	}

	if opts.Namespace != "test-ns" {
		t.Errorf("expected Namespace to be 'test-ns', got %s", opts.Namespace)
	}

	if !opts.DryRun {
		t.Error("expected DryRun to be true")
	}

	if !opts.Strict {
		t.Error("expected Strict to be true")
	}

	if !opts.NoHeaders {
		t.Error("expected NoHeaders to be true")
	}

	if !opts.ShowLabels {
		t.Error("expected ShowLabels to be true")
	}

	if !opts.Wide {
		t.Error("expected Wide to be true")
	}
}

func TestGlobalOptions_ShortFlags(t *testing.T) {
	opts := NewGlobalOptions()
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	opts.AddFlags(flags)

	// Test short flag parsing
	args := []string{
		"-v",
		"-o", "table",
		"-n", "short-ns",
		"-c", "custom-config.yaml",
		"-f", "output.yaml",
	}

	err := flags.Parse(args)
	if err != nil {
		t.Fatalf("failed to parse short flags: %v", err)
	}

	// Verify short flag values were set
	if !opts.Verbose {
		t.Error("expected Verbose to be true from -v")
	}

	if opts.Output != "table" {
		t.Errorf("expected Output to be 'table' from -o, got %s", opts.Output)
	}

	if opts.Namespace != "short-ns" {
		t.Errorf("expected Namespace to be 'short-ns' from -n, got %s", opts.Namespace)
	}

	if opts.ConfigFile != "custom-config.yaml" {
		t.Errorf("expected ConfigFile to be 'custom-config.yaml' from -c, got %s", opts.ConfigFile)
	}

	if opts.OutputFile != "output.yaml" {
		t.Errorf("expected OutputFile to be 'output.yaml' from -f, got %s", opts.OutputFile)
	}
}

func TestGlobalOptions_ValidateAfterComplete(t *testing.T) {
	// Test that Complete calls Validate
	viper.Reset()
	viper.Set("output", "invalid-format")

	opts := NewGlobalOptions()
	err := opts.Complete()

	if err == nil {
		t.Error("expected error from Complete when viper has invalid output format")
	}

	viper.Reset()
}

func TestGlobalOptions_EnvironmentIntegration(t *testing.T) {
	// Test debug environment variable setting
	originalDebug := os.Getenv("KURE_DEBUG")
	defer os.Setenv("KURE_DEBUG", originalDebug)

	opts := &GlobalOptions{
		Output:  "yaml",
		Debug:   true,
		Verbose: false,
	}

	err := opts.Complete()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if os.Getenv("KURE_DEBUG") != "1" {
		t.Error("expected KURE_DEBUG=1 when debug is enabled")
	}

	if !opts.Verbose {
		t.Error("expected verbose to be enabled when debug is enabled")
	}
}
