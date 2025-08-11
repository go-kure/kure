package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
	
	"github.com/spf13/cobra"
	
	"github.com/go-kure/kure/pkg/cmd/kurel"
)

func TestMain_Integration(t *testing.T) {
	// Test that main function doesn't panic
	// We can't easily test main() directly because it calls os.Exit
	// So we test the underlying Execute() function instead
	
	// Save original command line args
	originalArgs := os.Args
	defer func() { os.Args = originalArgs }()
	
	// Test help command
	os.Args = []string{"kurel", "--help"}
	
	// This would normally call kurel.Execute() but we can't test that
	// directly without mocking os.Exit, so we test the command structure
	cmd := kurel.NewKurelCommand()
	if cmd == nil {
		t.Fatal("NewKurelCommand returned nil")
	}
	
	// Test that the command has the expected structure
	if cmd.Use != "kurel" {
		t.Errorf("Command name = %s, want kurel", cmd.Use)
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
	cmd := kurel.NewKurelCommand()
	
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
		"kurel",
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
	cmd := kurel.NewKurelCommand()
	
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
	cmd := kurel.NewKurelCommand()
	
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
	cmd := kurel.NewKurelCommand()
	
	subCommands := cmd.Commands()
	if len(subCommands) == 0 {
		t.Error("Expected subcommands, got none")
	}
	
	// Check for expected subcommands
	expectedCommands := []string{"build", "validate", "info", "schema", "config", "version"}
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
	cmd := kurel.NewKurelCommand()
	
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
	cmd := kurel.NewKurelCommand()
	
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
			cmd := kurel.NewKurelCommand()
			
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
	cmd := kurel.NewKurelCommand()
	
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
	// kurel.Execute function is available and the command structure is correct
	
	// Verify that kurel.NewKurelCommand creates a valid command
	cmd := kurel.NewKurelCommand()
	if cmd == nil {
		t.Fatal("kurel.NewKurelCommand() returned nil")
	}
	
	// This is what main() would call
	// We can't call kurel.Execute() directly in tests because it calls os.Exit
	// But we can verify the command structure
	if cmd.Use != "kurel" {
		t.Errorf("Expected command use 'kurel', got %s", cmd.Use)
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
	cmd := kurel.NewKurelCommand()
	
	// Test that persistent pre-run doesn't fail with basic args
	err := cmd.PersistentPreRunE(cmd, []string{})
	if err != nil {
		t.Errorf("PersistentPreRunE failed: %v", err)
	}
}

func TestMain_CommandUsage(t *testing.T) {
	// Test command usage generation
	cmd := kurel.NewKurelCommand()
	
	usage := cmd.UsageString()
	if usage == "" {
		t.Error("Command usage string is empty")
	}
	
	if !strings.Contains(usage, "kurel") {
		t.Error("Usage string should contain 'kurel'")
	}
}

func TestMain_BuildCommand(t *testing.T) {
	// Test that build command is available
	cmd := kurel.NewKurelCommand()
	
	var buildCmd *cobra.Command
	for _, subCmd := range cmd.Commands() {
		if strings.HasPrefix(subCmd.Use, "build") {
			buildCmd = subCmd
			break
		}
	}
	
	if buildCmd == nil {
		t.Error("Build command not found")
	} else {
		if buildCmd.Short == "" {
			t.Error("Build command should have a short description")
		}
		if buildCmd.Long == "" {
			t.Error("Build command should have a long description")
		}
	}
}

func TestMain_ValidateCommand(t *testing.T) {
	// Test that validate command is available
	cmd := kurel.NewKurelCommand()
	
	var validateCmd *cobra.Command
	for _, subCmd := range cmd.Commands() {
		if strings.HasPrefix(subCmd.Use, "validate") {
			validateCmd = subCmd
			break
		}
	}
	
	if validateCmd == nil {
		t.Error("Validate command not found")
	} else {
		if validateCmd.Short == "" {
			t.Error("Validate command should have a short description")
		}
		if validateCmd.Long == "" {
			t.Error("Validate command should have a long description")
		}
	}
}

func TestMain_InfoCommand(t *testing.T) {
	// Test that info command is available
	cmd := kurel.NewKurelCommand()
	
	var infoCmd *cobra.Command
	for _, subCmd := range cmd.Commands() {
		if strings.HasPrefix(subCmd.Use, "info") {
			infoCmd = subCmd
			break
		}
	}
	
	if infoCmd == nil {
		t.Error("Info command not found")
	} else {
		if infoCmd.Short == "" {
			t.Error("Info command should have a short description")
		}
		if infoCmd.Long == "" {
			t.Error("Info command should have a long description")
		}
	}
}

func TestMain_SchemaCommand(t *testing.T) {
	// Test that schema command is available
	cmd := kurel.NewKurelCommand()
	
	var schemaCmd *cobra.Command
	for _, subCmd := range cmd.Commands() {
		if strings.HasPrefix(subCmd.Use, "schema") {
			schemaCmd = subCmd
			break
		}
	}
	
	if schemaCmd == nil {
		t.Error("Schema command not found")
	} else {
		if schemaCmd.Short == "" {
			t.Error("Schema command should have a short description")
		}
		if schemaCmd.Long == "" {
			t.Error("Schema command should have a long description")
		}
	}
}

func TestMain_ConfigCommand(t *testing.T) {
	// Test that config command is available
	cmd := kurel.NewKurelCommand()
	
	var configCmd *cobra.Command
	for _, subCmd := range cmd.Commands() {
		if strings.HasPrefix(subCmd.Use, "config") {
			configCmd = subCmd
			break
		}
	}
	
	if configCmd == nil {
		t.Error("Config command not found")
	} else {
		if configCmd.Short == "" {
			t.Error("Config command should have a short description")
		}
		if configCmd.Long == "" {
			t.Error("Config command should have a long description")
		}
	}
}

func TestMain_BuildCommandFlags(t *testing.T) {
	// Test build command specific flags
	cmd := kurel.NewKurelCommand()
	
	var buildCmd *cobra.Command
	for _, subCmd := range cmd.Commands() {
		if strings.HasPrefix(subCmd.Use, "build") {
			buildCmd = subCmd
			break
		}
	}
	
	if buildCmd == nil {
		t.Skip("Build command not found, skipping flag test")
		return
	}
	
	// Check for expected build-specific flags
	expectedBuildFlags := []string{"output", "values", "format"}
	for _, flagName := range expectedBuildFlags {
		flag := buildCmd.Flags().Lookup(flagName)
		if flag == nil {
			t.Errorf("Expected build command flag %s not found", flagName)
		}
	}
}

func TestMain_CommandAliases(t *testing.T) {
	// Test command aliases if any exist
	cmd := kurel.NewKurelCommand()
	
	// Check if any commands have aliases
	for _, subCmd := range cmd.Commands() {
		if len(subCmd.Aliases) > 0 {
			// If aliases exist, they should be non-empty strings
			for _, alias := range subCmd.Aliases {
				if alias == "" {
					t.Errorf("Command %s has empty alias", subCmd.Use)
				}
			}
		}
	}
}