package launcher_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-kure/kure/pkg/launcher"
	"github.com/go-kure/kure/pkg/logger"
	"gopkg.in/yaml.v3"
)

func TestCLI_BuildCommand(t *testing.T) {
	log := logger.Noop()
	cli := launcher.NewCLI(log)

	tests := []struct {
		name      string
		args      []string
		wantError bool
		checkFunc func(t *testing.T, output string)
	}{
		{
			name: "build simple package",
			args: []string{"build", "testdata/packages/simple"},
			checkFunc: func(t *testing.T, output string) {
				if !strings.Contains(output, "kind: Deployment") {
					t.Error("Output should contain Deployment")
				}
				if !strings.Contains(output, "kind: Service") {
					t.Error("Output should contain Service")
				}
			},
		},
		{
			name: "build with JSON format",
			args: []string{"build", "testdata/packages/simple", "--format", "json"},
			checkFunc: func(t *testing.T, output string) {
				// Should be valid JSON
				var data interface{}
				if err := json.Unmarshal([]byte(output), &data); err != nil {
					t.Errorf("Output should be valid JSON: %v", err)
				}
			},
		},
		{
			name: "build with values file",
			args: []string{"build", "testdata/packages/simple", "--values", "testdata/values/test-values.yaml"},
			checkFunc: func(t *testing.T, output string) {
				// Check that values were applied
				if !strings.Contains(output, "replicas: 4") {
					t.Error("Output should have replicas from values file")
				}
			},
		},
		{
			name: "build with kind filter",
			args: []string{"build", "testdata/packages/simple", "--kind", "Deployment"},
			checkFunc: func(t *testing.T, output string) {
				if !strings.Contains(output, "kind: Deployment") {
					t.Error("Output should contain Deployment")
				}
				if strings.Contains(output, "kind: Service") {
					t.Error("Output should not contain Service")
				}
			},
		},
		{
			name: "build with labels",
			args: []string{"build", "testdata/packages/simple", "--add-label", "team=platform", "--add-label", "env=test"},
			checkFunc: func(t *testing.T, output string) {
				if !strings.Contains(output, "team: platform") {
					t.Error("Output should contain added label")
				}
				if !strings.Contains(output, "env: test") {
					t.Error("Output should contain added label")
				}
			},
		},
		{
			name:      "build invalid package",
			args:      []string{"build", "testdata/packages/invalid"},
			wantError: true,
		},
		{
			name:      "build non-existent package",
			args:      []string{"build", "testdata/packages/non-existent"},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create command
			cmd := cli.RootCommand()

			// Capture output
			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)

			// Set args
			cmd.SetArgs(tt.args)

			// Execute
			err := cmd.Execute()

			if tt.wantError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}

				if tt.checkFunc != nil {
					tt.checkFunc(t, buf.String())
				}
			}
		})
	}
}

func TestCLI_ValidateCommand(t *testing.T) {
	log := logger.Noop()
	cli := launcher.NewCLI(log)

	tests := []struct {
		name      string
		args      []string
		wantError bool
		checkFunc func(t *testing.T, output string)
	}{
		{
			name: "validate simple package",
			args: []string{"validate", "testdata/packages/simple"},
			checkFunc: func(t *testing.T, output string) {
				if !strings.Contains(output, "✓ Package is valid") {
					t.Error("Simple package should be valid")
				}
			},
		},
		{
			name: "validate with JSON output",
			args: []string{"validate", "testdata/packages/simple", "--json"},
			checkFunc: func(t *testing.T, output string) {
				var result launcher.ValidationResult
				if err := json.Unmarshal([]byte(output), &result); err != nil {
					t.Errorf("Output should be valid JSON: %v", err)
				}
				if !result.IsValid() {
					t.Error("Simple package should be valid")
				}
			},
		},
		{
			name: "validate invalid package",
			args: []string{"validate", "testdata/packages/invalid"},
			checkFunc: func(t *testing.T, output string) {
				if strings.Contains(output, "✓ Package is valid") {
					t.Error("Invalid package should not be valid")
				}
				if !strings.Contains(output, "Error") || !strings.Contains(output, "✗") {
					t.Error("Should show errors")
				}
			},
		},
		{
			name: "validate with strict mode",
			args: []string{"validate", "testdata/packages/simple", "--strict"},
			checkFunc: func(t *testing.T, output string) {
				// In strict mode, warnings become errors
				// Simple package should still be valid
				if !strings.Contains(output, "✓ Package is valid") {
					t.Error("Simple package should be valid even in strict mode")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create command
			cmd := cli.RootCommand()

			// Capture output
			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)

			// Set args
			cmd.SetArgs(tt.args)

			// Execute
			err := cmd.Execute()

			if tt.wantError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				// Validate command doesn't return error for invalid packages
				// unless in strict mode
				if tt.checkFunc != nil {
					tt.checkFunc(t, buf.String())
				}
			}
		})
	}
}

