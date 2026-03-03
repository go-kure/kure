package generate

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-kure/kure/pkg/cli"
	"github.com/go-kure/kure/pkg/cmd/shared/options"
)

func TestNewAppCommand(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	factory := cli.NewFactory(globalOpts)
	cmd := NewAppCommand(factory)

	if cmd == nil {
		t.Fatal("expected non-nil app command")
	}

	if extractCommandName(cmd.Use) != "app" {
		t.Errorf("expected command name 'app', got %s", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("expected non-empty short description")
	}

	if cmd.Long == "" {
		t.Error("expected non-empty long description")
	}
}

func TestAppOptionsAddFlags(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	factory := cli.NewFactory(globalOpts)
	cmd := NewAppCommand(factory)

	// Check that expected flags exist
	flags := []string{"input-dir", "output-dir", "output-file"}
	for _, flag := range flags {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("expected flag %q to exist", flag)
		}
	}
}

func TestAppOptionsComplete(t *testing.T) {
	tests := []struct {
		name         string
		opts         *AppOptions
		globalOpts   *options.GlobalOptions
		expectOutDir string
	}{
		{
			name: "default options",
			opts: &AppOptions{
				ConfigFiles: []string{"app.yaml"},
				OutputDir:   "out/apps",
			},
			globalOpts:   &options.GlobalOptions{DryRun: false},
			expectOutDir: "out/apps",
		},
		{
			name: "dry-run without output file",
			opts: &AppOptions{
				ConfigFiles: []string{"app.yaml"},
				OutputDir:   "out/apps",
				OutputFile:  "",
			},
			globalOpts:   &options.GlobalOptions{DryRun: true},
			expectOutDir: "out/apps",
		},
		{
			name: "global output file",
			opts: &AppOptions{
				ConfigFiles: []string{"app.yaml"},
				OutputDir:   "out/apps",
				OutputFile:  "",
			},
			globalOpts:   &options.GlobalOptions{OutputFile: "custom.yaml"},
			expectOutDir: "out/apps",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := cli.NewFactory(tt.globalOpts)
			tt.opts.Factory = factory
			tt.opts.IOStreams = factory.IOStreams()

			err := tt.opts.Complete()
			if err != nil {
				t.Fatalf("Complete() error = %v", err)
			}

			if tt.opts.OutputDir != tt.expectOutDir {
				t.Errorf("OutputDir = %q, want %q", tt.opts.OutputDir, tt.expectOutDir)
			}
		})
	}
}

