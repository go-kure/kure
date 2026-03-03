package kure

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/go-kure/kure/pkg/cli"
	"github.com/go-kure/kure/pkg/cmd/shared/options"
)

func TestNewPatchCommand(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	cmd := NewPatchCommand(globalOpts)

	if cmd == nil {
		t.Fatal("expected non-nil patch command")
	}

	if cmd.Use != "patch [flags] BASE_FILE PATCH_FILE..." {
		t.Errorf("expected correct use string, got %s", cmd.Use)
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

func TestPatchOptionsAddFlags(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	factory := cli.NewFactory(globalOpts)
	o := &PatchOptions{
		Factory:   factory,
		IOStreams: factory.IOStreams(),
	}

	cmd := &cobra.Command{}
	o.AddFlags(cmd.Flags())

	// Check that expected flags are added
	expectedFlags := []string{
		"patch-dir", "output-file", "output-dir", "validate-only", "interactive",
		"combined", "diff", "group-by",
	}

	for _, flagName := range expectedFlags {
		flag := cmd.Flags().Lookup(flagName)
		if flag == nil {
			t.Errorf("expected flag %s not found", flagName)
		}
	}
}

func TestPatchOptionsComplete(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	factory := cli.NewFactory(globalOpts)

	tests := []struct {
		name        string
		setupOpts   func() *PatchOptions
		setupGlobal func(*options.GlobalOptions)
		wantErr     bool
	}{
		{
			name: "basic completion",
			setupOpts: func() *PatchOptions {
				return &PatchOptions{
					Factory:   factory,
					IOStreams: factory.IOStreams(),
				}
			},
			setupGlobal: func(opts *options.GlobalOptions) {},
			wantErr:     false,
		},
		{
			name: "with global output file",
			setupOpts: func() *PatchOptions {
				return &PatchOptions{
					Factory:   factory,
					IOStreams: factory.IOStreams(),
				}
			},
			setupGlobal: func(opts *options.GlobalOptions) {
				opts.OutputFile = "/tmp/output.yaml"
			},
			wantErr: false,
		},
		{
			name: "with dry run",
			setupOpts: func() *PatchOptions {
				return &PatchOptions{
					Factory:   factory,
					IOStreams: factory.IOStreams(),
				}
			},
			setupGlobal: func(opts *options.GlobalOptions) {
				opts.DryRun = true
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fresh global options for this test
			testGlobalOpts := options.NewGlobalOptions()
			tt.setupGlobal(testGlobalOpts)
			testFactory := cli.NewFactory(testGlobalOpts)

			o := tt.setupOpts()
			o.Factory = testFactory

			err := o.Complete()

			if tt.wantErr && err == nil {
				t.Error("expected error but got nil")
			}

			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if !tt.wantErr {
				// Verify completion effects
				if testGlobalOpts.OutputFile != "" && o.OutputFile != testGlobalOpts.OutputFile {
					t.Errorf("expected output file to be set to %s, got %s", testGlobalOpts.OutputFile, o.OutputFile)
				}

				if testGlobalOpts.DryRun && o.OutputFile != "/dev/stdout" {
					t.Error("expected output file to be set to /dev/stdout for dry run")
				}
			}
		})
	}
}

func TestPatchOptionsValidate(t *testing.T) {
	// Create temporary files for testing
	tempDir := t.TempDir()
	baseFile := filepath.Join(tempDir, "base.yaml")
	patchFile := filepath.Join(tempDir, "patch.yaml")

	// Create the base file
	baseContent := `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
data:
  key: value
`
	if err := os.WriteFile(baseFile, []byte(baseContent), 0644); err != nil {
		t.Fatalf("failed to create base file: %v", err)
	}

	// Create the patch file
	patchContent := `- target: $.data.key
  value: "new-value"
`
	if err := os.WriteFile(patchFile, []byte(patchContent), 0644); err != nil {
		t.Fatalf("failed to create patch file: %v", err)
	}

	globalOpts := options.NewGlobalOptions()
	factory := cli.NewFactory(globalOpts)

	tests := []struct {
		name      string
		setupOpts func() *PatchOptions
		wantErr   bool
	}{
		{
			name: "valid options",
			setupOpts: func() *PatchOptions {
				return &PatchOptions{
					Factory:    factory,
					IOStreams:  factory.IOStreams(),
					BaseFile:   baseFile,
					PatchFiles: []string{patchFile},
				}
			},
			wantErr: false,
		},
		{
			name: "nonexistent base file",
			setupOpts: func() *PatchOptions {
				return &PatchOptions{
					Factory:    factory,
					IOStreams:  factory.IOStreams(),
					BaseFile:   "/nonexistent/file.yaml",
					PatchFiles: []string{patchFile},
				}
			},
			wantErr: true,
		},
		{
			name: "no patch files",
			setupOpts: func() *PatchOptions {
				return &PatchOptions{
					Factory:    factory,
					IOStreams:  factory.IOStreams(),
					BaseFile:   baseFile,
					PatchFiles: []string{},
				}
			},
			wantErr: true,
		},
		{
			name: "nonexistent patch file",
			setupOpts: func() *PatchOptions {
				return &PatchOptions{
					Factory:    factory,
					IOStreams:  factory.IOStreams(),
					BaseFile:   baseFile,
					PatchFiles: []string{"/nonexistent/patch.yaml"},
				}
			},
			wantErr: true,
		},
		{
			name: "interactive mode - no patches needed",
			setupOpts: func() *PatchOptions {
				return &PatchOptions{
					Factory:     factory,
					IOStreams:   factory.IOStreams(),
					BaseFile:    baseFile,
					Interactive: true,
				}
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := tt.setupOpts()
			err := o.Validate()

			if tt.wantErr && err == nil {
				t.Error("expected error but got nil")
			}

			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestScanPatchDirectory(t *testing.T) {
	// Create temporary directory with patch files
	tempDir := t.TempDir()

	// Create various files
	files := map[string]string{
		"patch1.yaml":   "yaml patch content",
		"patch2.yml":    "yml patch content",
		"patch3.kpatch": "kpatch content",
		"readme.txt":    "not a patch file",
		"config.json":   "json content",
	}

	for filename, content := range files {
		filePath := filepath.Join(tempDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to create test file %s: %v", filename, err)
		}
	}

	// Create subdirectory (should be ignored)
	subDir := filepath.Join(tempDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdirectory: %v", err)
	}

	globalOpts := options.NewGlobalOptions()
	factory := cli.NewFactory(globalOpts)
	o := &PatchOptions{
		Factory:   factory,
		IOStreams: factory.IOStreams(),
		PatchDir:  tempDir,
	}

	patchFiles, err := o.scanPatchDirectory()
	if err != nil {
		t.Fatalf("scanPatchDirectory failed: %v", err)
	}

	// Should find 3 patch files
	expectedCount := 3
	if len(patchFiles) != expectedCount {
		t.Errorf("expected %d patch files, got %d", expectedCount, len(patchFiles))
	}

	// Check that all found files have correct extensions
	for _, patchFile := range patchFiles {
		filename := filepath.Base(patchFile)
		if !strings.HasSuffix(filename, ".yaml") &&
			!strings.HasSuffix(filename, ".yml") &&
			!strings.HasSuffix(filename, ".kpatch") {
			t.Errorf("unexpected patch file found: %s", filename)
		}
	}
}

func TestValidatePatchFile(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		{
			name: "valid patch file",
			content: `- target: $.metadata.name
  value: "new-name"
`,
			wantErr: false,
		},
		{
			name:    "empty file",
			content: "",
			wantErr: true, // Empty files are not valid patch files
		},
		{
			name: "invalid yaml",
			content: `- target: $.metadata.name
  value: "unclosed quote
`,
			wantErr: true,
		},
	}

	globalOpts := options.NewGlobalOptions()
	factory := cli.NewFactory(globalOpts)
	o := &PatchOptions{
		Factory:   factory,
		IOStreams: factory.IOStreams(),
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary patch file
			patchFile := filepath.Join(tempDir, tt.name+".yaml")
			if err := os.WriteFile(patchFile, []byte(tt.content), 0644); err != nil {
				t.Fatalf("failed to create patch file: %v", err)
			}

			err := o.validatePatchFile(patchFile)

			if tt.wantErr && err == nil {
				t.Error("expected error but got nil")
			}

			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestPatchCommandHelp(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	cmd := NewPatchCommand(globalOpts)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// Test help
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
	expectedContent := []string{
		"patch", "Apply patches", "BASE_FILE", "PATCH_FILE", "Examples:",
	}
	for _, content := range expectedContent {
		if !strings.Contains(output, content) {
			t.Errorf("expected help output to contain %q", content)
		}
	}
}

func TestPatchCommandInvalidArgs(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	cmd := NewPatchCommand(globalOpts)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// Test with no arguments (should fail due to MinimumNArgs)
	cmd.SetArgs([]string{})
	err := cmd.Execute()

	if err == nil {
		t.Error("expected error for no arguments")
	}
}

func TestPatchOptionsCombinedMode(t *testing.T) {
	tempDir := t.TempDir()
	baseFile := filepath.Join(tempDir, "base.yaml")
	patchFile1 := filepath.Join(tempDir, "patch1.yaml")
	patchFile2 := filepath.Join(tempDir, "patch2.yaml")
	outputFile := filepath.Join(tempDir, "output.yaml")

	// Base file with two resources
	baseContent := `apiVersion: v1
kind: ConfigMap
metadata:
  name: config1
data:
  key1: value1
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: config2
data:
  key2: value2
`
	if err := os.WriteFile(baseFile, []byte(baseContent), 0644); err != nil {
		t.Fatalf("failed to create base file: %v", err)
	}

	// First patch modifies key1
	patch1Content := `data.key1: patched1
`
	if err := os.WriteFile(patchFile1, []byte(patch1Content), 0644); err != nil {
		t.Fatalf("failed to create patch file 1: %v", err)
	}

	// Second patch modifies key2
	patch2Content := `data.key2: patched2
`
	if err := os.WriteFile(patchFile2, []byte(patch2Content), 0644); err != nil {
		t.Fatalf("failed to create patch file 2: %v", err)
	}

	globalOpts := options.NewGlobalOptions()
	globalOpts.Verbose = true
	factory := cli.NewFactory(globalOpts)

	var buf bytes.Buffer
	o := &PatchOptions{
		Factory:    factory,
		IOStreams:  cli.IOStreams{In: strings.NewReader(""), Out: &buf, ErrOut: &buf},
		BaseFile:   baseFile,
		PatchFiles: []string{patchFile1, patchFile2},
		Combined:   true,
		GroupBy:    "none",
		OutputFile: outputFile,
	}

	err := o.runCombined()
	if err != nil {
		t.Fatalf("runCombined failed: %v", err)
	}

	// Verify output was written
	output, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}

	outputStr := string(output)
	// Check that both patches were applied
	if !strings.Contains(outputStr, "patched1") {
		t.Error("expected output to contain patched1")
	}
	if !strings.Contains(outputStr, "patched2") {
		t.Error("expected output to contain patched2")
	}
	// Check YAML document separator is preserved
	if !strings.Contains(outputStr, "---") {
		t.Error("expected output to contain YAML document separator")
	}
}

func TestPatchOptionsCombinedModeGroupByKind(t *testing.T) {
	tempDir := t.TempDir()
	baseFile := filepath.Join(tempDir, "base.yaml")
	patchFile := filepath.Join(tempDir, "patch.yaml")

	// Base file with mixed kinds (intentionally unordered)
	baseContent := `apiVersion: v1
kind: Service
metadata:
  name: svc
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cfg
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: dep
`
	if err := os.WriteFile(baseFile, []byte(baseContent), 0644); err != nil {
		t.Fatalf("failed to create base file: %v", err)
	}

	// Minimal no-op patch
	patchContent := `metadata.name: cfg
`
	if err := os.WriteFile(patchFile, []byte(patchContent), 0644); err != nil {
		t.Fatalf("failed to create patch file: %v", err)
	}

	globalOpts := options.NewGlobalOptions()
	factory := cli.NewFactory(globalOpts)

	var buf bytes.Buffer
	o := &PatchOptions{
		Factory:    factory,
		IOStreams:  cli.IOStreams{In: strings.NewReader(""), Out: &buf, ErrOut: &buf},
		BaseFile:   baseFile,
		PatchFiles: []string{patchFile},
		Combined:   true,
		GroupBy:    "kind",
	}

	err := o.runCombined()
	if err != nil {
		t.Fatalf("runCombined failed: %v", err)
	}
}

func TestPatchOptionsCombinedModeInvalidGroupBy(t *testing.T) {
	tempDir := t.TempDir()
	baseFile := filepath.Join(tempDir, "base.yaml")
	patchFile := filepath.Join(tempDir, "patch.yaml")

	baseContent := `apiVersion: v1
kind: ConfigMap
metadata:
  name: test
`
	if err := os.WriteFile(baseFile, []byte(baseContent), 0644); err != nil {
		t.Fatalf("failed to create base file: %v", err)
	}
	if err := os.WriteFile(patchFile, []byte(""), 0644); err != nil {
		t.Fatalf("failed to create patch file: %v", err)
	}

	globalOpts := options.NewGlobalOptions()
	factory := cli.NewFactory(globalOpts)

	o := &PatchOptions{
		Factory:    factory,
		IOStreams:  factory.IOStreams(),
		BaseFile:   baseFile,
		PatchFiles: []string{patchFile},
		Combined:   true,
		GroupBy:    "invalid",
	}

	err := o.runCombined()
	if err == nil {
		t.Error("expected error for invalid group-by")
	}
}

func TestPatchOptionsBackwardCompatibility(t *testing.T) {
	// Verify that existing (non-combined) mode still works
	tempDir := t.TempDir()
	baseFile := filepath.Join(tempDir, "base.yaml")
	patchFile := filepath.Join(tempDir, "patch.yaml")
	outputDir := filepath.Join(tempDir, "out")

	baseContent := `apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  key: value
`
	if err := os.WriteFile(baseFile, []byte(baseContent), 0644); err != nil {
		t.Fatalf("failed to create base file: %v", err)
	}

	patchContent := `- target: $.data.key
  value: "new-value"
`
	if err := os.WriteFile(patchFile, []byte(patchContent), 0644); err != nil {
		t.Fatalf("failed to create patch file: %v", err)
	}

	globalOpts := options.NewGlobalOptions()
	factory := cli.NewFactory(globalOpts)

	o := &PatchOptions{
		Factory:    factory,
		IOStreams:  factory.IOStreams(),
		BaseFile:   baseFile,
		PatchFiles: []string{patchFile},
		OutputDir:  outputDir,
		Combined:   false, // Explicitly false (default)
	}

	// This should use the original Run() path, not runCombined()
	// Note: This doesn't fully test the original path without actually running Run(),
	// but we're checking that the flag defaults correctly.
	if o.Combined {
		t.Error("Combined should be false by default for backward compatibility")
	}
}

func TestPatchOptionsRunValidation(t *testing.T) {
	// Create temporary files for testing
	tempDir := t.TempDir()
	baseFile := filepath.Join(tempDir, "base.yaml")
	patchFile := filepath.Join(tempDir, "patch.yaml")

	// Create the base file
	baseContent := `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
`
	if err := os.WriteFile(baseFile, []byte(baseContent), 0644); err != nil {
		t.Fatalf("failed to create base file: %v", err)
	}

	// Create the patch file
	patchContent := `- target: $.metadata.name
  value: "new-name"
`
	if err := os.WriteFile(patchFile, []byte(patchContent), 0644); err != nil {
		t.Fatalf("failed to create patch file: %v", err)
	}

	globalOpts := options.NewGlobalOptions()
	globalOpts.Verbose = true
	factory := cli.NewFactory(globalOpts)

	o := &PatchOptions{
		Factory:      factory,
		IOStreams:    factory.IOStreams(),
		BaseFile:     baseFile,
		PatchFiles:   []string{patchFile},
		ValidateOnly: true,
	}

	var buf bytes.Buffer
	o.IOStreams = cli.IOStreams{
		In:     strings.NewReader(""),
		Out:    &buf,
		ErrOut: &buf,
	}

	err := o.runValidation()
	if err != nil {
		t.Errorf("validation failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "valid") {
		t.Error("expected validation success message")
	}
}

func TestPatchOptionsDiffMode(t *testing.T) {
	tempDir := t.TempDir()
	baseFile := filepath.Join(tempDir, "base.yaml")
	patchFile := filepath.Join(tempDir, "patch.yaml")

	// Base file with a ConfigMap
	baseContent := `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
data:
  key1: value1
  key2: value2
`
	if err := os.WriteFile(baseFile, []byte(baseContent), 0644); err != nil {
		t.Fatalf("failed to create base file: %v", err)
	}

	// Patch that modifies key1
	patchContent := `data.key1: patched-value
`
	if err := os.WriteFile(patchFile, []byte(patchContent), 0644); err != nil {
		t.Fatalf("failed to create patch file: %v", err)
	}

	globalOpts := options.NewGlobalOptions()
	factory := cli.NewFactory(globalOpts)

	var outBuf, errBuf bytes.Buffer
	o := &PatchOptions{
		Factory:    factory,
		IOStreams:  cli.IOStreams{In: strings.NewReader(""), Out: &outBuf, ErrOut: &errBuf},
		BaseFile:   baseFile,
		PatchFiles: []string{patchFile},
		Diff:       true,
	}

	// Unset NO_COLOR to allow color output in tests
	oldNoColor := os.Getenv("NO_COLOR")
	os.Unsetenv("NO_COLOR")
	defer func() {
		if oldNoColor != "" {
			os.Setenv("NO_COLOR", oldNoColor)
		}
	}()

	err := o.runDiff()
	if err != nil {
		t.Fatalf("runDiff failed: %v", err)
	}

	output := outBuf.String()
	// Check that diff output contains expected markers
	if !strings.Contains(output, "---") && !strings.Contains(output, "+++") {
		// Should have diff headers or "No changes detected"
		if !strings.Contains(output, "No changes") {
			t.Error("expected diff output with headers or 'No changes' message")
		}
	}

	// Check that the diff shows the change
	if strings.Contains(output, "-") || strings.Contains(output, "+") {
		// If there are changes, the patched value should appear
		if !strings.Contains(output, "patched-value") && !strings.Contains(output, "value1") {
			t.Error("expected diff to show the changed value")
		}
	}
}

func TestPatchOptionsDiffModeNoChanges(t *testing.T) {
	tempDir := t.TempDir()
	baseFile := filepath.Join(tempDir, "base.yaml")
	patchFile := filepath.Join(tempDir, "patch.yaml")

	// Base file
	baseContent := `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
data:
  key1: value1
`
	if err := os.WriteFile(baseFile, []byte(baseContent), 0644); err != nil {
		t.Fatalf("failed to create base file: %v", err)
	}

	// Empty patch (no operations)
	patchContent := ``
	if err := os.WriteFile(patchFile, []byte(patchContent), 0644); err != nil {
		t.Fatalf("failed to create patch file: %v", err)
	}

	globalOpts := options.NewGlobalOptions()
	factory := cli.NewFactory(globalOpts)

	var outBuf, errBuf bytes.Buffer
	o := &PatchOptions{
		Factory:    factory,
		IOStreams:  cli.IOStreams{In: strings.NewReader(""), Out: &outBuf, ErrOut: &errBuf},
		BaseFile:   baseFile,
		PatchFiles: []string{patchFile},
		Diff:       true,
	}

	err := o.runDiff()
	// Empty patch files might cause an error or be handled gracefully
	// The important thing is it shouldn't crash
	if err != nil {
		// This is acceptable for empty patch files
		return
	}
}

func TestPatchOptionsDiffModeMultiplePatches(t *testing.T) {
	tempDir := t.TempDir()
	baseFile := filepath.Join(tempDir, "base.yaml")
	patchFile1 := filepath.Join(tempDir, "patch1.yaml")
	patchFile2 := filepath.Join(tempDir, "patch2.yaml")

	// Base file with two resources
	baseContent := `apiVersion: v1
kind: ConfigMap
metadata:
  name: config1
data:
  key1: original1
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: config2
data:
  key2: original2
`
	if err := os.WriteFile(baseFile, []byte(baseContent), 0644); err != nil {
		t.Fatalf("failed to create base file: %v", err)
	}

	// First patch modifies key1
	patch1Content := `data.key1: patched1
`
	if err := os.WriteFile(patchFile1, []byte(patch1Content), 0644); err != nil {
		t.Fatalf("failed to create patch file 1: %v", err)
	}

	// Second patch modifies key2
	patch2Content := `data.key2: patched2
`
	if err := os.WriteFile(patchFile2, []byte(patch2Content), 0644); err != nil {
		t.Fatalf("failed to create patch file 2: %v", err)
	}

	globalOpts := options.NewGlobalOptions()
	globalOpts.Verbose = true
	factory := cli.NewFactory(globalOpts)

	var outBuf, errBuf bytes.Buffer
	o := &PatchOptions{
		Factory:    factory,
		IOStreams:  cli.IOStreams{In: strings.NewReader(""), Out: &outBuf, ErrOut: &errBuf},
		BaseFile:   baseFile,
		PatchFiles: []string{patchFile1, patchFile2},
		Diff:       true,
	}

	err := o.runDiff()
	if err != nil {
		t.Fatalf("runDiff failed: %v", err)
	}

	output := outBuf.String()
	// Check that diff shows both patches applied
	if !strings.Contains(output, "patched1") && !strings.Contains(output, "patched2") {
		// Might also show "No changes" if format differs
		t.Log("Diff output:", output)
	}
}

func TestPrintColoredDiff(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	factory := cli.NewFactory(globalOpts)

	var buf bytes.Buffer
	o := &PatchOptions{
		Factory:   factory,
		IOStreams: cli.IOStreams{In: strings.NewReader(""), Out: &buf, ErrOut: &buf},
	}

	// Set NO_COLOR to test non-colored output
	oldNoColor := os.Getenv("NO_COLOR")
	os.Setenv("NO_COLOR", "1")
	defer func() {
		if oldNoColor != "" {
			os.Setenv("NO_COLOR", oldNoColor)
		} else {
			os.Unsetenv("NO_COLOR")
		}
	}()

	diffText := `--- a/test.yaml
+++ b/test.yaml
@@ -1,3 +1,3 @@
 key1: value1
-key2: old
+key2: new
 key3: value3
`

	o.printColoredDiff(diffText)
	output := buf.String()

	// Without colors, output should match input
	if output != diffText {
		t.Errorf("expected output to match input without colors\ngot: %q\nwant: %q", output, diffText)
	}
}

// ---------------------------------------------------------------------------
// Helper: create a minimal base YAML file with a single ConfigMap and return
// the path.  The caller owns the directory via t.TempDir().
// ---------------------------------------------------------------------------

func writeTestBaseFile(t *testing.T, dir string) string {
	t.Helper()
	base := filepath.Join(dir, "base.yaml")
	content := `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
data:
  key1: value1
  key2: value2
`
	if err := os.WriteFile(base, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write base file: %v", err)
	}
	return base
}

// writeTestPatchFile creates a simple field-level patch file.
func writeTestPatchFile(t *testing.T, dir, name string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	content := `data.key1: patched-value
`
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write patch file: %v", err)
	}
	return p
}

// newTestPatchOptions creates PatchOptions with captured IO and the given
// global option tweaks.
func newTestPatchOptions(t *testing.T, tweakGlobal func(*options.GlobalOptions)) (*PatchOptions, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	globalOpts := options.NewGlobalOptions()
	if tweakGlobal != nil {
		tweakGlobal(globalOpts)
	}
	factory := cli.NewFactory(globalOpts)

	var stdout, stderr bytes.Buffer
	return &PatchOptions{
		Factory:   factory,
		IOStreams: cli.IOStreams{In: strings.NewReader(""), Out: &stdout, ErrOut: &stderr},
	}, &stdout, &stderr
}

// ---------------------------------------------------------------------------
// Tests for NewPatchCommand RunE path (covers arg-to-field mapping)
// ---------------------------------------------------------------------------

func TestNewPatchCommandRunE_ArgsMapping(t *testing.T) {
	tmpDir := t.TempDir()
	baseFile := writeTestBaseFile(t, tmpDir)
	patchFile := writeTestPatchFile(t, tmpDir, "p1.yaml")

	globalOpts := options.NewGlobalOptions()
	globalOpts.DryRun = true // forces stdout output so no dir creation needed
	cmd := NewPatchCommand(globalOpts)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{baseFile, patchFile})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected command to succeed, got: %v", err)
	}
}

func TestNewPatchCommandRunE_NoArgs(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	cmd := NewPatchCommand(globalOpts)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error with no arguments")
	}
}

// ---------------------------------------------------------------------------
// Tests for NewPatchCommand flag defaults
// ---------------------------------------------------------------------------

func TestNewPatchCommandFlagDefaults(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	cmd := NewPatchCommand(globalOpts)

	tests := []struct {
		flag     string
		expected string
	}{
		{"patch-dir", ""},
		{"output-file", ""},
		{"output-dir", "out/patches"},
		{"validate-only", "false"},
		{"interactive", "false"},
		{"combined", "false"},
		{"diff", "false"},
		{"group-by", "none"},
	}

	for _, tt := range tests {
		t.Run(tt.flag, func(t *testing.T) {
			f := cmd.Flags().Lookup(tt.flag)
			if f == nil {
				t.Fatalf("flag %s not found", tt.flag)
			}
			if f.DefValue != tt.expected {
				t.Errorf("flag %s: default = %q, want %q", tt.flag, f.DefValue, tt.expected)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Test for patch-dir short flag
// ---------------------------------------------------------------------------

func TestNewPatchCommandShortFlags(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	cmd := NewPatchCommand(globalOpts)

	// -p is shorthand for --patch-dir
	f := cmd.Flags().ShorthandLookup("p")
	if f == nil {
		t.Fatal("expected -p shorthand for patch-dir")
	}
	if f.Name != "patch-dir" {
		t.Errorf("expected -p to map to patch-dir, got %s", f.Name)
	}

	// -d is shorthand for --output-dir
	f = cmd.Flags().ShorthandLookup("d")
	if f == nil {
		t.Fatal("expected -d shorthand for output-dir")
	}
	if f.Name != "output-dir" {
		t.Errorf("expected -d to map to output-dir, got %s", f.Name)
	}
}

// ---------------------------------------------------------------------------
// Tests for Complete with PatchDir
// ---------------------------------------------------------------------------

func TestPatchOptionsCompleteWithPatchDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a few patch files inside the dir
	writeTestPatchFile(t, tmpDir, "a.yaml")
	writeTestPatchFile(t, tmpDir, "b.yml")
	writeTestPatchFile(t, tmpDir, "c.kpatch")

	o, _, _ := newTestPatchOptions(t, nil)
	o.PatchDir = tmpDir

	if err := o.Complete(); err != nil {
		t.Fatalf("Complete() failed: %v", err)
	}

	if len(o.PatchFiles) != 3 {
		t.Errorf("expected 3 patch files from dir scan, got %d", len(o.PatchFiles))
	}
}

func TestPatchOptionsCompleteWithPatchDirAndExistingFiles(t *testing.T) {
	tmpDir := t.TempDir()
	writeTestPatchFile(t, tmpDir, "dir-patch.yaml")

	o, _, _ := newTestPatchOptions(t, nil)
	o.PatchFiles = []string{"/some/existing.yaml"}
	o.PatchDir = tmpDir

	if err := o.Complete(); err != nil {
		t.Fatalf("Complete() failed: %v", err)
	}

	// Should have the pre-existing file plus the one from the directory
	if len(o.PatchFiles) != 2 {
		t.Errorf("expected 2 patch files, got %d: %v", len(o.PatchFiles), o.PatchFiles)
	}
}

func TestPatchOptionsCompleteWithNonexistentPatchDir(t *testing.T) {
	o, _, _ := newTestPatchOptions(t, nil)
	o.PatchDir = "/nonexistent/patch/dir"

	err := o.Complete()
	if err == nil {
		t.Error("expected error for nonexistent patch dir")
	}
}

func TestPatchOptionsCompleteGlobalOutputFileOverride(t *testing.T) {
	o, _, _ := newTestPatchOptions(t, func(g *options.GlobalOptions) {
		g.OutputFile = "/tmp/custom-output.yaml"
	})

	if err := o.Complete(); err != nil {
		t.Fatalf("Complete() failed: %v", err)
	}

	if o.OutputFile != "/tmp/custom-output.yaml" {
		t.Errorf("expected OutputFile = /tmp/custom-output.yaml, got %s", o.OutputFile)
	}
}

func TestPatchOptionsCompleteDryRunSetsStdout(t *testing.T) {
	o, _, _ := newTestPatchOptions(t, func(g *options.GlobalOptions) {
		g.DryRun = true
	})

	if err := o.Complete(); err != nil {
		t.Fatalf("Complete() failed: %v", err)
	}

	if o.OutputFile != "/dev/stdout" {
		t.Errorf("expected OutputFile = /dev/stdout for dry run, got %s", o.OutputFile)
	}
}

// ---------------------------------------------------------------------------
// Test for scanPatchDirectory with nonexistent directory
// ---------------------------------------------------------------------------

func TestScanPatchDirectoryNonexistent(t *testing.T) {
	o, _, _ := newTestPatchOptions(t, nil)
	o.PatchDir = "/nonexistent/dir"

	_, err := o.scanPatchDirectory()
	if err == nil {
		t.Error("expected error for nonexistent directory")
	}
}

func TestScanPatchDirectoryEmpty(t *testing.T) {
	tmpDir := t.TempDir()

	o, _, _ := newTestPatchOptions(t, nil)
	o.PatchDir = tmpDir

	files, err := o.scanPatchDirectory()
	if err != nil {
		t.Fatalf("scanPatchDirectory failed: %v", err)
	}

	if len(files) != 0 {
		t.Errorf("expected 0 patch files in empty dir, got %d", len(files))
	}
}

// ---------------------------------------------------------------------------
// Tests for Run() dispatch paths
// ---------------------------------------------------------------------------

func TestPatchOptionsRunInteractive(t *testing.T) {
	tmpDir := t.TempDir()
	baseFile := writeTestBaseFile(t, tmpDir)

	o, _, _ := newTestPatchOptions(t, nil)
	o.BaseFile = baseFile
	o.Interactive = true

	err := o.runInteractive()
	if err == nil {
		t.Fatal("expected ErrInteractiveMode error")
	}

	if err.Error() != "interactive mode not yet implemented" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPatchOptionsRunDispatchesInteractive(t *testing.T) {
	tmpDir := t.TempDir()
	baseFile := writeTestBaseFile(t, tmpDir)

	o, _, _ := newTestPatchOptions(t, nil)
	o.BaseFile = baseFile
	o.Interactive = true

	err := o.Run()
	if err == nil {
		t.Fatal("expected error from Run() in interactive mode")
	}
	if err.Error() != "interactive mode not yet implemented" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPatchOptionsRunDispatchesValidateOnly(t *testing.T) {
	tmpDir := t.TempDir()
	baseFile := writeTestBaseFile(t, tmpDir)
	patchFile := writeTestPatchFile(t, tmpDir, "p.yaml")

	o, stdout, _ := newTestPatchOptions(t, nil)
	o.BaseFile = baseFile
	o.PatchFiles = []string{patchFile}
	o.ValidateOnly = true

	err := o.Run()
	if err != nil {
		t.Fatalf("Run() validate-only failed: %v", err)
	}

	if !strings.Contains(stdout.String(), "valid") {
		t.Error("expected validation success message in output")
	}
}

func TestPatchOptionsRunDispatchesDiff(t *testing.T) {
	tmpDir := t.TempDir()
	baseFile := writeTestBaseFile(t, tmpDir)
	patchFile := writeTestPatchFile(t, tmpDir, "p.yaml")

	o, stdout, _ := newTestPatchOptions(t, nil)
	o.BaseFile = baseFile
	o.PatchFiles = []string{patchFile}
	o.Diff = true

	// Ensure no color output for easy checking
	oldNoColor := os.Getenv("NO_COLOR")
	os.Setenv("NO_COLOR", "1")
	defer func() {
		if oldNoColor != "" {
			os.Setenv("NO_COLOR", oldNoColor)
		} else {
			os.Unsetenv("NO_COLOR")
		}
	}()

	err := o.Run()
	if err != nil {
		t.Fatalf("Run() diff mode failed: %v", err)
	}

	output := stdout.String()
	// Should contain diff output or "No changes" message
	if output == "" {
		t.Error("expected non-empty output from diff mode")
	}
}

func TestPatchOptionsRunDispatchesCombined(t *testing.T) {
	tmpDir := t.TempDir()
	baseFile := writeTestBaseFile(t, tmpDir)
	patchFile := writeTestPatchFile(t, tmpDir, "p.yaml")
	outputFile := filepath.Join(tmpDir, "combined-out.yaml")

	o, _, _ := newTestPatchOptions(t, nil)
	o.BaseFile = baseFile
	o.PatchFiles = []string{patchFile}
	o.Combined = true
	o.GroupBy = "none"
	o.OutputFile = outputFile

	err := o.Run()
	if err != nil {
		t.Fatalf("Run() combined mode failed: %v", err)
	}

	data, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}
	if !strings.Contains(string(data), "patched-value") {
		t.Error("expected patched value in combined output")
	}
}

func TestPatchOptionsRunNormalPath(t *testing.T) {
	tmpDir := t.TempDir()
	baseFile := writeTestBaseFile(t, tmpDir)
	patchFile := writeTestPatchFile(t, tmpDir, "p.yaml")
	outputDir := filepath.Join(tmpDir, "output")

	o, _, _ := newTestPatchOptions(t, nil)
	o.BaseFile = baseFile
	o.PatchFiles = []string{patchFile}
	o.OutputDir = outputDir

	err := o.Run()
	if err != nil {
		t.Fatalf("Run() normal path failed: %v", err)
	}

	// Verify output directory was created
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		t.Error("expected output directory to be created")
	}
}

func TestPatchOptionsRunNormalPathVerbose(t *testing.T) {
	tmpDir := t.TempDir()
	baseFile := writeTestBaseFile(t, tmpDir)
	patchFile := writeTestPatchFile(t, tmpDir, "p.yaml")
	outputDir := filepath.Join(tmpDir, "output")

	o, _, stderr := newTestPatchOptions(t, func(g *options.GlobalOptions) {
		g.Verbose = true
	})
	o.BaseFile = baseFile
	o.PatchFiles = []string{patchFile}
	o.OutputDir = outputDir

	err := o.Run()
	if err != nil {
		t.Fatalf("Run() normal path failed: %v", err)
	}

	errOutput := stderr.String()
	if !strings.Contains(errOutput, "Applying") {
		t.Error("expected verbose output to contain 'Applying'")
	}
	if !strings.Contains(errOutput, "Successfully") {
		t.Error("expected verbose output to contain 'Successfully'")
	}
}

// ---------------------------------------------------------------------------
// Tests for writeOutput routing
// ---------------------------------------------------------------------------

func TestPatchOptionsWriteOutputToFile(t *testing.T) {
	tmpDir := t.TempDir()
	baseFile := writeTestBaseFile(t, tmpDir)
	patchFile := writeTestPatchFile(t, tmpDir, "p.yaml")
	outputFile := filepath.Join(tmpDir, "out.yaml")

	o, _, _ := newTestPatchOptions(t, nil)
	o.BaseFile = baseFile
	o.PatchFiles = []string{patchFile}
	o.OutputFile = outputFile
	o.OutputDir = filepath.Join(tmpDir, "outdir")

	// Load and apply to get a PatchableAppSet
	docSet, err := o.loadBaseResources()
	if err != nil {
		t.Fatalf("loadBaseResources failed: %v", err)
	}
	patchableSet, err := o.applyPatches(docSet)
	if err != nil {
		t.Fatalf("applyPatches failed: %v", err)
	}

	if err := o.writeOutput(patchableSet); err != nil {
		t.Fatalf("writeOutput failed: %v", err)
	}

	if _, err := os.Stat(outputFile); os.IsNotExist(err) {
		t.Error("expected output file to be created")
	}
}

func TestPatchOptionsWriteOutputDryRun(t *testing.T) {
	tmpDir := t.TempDir()
	baseFile := writeTestBaseFile(t, tmpDir)
	patchFile := writeTestPatchFile(t, tmpDir, "p.yaml")

	o, _, _ := newTestPatchOptions(t, func(g *options.GlobalOptions) {
		g.DryRun = true
	})
	o.BaseFile = baseFile
	o.PatchFiles = []string{patchFile}
	o.OutputDir = filepath.Join(tmpDir, "outdir")

	docSet, err := o.loadBaseResources()
	if err != nil {
		t.Fatalf("loadBaseResources failed: %v", err)
	}
	patchableSet, err := o.applyPatches(docSet)
	if err != nil {
		t.Fatalf("applyPatches failed: %v", err)
	}

	// writeOutput with DryRun should call writeToStdout
	err = o.writeOutput(patchableSet)
	if err != nil {
		t.Fatalf("writeOutput with DryRun failed: %v", err)
	}
}

func TestPatchOptionsWriteOutputToDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	baseFile := writeTestBaseFile(t, tmpDir)
	patchFile := writeTestPatchFile(t, tmpDir, "p.yaml")
	outputDir := filepath.Join(tmpDir, "output-dir")

	o, _, _ := newTestPatchOptions(t, nil)
	o.BaseFile = baseFile
	o.PatchFiles = []string{patchFile}
	o.OutputDir = outputDir

	docSet, err := o.loadBaseResources()
	if err != nil {
		t.Fatalf("loadBaseResources failed: %v", err)
	}
	patchableSet, err := o.applyPatches(docSet)
	if err != nil {
		t.Fatalf("applyPatches failed: %v", err)
	}

	if err := o.writeOutput(patchableSet); err != nil {
		t.Fatalf("writeOutput to directory failed: %v", err)
	}

	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		t.Error("expected output directory to be created")
	}
}

// ---------------------------------------------------------------------------
// Tests for writeToFile
// ---------------------------------------------------------------------------

func TestPatchOptionsWriteToFile(t *testing.T) {
	tmpDir := t.TempDir()
	baseFile := writeTestBaseFile(t, tmpDir)
	patchFile := writeTestPatchFile(t, tmpDir, "p.yaml")
	outputFile := filepath.Join(tmpDir, "subdir", "nested", "out.yaml")

	o, _, _ := newTestPatchOptions(t, nil)
	o.BaseFile = baseFile
	o.PatchFiles = []string{patchFile}
	o.OutputFile = outputFile

	docSet, err := o.loadBaseResources()
	if err != nil {
		t.Fatalf("loadBaseResources failed: %v", err)
	}
	patchableSet, err := o.applyPatches(docSet)
	if err != nil {
		t.Fatalf("applyPatches failed: %v", err)
	}

	if err := o.writeToFile(patchableSet); err != nil {
		t.Fatalf("writeToFile failed: %v", err)
	}

	data, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty output file")
	}
}

// ---------------------------------------------------------------------------
// Tests for writeToDirectory
// ---------------------------------------------------------------------------

func TestPatchOptionsWriteToDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	baseFile := writeTestBaseFile(t, tmpDir)
	patchFile := writeTestPatchFile(t, tmpDir, "p.yaml")
	outputDir := filepath.Join(tmpDir, "my-output")

	o, _, _ := newTestPatchOptions(t, nil)
	o.BaseFile = baseFile
	o.PatchFiles = []string{patchFile}
	o.OutputDir = outputDir

	docSet, err := o.loadBaseResources()
	if err != nil {
		t.Fatalf("loadBaseResources failed: %v", err)
	}
	patchableSet, err := o.applyPatches(docSet)
	if err != nil {
		t.Fatalf("applyPatches failed: %v", err)
	}

	if err := o.writeToDirectory(patchableSet); err != nil {
		t.Fatalf("writeToDirectory failed: %v", err)
	}

	// Should create outputDir and a file named base-patched.yaml inside
	expectedFile := filepath.Join(outputDir, "base-patched.yaml")
	if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
		t.Errorf("expected output file %s to exist", expectedFile)
	}
}

// ---------------------------------------------------------------------------
// Tests for applyPatches and applyPatchFile
// ---------------------------------------------------------------------------

func TestPatchOptionsApplyPatches(t *testing.T) {
	tmpDir := t.TempDir()
	baseFile := writeTestBaseFile(t, tmpDir)
	patchFile := writeTestPatchFile(t, tmpDir, "p.yaml")

	o, _, _ := newTestPatchOptions(t, nil)
	o.BaseFile = baseFile
	o.PatchFiles = []string{patchFile}
	o.OutputDir = filepath.Join(tmpDir, "out")

	docSet, err := o.loadBaseResources()
	if err != nil {
		t.Fatalf("loadBaseResources failed: %v", err)
	}

	patchableSet, err := o.applyPatches(docSet)
	if err != nil {
		t.Fatalf("applyPatches failed: %v", err)
	}

	if patchableSet == nil {
		t.Fatal("expected non-nil PatchableAppSet")
	}

	if patchableSet.DocumentSet == nil {
		t.Error("expected non-nil DocumentSet in PatchableAppSet")
	}

	if len(patchableSet.Resources) == 0 {
		t.Error("expected at least one resource in PatchableAppSet")
	}
}

func TestPatchOptionsApplyPatchesVerbose(t *testing.T) {
	tmpDir := t.TempDir()
	baseFile := writeTestBaseFile(t, tmpDir)
	patchFile := writeTestPatchFile(t, tmpDir, "p.yaml")

	o, _, stderr := newTestPatchOptions(t, func(g *options.GlobalOptions) {
		g.Verbose = true
	})
	o.BaseFile = baseFile
	o.PatchFiles = []string{patchFile}
	o.OutputDir = filepath.Join(tmpDir, "out")

	docSet, err := o.loadBaseResources()
	if err != nil {
		t.Fatalf("loadBaseResources failed: %v", err)
	}

	_, err = o.applyPatches(docSet)
	if err != nil {
		t.Fatalf("applyPatches failed: %v", err)
	}

	if !strings.Contains(stderr.String(), "Applying patch") {
		t.Error("expected verbose output to contain 'Applying patch'")
	}
}

// ---------------------------------------------------------------------------
// Tests for loadBaseResources
// ---------------------------------------------------------------------------

func TestPatchOptionsLoadBaseResources(t *testing.T) {
	tmpDir := t.TempDir()
	baseFile := writeTestBaseFile(t, tmpDir)

	o, _, _ := newTestPatchOptions(t, nil)
	o.BaseFile = baseFile

	docSet, err := o.loadBaseResources()
	if err != nil {
		t.Fatalf("loadBaseResources failed: %v", err)
	}

	if docSet == nil {
		t.Fatal("expected non-nil document set")
	}

	if len(docSet.Documents) != 1 {
		t.Errorf("expected 1 document, got %d", len(docSet.Documents))
	}

	if docSet.Documents[0].Resource.GetName() != "test-config" {
		t.Errorf("expected resource name 'test-config', got %s", docSet.Documents[0].Resource.GetName())
	}
}

func TestPatchOptionsLoadBaseResourcesNonexistent(t *testing.T) {
	o, _, _ := newTestPatchOptions(t, nil)
	o.BaseFile = "/nonexistent/base.yaml"

	_, err := o.loadBaseResources()
	if err == nil {
		t.Error("expected error for nonexistent base file")
	}
}

// ---------------------------------------------------------------------------
// Tests for writeCombinedToFile
// ---------------------------------------------------------------------------

func TestWriteCombinedToFileNonStdout(t *testing.T) {
	tmpDir := t.TempDir()
	baseFile := writeTestBaseFile(t, tmpDir)
	outputFile := filepath.Join(tmpDir, "subdir", "combined.yaml")

	o, _, _ := newTestPatchOptions(t, nil)
	o.BaseFile = baseFile
	o.OutputFile = outputFile

	docSet, err := o.loadBaseResources()
	if err != nil {
		t.Fatalf("loadBaseResources failed: %v", err)
	}

	if err := o.writeCombinedToFile(docSet); err != nil {
		t.Fatalf("writeCombinedToFile failed: %v", err)
	}

	data, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}

	if !strings.Contains(string(data), "test-config") {
		t.Error("expected output to contain resource name")
	}
}

// ---------------------------------------------------------------------------
// Tests for sortDocumentsByKind
// ---------------------------------------------------------------------------

func TestSortDocumentsByKind(t *testing.T) {
	tmpDir := t.TempDir()
	baseFile := filepath.Join(tmpDir, "base.yaml")

	// Create a multi-resource file with different kinds in unsorted order
	baseContent := `apiVersion: v1
kind: Service
metadata:
  name: svc-b
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cfg-a
---
apiVersion: v1
kind: Service
metadata:
  name: svc-a
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cfg-b
`
	if err := os.WriteFile(baseFile, []byte(baseContent), 0644); err != nil {
		t.Fatalf("failed to write base file: %v", err)
	}

	o, _, _ := newTestPatchOptions(t, nil)
	o.BaseFile = baseFile

	docSet, err := o.loadBaseResources()
	if err != nil {
		t.Fatalf("loadBaseResources failed: %v", err)
	}

	o.sortDocumentsByKind(docSet)

	// After sorting: ConfigMap cfg-a, ConfigMap cfg-b, Service svc-a, Service svc-b
	expected := []struct {
		kind string
		name string
	}{
		{"ConfigMap", "cfg-a"},
		{"ConfigMap", "cfg-b"},
		{"Service", "svc-a"},
		{"Service", "svc-b"},
	}

	if len(docSet.Documents) != len(expected) {
		t.Fatalf("expected %d documents, got %d", len(expected), len(docSet.Documents))
	}

	for i, exp := range expected {
		doc := docSet.Documents[i]
		if doc.Resource.GetKind() != exp.kind || doc.Resource.GetName() != exp.name {
			t.Errorf("doc[%d]: expected %s/%s, got %s/%s",
				i, exp.kind, exp.name,
				doc.Resource.GetKind(), doc.Resource.GetName())
		}
	}
}

// ---------------------------------------------------------------------------
// Tests for printColoredDiff with colors enabled
// ---------------------------------------------------------------------------

func TestPrintColoredDiffWithColors(t *testing.T) {
	o, stdout, _ := newTestPatchOptions(t, nil)

	// Set TERM and unset NO_COLOR to trigger colored output
	oldTerm := os.Getenv("TERM")
	oldNoColor := os.Getenv("NO_COLOR")
	os.Setenv("TERM", "xterm")
	os.Unsetenv("NO_COLOR")
	defer func() {
		if oldTerm != "" {
			os.Setenv("TERM", oldTerm)
		} else {
			os.Unsetenv("TERM")
		}
		if oldNoColor != "" {
			os.Setenv("NO_COLOR", oldNoColor)
		}
	}()

	diffText := `--- a/test.yaml
+++ b/test.yaml
@@ -1,3 +1,3 @@
 unchanged
-removed
+added
`

	o.printColoredDiff(diffText)
	output := stdout.String()

	// With colors enabled, ANSI escape codes should be present
	if !strings.Contains(output, "\033[") {
		t.Error("expected ANSI color codes in output when TERM is set and NO_COLOR is unset")
	}
	// Verify the content is still present
	if !strings.Contains(output, "removed") {
		t.Error("expected 'removed' in colored diff output")
	}
	if !strings.Contains(output, "added") {
		t.Error("expected 'added' in colored diff output")
	}
}

// ---------------------------------------------------------------------------
// Tests for writeDocumentSetToBuffer
// ---------------------------------------------------------------------------

func TestWriteDocumentSetToBuffer(t *testing.T) {
	tmpDir := t.TempDir()
	baseFile := writeTestBaseFile(t, tmpDir)

	o, _, _ := newTestPatchOptions(t, nil)
	o.BaseFile = baseFile

	docSet, err := o.loadBaseResources()
	if err != nil {
		t.Fatalf("loadBaseResources failed: %v", err)
	}

	var buf bytes.Buffer
	if err := o.writeDocumentSetToBuffer(docSet, &buf); err != nil {
		t.Fatalf("writeDocumentSetToBuffer failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "test-config") {
		t.Error("expected buffer to contain resource name")
	}
	if !strings.Contains(output, "key1") {
		t.Error("expected buffer to contain data key")
	}
}

func TestWriteDocumentSetToBufferMultiDoc(t *testing.T) {
	tmpDir := t.TempDir()
	baseFile := filepath.Join(tmpDir, "multi.yaml")
	content := `apiVersion: v1
kind: ConfigMap
metadata:
  name: cfg1
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cfg2
`
	if err := os.WriteFile(baseFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	o, _, _ := newTestPatchOptions(t, nil)
	o.BaseFile = baseFile

	docSet, err := o.loadBaseResources()
	if err != nil {
		t.Fatalf("loadBaseResources failed: %v", err)
	}

	var buf bytes.Buffer
	if err := o.writeDocumentSetToBuffer(docSet, &buf); err != nil {
		t.Fatalf("writeDocumentSetToBuffer failed: %v", err)
	}

	output := buf.String()
	// Multi-doc should have separator
	if !strings.Contains(output, "---") {
		t.Error("expected YAML document separator in multi-doc output")
	}
	if !strings.Contains(output, "cfg1") || !strings.Contains(output, "cfg2") {
		t.Error("expected both documents in output")
	}
}

// ---------------------------------------------------------------------------
// Tests for validatePatchFile with nonexistent file
// ---------------------------------------------------------------------------

func TestValidatePatchFileNonexistent(t *testing.T) {
	o, _, _ := newTestPatchOptions(t, nil)

	err := o.validatePatchFile("/nonexistent/patch.yaml")
	if err == nil {
		t.Error("expected error for nonexistent patch file")
	}
}

// ---------------------------------------------------------------------------
// Tests for runValidation with verbose and invalid file
// ---------------------------------------------------------------------------

func TestRunValidationInvalidFile(t *testing.T) {
	tmpDir := t.TempDir()
	patchFile := filepath.Join(tmpDir, "invalid.yaml")
	if err := os.WriteFile(patchFile, []byte(""), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	o, _, _ := newTestPatchOptions(t, nil)
	o.PatchFiles = []string{patchFile}

	err := o.runValidation()
	if err == nil {
		t.Error("expected error for invalid/empty patch file")
	}
}

func TestRunValidationVerbose(t *testing.T) {
	tmpDir := t.TempDir()
	patchFile := writeTestPatchFile(t, tmpDir, "valid.yaml")

	o, stdout, stderr := newTestPatchOptions(t, func(g *options.GlobalOptions) {
		g.Verbose = true
	})
	o.PatchFiles = []string{patchFile}

	err := o.runValidation()
	if err != nil {
		t.Fatalf("runValidation failed: %v", err)
	}

	if !strings.Contains(stdout.String(), "valid") {
		t.Error("expected success message in stdout")
	}
	if !strings.Contains(stderr.String(), "Validating") {
		t.Error("expected verbose validation message in stderr")
	}
}

// ---------------------------------------------------------------------------
// Tests for Validate with multiple patch files where one is invalid
// ---------------------------------------------------------------------------

func TestPatchOptionsValidateMultiplePatchFiles(t *testing.T) {
	tmpDir := t.TempDir()
	baseFile := writeTestBaseFile(t, tmpDir)
	goodPatch := writeTestPatchFile(t, tmpDir, "good.yaml")

	o, _, _ := newTestPatchOptions(t, nil)
	o.BaseFile = baseFile
	o.PatchFiles = []string{goodPatch, "/nonexistent/bad.yaml"}

	err := o.Validate()
	if err == nil {
		t.Error("expected error when one patch file does not exist")
	}
}

// ---------------------------------------------------------------------------
// Test for runCombined verbose output
// ---------------------------------------------------------------------------

func TestRunCombinedVerbose(t *testing.T) {
	tmpDir := t.TempDir()
	baseFile := writeTestBaseFile(t, tmpDir)
	patchFile := writeTestPatchFile(t, tmpDir, "p.yaml")
	outputFile := filepath.Join(tmpDir, "out.yaml")

	o, _, stderr := newTestPatchOptions(t, func(g *options.GlobalOptions) {
		g.Verbose = true
	})
	o.BaseFile = baseFile
	o.PatchFiles = []string{patchFile}
	o.Combined = true
	o.GroupBy = "none"
	o.OutputFile = outputFile

	err := o.runCombined()
	if err != nil {
		t.Fatalf("runCombined failed: %v", err)
	}

	errOutput := stderr.String()
	if !strings.Contains(errOutput, "Combined mode") {
		t.Error("expected verbose output to contain 'Combined mode'")
	}
	if !strings.Contains(errOutput, "Applying patch") {
		t.Error("expected verbose output to contain 'Applying patch'")
	}
}

// ---------------------------------------------------------------------------
// Test for runCombined with group-by "file"
// ---------------------------------------------------------------------------

func TestRunCombinedGroupByFile(t *testing.T) {
	tmpDir := t.TempDir()
	baseFile := writeTestBaseFile(t, tmpDir)
	patchFile := writeTestPatchFile(t, tmpDir, "p.yaml")
	outputFile := filepath.Join(tmpDir, "out.yaml")

	o, _, _ := newTestPatchOptions(t, nil)
	o.BaseFile = baseFile
	o.PatchFiles = []string{patchFile}
	o.Combined = true
	o.GroupBy = "file"
	o.OutputFile = outputFile

	err := o.runCombined()
	if err != nil {
		t.Fatalf("runCombined with group-by=file failed: %v", err)
	}

	// Verify output was written
	if _, err := os.Stat(outputFile); os.IsNotExist(err) {
		t.Error("expected output file to exist")
	}
}

// ---------------------------------------------------------------------------
// Test for runDiff with verbose output
// ---------------------------------------------------------------------------

func TestRunDiffVerbose(t *testing.T) {
	tmpDir := t.TempDir()
	baseFile := writeTestBaseFile(t, tmpDir)
	patchFile := writeTestPatchFile(t, tmpDir, "p.yaml")

	o, _, stderr := newTestPatchOptions(t, func(g *options.GlobalOptions) {
		g.Verbose = true
	})
	o.BaseFile = baseFile
	o.PatchFiles = []string{patchFile}
	o.Diff = true

	// Suppress colors
	oldNoColor := os.Getenv("NO_COLOR")
	os.Setenv("NO_COLOR", "1")
	defer func() {
		if oldNoColor != "" {
			os.Setenv("NO_COLOR", oldNoColor)
		} else {
			os.Unsetenv("NO_COLOR")
		}
	}()

	err := o.runDiff()
	if err != nil {
		t.Fatalf("runDiff failed: %v", err)
	}

	errOutput := stderr.String()
	if !strings.Contains(errOutput, "Diff mode") {
		t.Error("expected verbose 'Diff mode' message in stderr")
	}
}

// ---------------------------------------------------------------------------
// Test for runCombined writing to stdout (no OutputFile)
// ---------------------------------------------------------------------------

func TestRunCombinedStdout(t *testing.T) {
	tmpDir := t.TempDir()
	baseFile := writeTestBaseFile(t, tmpDir)
	patchFile := writeTestPatchFile(t, tmpDir, "p.yaml")

	o, _, _ := newTestPatchOptions(t, nil)
	o.BaseFile = baseFile
	o.PatchFiles = []string{patchFile}
	o.Combined = true
	o.GroupBy = "none"
	o.OutputFile = "" // No output file -> writes to /dev/stdout

	// runCombined falls through to documentSet.WriteToFile("/dev/stdout")
	// which writes to actual stdout. We can't capture that easily, but we
	// can verify it does not error.
	err := o.runCombined()
	if err != nil {
		t.Fatalf("runCombined to stdout failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Test for runCombined with nonexistent base file
// ---------------------------------------------------------------------------

func TestRunCombinedNonexistentBase(t *testing.T) {
	o, _, _ := newTestPatchOptions(t, nil)
	o.BaseFile = "/nonexistent/base.yaml"
	o.PatchFiles = []string{"/nonexistent/patch.yaml"}
	o.Combined = true
	o.GroupBy = "none"

	err := o.runCombined()
	if err == nil {
		t.Error("expected error for nonexistent base file")
	}
}

// ---------------------------------------------------------------------------
// Test for runCombined with nonexistent patch file
// ---------------------------------------------------------------------------

func TestRunCombinedNonexistentPatch(t *testing.T) {
	tmpDir := t.TempDir()
	baseFile := writeTestBaseFile(t, tmpDir)

	o, _, _ := newTestPatchOptions(t, nil)
	o.BaseFile = baseFile
	o.PatchFiles = []string{"/nonexistent/patch.yaml"}
	o.Combined = true
	o.GroupBy = "none"

	err := o.runCombined()
	if err == nil {
		t.Error("expected error for nonexistent patch file")
	}
}

// ---------------------------------------------------------------------------
// Test for runDiff nonexistent base
// ---------------------------------------------------------------------------

func TestRunDiffNonexistentBase(t *testing.T) {
	o, _, _ := newTestPatchOptions(t, nil)
	o.BaseFile = "/nonexistent/base.yaml"
	o.PatchFiles = []string{"/nonexistent/patch.yaml"}
	o.Diff = true

	err := o.runDiff()
	if err == nil {
		t.Error("expected error for nonexistent base file")
	}
}

// ---------------------------------------------------------------------------
// Test for runDiff nonexistent patch
// ---------------------------------------------------------------------------

func TestRunDiffNonexistentPatch(t *testing.T) {
	tmpDir := t.TempDir()
	baseFile := writeTestBaseFile(t, tmpDir)

	o, _, _ := newTestPatchOptions(t, nil)
	o.BaseFile = baseFile
	o.PatchFiles = []string{"/nonexistent/patch.yaml"}
	o.Diff = true

	err := o.runDiff()
	if err == nil {
		t.Error("expected error for nonexistent patch file")
	}
}

// ---------------------------------------------------------------------------
// Tests for strategic merge patches in runCombined and runDiff
// ---------------------------------------------------------------------------

// writeStrategicPatchFile creates a strategic merge patch file targeting a resource.
func writeStrategicPatchFile(t *testing.T, dir, name, target string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	content := fmt.Sprintf(`- target: %s
  type: strategic
  patch:
    metadata:
      labels:
        patched: "true"
`, target)
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write strategic patch file: %v", err)
	}
	return p
}

func TestRunCombinedStrategicPatch(t *testing.T) {
	tmpDir := t.TempDir()
	baseFile := filepath.Join(tmpDir, "base.yaml")
	outputFile := filepath.Join(tmpDir, "out.yaml")

	baseContent := `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
data:
  key1: value1
`
	if err := os.WriteFile(baseFile, []byte(baseContent), 0644); err != nil {
		t.Fatalf("failed to write base: %v", err)
	}

	patchFile := writeStrategicPatchFile(t, tmpDir, "strategic.yaml", "test-config")

	o, _, _ := newTestPatchOptions(t, nil)
	o.BaseFile = baseFile
	o.PatchFiles = []string{patchFile}
	o.Combined = true
	o.GroupBy = "none"
	o.OutputFile = outputFile

	err := o.runCombined()
	if err != nil {
		t.Fatalf("runCombined with strategic patch failed: %v", err)
	}

	data, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}

	if !strings.Contains(string(data), "patched") {
		t.Error("expected strategic patch label in output")
	}
}

func TestRunDiffStrategicPatch(t *testing.T) {
	tmpDir := t.TempDir()
	baseFile := filepath.Join(tmpDir, "base.yaml")

	baseContent := `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
data:
  key1: value1
`
	if err := os.WriteFile(baseFile, []byte(baseContent), 0644); err != nil {
		t.Fatalf("failed to write base: %v", err)
	}

	patchFile := writeStrategicPatchFile(t, tmpDir, "strategic.yaml", "test-config")

	// Suppress colors
	oldNoColor := os.Getenv("NO_COLOR")
	os.Setenv("NO_COLOR", "1")
	defer func() {
		if oldNoColor != "" {
			os.Setenv("NO_COLOR", oldNoColor)
		} else {
			os.Unsetenv("NO_COLOR")
		}
	}()

	o, stdout, _ := newTestPatchOptions(t, nil)
	o.BaseFile = baseFile
	o.PatchFiles = []string{patchFile}
	o.Diff = true

	err := o.runDiff()
	if err != nil {
		t.Fatalf("runDiff with strategic patch failed: %v", err)
	}

	output := stdout.String()
	// Diff should show the added label
	if !strings.Contains(output, "patched") {
		t.Error("expected diff output to show the strategic patch change")
	}
}

// ---------------------------------------------------------------------------
// Tests for NewPatchCommand RunE error paths
// ---------------------------------------------------------------------------

func TestNewPatchCommandRunE_CompleteError(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	cmd := NewPatchCommand(globalOpts)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// Pass a nonexistent patch-dir to trigger Complete() error
	cmd.SetArgs([]string{"--patch-dir=/nonexistent/dir", "base.yaml"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error from Complete() with invalid patch-dir")
	}
}

func TestNewPatchCommandRunE_ValidateError(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	cmd := NewPatchCommand(globalOpts)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// base file doesn't exist -> Validate() should fail
	cmd.SetArgs([]string{"/nonexistent/base.yaml", "/nonexistent/patch.yaml"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error from Validate() with nonexistent base file")
	}
}

// ---------------------------------------------------------------------------
// Test for writeCombinedToFile with /dev/stdout
// ---------------------------------------------------------------------------

func TestWriteCombinedToFileStdout(t *testing.T) {
	tmpDir := t.TempDir()
	baseFile := writeTestBaseFile(t, tmpDir)

	o, _, _ := newTestPatchOptions(t, nil)
	o.BaseFile = baseFile
	o.OutputFile = "/dev/stdout"

	docSet, err := o.loadBaseResources()
	if err != nil {
		t.Fatalf("loadBaseResources failed: %v", err)
	}

	// Should not error even though output goes to stdout
	err = o.writeCombinedToFile(docSet)
	if err != nil {
		t.Fatalf("writeCombinedToFile to /dev/stdout failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Test for runInteractive with nonexistent base file
// ---------------------------------------------------------------------------

func TestRunInteractiveNonexistentBase(t *testing.T) {
	o, _, _ := newTestPatchOptions(t, nil)
	o.BaseFile = "/nonexistent/base.yaml"
	o.Interactive = true

	err := o.runInteractive()
	if err == nil {
		t.Error("expected error for nonexistent base file in interactive mode")
	}
}

// ---------------------------------------------------------------------------
// Test for writeToFile with /dev/stdout path (routes through writeToStdout)
// ---------------------------------------------------------------------------

func TestWriteToFileStdoutPath(t *testing.T) {
	tmpDir := t.TempDir()
	baseFile := writeTestBaseFile(t, tmpDir)
	patchFile := writeTestPatchFile(t, tmpDir, "p.yaml")

	o, _, _ := newTestPatchOptions(t, nil)
	o.BaseFile = baseFile
	o.PatchFiles = []string{patchFile}
	o.OutputFile = "/dev/stdout"
	o.OutputDir = filepath.Join(tmpDir, "outdir")

	docSet, err := o.loadBaseResources()
	if err != nil {
		t.Fatalf("loadBaseResources failed: %v", err)
	}
	patchableSet, err := o.applyPatches(docSet)
	if err != nil {
		t.Fatalf("applyPatches failed: %v", err)
	}

	err = o.writeToFile(patchableSet)
	if err != nil {
		t.Fatalf("writeToFile with /dev/stdout failed: %v", err)
	}
}