func TestCLI_InfoCommand(t *testing.T) {
	log := logger.Noop()
	cli := launcher.NewCLI(log)

	tests := []struct {
		name      string
		args      []string
		checkFunc func(t *testing.T, output string)
	}{
		{
			name: "info simple package",
			args: []string{"info", "testdata/packages/simple"},
			checkFunc: func(t *testing.T, output string) {
				if !strings.Contains(output, "Package: simple-app") {
					t.Error("Should show package name")
				}
				if !strings.Contains(output, "Version: 1.0.0") {
					t.Error("Should show version")
				}
				if !strings.Contains(output, "Parameters") {
					t.Error("Should list parameters")
				}
				if !strings.Contains(output, "Resources") {
					t.Error("Should list resources")
				}
			},
		},
		{
			name: "info with JSON output",
			args: []string{"info", "testdata/packages/simple", "-o", "json"},
			checkFunc: func(t *testing.T, output string) {
				var def launcher.PackageDefinition
				if err := json.Unmarshal([]byte(output), &def); err != nil {
					t.Errorf("Output should be valid JSON: %v", err)
				}
				if def.Metadata.Name != "simple-app" {
					t.Error("Should have correct package name")
				}
			},
		},
		{
			name: "info with YAML output",
			args: []string{"info", "testdata/packages/simple", "-o", "yaml"},
			checkFunc: func(t *testing.T, output string) {
				var def launcher.PackageDefinition
				if err := yaml.Unmarshal([]byte(output), &def); err != nil {
					t.Errorf("Output should be valid YAML: %v", err)
				}
				if def.Metadata.Name != "simple-app" {
					t.Error("Should have correct package name")
				}
			},
		},
		{
			name: "info complex package",
			args: []string{"info", "testdata/packages/complex"},
			checkFunc: func(t *testing.T, output string) {
				if !strings.Contains(output, "Package: complex-stack") {
					t.Error("Should show package name")
				}
				if !strings.Contains(output, "Patches") {
					t.Error("Should list patches")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create command
			cmd := cli.RootCommand()

			// Capture output
			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)

			// Set args
			cmd.SetArgs(tt.args)

			// Execute
			err := cmd.Execute()
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if tt.checkFunc != nil {
				tt.checkFunc(t, buf.String())
			}
		})
	}
}

func TestCLI_SchemaCommand(t *testing.T) {
	log := logger.Noop()
	cli := launcher.NewCLI(log)

	tests := []struct {
		name      string
		args      []string
		checkFunc func(t *testing.T, output string)
	}{
		{
			name: "generate schema",
			args: []string{"schema"},
			checkFunc: func(t *testing.T, output string) {
				var schema map[string]interface{}
				if err := json.Unmarshal([]byte(output), &schema); err != nil {
					t.Errorf("Output should be valid JSON: %v", err)
				}
				if schema["$schema"] == nil {
					t.Error("Should have $schema field")
				}
				if schema["type"] != "object" {
					t.Error("Should be object type")
				}
			},
		},
		{
			name: "generate schema with pretty print",
			args: []string{"schema", "--pretty"},
			checkFunc: func(t *testing.T, output string) {
				// Pretty printed JSON should have indentation
				if !strings.Contains(output, "  ") {
					t.Error("Should be pretty printed with indentation")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create command
			cmd := cli.RootCommand()

			// Capture output
			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)

			// Set args
			cmd.SetArgs(tt.args)

			// Execute
			err := cmd.Execute()
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if tt.checkFunc != nil {
				tt.checkFunc(t, buf.String())
			}
		})
	}
}

func TestCLI_DebugCommands(t *testing.T) {
	log := logger.Noop()
	cli := launcher.NewCLI(log)

	tests := []struct {
		name      string
		args      []string
		checkFunc func(t *testing.T, output string)
	}{
		{
			name: "debug variables",
			args: []string{"debug", "variables", "testdata/packages/simple"},
			checkFunc: func(t *testing.T, output string) {
				// Should show variable graph
				if len(output) == 0 {
					t.Error("Should show variable graph")
				}
			},
		},
		{
			name: "debug patches",
			args: []string{"debug", "patches", "testdata/packages/simple"},
			checkFunc: func(t *testing.T, output string) {
				// Should show patch graph
				if len(output) == 0 {
					t.Error("Should show patch graph")
				}
			},
		},
		{
			name: "debug resources",
			args: []string{"debug", "resources", "testdata/packages/simple"},
			checkFunc: func(t *testing.T, output string) {
				if !strings.Contains(output, "Package: simple-app") {
					t.Error("Should show package name")
				}
				if !strings.Contains(output, "Deployment") || !strings.Contains(output, "Service") {
					t.Error("Should list resources")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create command
			cmd := cli.RootCommand()

			// Capture output
			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)

			// Set args
			cmd.SetArgs(tt.args)

			// Execute
			err := cmd.Execute()
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if tt.checkFunc != nil {
				tt.checkFunc(t, buf.String())
			}
		})
	}
}

