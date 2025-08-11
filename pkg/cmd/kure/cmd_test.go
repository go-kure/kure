package kure

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"

	"github.com/go-kure/kure/pkg/cmd/shared/options"
)

func TestNewKureCommand(t *testing.T) {
	cmd := NewKureCommand()
	
	if cmd == nil {
		t.Fatal("expected non-nil command")
	}
	
	if cmd.Use != "kure" {
		t.Errorf("expected command name 'kure', got %s", cmd.Use)
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

func TestKureCommandSubcommands(t *testing.T) {
	cmd := NewKureCommand()
	
	expectedSubcommands := []string{
		"generate", "validate", "config", "version",
	}
	
	commands := cmd.Commands()
	if len(commands) < len(expectedSubcommands) {
		t.Errorf("expected at least %d subcommands, got %d", len(expectedSubcommands), len(commands))
	}
	
	// Check that expected subcommands exist
	commandMap := make(map[string]*cobra.Command)
	for _, subCmd := range commands {
		commandMap[extractCommandName(subCmd.Use)] = subCmd
	}
	
	for _, expectedCmd := range expectedSubcommands {
		if _, exists := commandMap[expectedCmd]; !exists {
			t.Errorf("expected subcommand %s not found", expectedCmd)
		}
	}
}

func TestKureCommandFlags(t *testing.T) {
	cmd := NewKureCommand()
	
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

func TestKureCommandHelp(t *testing.T) {
	cmd := NewKureCommand()
	
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
	expectedContent := []string{"kure", "Usage:", "Available Commands:", "Flags:"}
	for _, content := range expectedContent {
		if !containsString(output, content) {
			t.Errorf("expected help output to contain %q", content)
		}
	}
}

func TestKureCommandPersistentPreRun(t *testing.T) {
	cmd := NewKureCommand()
	
	// Mock arguments for testing
	cmd.SetArgs([]string{"--output=json", "--verbose"})
	
	// Execute persistent pre-run
	err := cmd.PersistentPreRunE(cmd, []string{})
	if err != nil {
		t.Errorf("persistent pre-run failed: %v", err)
	}
}

func TestNewGenerateCommand(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	cmd := newGenerateCommand(globalOpts)
	
	if cmd == nil {
		t.Fatal("expected non-nil generate command")
	}
	
	if cmd.Use != "generate" {
		t.Errorf("expected command name 'generate', got %s", cmd.Use)
	}
}

func TestNewValidateCommand(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	cmd := newValidateCommand(globalOpts)
	
	if cmd == nil {
		t.Fatal("expected non-nil validate command")
	}
	
	if cmd.Use != "validate" {
		t.Errorf("expected command name 'validate', got %s", cmd.Use)
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

func TestKureCommandInvalidFlags(t *testing.T) {
	cmd := NewKureCommand()
	
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	
	// Test with invalid output format - this should be caught during validation
	cmd.SetArgs([]string{"--output=invalid-format", "version"})
	err := cmd.Execute()
	
	if err == nil {
		t.Error("expected error for invalid output format")
	}
}

func TestKureCommandVersion(t *testing.T) {
	cmd := NewKureCommand()
	
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	
	// Test version command
	cmd.SetArgs([]string{"version"})
	err := cmd.Execute()
	
	if err != nil {
		t.Errorf("version command failed: %v", err)
	}
	
	// Version command writes to stdout, so check that
	// Output is actually written to the buffer we set up
	// Note: the version command might write to stderr in some cases
}

func TestKureCommandCompletion(t *testing.T) {
	cmd := NewKureCommand()
	
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	
	// Test completion command
	cmd.SetArgs([]string{"completion", "bash"})
	err := cmd.Execute()
	
	if err != nil {
		t.Errorf("completion command failed: %v", err)
	}
	
	// Note: Completion output might be written to stdout directly by cobra,
	// not necessarily through our buffer
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
		if char == ' ' || char == '[' {
			return use[:i]
		}
	}
	return use
}

func TestKureCommandExecuteError(t *testing.T) {
	// This test verifies error handling in Execute function
	// We can't easily test the actual Execute function without mocking os.Exit
	// So we'll test the command structure instead
	cmd := NewKureCommand()
	
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

func TestKureCommandFlagValidation(t *testing.T) {
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
			cmd := NewKureCommand()
			
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