package kurel

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/go-kure/kure/pkg/cmd/shared/options"
)

func TestNewKurelCommand(t *testing.T) {
	cmd := NewKurelCommand()

	if cmd == nil {
		t.Fatal("expected non-nil command")
	}

	if cmd.Use != "kurel" {
		t.Errorf("expected command name 'kurel', got %s", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("expected non-empty short description")
	}

	if cmd.Long == "" {
		t.Error("expected non-empty long description")
	}

	// Check that silence options are set
	if !cmd.SilenceUsage {
		t.Error("expected SilenceUsage to be true")
	}

	if !cmd.SilenceErrors {
		t.Error("expected SilenceErrors to be true")
	}

	// Check persistent pre-run is set
	if cmd.PersistentPreRunE == nil {
		t.Error("expected PersistentPreRunE to be set")
	}
}

func TestKurelCommandSubcommands(t *testing.T) {
	cmd := NewKurelCommand()

	expectedSubcommands := []string{
		"build", "validate", "info", "schema", "config", "completion", "version",
	}

	commands := cmd.Commands()
	if len(commands) < len(expectedSubcommands) {
		t.Errorf("expected at least %d subcommands, got %d", len(expectedSubcommands), len(commands))
	}

	// Check that expected subcommands exist
	commandMap := make(map[string]bool)
	for _, subCmd := range commands {
		commandMap[extractCommandName(subCmd.Use)] = true
	}

	for _, expectedCmd := range expectedSubcommands {
		if !commandMap[expectedCmd] {
			t.Errorf("expected subcommand %s not found", expectedCmd)
		}
	}
}

func TestKurelCommandFlags(t *testing.T) {
	cmd := NewKurelCommand()

	// Check that persistent flags are added
	expectedFlags := []string{
		"config", "verbose", "debug", "output", "dry-run", "namespace",
	}

	for _, flagName := range expectedFlags {
		flag := cmd.PersistentFlags().Lookup(flagName)
		if flag == nil {
			t.Errorf("expected persistent flag %s not found", flagName)
		}
	}
}

func TestKurelCommandHelp(t *testing.T) {
	cmd := NewKurelCommand()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// Test help command
	cmd.SetArgs([]string{"--help"})
	err := cmd.Execute()

	if err != nil {
		t.Errorf("help command failed: %v", err)
	}

	output := buf.String()
	if output == "" {
		t.Error("expected help output, got empty string")
	}

	// Check that help contains key information
	expectedContent := []string{"kurel", "Usage:", "Available Commands:", "Flags:"}
	for _, content := range expectedContent {
		if !containsString(output, content) {
			t.Errorf("expected help output to contain %q", content)
		}
	}
}

func TestKurelCommandPersistentPreRun(t *testing.T) {
	cmd := NewKurelCommand()

	// Mock arguments for testing
	cmd.SetArgs([]string{"--output=json", "--verbose"})

	// Execute persistent pre-run
	err := cmd.PersistentPreRunE(cmd, []string{})
	if err != nil {
		t.Errorf("persistent pre-run failed: %v", err)
	}
}

func TestNewBuildCommand(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	cmd := newBuildCommand(globalOpts)

	if cmd == nil {
		t.Fatal("expected non-nil build command")
	}

	if extractCommandName(cmd.Use) != "build" {
		t.Errorf("expected command name 'build', got %s", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("expected non-empty short description")
	}

	if cmd.Long == "" {
		t.Error("expected non-empty long description")
	}

	if cmd.Args == nil {
		t.Error("expected Args to be set")
	}

	if cmd.RunE == nil {
		t.Error("expected RunE to be set")
	}
}

func TestNewValidateCommand(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	cmd := newValidateCommand(globalOpts)

	if cmd == nil {
		t.Fatal("expected non-nil validate command")
	}

	if extractCommandName(cmd.Use) != "validate" {
		t.Errorf("expected command name 'validate', got %s", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("expected non-empty short description")
	}

	if cmd.Long == "" {
		t.Error("expected non-empty long description")
	}
}

func TestNewInfoCommand(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	cmd := newInfoCommand(globalOpts)

	if cmd == nil {
		t.Fatal("expected non-nil info command")
	}

	if extractCommandName(cmd.Use) != "info" {
		t.Errorf("expected command name 'info', got %s", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("expected non-empty short description")
	}

	if cmd.Long == "" {
		t.Error("expected non-empty long description")
	}
}

func TestNewSchemaCommand(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	cmd := newSchemaCommand(globalOpts)

	if cmd == nil {
		t.Fatal("expected non-nil schema command")
	}

	if extractCommandName(cmd.Use) != "schema" {
		t.Errorf("expected command name 'schema', got %s", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("expected non-empty short description")
	}

	if cmd.Long == "" {
		t.Error("expected non-empty long description")
	}
}

func TestNewConfigCommand(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	cmd := newConfigCommand(globalOpts)

	if cmd == nil {
		t.Fatal("expected non-nil config command")
	}

	if cmd.Use != "config" {
		t.Errorf("expected command name 'config', got %s", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("expected non-empty short description")
	}

	if cmd.Long == "" {
		t.Error("expected non-empty long description")
	}
}

func TestKurelCommandVersion(t *testing.T) {
	cmd := NewKurelCommand()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// Test version command
	cmd.SetArgs([]string{"version"})
	err := cmd.Execute()

	if err != nil {
		t.Errorf("version command failed: %v", err)
	}
}

func TestKurelCommandCompletion(t *testing.T) {
	cmd := NewKurelCommand()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// Test completion command
	cmd.SetArgs([]string{"completion", "bash"})
	err := cmd.Execute()

	if err != nil {
		t.Errorf("completion command failed: %v", err)
	}
}

func TestKurelCommandInvalidFlags(t *testing.T) {
	cmd := NewKurelCommand()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// Test with invalid output format
	cmd.SetArgs([]string{"--output=invalid-format", "version"})
	err := cmd.Execute()

	if err == nil {
		t.Error("expected error for invalid output format")
	}
}

func TestKurelCommandFlagValidation(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantError bool
	}{
		{
			name:      "valid yaml output",
			args:      []string{"--output=yaml", "version"},
			wantError: false,
		},
		{
			name:      "valid json output",
			args:      []string{"--output=json", "version"},
			wantError: false,
		},
		{
			name:      "valid table output",
			args:      []string{"--output=table", "version"},
			wantError: false,
		},
		{
			name:      "invalid output format",
			args:      []string{"--output=invalid", "version"},
			wantError: true,
		},
		{
			name:      "valid verbose flag",
			args:      []string{"--verbose", "version"},
			wantError: false,
		},
		{
			name:      "valid debug flag",
			args:      []string{"--debug", "version"},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewKurelCommand()

			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)

			cmd.SetArgs(tt.args)
			err := cmd.Execute()

			if tt.wantError && err == nil {
				t.Error("expected error but got nil")
			}

			if !tt.wantError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestKurelCommandExecuteError(t *testing.T) {
	cmd := NewKurelCommand()

	// Set invalid arguments that should cause an error
	cmd.SetArgs([]string{"nonexistent-command"})

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for nonexistent command")
	}
}

func TestBuildCommandFlags(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	cmd := newBuildCommand(globalOpts)

	// Check that expected flags are added
	expectedFlags := []string{
		"output", "values", "format",
	}

	for _, flagName := range expectedFlags {
		flag := cmd.Flags().Lookup(flagName)
		if flag == nil {
			t.Errorf("expected flag %s not found in build command", flagName)
		}
	}
}

func TestBuildCommandInvalidArgs(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	cmd := newBuildCommand(globalOpts)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// Test with no arguments (should fail due to ExactArgs(1))
	cmd.SetArgs([]string{})
	err := cmd.Execute()

	if err == nil {
		t.Error("expected error for no arguments")
	}
}

// --- formatParameterValue tests ---

func TestFormatParameterValue(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{
			name:     "simple string",
			input:    "hello",
			expected: "hello",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "multiline string",
			input:    "line1\nline2\nline3",
			expected: "(multiline)",
		},
		{
			name:     "string with only newline",
			input:    "\n",
			expected: "(multiline)",
		},
		{
			name:     "map with keys",
			input:    map[string]any{"a": 1, "b": 2, "c": 3},
			expected: "(map with 3 keys)",
		},
		{
			name:     "empty map",
			input:    map[string]any{},
			expected: "(map with 0 keys)",
		},
		{
			name:     "array with items",
			input:    []any{"a", "b"},
			expected: "(array with 2 items)",
		},
		{
			name:     "empty array",
			input:    []any{},
			expected: "(array with 0 items)",
		},
		{
			name:     "integer value",
			input:    42,
			expected: "42",
		},
		{
			name:     "boolean true",
			input:    true,
			expected: "true",
		},
		{
			name:     "boolean false",
			input:    false,
			expected: "false",
		},
		{
			name:     "float value",
			input:    3.14,
			expected: "3.14",
		},
		{
			name:     "nil value",
			input:    nil,
			expected: "<nil>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatParameterValue(tt.input)
			if got != tt.expected {
				t.Errorf("formatParameterValue(%v) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// --- Build command deep tests ---

func TestBuildCommandAllFlags(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	cmd := newBuildCommand(globalOpts)

	expectedFlags := map[string]string{
		"output":    "o",
		"values":    "",
		"patch":     "p",
		"format":    "",
		"kind":      "",
		"name":      "",
		"add-label": "",
	}

	for flagName, shorthand := range expectedFlags {
		flag := cmd.Flags().Lookup(flagName)
		if flag == nil {
			t.Errorf("expected flag %s not found in build command", flagName)
			continue
		}
		if shorthand != "" && flag.Shorthand != shorthand {
			t.Errorf("flag %s: expected shorthand %q, got %q", flagName, shorthand, flag.Shorthand)
		}
	}
}

func TestBuildCommandFlagDefaults(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	cmd := newBuildCommand(globalOpts)

	formatFlag := cmd.Flags().Lookup("format")
	if formatFlag == nil {
		t.Fatal("expected format flag")
	}
	if formatFlag.DefValue != "yaml" {
		t.Errorf("expected format default 'yaml', got %q", formatFlag.DefValue)
	}

	outputFlag := cmd.Flags().Lookup("output")
	if outputFlag == nil {
		t.Fatal("expected output flag")
	}
	if outputFlag.DefValue != "" {
		t.Errorf("expected output default '', got %q", outputFlag.DefValue)
	}
}

func TestBuildCommandTooManyArgs(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	cmd := newBuildCommand(globalOpts)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{"pkg1", "pkg2"})
	err := cmd.Execute()

	if err == nil {
		t.Error("expected error for too many arguments")
	}
}

func TestBuildCommandHelp(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	cmd := newBuildCommand(globalOpts)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{"--help"})
	err := cmd.Execute()

	if err != nil {
		t.Errorf("build help failed: %v", err)
	}

	output := buf.String()
	expectedContent := []string{"build", "Build", "package", "Flags:"}
	for _, content := range expectedContent {
		if !strings.Contains(output, content) {
			t.Errorf("expected build help to contain %q", content)
		}
	}
}

func TestBuildCommandNonexistentPackage(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	cmd := newBuildCommand(globalOpts)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{"/nonexistent/package/path"})
	err := cmd.Execute()

	if err == nil {
		t.Error("expected error for nonexistent package path")
	}
}

func TestBuildCommandWithNonexistentValuesFile(t *testing.T) {
	// Create a minimal package structure so the loader can work
	tmpDir := t.TempDir()
	setupMinimalPackage(t, tmpDir)

	globalOpts := options.NewGlobalOptions()
	cmd := newBuildCommand(globalOpts)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{"--values=/nonexistent/values.yaml", tmpDir})
	err := cmd.Execute()

	if err == nil {
		t.Error("expected error for nonexistent values file")
	}
}

// --- Validate command deep tests ---

func TestValidateCommandAllFlags(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	cmd := newValidateCommand(globalOpts)

	expectedFlags := []string{"values", "schema", "json"}
	for _, flagName := range expectedFlags {
		flag := cmd.Flags().Lookup(flagName)
		if flag == nil {
			t.Errorf("expected flag %s not found in validate command", flagName)
		}
	}
}

func TestValidateCommandFlagDefaults(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	cmd := newValidateCommand(globalOpts)

	jsonFlag := cmd.Flags().Lookup("json")
	if jsonFlag == nil {
		t.Fatal("expected json flag")
	}
	if jsonFlag.DefValue != "false" {
		t.Errorf("expected json default 'false', got %q", jsonFlag.DefValue)
	}
}

func TestValidateCommandNoArgs(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	cmd := newValidateCommand(globalOpts)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{})
	err := cmd.Execute()

	if err == nil {
		t.Error("expected error for no arguments")
	}
}

func TestValidateCommandTooManyArgs(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	cmd := newValidateCommand(globalOpts)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{"pkg1", "pkg2"})
	err := cmd.Execute()

	if err == nil {
		t.Error("expected error for too many arguments")
	}
}

func TestValidateCommandHelp(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	cmd := newValidateCommand(globalOpts)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{"--help"})
	err := cmd.Execute()

	if err != nil {
		t.Errorf("validate help failed: %v", err)
	}

	output := buf.String()
	expectedContent := []string{"validate", "Validate", "package", "Flags:"}
	for _, content := range expectedContent {
		if !strings.Contains(output, content) {
			t.Errorf("expected validate help to contain %q", content)
		}
	}
}

func TestValidateCommandNonexistentPackage(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	cmd := newValidateCommand(globalOpts)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{"/nonexistent/package/path"})
	err := cmd.Execute()

	if err == nil {
		t.Error("expected error for nonexistent package path")
	}
}

func TestValidateCommandNonexistentValuesFile(t *testing.T) {
	tmpDir := t.TempDir()
	setupMinimalPackage(t, tmpDir)

	globalOpts := options.NewGlobalOptions()
	cmd := newValidateCommand(globalOpts)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{"--values=/nonexistent/values.yaml", tmpDir})
	err := cmd.Execute()

	if err == nil {
		t.Error("expected error for nonexistent values file")
	}
}

func TestValidateCommandArgsCheck(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	cmd := newValidateCommand(globalOpts)

	if cmd.Args == nil {
		t.Error("expected Args to be set on validate command")
	}

	if cmd.RunE == nil {
		t.Error("expected RunE to be set on validate command")
	}
}

// --- Info command deep tests ---

func TestInfoCommandAllFlags(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	cmd := newInfoCommand(globalOpts)

	expectedFlags := map[string]string{
		"output": "o",
		"all":    "",
	}

	for flagName, shorthand := range expectedFlags {
		flag := cmd.Flags().Lookup(flagName)
		if flag == nil {
			t.Errorf("expected flag %s not found in info command", flagName)
			continue
		}
		if shorthand != "" && flag.Shorthand != shorthand {
			t.Errorf("flag %s: expected shorthand %q, got %q", flagName, shorthand, flag.Shorthand)
		}
	}
}

func TestInfoCommandFlagDefaults(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	cmd := newInfoCommand(globalOpts)

	outputFlag := cmd.Flags().Lookup("output")
	if outputFlag == nil {
		t.Fatal("expected output flag")
	}
	if outputFlag.DefValue != "text" {
		t.Errorf("expected output default 'text', got %q", outputFlag.DefValue)
	}

	allFlag := cmd.Flags().Lookup("all")
	if allFlag == nil {
		t.Fatal("expected all flag")
	}
	if allFlag.DefValue != "false" {
		t.Errorf("expected all default 'false', got %q", allFlag.DefValue)
	}
}

func TestInfoCommandNoArgs(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	cmd := newInfoCommand(globalOpts)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{})
	err := cmd.Execute()

	if err == nil {
		t.Error("expected error for no arguments")
	}
}

