package shared

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-kure/kure/pkg/cmd/shared/options"
	"github.com/spf13/viper"
)

func TestInitConfig_WithConfigFile(t *testing.T) {
	// Create a temporary config file
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "test.yaml")

	configContent := `
verbose: true
debug: false
output: json
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("failed to create test config file: %v", err)
	}

	// Reset viper
	viper.Reset()

	// Test with explicit config file
	globalOpts := &options.GlobalOptions{
		ConfigFile: configFile,
		Verbose:    false,
	}

	InitConfig("testapp", globalOpts)

	// Verify config was loaded
	if !viper.GetBool("verbose") {
		t.Error("expected verbose to be true from config file")
	}

	if viper.GetBool("debug") {
		t.Error("expected debug to be false from config file")
	}

	if viper.GetString("output") != "json" {
		t.Errorf("expected output to be 'json', got %s", viper.GetString("output"))
	}

	viper.Reset()
}

func TestInitConfig_WithHomeConfig(t *testing.T) {
	// Create a temporary home directory
	tempHome := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempHome)
	defer os.Setenv("HOME", originalHome)

	// Create config file in home directory
	configContent := `
verbose: false
debug: true
output: table
namespace: test-namespace
`

	configFile := filepath.Join(tempHome, ".testapp.yaml")
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("failed to create home config file: %v", err)
	}

	// Reset viper
	viper.Reset()

	// Test with home config discovery
	globalOpts := &options.GlobalOptions{
		ConfigFile: "", // No explicit config file
		Verbose:    false,
	}

	InitConfig("testapp", globalOpts)

	// Verify config was loaded
	if viper.GetBool("verbose") {
		t.Error("expected verbose to be false from home config")
	}

	if !viper.GetBool("debug") {
		t.Error("expected debug to be true from home config")
	}

	if viper.GetString("output") != "table" {
		t.Errorf("expected output to be 'table', got %s", viper.GetString("output"))
	}

	if viper.GetString("namespace") != "test-namespace" {
		t.Errorf("expected namespace to be 'test-namespace', got %s", viper.GetString("namespace"))
	}

	viper.Reset()
}

func TestInitConfig_WithCurrentDirConfig(t *testing.T) {
	// Create a temporary directory for current directory config
	tempDir := t.TempDir()
	originalWd, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(originalWd)

	// Create config file in current directory
	configContent := `
