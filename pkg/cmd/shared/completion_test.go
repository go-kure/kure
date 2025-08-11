package shared

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
)

func TestNewCompletionCommand(t *testing.T) {
	cmd := NewCompletionCommand()
	if cmd == nil {
		t.Fatal("expected non-nil completion command")
	}

	// Test command properties
	if cmd.Use != "completion [bash|zsh|fish|powershell]" {
		t.Errorf("expected Use to be 'completion [bash|zsh|fish|powershell]', got %s", cmd.Use)
	}

	if cmd.Short != "Generate completion script" {
		t.Errorf("expected Short to be 'Generate completion script', got %s", cmd.Short)
	}

	expectedLong := `Generate the autocompletion script for the specified shell.
See each sub-command's help for details on how to use the generated script.`
	if cmd.Long != expectedLong {
		t.Errorf("expected Long description to match, got %s", cmd.Long)
	}

	if !cmd.DisableFlagsInUseLine {
		t.Error("expected DisableFlagsInUseLine to be true")
	}

	expectedValidArgs := []string{"bash", "zsh", "fish", "powershell"}
	if len(cmd.ValidArgs) != len(expectedValidArgs) {
		t.Errorf("expected %d valid args, got %d", len(expectedValidArgs), len(cmd.ValidArgs))
	}

	for i, arg := range expectedValidArgs {
		if i >= len(cmd.ValidArgs) || cmd.ValidArgs[i] != arg {
			t.Errorf("expected ValidArgs[%d] to be %s, got %v", i, arg, cmd.ValidArgs)
		}
	}
}

func TestCompletionCommand_BashCompletion(t *testing.T) {
	// Create a root command to test completion generation
	rootCmd := &cobra.Command{
		Use:   "test",
		Short: "Test command",
	}

	completionCmd := NewCompletionCommand()
	rootCmd.AddCommand(completionCmd)

	// Capture output
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)

	// Execute bash completion
	rootCmd.SetArgs([]string{"completion", "bash"})
	err := rootCmd.Execute()

	if err != nil {
		t.Fatalf("unexpected error executing bash completion: %v", err)
	}

	// Note: The completion command outputs directly to os.Stdout,
	// so we can't capture the output in tests without modifying the code.
	// We just verify that the command executes without error.
}

func TestCompletionCommand_ZshCompletion(t *testing.T) {
	// Create a root command to test completion generation
	rootCmd := &cobra.Command{
		Use:   "test",
		Short: "Test command",
	}

	completionCmd := NewCompletionCommand()
	rootCmd.AddCommand(completionCmd)

	// Capture output
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)

	// Execute zsh completion
	rootCmd.SetArgs([]string{"completion", "zsh"})
	err := rootCmd.Execute()

	if err != nil {
		t.Fatalf("unexpected error executing zsh completion: %v", err)
	}

	// Note: The completion command outputs directly to os.Stdout,
	// so we can't capture the output in tests without modifying the code.
	// We just verify that the command executes without error.
}

func TestCompletionCommand_FishCompletion(t *testing.T) {
	// Create a root command to test completion generation
	rootCmd := &cobra.Command{
		Use:   "test",
		Short: "Test command",
	}

	completionCmd := NewCompletionCommand()
	rootCmd.AddCommand(completionCmd)

	// Capture output
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)

	// Execute fish completion
	rootCmd.SetArgs([]string{"completion", "fish"})
	err := rootCmd.Execute()

	if err != nil {
		t.Fatalf("unexpected error executing fish completion: %v", err)
	}

	// Note: The completion command outputs directly to os.Stdout,
	// so we can't capture the output in tests without modifying the code.
	// We just verify that the command executes without error.
}

func TestCompletionCommand_PowerShellCompletion(t *testing.T) {
	// Create a root command to test completion generation
	rootCmd := &cobra.Command{
		Use:   "test",
		Short: "Test command",
	}

	completionCmd := NewCompletionCommand()
	rootCmd.AddCommand(completionCmd)

	// Capture output
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)

	// Execute powershell completion
	rootCmd.SetArgs([]string{"completion", "powershell"})
	err := rootCmd.Execute()

	if err != nil {
		t.Fatalf("unexpected error executing powershell completion: %v", err)
	}

	// Note: The completion command outputs directly to os.Stdout,
	// so we can't capture the output in tests without modifying the code.
	// We just verify that the command executes without error.
}

func TestCompletionCommand_InvalidArg(t *testing.T) {
	// Create a root command to test completion generation
	rootCmd := &cobra.Command{
		Use:   "test",
		Short: "Test command",
	}

	completionCmd := NewCompletionCommand()
	rootCmd.AddCommand(completionCmd)

	// Capture output
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)

	// Execute with invalid shell type
	rootCmd.SetArgs([]string{"completion", "invalid-shell"})
	err := rootCmd.Execute()

	if err == nil {
		t.Error("expected error when using invalid shell type")
	}
}

func TestCompletionCommand_NoArgs(t *testing.T) {
	// Create a root command to test completion generation
	rootCmd := &cobra.Command{
		Use:   "test",
		Short: "Test command",
	}

	completionCmd := NewCompletionCommand()
	rootCmd.AddCommand(completionCmd)

	// Capture output
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)

	// Execute without arguments
	rootCmd.SetArgs([]string{"completion"})
	err := rootCmd.Execute()

	if err == nil {
		t.Error("expected error when no shell type provided")
	}
}

func TestCompletionCommand_TooManyArgs(t *testing.T) {
	// Create a root command to test completion generation
	rootCmd := &cobra.Command{
		Use:   "test",
		Short: "Test command",
	}

	completionCmd := NewCompletionCommand()
	rootCmd.AddCommand(completionCmd)

	// Capture output
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)

	// Execute with too many arguments
	rootCmd.SetArgs([]string{"completion", "bash", "extra"})
	err := rootCmd.Execute()

	if err == nil {
		t.Error("expected error when too many arguments provided")
	}
}

func TestCompletionCommand_ArgsValidation(t *testing.T) {
	cmd := NewCompletionCommand()

	// Test Args function behavior
	if cmd.Args == nil {
		t.Fatal("expected Args function to be set")
	}

	// Create a test command to validate args
	testCmd := &cobra.Command{Use: "test"}

	// Test valid args - these should pass validation
	validShells := []string{"bash", "zsh", "fish", "powershell"}
	for _, shell := range validShells {
		err := cmd.Args(testCmd, []string{shell})
		if err != nil {
			t.Errorf("expected valid shell %s to pass validation, got error: %v", shell, err)
		}
	}

	// Test no args - should fail (ExactArgs(1))
	err := cmd.Args(testCmd, []string{})
	if err == nil {
		t.Error("expected no args to fail validation due to ExactArgs(1)")
	}

	// Test too many args - should fail (ExactArgs(1))
	err = cmd.Args(testCmd, []string{"bash", "extra"})
	if err == nil {
		t.Error("expected too many args to fail validation due to ExactArgs(1)")
	}

	// Note: Testing invalid args is complex because OnlyValidArgs validation
	// happens during command execution, not in the Args function alone
	// The Args function combines ExactArgs(1) and OnlyValidArgs, but
	// OnlyValidArgs may not reject at the Args level during testing
}