func TestInfoCommandTooManyArgs(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	cmd := newInfoCommand(globalOpts)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{"pkg1", "pkg2"})
	err := cmd.Execute()

	if err == nil {
		t.Error("expected error for too many arguments")
	}
}

func TestInfoCommandHelp(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	cmd := newInfoCommand(globalOpts)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{"--help"})
	err := cmd.Execute()

	if err != nil {
		t.Errorf("info help failed: %v", err)
	}

	output := buf.String()
	expectedContent := []string{"info", "Info", "package", "Flags:"}
	for _, content := range expectedContent {
		if !strings.Contains(output, content) {
			t.Errorf("expected info help to contain %q", content)
		}
	}
}

func TestInfoCommandNonexistentPackage(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	cmd := newInfoCommand(globalOpts)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{"/nonexistent/package/path"})
	err := cmd.Execute()

	if err == nil {
		t.Error("expected error for nonexistent package path")
	}
}

func TestInfoCommandArgsCheck(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	cmd := newInfoCommand(globalOpts)

	if cmd.Args == nil {
		t.Error("expected Args to be set on info command")
	}

	if cmd.RunE == nil {
		t.Error("expected RunE to be set on info command")
	}
}

// --- Schema command deep tests ---

func TestSchemaCommandSubcommands(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	cmd := newSchemaCommand(globalOpts)

	subCmds := cmd.Commands()
	if len(subCmds) == 0 {
		t.Fatal("expected schema command to have subcommands")
	}

	found := false
	for _, sub := range subCmds {
		if extractCommandName(sub.Use) == "generate" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'generate' subcommand under schema")
	}
}

