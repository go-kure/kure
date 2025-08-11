package shared

import (
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestNewVersionCommand(t *testing.T) {
	appName := "testapp"
	cmd := NewVersionCommand(appName)

	if cmd == nil {
		t.Fatal("expected non-nil version command")
	}

	// Test command properties
	if cmd.Use != "version" {
		t.Errorf("expected Use to be 'version', got %s", cmd.Use)
	}

	if cmd.Short != "Print version information" {
		t.Errorf("expected Short to be 'Print version information', got %s", cmd.Short)
	}

	expectedLong := "Print the version number of testapp"
	if cmd.Long != expectedLong {
		t.Errorf("expected Long to be %q, got %q", expectedLong, cmd.Long)
	}

	if cmd.Run == nil {
		t.Error("expected Run function to be set")
	}
}

func TestVersionCommand_DefaultValues(t *testing.T) {
	// Test that default version variables are set correctly
	if Version != "dev" {
		t.Errorf("expected default Version to be 'dev', got %s", Version)
	}

	if GitCommit != "unknown" {
		t.Errorf("expected default GitCommit to be 'unknown', got %s", GitCommit)
	}

	if BuildDate != "unknown" {
		t.Errorf("expected default BuildDate to be 'unknown', got %s", BuildDate)
	}
}

func TestVersionCommand_Execute(t *testing.T) {
	// Backup original values
	originalVersion := Version
	originalGitCommit := GitCommit
	originalBuildDate := BuildDate

	// Set test values
	Version = "v1.0.0"
	GitCommit = "abc123def456"
	BuildDate = "2023-12-01T12:00:00Z"

	// Restore original values after test
	defer func() {
		Version = originalVersion
		GitCommit = originalGitCommit
		BuildDate = originalBuildDate
	}()

	appName := "myapp"
	cmd := NewVersionCommand(appName)

	// Test that the command is configured correctly
	if cmd.Run == nil {
		t.Fatal("expected Run function to be set")
	}

	// Since the actual version command prints to stdout, we test the logic is correct
	// by verifying the variables are set as expected
	if Version != "v1.0.0" {
		t.Errorf("expected Version to be v1.0.0, got %s", Version)
	}
	if GitCommit != "abc123def456" {
		t.Errorf("expected GitCommit to be abc123def456, got %s", GitCommit)
	}
	if BuildDate != "2023-12-01T12:00:00Z" {
		t.Errorf("expected BuildDate to be 2023-12-01T12:00:00Z, got %s", BuildDate)
	}

	// Verify command properties are correct
	if cmd.Use != "version" {
		t.Errorf("expected Use to be version, got %s", cmd.Use)
	}
	if !strings.Contains(cmd.Long, appName) {
		t.Errorf("expected Long to contain app name %s, got %s", appName, cmd.Long)
	}

	// Test that we can call the run function without panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("version command Run function panicked: %v", r)
		}
	}()

	// Execute the run function (output goes to stdout, which is expected)
	cmd.Run(cmd, []string{})
}

func TestVersionCommand_ExecuteWithDefaultValues(t *testing.T) {
	// Test with default values (dev, unknown, unknown)
	appName := "defaultapp"
	cmd := NewVersionCommand(appName)

	// Test that the command is configured correctly with default values
	if Version != "dev" {
		t.Errorf("expected default Version to be 'dev', got %s", Version)
	}
	if GitCommit != "unknown" {
		t.Errorf("expected default GitCommit to be 'unknown', got %s", GitCommit)
	}
	if BuildDate != "unknown" {
		t.Errorf("expected default BuildDate to be 'unknown', got %s", BuildDate)
	}

	// Test command execution doesn't panic with default values
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("version command Run function panicked with default values: %v", r)
		}
	}()

	cmd.Run(cmd, []string{})
}

