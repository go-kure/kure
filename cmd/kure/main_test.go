package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
	
	"github.com/go-kure/kure/pkg/cmd/kure"
)

func TestMain_Integration(t *testing.T) {
	// Test that main function doesn't panic
	// We can't easily test main() directly because it calls os.Exit
	// So we test the underlying Execute() function instead
	
	// Save original command line args
	originalArgs := os.Args
	defer func() { os.Args = originalArgs }()
	
	// Test help command
	os.Args = []string{"kure", "--help"}
	
	// This would normally call kure.Execute() but we can't test that
	// directly without mocking os.Exit, so we test the command structure
	cmd := kure.NewKureCommand()
	if cmd == nil {
		t.Fatal("NewKureCommand returned nil")
	}
	
	// Test that the command has the expected structure
	if cmd.Use != "kure" {
		t.Errorf("Command name = %s, want kure", cmd.Use)
	}
	
	if cmd.Short == "" {
		t.Error("Command should have a short description")
	}
	
	if cmd.Long == "" {
		t.Error("Command should have a long description")
	}
}

func TestMain_HelpCommand(t *testing.T) {
	// Create the command and test help output
	cmd := kure.NewKureCommand()
	
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	
	// Test help command
	cmd.SetArgs([]string{"--help"})
	err := cmd.Execute()
	
	if err != nil {
		t.Errorf("Help command failed: %v", err)
	}
	
	output := buf.String()
	if output == "" {
		t.Error("Help command produced no output")
	}
	
	// Check for expected content in help
	expectedContent := []string{
		"kure",
		"Usage:",
		"Available Commands:",
		"Flags:",
	}
	
	for _, content := range expectedContent {
		if !strings.Contains(output, content) {
			t.Errorf("Help output missing expected content: %s", content)
		}
	}
}

func TestMain_VersionCommand(t *testing.T) {
	// Create the command and test version output
	cmd := kure.NewKureCommand()
	
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	
	// Test version command
	cmd.SetArgs([]string{"version"})
	err := cmd.Execute()
	
	if err != nil {
		t.Errorf("Version command failed: %v", err)
	}
	
	// Version command should execute without error
	// The actual version output format depends on the implementation
}

func TestMain_InvalidCommand(t *testing.T) {
	// Create the command and test invalid command handling
	cmd := kure.NewKureCommand()
	
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	
	// Test invalid command
	cmd.SetArgs([]string{"invalid-command"})
	err := cmd.Execute()
	
	if err == nil {
		t.Error("Expected error for invalid command, got nil")
	}
}

func TestMain_CommandStructure(t *testing.T) {
	// Test that the main command has expected subcommands
	cmd := kure.NewKureCommand()
	
	subCommands := cmd.Commands()
	if len(subCommands) == 0 {
		t.Error("Expected subcommands, got none")
	}
	
	// Check for expected subcommands
	expectedCommands := []string{"generate", "validate", "config", "version"}
	commandNames := make(map[string]bool)
	
	for _, subCmd := range subCommands {
		// Extract command name (first word of Use field)
		parts := strings.Fields(subCmd.Use)
		if len(parts) > 0 {
			commandNames[parts[0]] = true
		}
	}
	
	for _, expectedCmd := range expectedCommands {
		if !commandNames[expectedCmd] {
			t.Errorf("Expected subcommand %s not found", expectedCmd)
		}
	}
}

func TestMain_PersistentFlags(t *testing.T) {
	// Test that persistent flags are properly configured
	cmd := kure.NewKureCommand()
	
	expectedFlags := []string{
		"config",
		"verbose", 
		"debug",
		"output",
		"dry-run",
		"namespace",
	}
	
	for _, flagName := range expectedFlags {
		flag := cmd.PersistentFlags().Lookup(flagName)
		if flag == nil {
			t.Errorf("Expected persistent flag %s not found", flagName)
		}
	}
}

func TestMain_CommandDefaults(t *testing.T) {
	// Test that the main command has expected defaults
	cmd := kure.NewKureCommand()
	
	// Check silence settings
	if !cmd.SilenceUsage {
		t.Error("Expected SilenceUsage to be true")
	}
	
	if !cmd.SilenceErrors {
		t.Error("Expected SilenceErrors to be true")
	}
	
	// Check that persistent pre-run is configured
	if cmd.PersistentPreRunE == nil {
		t.Error("Expected PersistentPreRunE to be set")
	}
}

func TestMain_FlagValidation(t *testing.T) {
	// Test flag validation
	tests := []struct {
		name      string
		args      []string
		wantError bool
	}{
		{
			name:      "valid output yaml",
			args:      []string{"--output=yaml", "version"},
			wantError: false,
		},
		{
			name:      "valid output json",
			args:      []string{"--output=json", "version"},
			wantError: false,
		},
		{
			name:      "invalid output format",
			args:      []string{"--output=invalid", "version"},
			wantError: true,
		},
		{
			name:      "verbose flag",
			args:      []string{"--verbose", "version"},
			wantError: false,
		},
		{
			name:      "debug flag",
			args:      []string{"--debug", "version"},
			wantError: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := kure.NewKureCommand()
			
			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)
			
			cmd.SetArgs(tt.args)
			err := cmd.Execute()
			
			if tt.wantError && err == nil {
				t.Error("Expected error but got nil")
			}
			
			if !tt.wantError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestMain_CompletionCommand(t *testing.T) {
	// Test completion command
	cmd := kure.NewKureCommand()
	
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	
	// Test bash completion
	cmd.SetArgs([]string{"completion", "bash"})
	err := cmd.Execute()
	
	if err != nil {
		t.Errorf("Completion command failed: %v", err)
	}
}

func TestMain_ExecuteFunction(t *testing.T) {
	// We can't directly test the Execute() function from main
	// because it calls os.Exit, but we can verify that the
	// kure.Execute function is available and the command structure is correct
	
	// Verify that kure.NewKureCommand creates a valid command
	cmd := kure.NewKureCommand()
	if cmd == nil {
		t.Fatal("kure.NewKureCommand() returned nil")
	}
	
	// This is what main() would call
	// We can't call kure.Execute() directly in tests because it calls os.Exit
	// But we can verify the command structure
	if cmd.Use != "kure" {
		t.Errorf("Expected command use 'kure', got %s", cmd.Use)
	}
	
	// Verify the command can be executed (with help to avoid os.Exit)
	cmd.SetArgs([]string{"--help"})
	
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	
	err := cmd.Execute()
	if err != nil {
		t.Errorf("Command execution failed: %v", err)
	}
}

func TestMain_PersistentPreRun(t *testing.T) {
	// Test persistent pre-run functionality
	cmd := kure.NewKureCommand()
	
	// Test that persistent pre-run doesn't fail with basic args
	err := cmd.PersistentPreRunE(cmd, []string{})
	if err != nil {
		t.Errorf("PersistentPreRunE failed: %v", err)
	}
}

func TestMain_CommandUsage(t *testing.T) {
	// Test command usage generation
	cmd := kure.NewKureCommand()
	
	usage := cmd.UsageString()
	if usage == "" {
		t.Error("Command usage string is empty")
	}
	
	if !strings.Contains(usage, "kure") {
		t.Error("Usage string should contain 'kure'")
	}
}