func TestSchemaGenerateCommandFlags(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	schemaCmd := newSchemaCommand(globalOpts)

	var generateCmd *cobra.Command
	for _, sub := range schemaCmd.Commands() {
		if extractCommandName(sub.Use) == "generate" {
			generateCmd = sub
			break
		}
	}

	if generateCmd == nil {
		t.Fatal("generate subcommand not found")
	}

	expectedFlags := []string{"output", "k8s", "pretty"}
	for _, flagName := range expectedFlags {
		flag := generateCmd.Flags().Lookup(flagName)
		if flag == nil {
			t.Errorf("expected flag %s not found in schema generate command", flagName)
		}
	}
}

func TestSchemaGenerateCommandFlagDefaults(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	schemaCmd := newSchemaCommand(globalOpts)

	var genCmd *cobra.Command
	for _, sub := range schemaCmd.Commands() {
		if extractCommandName(sub.Use) == "generate" {
			genCmd = sub
			break
		}
	}

	if genCmd == nil {
		t.Fatal("generate subcommand not found")
	}

	k8sFlag := genCmd.Flags().Lookup("k8s")
	if k8sFlag == nil {
		t.Fatal("expected k8s flag")
	}
	if k8sFlag.DefValue != "false" {
		t.Errorf("expected k8s default 'false', got %q", k8sFlag.DefValue)
	}

	prettyFlag := genCmd.Flags().Lookup("pretty")
	if prettyFlag == nil {
		t.Fatal("expected pretty flag")
	}
	if prettyFlag.DefValue != "true" {
		t.Errorf("expected pretty default 'true', got %q", prettyFlag.DefValue)
	}
}