func TestAppOptionsValidate(t *testing.T) {
	// Create a temp config file
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "app.yaml")
	if err := os.WriteFile(configFile, []byte("name: test"), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		opts    *AppOptions
		wantErr bool
	}{
		{
			name: "valid options with config file",
			opts: &AppOptions{
				ConfigFiles: []string{configFile},
			},
			wantErr: false,
		},
		{
			name: "missing config files",
			opts: &AppOptions{
				ConfigFiles: []string{},
			},
			wantErr: true,
		},
		{
			name: "config file does not exist",
			opts: &AppOptions{
				ConfigFiles: []string{"/nonexistent/file.yaml"},
			},
			wantErr: true,
		},
		{
			name: "multiple valid config files",
			opts: &AppOptions{
				ConfigFiles: []string{configFile, configFile},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.opts.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAppOptionsScanInputDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create some YAML files
	if err := os.WriteFile(filepath.Join(tmpDir, "app1.yaml"), []byte("name: app1"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "app2.yml"), []byte("name: app2"), 0644); err != nil {
		t.Fatal(err)
	}
	// Create a non-yaml file (should be ignored)
	if err := os.WriteFile(filepath.Join(tmpDir, "readme.txt"), []byte("readme"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a subdirectory with yaml
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "app3.yaml"), []byte("name: app3"), 0644); err != nil {
		t.Fatal(err)
	}

	globalOpts := &options.GlobalOptions{}
	factory := cli.NewFactory(globalOpts)

	opts := &AppOptions{
		InputDir:  tmpDir,
		Factory:   factory,
		IOStreams: factory.IOStreams(),
	}

	files, err := opts.scanInputDirectory()
	if err != nil {
		t.Fatalf("scanInputDirectory() error = %v", err)
	}

	// Should find 3 yaml files
	if len(files) != 3 {
		t.Errorf("scanInputDirectory() found %d files, want 3", len(files))
	}
}

func TestAppOptionsLoadApplications(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a valid app workload config based on the real example format
	appConfig := `
apiVersion: generators.gokure.dev/v1alpha1
kind: AppWorkload
metadata:
  name: test-app
  namespace: default
spec:
  workload: Deployment
  replicas: 1
  containers:
    - name: nginx
      image: nginx:latest
`
	configFile := filepath.Join(tmpDir, "app.yaml")
	if err := os.WriteFile(configFile, []byte(appConfig), 0644); err != nil {
		t.Fatal(err)
	}

	globalOpts := &options.GlobalOptions{}
	factory := cli.NewFactory(globalOpts)

	opts := &AppOptions{
		ConfigFiles: []string{configFile},
		Factory:     factory,
		IOStreams:   factory.IOStreams(),
	}

	apps, err := opts.loadApplications()
	if err != nil {
		t.Fatalf("loadApplications() error = %v", err)
	}

	if len(apps) != 1 {
		t.Errorf("loadApplications() returned %d apps, want 1", len(apps))
	}

	if apps[0].Name != "test-app" {
		t.Errorf("app name = %q, want %q", apps[0].Name, "test-app")
	}
}

func TestAppOptionsRun(t *testing.T) {
	t.Run("no applications found", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create an empty yaml file
		configFile := filepath.Join(tmpDir, "empty.yaml")
		if err := os.WriteFile(configFile, []byte(""), 0644); err != nil {
			t.Fatal(err)
		}

		globalOpts := &options.GlobalOptions{Verbose: true}
		factory := cli.NewFactory(globalOpts)

		var stdout, stderr bytes.Buffer
		ioStreams := cli.IOStreams{
			Out:    &stdout,
			ErrOut: &stderr,
		}

		opts := &AppOptions{
			ConfigFiles: []string{configFile},
			OutputDir:   filepath.Join(tmpDir, "out"),
			Factory:     factory,
			IOStreams:   ioStreams,
		}

		if err := opts.Complete(); err != nil {
			t.Fatalf("Complete() error = %v", err)
		}

		if err := opts.Validate(); err != nil {
			t.Fatalf("Validate() error = %v", err)
		}

		err := opts.Run()
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}

		// Check that "No applications found" was printed
		if !bytes.Contains(stderr.Bytes(), []byte("No applications found")) {
			t.Error("expected 'No applications found' message in stderr")
		}
	})

	t.Run("with valid application", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create a valid app workload config based on the real example format
		appConfig := `
apiVersion: generators.gokure.dev/v1alpha1
kind: AppWorkload
metadata:
  name: test-app
  namespace: default
spec:
  workload: Deployment
  replicas: 1
  containers:
    - name: nginx
      image: nginx:latest
`
		configFile := filepath.Join(tmpDir, "app.yaml")
		if err := os.WriteFile(configFile, []byte(appConfig), 0644); err != nil {
			t.Fatal(err)
		}

		globalOpts := &options.GlobalOptions{DryRun: true}
		factory := cli.NewFactory(globalOpts)

		var stdout, stderr bytes.Buffer
		ioStreams := cli.IOStreams{
			Out:    &stdout,
			ErrOut: &stderr,
		}

		opts := &AppOptions{
			ConfigFiles: []string{configFile},
			OutputDir:   filepath.Join(tmpDir, "out"),
			OutputFile:  "/dev/stdout",
			Factory:     factory,
			IOStreams:   ioStreams,
		}

		if err := opts.Complete(); err != nil {
			t.Fatalf("Complete() error = %v", err)
		}

		if err := opts.Validate(); err != nil {
			t.Fatalf("Validate() error = %v", err)
		}

		// Run should succeed
		err := opts.Run()
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	})
}

func TestAppWriteToFile(t *testing.T) {
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "output", "test.yaml")

	globalOpts := &options.GlobalOptions{}
	factory := cli.NewFactory(globalOpts)

	var stdout bytes.Buffer
	ioStreams := cli.IOStreams{
		Out:    &stdout,
		ErrOut: &bytes.Buffer{},
	}

	opts := &AppOptions{
		OutputFile: outputFile,
		Factory:    factory,
		IOStreams:  ioStreams,
	}

	testContent := []byte("test content")
	err := opts.writeToFile(testContent)
	if err != nil {
		t.Fatalf("writeToFile() error = %v", err)
	}

	// Verify file was created
	content, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	if string(content) != string(testContent) {
		t.Errorf("file content = %q, want %q", string(content), string(testContent))
	}
}