strict: true
dry-run: true
`

	configFile := ".testcurrent.yaml"
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("failed to create current dir config file: %v", err)
	}

	// Reset viper
	viper.Reset()

	// Test with current directory config discovery
	globalOpts := &options.GlobalOptions{
		ConfigFile: "", // No explicit config file
		Verbose:    false,
	}

	InitConfig("testcurrent", globalOpts)

	// Verify config was loaded
	if !viper.GetBool("strict") {
		t.Error("expected strict to be true from current dir config")
	}

	if !viper.GetBool("dry-run") {
		t.Error("expected dry-run to be true from current dir config")
	}

	viper.Reset()
}

func TestInitConfig_NoConfigFile(t *testing.T) {
	// Create a temporary home directory without config
	tempHome := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempHome)
	defer os.Setenv("HOME", originalHome)

	// Reset viper
	viper.Reset()

	// Test with no config file
	globalOpts := &options.GlobalOptions{
		ConfigFile: "",
		Verbose:    false,
	}

	// This should not fail even without config file
	InitConfig("nonexistent", globalOpts)

	// Should still work with default viper settings
	viper.Set("test", "value")
	if viper.GetString("test") != "value" {
		t.Error("viper should still be functional without config file")
	}

	viper.Reset()
}

func TestInitConfig_VerboseOutput(t *testing.T) {
	// Create a temporary config file
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "verbose-test.yaml")

	configContent := `verbose: true`
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("failed to create test config file: %v", err)
	}

	// Reset viper
	viper.Reset()

	// Capture stderr
	originalStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	// Test with verbose output
	globalOpts := &options.GlobalOptions{
		ConfigFile: configFile,
		Verbose:    true,
	}

	InitConfig("verbosetest", globalOpts)

	// Close writer and restore stderr
	w.Close()
	os.Stderr = originalStderr

	// Read captured output
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Verify verbose output was produced
	if !bytes.Contains(buf.Bytes(), []byte("Using config file:")) {
		t.Errorf("expected verbose output about config file, got: %s", output)
	}

	if !bytes.Contains(buf.Bytes(), []byte(configFile)) {
		t.Errorf("expected config file path in output, got: %s", output)
	}

	viper.Reset()
}

func TestInitConfig_EnvironmentVariables(t *testing.T) {
	// Reset viper
	viper.Reset()

	// Set environment variables with app prefix
	os.Setenv("TESTENV_VERBOSE", "true")
	os.Setenv("TESTENV_DEBUG", "false")
	os.Setenv("TESTENV_OUTPUT", "json")
	defer os.Unsetenv("TESTENV_VERBOSE")
	defer os.Unsetenv("TESTENV_DEBUG")
	defer os.Unsetenv("TESTENV_OUTPUT")

	// Test environment variable integration
	globalOpts := &options.GlobalOptions{
		ConfigFile: "",
		Verbose:    false,
	}

	InitConfig("testenv", globalOpts)

	// Verify environment variables are available through viper
	if !viper.GetBool("verbose") {
		t.Error("expected verbose to be true from TESTENV_VERBOSE")
	}

	if viper.GetBool("debug") {
		t.Error("expected debug to be false from TESTENV_DEBUG")
	}

	if viper.GetString("output") != "json" {
		t.Errorf("expected output to be 'json' from TESTENV_OUTPUT, got %s", viper.GetString("output"))
	}

	viper.Reset()
}

func TestInitConfig_HomeDirectoryError(t *testing.T) {
	// Create a backup of the original home env var
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)

	// Unset HOME to simulate UserHomeDir error
	os.Unsetenv("HOME")

	// Reset viper
	viper.Reset()

	globalOpts := &options.GlobalOptions{
		ConfigFile: "",
		Verbose:    false,
	}

	// This should not panic or fail, it should just return early
	InitConfig("testhomeerror", globalOpts)

	// Viper should still be functional
	viper.Set("test", "value")
	if viper.GetString("test") != "value" {
		t.Error("viper should still be functional after home dir error")
	}

	viper.Reset()
}

func TestInitConfig_InvalidConfigFile(t *testing.T) {
	// Reset viper
	viper.Reset()

	// Test with non-existent config file
	globalOpts := &options.GlobalOptions{
		ConfigFile: "/path/to/nonexistent/config.yaml",
		Verbose:    false,
	}

	// This should not panic, just fail to read config silently
	InitConfig("testinvalid", globalOpts)

	// Environment variables should still work
	os.Setenv("TESTINVALID_TEST", "value")
	defer os.Unsetenv("TESTINVALID_TEST")

	if viper.GetString("test") != "value" {
		t.Error("environment variables should still work with invalid config file")
	}

	viper.Reset()
}

func TestInitConfig_ConfigTypes(t *testing.T) {
	tests := []struct {
		name        string
		configFile  string
		content     string
		expectedKey string
		expectedVal interface{}
	}{
		{
			name:        "yaml config",
			configFile:  "test.yaml",
			content:     "verbose: true\noutput: yaml",
			expectedKey: "verbose",
			expectedVal: true,
		},
		{
			name:        "yml config",
			configFile:  "test.yml",
			content:     "debug: false\nnamespace: test",
			expectedKey: "namespace",
			expectedVal: "test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temporary config file
			tempDir := t.TempDir()
			configPath := filepath.Join(tempDir, tt.configFile)

			err := os.WriteFile(configPath, []byte(tt.content), 0644)
			if err != nil {
				t.Fatalf("failed to create test config file: %v", err)
			}

			// Reset viper
			viper.Reset()

			globalOpts := &options.GlobalOptions{
				ConfigFile: configPath,
				Verbose:    false,
			}

			InitConfig("testtype", globalOpts)

			// Verify config was loaded correctly
			if viper.Get(tt.expectedKey) != tt.expectedVal {
				t.Errorf("expected %s to be %v, got %v", tt.expectedKey, tt.expectedVal, viper.Get(tt.expectedKey))
			}

			viper.Reset()
		})
	}
}