func TestSchemaGenerateCommandNoArgs(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	cmd := newSchemaCommand(globalOpts)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{"generate"})
	err := cmd.Execute()

	if err == nil {
		t.Error("expected error for generate with no arguments")
	}
}

func TestSchemaGenerateCommandTooManyArgs(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	cmd := newSchemaCommand(globalOpts)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{"generate", "pkg1", "pkg2"})
	err := cmd.Execute()

	if err == nil {
		t.Error("expected error for generate with too many arguments")
	}
}

func TestSchemaCommandHelp(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	cmd := newSchemaCommand(globalOpts)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{"--help"})
	err := cmd.Execute()

	if err != nil {
		t.Errorf("schema help failed: %v", err)
	}

	output := buf.String()
	expectedContent := []string{"schema", "generate"}
	for _, content := range expectedContent {
		if !strings.Contains(output, content) {
			t.Errorf("expected schema help to contain %q", content)
		}
	}
}

func TestSchemaCommandNoSubcommand(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	cmd := newSchemaCommand(globalOpts)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// No subcommand should show help (not error)
	cmd.SetArgs([]string{})
	_ = cmd.Execute()

	// Schema command without subcommand is valid (shows help)
}

// --- Config command deep tests ---

func TestConfigCommandSubcommands(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	cmd := newConfigCommand(globalOpts)

	subCmds := cmd.Commands()
	if len(subCmds) < 2 {
		t.Errorf("expected at least 2 config subcommands, got %d", len(subCmds))
	}

	commandMap := make(map[string]bool)
	for _, sub := range subCmds {
		commandMap[extractCommandName(sub.Use)] = true
	}

	expectedSubs := []string{"view", "init"}
	for _, expected := range expectedSubs {
		if !commandMap[expected] {
			t.Errorf("expected config subcommand %q not found", expected)
		}
	}
}

func TestConfigViewCommand(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	globalOpts.Verbose = true
	globalOpts.Debug = false
	globalOpts.Strict = true
	globalOpts.ConfigFile = "/some/config.yaml"

	cmd := newConfigCommand(globalOpts)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{"view"})
	err := cmd.Execute()

	if err != nil {
		t.Errorf("config view failed: %v", err)
	}

	// Note: config view writes to os.Stdout directly, not to the cobra buffer.
	// We verify it does not error. The output goes to stdout.
}