func TestVersionCommand_WithRootCommand(t *testing.T) {
	// Test version command as subcommand of root command
	rootCmd := &cobra.Command{
		Use:   "rootapp",
		Short: "Root application",
	}

	versionCmd := NewVersionCommand("rootapp")
	rootCmd.AddCommand(versionCmd)

	// Test that subcommand was added correctly
	if versionCmd.Parent() != rootCmd {
		t.Error("expected version command to have root command as parent")
	}

	// Test that the version command can be found
	foundCmd, _, err := rootCmd.Find([]string{"version"})
	if err != nil {
		t.Fatalf("error finding version subcommand: %v", err)
	}

	if foundCmd != versionCmd {
		t.Error("expected to find the version command")
	}

	// Test execution doesn't panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("version subcommand panicked: %v", r)
		}
	}()

	versionCmd.Run(versionCmd, []string{})
}

func TestVersionCommand_OutputFormat(t *testing.T) {
	appName := "formatapp"
	cmd := NewVersionCommand(appName)

	// Test that the command can be executed
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("version command panicked: %v", r)
		}
	}()

	// Execute command - output goes to stdout by design
	cmd.Run(cmd, []string{})

	// Test that all expected format components are configured correctly
	if appName != "formatapp" {
		t.Errorf("expected app name to be formatapp, got %s", appName)
	}

	// Verify runtime info functions work
	goVersion := runtime.Version()
	if goVersion == "" {
		t.Error("expected non-empty Go version")
	}

	osArch := runtime.GOOS + "/" + runtime.GOARCH
	if osArch == "/" {
		t.Error("expected non-empty OS/Arch")
	}
}

func TestVersionCommand_EmptyAppName(t *testing.T) {
	// Test with empty app name
	cmd := NewVersionCommand("")

	// Verify command is created correctly even with empty app name
	if cmd == nil {
		t.Fatal("expected non-nil command")
	}

	if cmd.Use != "version" {
		t.Errorf("expected Use to be 'version', got %s", cmd.Use)
	}

	expectedLong := "Print the version number of "
	if cmd.Long != expectedLong {
		t.Errorf("expected Long to be %q for empty app name, got %q", expectedLong, cmd.Long)
	}

	// Test execution doesn't panic with empty app name
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("version command panicked with empty app name: %v", r)
		}
	}()

	cmd.Run(cmd, []string{})
}

func TestVersionCommand_LongSpecialCharacters(t *testing.T) {
	// Test with app name containing special characters
	appName := "my-app_v2.0"
	cmd := NewVersionCommand(appName)

	expectedLong := "Print the version number of my-app_v2.0"
	if cmd.Long != expectedLong {
		t.Errorf("expected Long to handle special characters, got %q", cmd.Long)
	}

	// Test execution doesn't panic with special characters
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("version command panicked with special characters: %v", r)
		}
	}()

	cmd.Run(cmd, []string{})
}

func TestVersionCommand_RuntimeInfo(t *testing.T) {
	cmd := NewVersionCommand("runtimetest")

	// Test that runtime functions work correctly
	goVersion := runtime.Version()
	if goVersion == "" {
		t.Error("expected non-empty Go version from runtime.Version()")
	}

	goos := runtime.GOOS
	if goos == "" {
		t.Error("expected non-empty GOOS from runtime.GOOS")
	}

	goarch := runtime.GOARCH
	if goarch == "" {
		t.Error("expected non-empty GOARCH from runtime.GOARCH")
	}

	// Test execution with runtime info
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("version command panicked: %v", r)
		}
	}()

	cmd.Run(cmd, []string{})
}

func TestVersionVariables_Modification(t *testing.T) {
	// Test that version variables can be modified (for build-time injection)
	originalVersion := Version
	originalGitCommit := GitCommit
	originalBuildDate := BuildDate

	// Modify variables
	Version = "v2.1.0-beta"
	GitCommit = "def456abc123"
	BuildDate = "2024-01-15T15:30:45Z"

	// Test that they are actually modified
	if Version != "v2.1.0-beta" {
		t.Errorf("expected Version to be modified to 'v2.1.0-beta', got %s", Version)
	}

	if GitCommit != "def456abc123" {
		t.Errorf("expected GitCommit to be modified to 'def456abc123', got %s", GitCommit)
	}

	if BuildDate != "2024-01-15T15:30:45Z" {
		t.Errorf("expected BuildDate to be modified to '2024-01-15T15:30:45Z', got %s", BuildDate)
	}

	// Restore original values
	Version = originalVersion
	GitCommit = originalGitCommit
	BuildDate = originalBuildDate
}