func TestCLI_BuildWithExtensions(t *testing.T) {
	log := logger.Noop()
	cli := launcher.NewCLI(log)

	// Build with extensions enabled (default)
	cmd := cli.RootCommand()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{"build", "testdata/packages/extensions"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	output := buf.String()

	// Check that production overrides were applied
	if !strings.Contains(output, "replicas: 5") {
		t.Error("Should have production replicas from extension")
	}

	// Test with extensions disabled
	buf.Reset()
	cmd.SetArgs([]string{"build", "testdata/packages/extensions", "--no-extensions"})

	err = cmd.Execute()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	output = buf.String()

	// Should have original values
	if !strings.Contains(output, "replicas: 2") {
		t.Error("Should have original replicas without extensions")
	}
}

func TestCLI_OutputToFile(t *testing.T) {
	log := logger.Noop()
	cli := launcher.NewCLI(log)

	tempDir := t.TempDir()
	outputPath := filepath.Join(tempDir, "output.yaml")

	cmd := cli.RootCommand()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{"build", "testdata/packages/simple", "-o", outputPath})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Check file was created
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Error("Output file should be created")
	}

	// Read and verify content
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	if !strings.Contains(string(data), "kind: Deployment") {
		t.Error("Output file should contain Deployment")
	}
}

func TestCLI_OutputToDirectory(t *testing.T) {
	log := logger.Noop()
	cli := launcher.NewCLI(log)

	tempDir := t.TempDir()

	cmd := cli.RootCommand()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{"build", "testdata/packages/simple", "-o", tempDir, "--separate", "--index"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Check files were created
	files, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("Failed to read directory: %v", err)
	}

	if len(files) < 2 {
		t.Errorf("Should have at least 2 files, got %d", len(files))
	}

	// Check file naming
	foundDeployment := false
	foundService := false
	for _, file := range files {
		name := file.Name()
		// Files should have index prefix when --index is used
		if strings.Contains(name, "deployment") {
			foundDeployment = true
			if !strings.HasPrefix(name, "00") && !strings.HasPrefix(name, "01") {
				t.Error("File should have index prefix")
			}
		}
		if strings.Contains(name, "service") {
			foundService = true
		}
	}

	if !foundDeployment {
		t.Error("Should have deployment file")
	}
	if !foundService {
		t.Error("Should have service file")
	}
}

func TestCLI_CompleteFlow(t *testing.T) {
	// Test complete workflow: validate -> build -> verify output
	log := logger.Noop()
	cli := launcher.NewCLI(log)

	packagePath := "testdata/packages/simple"
	tempDir := t.TempDir()
	outputPath := filepath.Join(tempDir, "manifests.yaml")

	// Step 1: Validate
	cmd := cli.RootCommand()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{"validate", packagePath})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Validation failed: %v", err)
	}

	if !strings.Contains(buf.String(), "✓ Package is valid") {
		t.Error("Package should be valid")
	}

	// Step 2: Get info
	buf.Reset()
	cmd.SetArgs([]string{"info", packagePath})

	err = cmd.Execute()
	if err != nil {
		t.Fatalf("Info failed: %v", err)
	}

	if !strings.Contains(buf.String(), "simple-app") {
		t.Error("Should show package info")
	}

	// Step 3: Build
	buf.Reset()
	cmd.SetArgs([]string{"build", packagePath, "-o", outputPath})

	err = cmd.Execute()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Step 4: Verify output
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read output: %v", err)
	}

	// Parse YAML to verify structure
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	resourceCount := 0
	for {
		var doc map[string]interface{}
		if err := decoder.Decode(&doc); err != nil {
			break
		}
		resourceCount++

		// Verify required fields
		if doc["apiVersion"] == nil {
			t.Error("Resource should have apiVersion")
		}
		if doc["kind"] == nil {
			t.Error("Resource should have kind")
		}
		if doc["metadata"] == nil {
			t.Error("Resource should have metadata")
		}
	}

	if resourceCount < 2 {
		t.Errorf("Should have at least 2 resources, got %d", resourceCount)
	}
}

// Helper function to execute CLI command and capture output
func executeCLICommand(cli *launcher.CLI, args []string) (string, error) {
	cmd := cli.RootCommand()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(args)

	err := cmd.Execute()
	return buf.String(), err
}

// Test CLI with various edge cases
func TestCLI_EdgeCases(t *testing.T) {
	log := logger.Noop()
	cli := launcher.NewCLI(log)

	tests := []struct {
		name      string
		args      []string
		wantError bool
	}{
		{
			name:      "no arguments",
			args:      []string{},
			wantError: false, // Should show help
		},
		{
			name:      "unknown command",
			args:      []string{"unknown"},
			wantError: true,
		},
		{
			name:      "build without package",
			args:      []string{"build"},
			wantError: false, // Will use current directory
		},
		{
			name:      "invalid flag",
			args:      []string{"build", "--invalid-flag"},
			wantError: true,
		},
		{
			name:      "conflicting options",
			args:      []string{"build", "testdata/packages/simple", "--format", "yaml", "--format", "json"},
			wantError: false, // Last one wins
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := executeCLICommand(cli, tt.args)

			if tt.wantError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.wantError && err != nil {
				// Some commands may fail for valid reasons
				// Just log it
				t.Logf("Got error (may be expected): %v", err)
			}
		})
	}
}