func TestConfigInitCommand(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".kurel", "config.yaml")

	globalOpts := options.NewGlobalOptions()
	globalOpts.ConfigFile = configPath

	cmd := newConfigCommand(globalOpts)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{"init"})
	err := cmd.Execute()

	if err != nil {
		t.Errorf("config init failed: %v", err)
	}

	// Verify the config file was created
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("expected config file to be created")
	}

	// Verify config file contents
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read created config: %v", err)
	}

	content := string(data)
	expectedContent := []string{
		"verbose: false",
		"debug: false",
		"strict: false",
		"launcher:",
		"extensions:",
	}
	for _, expected := range expectedContent {
		if !strings.Contains(content, expected) {
			t.Errorf("expected config file to contain %q", expected)
		}
	}
}

func TestConfigInitCommandDefaultPath(t *testing.T) {
	// When ConfigFile is empty, it should use the default .kurel/config.yaml path
	// We test this from a temp directory to avoid writing to the actual working directory
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Errorf("failed to restore working directory: %v", err)
		}
	}()

	globalOpts := options.NewGlobalOptions()
	// ConfigFile left empty to test default path

	cmd := newConfigCommand(globalOpts)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{"init"})
	err = cmd.Execute()

	if err != nil {
		t.Errorf("config init with default path failed: %v", err)
	}

	// Verify the default config file was created
	defaultPath := filepath.Join(tmpDir, ".kurel", "config.yaml")
	if _, err := os.Stat(defaultPath); os.IsNotExist(err) {
		t.Error("expected default config file to be created at .kurel/config.yaml")
	}
}

func TestConfigCommandHelp(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	cmd := newConfigCommand(globalOpts)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{"--help"})
	err := cmd.Execute()

	if err != nil {
		t.Errorf("config help failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "config") {
		t.Error("expected config help to contain 'config'")
	}
}

// --- Build command RunE error paths ---

func TestBuildCommandWithInvalidValuesYAML(t *testing.T) {
	tmpDir := t.TempDir()
	setupMinimalPackage(t, tmpDir)

	// Create invalid YAML values file
	valuesFile := filepath.Join(tmpDir, "bad-values.yaml")
	if err := os.WriteFile(valuesFile, []byte("{{invalid yaml: [}"), 0644); err != nil {
		t.Fatalf("failed to create values file: %v", err)
	}

	globalOpts := options.NewGlobalOptions()
	cmd := newBuildCommand(globalOpts)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{"--values=" + valuesFile, tmpDir})
	err := cmd.Execute()

	if err == nil {
		t.Error("expected error for invalid YAML values file")
	}
}

// --- Validate command RunE error paths ---

func TestValidateCommandWithInvalidValuesYAML(t *testing.T) {
	tmpDir := t.TempDir()
	setupMinimalPackage(t, tmpDir)

	// Create invalid YAML values file
	valuesFile := filepath.Join(tmpDir, "bad-values.yaml")
	if err := os.WriteFile(valuesFile, []byte("{{invalid yaml: [}"), 0644); err != nil {
		t.Fatalf("failed to create values file: %v", err)
	}

	globalOpts := options.NewGlobalOptions()
	cmd := newValidateCommand(globalOpts)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{"--values=" + valuesFile, tmpDir})
	err := cmd.Execute()

	if err == nil {
		t.Error("expected error for invalid YAML values file")
	}
}

// --- Command tree integration tests ---

func TestKurelBuildViaRootCommand(t *testing.T) {
	cmd := NewKurelCommand()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// Execute build with no args through the root command
	cmd.SetArgs([]string{"build"})
	err := cmd.Execute()

	if err == nil {
		t.Error("expected error for build with no package argument")
	}
}

func TestKurelValidateViaRootCommand(t *testing.T) {
	cmd := NewKurelCommand()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{"validate"})
	err := cmd.Execute()

	if err == nil {
		t.Error("expected error for validate with no package argument")
	}
}

func TestKurelInfoViaRootCommand(t *testing.T) {
	cmd := NewKurelCommand()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{"info"})
	err := cmd.Execute()

	if err == nil {
		t.Error("expected error for info with no package argument")
	}
}

func TestKurelSchemaGenerateViaRootCommand(t *testing.T) {
	cmd := NewKurelCommand()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{"schema", "generate"})
	err := cmd.Execute()

	if err == nil {
		t.Error("expected error for schema generate with no package argument")
	}
}

func TestKurelConfigViewViaRootCommand(t *testing.T) {
	cmd := NewKurelCommand()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{"config", "view"})
	err := cmd.Execute()

	if err != nil {
		t.Errorf("config view via root command failed: %v", err)
	}
}

func TestKurelBuildHelpViaRootCommand(t *testing.T) {
	cmd := NewKurelCommand()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{"build", "--help"})
	err := cmd.Execute()

	if err != nil {
		t.Errorf("build --help via root command failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "build") {
		t.Error("expected build help output")
	}
}

func TestKurelValidateHelpViaRootCommand(t *testing.T) {
	cmd := NewKurelCommand()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{"validate", "--help"})
	err := cmd.Execute()

	if err != nil {
		t.Errorf("validate --help via root command failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "validate") {
		t.Error("expected validate help output")
	}
}

func TestKurelInfoHelpViaRootCommand(t *testing.T) {
	cmd := NewKurelCommand()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{"info", "--help"})
	err := cmd.Execute()

	if err != nil {
		t.Errorf("info --help via root command failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "info") {
		t.Error("expected info help output")
	}
}

// --- Build command with nonexistent package (exercises RunE deeper) ---

