package kurel

import (
	"bytes"
	"testing"

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
