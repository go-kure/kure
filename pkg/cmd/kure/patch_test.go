package kure

import (
	"bytes"
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