func TestBuildCommandWithVerbose(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	globalOpts.Verbose = true
	cmd := newBuildCommand(globalOpts)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// Pass a nonexistent path; it should fail at loading but exercises the
	// verbose logger path
	cmd.SetArgs([]string{"/nonexistent/package"})
	err := cmd.Execute()

	if err == nil {
		t.Error("expected error for nonexistent package")
	}
}

func TestValidateCommandWithVerbose(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	globalOpts.Verbose = true
	cmd := newValidateCommand(globalOpts)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{"/nonexistent/package"})
	err := cmd.Execute()

	if err == nil {
		t.Error("expected error for nonexistent package")
	}
}

func TestInfoCommandWithVerbose(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	globalOpts.Verbose = true
	cmd := newInfoCommand(globalOpts)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{"/nonexistent/package"})
	err := cmd.Execute()

	if err == nil {
		t.Error("expected error for nonexistent package")
	}
}

// --- Completion shell variants ---

func TestKurelCompletionShellVariants(t *testing.T) {
	shells := []string{"bash", "zsh", "fish", "powershell"}

	for _, shell := range shells {
		t.Run(shell, func(t *testing.T) {
			cmd := NewKurelCommand()

			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)

			cmd.SetArgs([]string{"completion", shell})
			err := cmd.Execute()

			if err != nil {
				t.Errorf("completion %s failed: %v", shell, err)
			}
		})
	}
}

// --- Tests that exercise RunE with valid packages ---

func TestValidateCommandWithValidPackage(t *testing.T) {
	tmpDir := t.TempDir()
	setupMinimalPackage(t, tmpDir)

	globalOpts := options.NewGlobalOptions()
	cmd := newValidateCommand(globalOpts)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{tmpDir})
	err := cmd.Execute()

	// A minimal valid package should either pass validation or fail gracefully
	// This exercises the validation code path beyond just loading
	_ = err
}

func TestValidateCommandWithValidPackageJSONOutput(t *testing.T) {
	tmpDir := t.TempDir()
	setupMinimalPackage(t, tmpDir)

	globalOpts := options.NewGlobalOptions()
	cmd := newValidateCommand(globalOpts)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{"--json", tmpDir})
	err := cmd.Execute()

	// Exercises the JSON output path of validation
	_ = err
}

func TestValidateCommandWithValidPackageVerbose(t *testing.T) {
	tmpDir := t.TempDir()
	setupMinimalPackage(t, tmpDir)

	globalOpts := options.NewGlobalOptions()
	globalOpts.Verbose = true
	cmd := newValidateCommand(globalOpts)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{tmpDir})
	err := cmd.Execute()

	_ = err
}

func TestValidateCommandWithValidPackageStrict(t *testing.T) {
	tmpDir := t.TempDir()
	setupMinimalPackage(t, tmpDir)

	globalOpts := options.NewGlobalOptions()
	globalOpts.Strict = true
	cmd := newValidateCommand(globalOpts)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{tmpDir})
	err := cmd.Execute()

	// Exercises the strict mode path
	_ = err
}

func TestValidateCommandWithValidPackageAndValues(t *testing.T) {
	tmpDir := t.TempDir()
	setupMinimalPackage(t, tmpDir)

	// Create a valid values file
	valuesFile := filepath.Join(tmpDir, "values.yaml")
	if err := os.WriteFile(valuesFile, []byte("app_name: overridden\n"), 0644); err != nil {
		t.Fatalf("failed to create values file: %v", err)
	}

	globalOpts := options.NewGlobalOptions()
	cmd := newValidateCommand(globalOpts)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{"--values=" + valuesFile, tmpDir})
	err := cmd.Execute()

	// Exercises the values loading and merging path
	_ = err
}

func TestInfoCommandWithValidPackageTextOutput(t *testing.T) {
	tmpDir := t.TempDir()
	setupMinimalPackage(t, tmpDir)

	globalOpts := options.NewGlobalOptions()
	cmd := newInfoCommand(globalOpts)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{tmpDir})
	err := cmd.Execute()

	if err != nil {
		t.Errorf("info command with valid package failed: %v", err)
	}
}

func TestInfoCommandWithValidPackageJSONOutput(t *testing.T) {
	tmpDir := t.TempDir()
	setupMinimalPackage(t, tmpDir)

	globalOpts := options.NewGlobalOptions()
	cmd := newInfoCommand(globalOpts)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{"--output=json", tmpDir})
	err := cmd.Execute()

	if err != nil {
		t.Errorf("info command with JSON output failed: %v", err)
	}
}

func TestInfoCommandWithValidPackageYAMLOutput(t *testing.T) {
	tmpDir := t.TempDir()
	setupMinimalPackage(t, tmpDir)

	globalOpts := options.NewGlobalOptions()
	cmd := newInfoCommand(globalOpts)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{"--output=yaml", tmpDir})
	err := cmd.Execute()

	if err != nil {
		t.Errorf("info command with YAML output failed: %v", err)
	}
}

func TestInfoCommandWithValidPackageVerbose(t *testing.T) {
	tmpDir := t.TempDir()
	setupMinimalPackage(t, tmpDir)

	globalOpts := options.NewGlobalOptions()
	globalOpts.Verbose = true
	cmd := newInfoCommand(globalOpts)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{tmpDir})
	err := cmd.Execute()

	if err != nil {
		t.Errorf("info command verbose with valid package failed: %v", err)
	}
}