func TestAppWriteToStdout(t *testing.T) {
	globalOpts := &options.GlobalOptions{}
	factory := cli.NewFactory(globalOpts)

	var stdout bytes.Buffer
	ioStreams := cli.IOStreams{
		Out:    &stdout,
		ErrOut: &bytes.Buffer{},
	}

	opts := &AppOptions{
		OutputFile: "/dev/stdout",
		Factory:    factory,
		IOStreams:  ioStreams,
	}

	testContent := []byte("test content")
	err := opts.writeToFile(testContent)
	if err != nil {
		t.Fatalf("writeToFile() error = %v", err)
	}

	if stdout.String() != string(testContent) {
		t.Errorf("stdout = %q, want %q", stdout.String(), string(testContent))
	}
}

func TestAppCompleteWithInputDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Create yaml files in input dir
	if err := os.WriteFile(filepath.Join(tmpDir, "app1.yaml"), []byte("name: app1"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "app2.yaml"), []byte("name: app2"), 0644); err != nil {
		t.Fatal(err)
	}

	globalOpts := &options.GlobalOptions{}
	factory := cli.NewFactory(globalOpts)

	opts := &AppOptions{
		ConfigFiles: []string{},
		InputDir:    tmpDir,
		Factory:     factory,
		IOStreams:   factory.IOStreams(),
	}

	err := opts.Complete()
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	// Should have found 2 config files
	if len(opts.ConfigFiles) != 2 {
		t.Errorf("ConfigFiles count = %d, want 2", len(opts.ConfigFiles))
	}
}

func TestNewAppCommandRunE(t *testing.T) {
	t.Run("missing config files returns error", func(t *testing.T) {
		globalOpts := options.NewGlobalOptions()
		factory := cli.NewFactory(globalOpts)
		cmd := NewAppCommand(factory)

		// Execute with no args should fail validation (no config files)
		cmd.SetArgs([]string{})
		err := cmd.Execute()
		if err == nil {
			t.Error("expected error when no config files specified")
		}
	})

	t.Run("nonexistent config file returns error", func(t *testing.T) {
		globalOpts := options.NewGlobalOptions()
		factory := cli.NewFactory(globalOpts)
		cmd := NewAppCommand(factory)

		cmd.SetArgs([]string{"/nonexistent/file.yaml"})
		err := cmd.Execute()
		if err == nil {
			t.Error("expected error for nonexistent config file")
		}
	})

	t.Run("valid config file runs successfully", func(t *testing.T) {
		tmpDir := t.TempDir()

		appConfig := `
apiVersion: generators.gokure.dev/v1alpha1
kind: AppWorkload
metadata:
  name: test-app
  namespace: default
spec:
  workload: Deployment
  replicas: 1
  containers:
    - name: nginx
      image: nginx:latest
`
		configFile := filepath.Join(tmpDir, "app.yaml")
		if err := os.WriteFile(configFile, []byte(appConfig), 0644); err != nil {
			t.Fatal(err)
		}

		globalOpts := options.NewGlobalOptions()
		globalOpts.DryRun = true
		factory := cli.NewFactory(globalOpts)
		cmd := NewAppCommand(factory)

		var stdout, stderr bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)

		cmd.SetArgs([]string{configFile})
		err := cmd.Execute()
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
	})
}

func TestAppWriteToDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "out", "apps")

	globalOpts := &options.GlobalOptions{}
	factory := cli.NewFactory(globalOpts)

	var stdout bytes.Buffer
	ioStreams := cli.IOStreams{
		Out:    &stdout,
		ErrOut: &bytes.Buffer{},
	}

	opts := &AppOptions{
		OutputDir: outputDir,
		Factory:   factory,
		IOStreams: ioStreams,
	}

	// Test with empty resources (still exercises the directory cleanup and creation)
	err := opts.writeToDirectory(nil)
	if err != nil {
		t.Fatalf("writeToDirectory(nil) error = %v", err)
	}

	// Output directory should not exist since no resources were written
	// (the function removes the output dir then only creates subdirs for resources)
}

func TestAppWriteOutputToDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	appConfig := `
apiVersion: generators.gokure.dev/v1alpha1
kind: AppWorkload
metadata:
  name: test-app
  namespace: default
spec:
  workload: Deployment
  replicas: 1
  containers:
    - name: nginx
      image: nginx:latest
`
	configFile := filepath.Join(tmpDir, "app.yaml")
	if err := os.WriteFile(configFile, []byte(appConfig), 0644); err != nil {
		t.Fatal(err)
	}

	globalOpts := &options.GlobalOptions{DryRun: false}
	factory := cli.NewFactory(globalOpts)

	var stdout, stderr bytes.Buffer
	ioStreams := cli.IOStreams{
		Out:    &stdout,
		ErrOut: &stderr,
	}

	outputDir := filepath.Join(tmpDir, "out", "apps")
	opts := &AppOptions{
		ConfigFiles: []string{configFile},
		OutputDir:   outputDir,
		OutputFile:  "", // no output file, should write to directory
		Factory:     factory,
		IOStreams:   ioStreams,
	}

	// Load applications and generate manifests, then write to directory
	apps, err := opts.loadApplications()
	if err != nil {
		t.Fatalf("loadApplications() error = %v", err)
	}
	if len(apps) == 0 {
		t.Fatal("expected at least one application")
	}

	resources, err := opts.generateManifests(apps)
	if err != nil {
		t.Fatalf("generateManifests() error = %v", err)
	}

	// Write output using the directory path (not dry-run, no output file)
	err = opts.writeOutput(resources)
	if err != nil {
		t.Fatalf("writeOutput() error = %v", err)
	}

	// Verify output directory was created
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		t.Errorf("expected output directory %q to exist", outputDir)
	}
}

func TestAppRunWithOutputFile(t *testing.T) {
	tmpDir := t.TempDir()

	appConfig := `
apiVersion: generators.gokure.dev/v1alpha1
kind: AppWorkload
metadata:
  name: test-app
  namespace: default
spec:
  workload: Deployment
  replicas: 1
  containers:
    - name: nginx
      image: nginx:latest
`
	configFile := filepath.Join(tmpDir, "app.yaml")
	if err := os.WriteFile(configFile, []byte(appConfig), 0644); err != nil {
		t.Fatal(err)
	}

	outputFile := filepath.Join(tmpDir, "output", "manifests.yaml")
	globalOpts := &options.GlobalOptions{DryRun: false, Verbose: true}
	factory := cli.NewFactory(globalOpts)

	var stdout, stderr bytes.Buffer
	ioStreams := cli.IOStreams{
		Out:    &stdout,
		ErrOut: &stderr,
	}

	opts := &AppOptions{
		ConfigFiles: []string{configFile},
		OutputDir:   filepath.Join(tmpDir, "out"),
		OutputFile:  outputFile,
		Factory:     factory,
		IOStreams:   ioStreams,
	}

	err := opts.Run()
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// Verify the file was created
	content, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	if len(content) == 0 {
		t.Error("expected non-empty output file")
	}
}

func TestAppFlagDefaults(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	factory := cli.NewFactory(globalOpts)
	cmd := NewAppCommand(factory)

	// Check default values
	outputDir, err := cmd.Flags().GetString("output-dir")
	if err != nil {
		t.Fatalf("GetString(output-dir) error = %v", err)
	}
	if outputDir != "out/apps" {
		t.Errorf("output-dir default = %q, want %q", outputDir, "out/apps")
	}

	inputDir, err := cmd.Flags().GetString("input-dir")
	if err != nil {
		t.Fatalf("GetString(input-dir) error = %v", err)
	}
	if inputDir != "" {
		t.Errorf("input-dir default = %q, want %q", inputDir, "")
	}

	outputFile, err := cmd.Flags().GetString("output-file")
	if err != nil {
		t.Fatalf("GetString(output-file) error = %v", err)
	}
	if outputFile != "" {
		t.Errorf("output-file default = %q, want %q", outputFile, "")
	}
}

func TestAppLoadApplicationsFromFile_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "bad.yaml")
	if err := os.WriteFile(configFile, []byte("not: valid: yaml: {{"), 0644); err != nil {
		t.Fatal(err)
	}

	globalOpts := &options.GlobalOptions{}
	factory := cli.NewFactory(globalOpts)

	opts := &AppOptions{
		ConfigFiles: []string{configFile},
		Factory:     factory,
		IOStreams:   factory.IOStreams(),
	}

	_, err := opts.loadApplications()
	if err == nil {
		t.Error("expected error for invalid YAML config file")
	}
}

func TestAppLoadApplicationsFromFile_NonexistentFile(t *testing.T) {
	globalOpts := &options.GlobalOptions{}
	factory := cli.NewFactory(globalOpts)

	opts := &AppOptions{
		ConfigFiles: []string{"/nonexistent/path/file.yaml"},
		Factory:     factory,
		IOStreams:   factory.IOStreams(),
	}

	_, err := opts.loadApplications()
	if err == nil {
		t.Error("expected error for nonexistent config file")
	}
}