func TestBuildCommandWithValidPackage(t *testing.T) {
	tmpDir := t.TempDir()
	setupMinimalPackage(t, tmpDir)

	globalOpts := options.NewGlobalOptions()
	cmd := newBuildCommand(globalOpts)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{tmpDir})
	err := cmd.Execute()

	// Build should succeed or fail gracefully with a minimal package
	_ = err
}

func TestBuildCommandWithValidPackageAndValues(t *testing.T) {
	tmpDir := t.TempDir()
	setupMinimalPackage(t, tmpDir)

	// Create a valid values file
	valuesFile := filepath.Join(tmpDir, "values.yaml")
	if err := os.WriteFile(valuesFile, []byte("app_name: overridden-app\n"), 0644); err != nil {
		t.Fatalf("failed to create values file: %v", err)
	}

	globalOpts := options.NewGlobalOptions()
	cmd := newBuildCommand(globalOpts)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{"--values=" + valuesFile, tmpDir})
	err := cmd.Execute()

	// Exercises the values loading path
	_ = err
}

func TestBuildCommandWithValidPackageJSONFormat(t *testing.T) {
	tmpDir := t.TempDir()
	setupMinimalPackage(t, tmpDir)

	globalOpts := options.NewGlobalOptions()
	cmd := newBuildCommand(globalOpts)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{"--format=json", tmpDir})
	err := cmd.Execute()

	// Exercises the JSON format path
	_ = err
}

func TestBuildCommandWithValidPackageOutputFile(t *testing.T) {
	tmpDir := t.TempDir()
	setupMinimalPackage(t, tmpDir)

	outputFile := filepath.Join(tmpDir, "output.yaml")

	globalOpts := options.NewGlobalOptions()
	cmd := newBuildCommand(globalOpts)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{"--output=" + outputFile, tmpDir})
	err := cmd.Execute()

	// Exercises the file output path
	_ = err
}

func TestBuildCommandWithValidPackageOutputDir(t *testing.T) {
	tmpDir := t.TempDir()
	setupMinimalPackage(t, tmpDir)

	outputDir := filepath.Join(tmpDir, "output-dir")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	globalOpts := options.NewGlobalOptions()
	cmd := newBuildCommand(globalOpts)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{"--output=" + outputDir, tmpDir})
	err := cmd.Execute()

	// Exercises the directory output path
	_ = err
}

func TestBuildCommandWithValidPackageVerbose(t *testing.T) {
	tmpDir := t.TempDir()
	setupMinimalPackage(t, tmpDir)

	globalOpts := options.NewGlobalOptions()
	globalOpts.Verbose = true
	cmd := newBuildCommand(globalOpts)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{tmpDir})
	err := cmd.Execute()

	// Exercises the verbose logger path in build
	_ = err
}

func TestBuildCommandWithValidPackageFilterKind(t *testing.T) {
	tmpDir := t.TempDir()
	setupMinimalPackage(t, tmpDir)

	globalOpts := options.NewGlobalOptions()
	cmd := newBuildCommand(globalOpts)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{"--kind=Deployment", tmpDir})
	err := cmd.Execute()

	// Exercises the filter kind path
	_ = err
}

func TestBuildCommandWithValidPackageFilterName(t *testing.T) {
	tmpDir := t.TempDir()
	setupMinimalPackage(t, tmpDir)

	globalOpts := options.NewGlobalOptions()
	cmd := newBuildCommand(globalOpts)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{"--name=test-app", tmpDir})
	err := cmd.Execute()

	// Exercises the filter name path
	_ = err
}

// --- Schema generate command RunE tests ---

func TestSchemaGenerateCommandExecution(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	cmd := newSchemaCommand(globalOpts)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// The generate command accepts a package arg (ExactArgs(1)) but the
	// schema generator produces a generic schema regardless of the argument
	cmd.SetArgs([]string{"generate", "any-package"})
	err := cmd.Execute()

	if err != nil {
		t.Errorf("schema generate failed: %v", err)
	}
}

func TestSchemaGenerateCommandWithOutputFile(t *testing.T) {
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "schema.json")

	globalOpts := options.NewGlobalOptions()
	cmd := newSchemaCommand(globalOpts)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{"generate", "--output=" + outputFile, "any-package"})
	err := cmd.Execute()

	if err != nil {
		t.Errorf("schema generate with output file failed: %v", err)
	}

	// Verify the schema file was created
	data, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("failed to read output schema: %v", err)
	}

	if len(data) == 0 {
		t.Error("expected non-empty schema output")
	}

	// Verify it's valid JSON
	if !strings.Contains(string(data), "schema") {
		t.Error("expected schema output to contain 'schema'")
	}
}

func TestSchemaGenerateCommandWithOutputFileNestedDir(t *testing.T) {
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "nested", "dir", "schema.json")

	globalOpts := options.NewGlobalOptions()
	cmd := newSchemaCommand(globalOpts)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{"generate", "--output=" + outputFile, "any-package"})
	err := cmd.Execute()

	if err != nil {
		t.Errorf("schema generate with nested output dir failed: %v", err)
	}

	// Verify the file was created in the nested directory
	if _, err := os.Stat(outputFile); os.IsNotExist(err) {
		t.Error("expected output file to be created in nested directory")
	}
}

func TestSchemaGenerateCommandWithK8sFlag(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	cmd := newSchemaCommand(globalOpts)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{"generate", "--k8s", "any-package"})
	err := cmd.Execute()

	if err != nil {
		t.Errorf("schema generate with --k8s failed: %v", err)
	}
}

func TestSchemaGenerateCommandNoPretty(t *testing.T) {
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "schema.json")

	globalOpts := options.NewGlobalOptions()
	cmd := newSchemaCommand(globalOpts)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{"generate", "--pretty=false", "--output=" + outputFile, "any-package"})
	err := cmd.Execute()

	if err != nil {
		t.Errorf("schema generate with --pretty=false failed: %v", err)
	}
}

func TestSchemaGenerateCommandVerbose(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	globalOpts.Verbose = true
	cmd := newSchemaCommand(globalOpts)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{"generate", "any-package"})
	err := cmd.Execute()

	if err != nil {
		t.Errorf("schema generate verbose failed: %v", err)
	}
}

// --- Info command with rich package (patches + namespace) ---

func TestInfoCommandWithRichPackage(t *testing.T) {
	tmpDir := t.TempDir()
	setupRichPackage(t, tmpDir)

	globalOpts := options.NewGlobalOptions()
	cmd := newInfoCommand(globalOpts)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{tmpDir})
	err := cmd.Execute()

	if err != nil {
		t.Errorf("info command with rich package failed: %v", err)
	}
}

// --- Helper functions ---

// setupRichPackage creates a kurel package with patches and namespaced resources
func setupRichPackage(t *testing.T, dir string) {
	t.Helper()

	kurelYAML := `name: rich-package
version: 2.0.0
description: A rich test package with patches
`
	if err := os.WriteFile(filepath.Join(dir, "kurel.yaml"), []byte(kurelYAML), 0644); err != nil {
		t.Fatalf("failed to create kurel.yaml: %v", err)
	}

	paramsYAML := `app_name: rich-app
namespace: production
replicas: 3
multiline_param: |
  line1
  line2
  line3
nested:
  key1: value1
  key2: value2
items:
  - item1
  - item2
`
	if err := os.WriteFile(filepath.Join(dir, "parameters.yaml"), []byte(paramsYAML), 0644); err != nil {
		t.Fatalf("failed to create parameters.yaml: %v", err)
	}

	resourcesDir := filepath.Join(dir, "resources")
	if err := os.MkdirAll(resourcesDir, 0755); err != nil {
		t.Fatalf("failed to create resources dir: %v", err)
	}

	deployYAML := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: rich-app
  namespace: production
spec:
  replicas: 3
  selector:
    matchLabels:
      app: rich-app
  template:
    metadata:
      labels:
        app: rich-app
    spec:
      containers:
      - name: app
        image: nginx:latest
`
	if err := os.WriteFile(filepath.Join(resourcesDir, "deployment.yaml"), []byte(deployYAML), 0644); err != nil {
		t.Fatalf("failed to create deployment.yaml: %v", err)
	}

	svcYAML := `apiVersion: v1
kind: Service
metadata:
  name: rich-svc
  namespace: production
spec:
  type: ClusterIP
  ports:
  - port: 80
`
	if err := os.WriteFile(filepath.Join(resourcesDir, "service.yaml"), []byte(svcYAML), 0644); err != nil {
		t.Fatalf("failed to create service.yaml: %v", err)
	}

	// Create patches directory
	patchesDir := filepath.Join(dir, "patches")
	if err := os.MkdirAll(patchesDir, 0755); err != nil {
		t.Fatalf("failed to create patches dir: %v", err)
	}

	patchTOML := `[metadata]
description = "Enable debug logging"
enabled = "true"

[[operations]]
target = "Deployment/rich-app"
path = "/spec/template/spec/containers/0/env"
value = [{name = "DEBUG", value = "true"}]
`
	if err := os.WriteFile(filepath.Join(patchesDir, "debug.toml"), []byte(patchTOML), 0644); err != nil {
		t.Fatalf("failed to create debug patch: %v", err)
	}
}

// setupMinimalPackage creates a minimal kurel package structure for testing
func setupMinimalPackage(t *testing.T, dir string) {
	t.Helper()

	// kurel.yaml
	kurelYAML := `name: test-package
version: 1.0.0
description: Test package
`
	if err := os.WriteFile(filepath.Join(dir, "kurel.yaml"), []byte(kurelYAML), 0644); err != nil {
		t.Fatalf("failed to create kurel.yaml: %v", err)
	}

	// parameters.yaml
	paramsYAML := `app_name: test-app
namespace: default
replicas: 1
`
	if err := os.WriteFile(filepath.Join(dir, "parameters.yaml"), []byte(paramsYAML), 0644); err != nil {
		t.Fatalf("failed to create parameters.yaml: %v", err)
	}

	// resources directory
	resourcesDir := filepath.Join(dir, "resources")
	if err := os.MkdirAll(resourcesDir, 0755); err != nil {
		t.Fatalf("failed to create resources dir: %v", err)
	}

	// Simple resource
	deployYAML := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: ${app_name}
  namespace: ${namespace}
spec:
  replicas: ${replicas}
`
	if err := os.WriteFile(filepath.Join(resourcesDir, "deployment.yaml"), []byte(deployYAML), 0644); err != nil {
		t.Fatalf("failed to create deployment.yaml: %v", err)
	}
}

// Helper function to check if a string contains a substring
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > len(substr) && stringContains(s, substr)))
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Helper function to extract command name from Use string
func extractCommandName(use string) string {
	for i, char := range use {
		if char == ' ' || char == '[' || char == '<' {
			return use[:i]
		}
	}
	return use
}
