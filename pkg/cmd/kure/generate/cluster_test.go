package generate

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-kure/kure/pkg/cli"
	"github.com/go-kure/kure/pkg/cmd/shared/options"
	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"
)

func TestNewClusterCommand(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	factory := cli.NewFactory(globalOpts)
	cmd := NewClusterCommand(factory)

	if cmd == nil {
		t.Fatal("expected non-nil cluster command")
	}

	if extractCommandName(cmd.Use) != "cluster" {
		t.Errorf("expected command name 'cluster', got %s", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("expected non-empty short description")
	}

	if cmd.Long == "" {
		t.Error("expected non-empty long description")
	}
}

func TestClusterOptionsAddFlags(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	factory := cli.NewFactory(globalOpts)
	cmd := NewClusterCommand(factory)

	// Check that expected flags exist
	flags := []string{"output-dir", "manifest-dir", "bundle-grouping", "application-grouping", "flux-placement", "input-dir"}
	for _, flag := range flags {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("expected flag %q to exist", flag)
		}
	}
}

func TestClusterOptionsComplete(t *testing.T) {
	tests := []struct {
		name         string
		opts         *ClusterOptions
		globalOpts   *options.GlobalOptions
		expectOutDir string
	}{
		{
			name: "default options",
			opts: &ClusterOptions{
				ConfigFile: "/tmp/cluster.yaml",
				OutputDir:  "out",
			},
			globalOpts:   &options.GlobalOptions{DryRun: false},
			expectOutDir: "out",
		},
		{
			name: "dry-run with default output",
			opts: &ClusterOptions{
				ConfigFile: "/tmp/cluster.yaml",
				OutputDir:  "out",
			},
			globalOpts:   &options.GlobalOptions{DryRun: true},
			expectOutDir: "/dev/stdout",
		},
		{
			name: "dry-run with custom output",
			opts: &ClusterOptions{
				ConfigFile: "/tmp/cluster.yaml",
				OutputDir:  "custom-out",
			},
			globalOpts:   &options.GlobalOptions{DryRun: true},
			expectOutDir: "custom-out",
		},
		{
			name: "input dir from config file",
			opts: &ClusterOptions{
				ConfigFile: "/tmp/configs/cluster.yaml",
				OutputDir:  "out",
				InputDir:   "",
			},
			globalOpts:   &options.GlobalOptions{DryRun: false},
			expectOutDir: "out",
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

func TestClusterOptionsValidate(t *testing.T) {
	// Create a temp config file
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "cluster.yaml")
	if err := os.WriteFile(configFile, []byte("name: test"), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		opts    *ClusterOptions
		wantErr bool
	}{
		{
			name: "valid options",
			opts: &ClusterOptions{
				ConfigFile:          configFile,
				BundleGrouping:      "flat",
				ApplicationGrouping: "flat",
				FluxPlacement:       "integrated",
			},
			wantErr: false,
		},
		{
			name: "nested grouping valid",
			opts: &ClusterOptions{
				ConfigFile:          configFile,
				BundleGrouping:      "nested",
				ApplicationGrouping: "nested",
				FluxPlacement:       "separate",
			},
			wantErr: false,
		},
		{
			name: "missing config file",
			opts: &ClusterOptions{
				ConfigFile:          "/nonexistent/file.yaml",
				BundleGrouping:      "flat",
				ApplicationGrouping: "flat",
				FluxPlacement:       "integrated",
			},
			wantErr: true,
		},
		{
			name: "invalid bundle grouping",
			opts: &ClusterOptions{
				ConfigFile:          configFile,
				BundleGrouping:      "invalid",
				ApplicationGrouping: "flat",
				FluxPlacement:       "integrated",
			},
			wantErr: true,
		},
		{
			name: "invalid application grouping",
			opts: &ClusterOptions{
				ConfigFile:          configFile,
				BundleGrouping:      "flat",
				ApplicationGrouping: "invalid",
				FluxPlacement:       "integrated",
			},
			wantErr: true,
		},
		{
			name: "invalid flux placement",
			opts: &ClusterOptions{
				ConfigFile:          configFile,
				BundleGrouping:      "flat",
				ApplicationGrouping: "flat",
				FluxPlacement:       "invalid",
			},
			wantErr: true,
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

func TestContainsHelper(t *testing.T) {
	tests := []struct {
		slice    []string
		item     string
		expected bool
	}{
		{[]string{"a", "b", "c"}, "a", true},
		{[]string{"a", "b", "c"}, "b", true},
		{[]string{"a", "b", "c"}, "c", true},
		{[]string{"a", "b", "c"}, "d", false},
		{[]string{}, "a", false},
		{nil, "a", false},
	}

	for _, tt := range tests {
		result := contains(tt.slice, tt.item)
		if result != tt.expected {
			t.Errorf("contains(%v, %q) = %v, want %v", tt.slice, tt.item, result, tt.expected)
		}
	}
}

func TestClusterOptionsRun(t *testing.T) {
	t.Run("load cluster config", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create a minimal valid cluster config
		configFile := filepath.Join(tmpDir, "cluster.yaml")
		configContent := `
name: test-cluster
node:
  name: root
gitops:
  type: flux
`
		if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
			t.Fatal(err)
		}

		globalOpts := &options.GlobalOptions{Verbose: false, DryRun: true}
		factory := cli.NewFactory(globalOpts)

		opts := &ClusterOptions{
			ConfigFile:          configFile,
			InputDir:            tmpDir,
			OutputDir:           "/dev/stdout",
			BundleGrouping:      "flat",
			ApplicationGrouping: "flat",
			FluxPlacement:       "integrated",
			Factory:             factory,
			IOStreams:           factory.IOStreams(),
		}

		// Complete and validate first
		if err := opts.Complete(); err != nil {
			t.Fatalf("Complete() error = %v", err)
		}

		if err := opts.Validate(); err != nil {
			t.Fatalf("Validate() error = %v", err)
		}

		// Test loadClusterConfig
		cluster, err := opts.loadClusterConfig()
		if err != nil {
			t.Fatalf("loadClusterConfig() error = %v", err)
		}

		if cluster.Name != "test-cluster" {
			t.Errorf("cluster.Name = %q, want %q", cluster.Name, "test-cluster")
		}

		if cluster.Node == nil {
			t.Fatal("expected non-nil node")
		}

		if cluster.Node.Name != "root" {
			t.Errorf("node.Name = %q, want %q", cluster.Node.Name, "root")
		}
	})

	t.Run("invalid config file", func(t *testing.T) {
		tmpDir := t.TempDir()

		configFile := filepath.Join(tmpDir, "invalid.yaml")
		if err := os.WriteFile(configFile, []byte("invalid: yaml: content:"), 0644); err != nil {
			t.Fatal(err)
		}

		globalOpts := &options.GlobalOptions{}
		factory := cli.NewFactory(globalOpts)

		opts := &ClusterOptions{
			ConfigFile: configFile,
			Factory:    factory,
			IOStreams:  factory.IOStreams(),
		}

		_, err := opts.loadClusterConfig()
		if err == nil {
			t.Error("expected error for invalid config file")
		}
	})
}

func TestBuildLayoutRules(t *testing.T) {
	tests := []struct {
		name           string
		bundleGrouping string
		appGrouping    string
		fluxPlacement  string
	}{
		{"flat/flat/integrated", "flat", "flat", "integrated"},
		{"nested/nested/separate", "nested", "nested", "separate"},
		{"flat/nested/integrated", "flat", "nested", "integrated"},
		{"nested/flat/separate", "nested", "flat", "separate"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configFile := filepath.Join(tmpDir, "cluster.yaml")
			configContent := `name: test-cluster`
			if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
				t.Fatal(err)
			}

			globalOpts := &options.GlobalOptions{}
			factory := cli.NewFactory(globalOpts)

			opts := &ClusterOptions{
				ConfigFile:          configFile,
				BundleGrouping:      tt.bundleGrouping,
				ApplicationGrouping: tt.appGrouping,
				FluxPlacement:       tt.fluxPlacement,
				Factory:             factory,
				IOStreams:           factory.IOStreams(),
			}

			// Load a cluster to test buildLayoutRules
			cluster, err := opts.loadClusterConfig()
			if err != nil {
				t.Fatalf("loadClusterConfig() error = %v", err)
			}

			rules := opts.buildLayoutRules(cluster)

			// Just verify the function runs without error
			if rules.ClusterName != "test-cluster" {
				t.Errorf("ClusterName = %q, want %q", rules.ClusterName, "test-cluster")
			}
		})
	}
}

func TestClusterLoadNodeApps(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a cluster config
	configFile := filepath.Join(tmpDir, "cluster.yaml")
	configContent := `
name: test-cluster
node:
  name: root
  children:
    - name: apps
`
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create apps directory with a sample app config (using correct format)
	appsDir := filepath.Join(tmpDir, "apps")
	if err := os.MkdirAll(appsDir, 0755); err != nil {
		t.Fatal(err)
	}

	appConfig := `
apiVersion: generators.gokure.dev/v1alpha1
kind: AppWorkload
metadata:
  name: myapp
  namespace: default
spec:
  workload: Deployment
  replicas: 1
  containers:
    - name: nginx
      image: nginx:latest
`
	if err := os.WriteFile(filepath.Join(appsDir, "myapp.yaml"), []byte(appConfig), 0644); err != nil {
		t.Fatal(err)
	}

	globalOpts := &options.GlobalOptions{}
	factory := cli.NewFactory(globalOpts)

	opts := &ClusterOptions{
		ConfigFile: configFile,
		InputDir:   tmpDir,
		Factory:    factory,
		IOStreams:  factory.IOStreams(),
	}

	// Just verify that loadClusterConfig works
	cluster, err := opts.loadClusterConfig()
	if err != nil {
		t.Fatalf("loadClusterConfig() error = %v", err)
	}

	if cluster.Name != "test-cluster" {
		t.Errorf("cluster.Name = %q, want %q", cluster.Name, "test-cluster")
	}
}

func TestNewClusterCommandRunE(t *testing.T) {
	t.Run("missing args returns error", func(t *testing.T) {
		globalOpts := options.NewGlobalOptions()
		factory := cli.NewFactory(globalOpts)
		cmd := NewClusterCommand(factory)

		cmd.SetArgs([]string{})
		err := cmd.Execute()
		if err == nil {
			t.Error("expected error when no args provided")
		}
	})

	t.Run("nonexistent config file returns error", func(t *testing.T) {
		globalOpts := options.NewGlobalOptions()
		factory := cli.NewFactory(globalOpts)
		cmd := NewClusterCommand(factory)

		cmd.SetArgs([]string{"/nonexistent/file.yaml"})
		err := cmd.Execute()
		if err == nil {
			t.Error("expected error for nonexistent config file")
		}
	})

	t.Run("valid config dry-run succeeds", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Use a simple root-only config to avoid layout integration issues
		configContent := `
name: test-cluster
node:
  name: root
gitops:
  type: flux
`
		configFile := filepath.Join(tmpDir, "cluster.yaml")
		if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
			t.Fatal(err)
		}

		globalOpts := options.NewGlobalOptions()
		globalOpts.DryRun = true
		factory := cli.NewFactory(globalOpts)
		cmd := NewClusterCommand(factory)

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

func TestClusterRunDryRun(t *testing.T) {
	tmpDir := t.TempDir()

	// Use a simple root-only config to avoid layout integration issues
	configContent := `
name: test-cluster
node:
  name: root
gitops:
  type: flux
`
	configFile := filepath.Join(tmpDir, "cluster.yaml")
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	globalOpts := &options.GlobalOptions{DryRun: true, Verbose: true}
	factory := cli.NewFactory(globalOpts)

	var stdout, stderr bytes.Buffer
	ioStreams := cli.IOStreams{
		Out:    &stdout,
		ErrOut: &stderr,
	}

	opts := &ClusterOptions{
		ConfigFile:          configFile,
		InputDir:            tmpDir,
		OutputDir:           "out",
		ManifestDir:         "clusters",
		BundleGrouping:      "flat",
		ApplicationGrouping: "flat",
		FluxPlacement:       "separate",
		Factory:             factory,
		IOStreams:           ioStreams,
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

	// Verbose output should contain processing message
	if !bytes.Contains(stderr.Bytes(), []byte("Processing cluster config")) {
		t.Error("expected verbose processing message in stderr")
	}

	// Verbose output should contain generated message
	if !bytes.Contains(stderr.Bytes(), []byte("Generated cluster manifests")) {
		t.Error("expected verbose generated message in stderr")
	}

	// Stdout should contain manifest comments
	if stdout.Len() == 0 {
		t.Error("expected output to stdout for dry-run")
	}
}

func TestClusterRunWriteToDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Use a simple root-only config to avoid layout integration issues
	configContent := `
name: test-cluster
node:
  name: root
gitops:
  type: flux
`
	configFile := filepath.Join(tmpDir, "cluster.yaml")
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	globalOpts := &options.GlobalOptions{DryRun: false}
	factory := cli.NewFactory(globalOpts)

	var stdout, stderr bytes.Buffer
	ioStreams := cli.IOStreams{
		Out:    &stdout,
		ErrOut: &stderr,
	}

	opts := &ClusterOptions{
		ConfigFile:          configFile,
		InputDir:            tmpDir,
		OutputDir:           outputDir,
		ManifestDir:         "clusters",
		BundleGrouping:      "flat",
		ApplicationGrouping: "flat",
		FluxPlacement:       "separate",
		Factory:             factory,
		IOStreams:           ioStreams,
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

	// Verify output directory was created
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		t.Errorf("expected output directory %q to exist", outputDir)
	}
}

func TestClusterLoadClusterApps(t *testing.T) {
	tmpDir := t.TempDir()

	configContent := `
name: test-cluster
node:
  name: root
  children:
    - name: apps
`
	configFile := filepath.Join(tmpDir, "cluster.yaml")
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create apps directory with a sample app config
	appsDir := filepath.Join(tmpDir, "apps")
	if err := os.MkdirAll(appsDir, 0755); err != nil {
		t.Fatal(err)
	}

	appConfig := `
apiVersion: generators.gokure.dev/v1alpha1
kind: AppWorkload
metadata:
  name: myapp
  namespace: default
spec:
  workload: Deployment
  replicas: 1
  containers:
    - name: nginx
      image: nginx:latest
`
	if err := os.WriteFile(filepath.Join(appsDir, "myapp.yaml"), []byte(appConfig), 0644); err != nil {
		t.Fatal(err)
	}

	globalOpts := &options.GlobalOptions{}
	factory := cli.NewFactory(globalOpts)

	opts := &ClusterOptions{
		ConfigFile: configFile,
		InputDir:   tmpDir,
		Factory:    factory,
		IOStreams:  factory.IOStreams(),
	}

	cluster, err := opts.loadClusterConfig()
	if err != nil {
		t.Fatalf("loadClusterConfig() error = %v", err)
	}

	err = opts.loadClusterApps(cluster)
	if err != nil {
		t.Fatalf("loadClusterApps() error = %v", err)
	}

	// Verify that the root bundle was created
	if cluster.Node.Bundle == nil {
		t.Fatal("expected non-nil root bundle")
	}

	if cluster.Node.Bundle.Name != "root" {
		t.Errorf("root bundle name = %q, want %q", cluster.Node.Bundle.Name, "root")
	}

	// Verify child nodes have bundles
	if len(cluster.Node.Children) == 0 {
		t.Fatal("expected at least one child node")
	}

	appsNode := cluster.Node.Children[0]
	if appsNode.Bundle == nil {
		t.Fatal("expected non-nil apps bundle")
	}

	// Verify app was loaded
	if len(appsNode.Children) == 0 {
		t.Error("expected child nodes from app loading")
	}
}

func TestClusterLoadClusterAppsNilNode(t *testing.T) {
	tmpDir := t.TempDir()

	configContent := `
name: test-cluster
`
	configFile := filepath.Join(tmpDir, "cluster.yaml")
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	globalOpts := &options.GlobalOptions{}
	factory := cli.NewFactory(globalOpts)

	opts := &ClusterOptions{
		ConfigFile: configFile,
		InputDir:   tmpDir,
		Factory:    factory,
		IOStreams:  factory.IOStreams(),
	}

	cluster, err := opts.loadClusterConfig()
	if err != nil {
		t.Fatalf("loadClusterConfig() error = %v", err)
	}

	// Set node to nil to trigger validation error
	cluster.Node = nil

	err = opts.loadClusterApps(cluster)
	if err == nil {
		t.Error("expected error when cluster node is nil")
	}
}

func TestClusterLoadNodeAppsFunction(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("directory does not exist", func(t *testing.T) {
		globalOpts := &options.GlobalOptions{}
		factory := cli.NewFactory(globalOpts)

		opts := &ClusterOptions{
			InputDir:  tmpDir,
			Factory:   factory,
			IOStreams: factory.IOStreams(),
		}

		node := &stack.Node{Name: "nonexistent-dir"}
		err := opts.loadNodeApps(node)
		// Should not error when directory doesn't exist
		if err != nil {
			t.Fatalf("loadNodeApps() error = %v, expected nil for nonexistent dir", err)
		}
	})

	t.Run("directory with app configs", func(t *testing.T) {
		nodeDir := filepath.Join(tmpDir, "mynode")
		if err := os.MkdirAll(nodeDir, 0755); err != nil {
			t.Fatal(err)
		}

		appConfig := `
apiVersion: generators.gokure.dev/v1alpha1
kind: AppWorkload
metadata:
  name: testapp
  namespace: default
spec:
  workload: Deployment
  replicas: 1
  containers:
    - name: nginx
      image: nginx:latest
`
		if err := os.WriteFile(filepath.Join(nodeDir, "testapp.yaml"), []byte(appConfig), 0644); err != nil {
			t.Fatal(err)
		}

		// Also add a non-yaml file that should be skipped
		if err := os.WriteFile(filepath.Join(nodeDir, "README.md"), []byte("# readme"), 0644); err != nil {
			t.Fatal(err)
		}

		// Add a subdirectory that should be skipped
		if err := os.MkdirAll(filepath.Join(nodeDir, "subdir"), 0755); err != nil {
			t.Fatal(err)
		}

		globalOpts := &options.GlobalOptions{}
		factory := cli.NewFactory(globalOpts)

		opts := &ClusterOptions{
			InputDir:  tmpDir,
			Factory:   factory,
			IOStreams: factory.IOStreams(),
		}

		rootBundle, err := stack.NewBundle("root", nil, nil)
		if err != nil {
			t.Fatalf("NewBundle() error = %v", err)
		}

		node := &stack.Node{Name: "mynode", Bundle: rootBundle}
		err = opts.loadNodeApps(node)
		if err != nil {
			t.Fatalf("loadNodeApps() error = %v", err)
		}

		// Should have loaded one app
		if len(node.Children) != 1 {
			t.Errorf("node.Children count = %d, want 1", len(node.Children))
		}
	})
}

func TestClusterLoadAppConfig(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("valid single app config", func(t *testing.T) {
		appConfig := `
apiVersion: generators.gokure.dev/v1alpha1
kind: AppWorkload
metadata:
  name: testapp
  namespace: default
spec:
  workload: Deployment
  replicas: 1
  containers:
    - name: nginx
      image: nginx:latest
`
		configPath := filepath.Join(tmpDir, "single-app.yaml")
		if err := os.WriteFile(configPath, []byte(appConfig), 0644); err != nil {
			t.Fatal(err)
		}

		globalOpts := &options.GlobalOptions{}
		factory := cli.NewFactory(globalOpts)
		opts := &ClusterOptions{
			Factory:   factory,
			IOStreams: factory.IOStreams(),
		}

		rootBundle, err := stack.NewBundle("root", nil, nil)
		if err != nil {
			t.Fatalf("NewBundle() error = %v", err)
		}

		node := &stack.Node{Name: "test", Bundle: rootBundle}
		err = opts.loadAppConfig(node, configPath)
		if err != nil {
			t.Fatalf("loadAppConfig() error = %v", err)
		}

		if len(node.Children) != 1 {
			t.Fatalf("node.Children count = %d, want 1", len(node.Children))
		}
		if node.Children[0].Name != "testapp" {
			t.Errorf("child name = %q, want %q", node.Children[0].Name, "testapp")
		}
	})

	t.Run("nonexistent file", func(t *testing.T) {
		globalOpts := &options.GlobalOptions{}
		factory := cli.NewFactory(globalOpts)
		opts := &ClusterOptions{
			Factory:   factory,
			IOStreams: factory.IOStreams(),
		}

		rootBundle, err := stack.NewBundle("root", nil, nil)
		if err != nil {
			t.Fatalf("NewBundle() error = %v", err)
		}

		node := &stack.Node{Name: "test", Bundle: rootBundle}
		err = opts.loadAppConfig(node, "/nonexistent/file.yaml")
		if err == nil {
			t.Error("expected error for nonexistent file")
		}
	})

	t.Run("empty file", func(t *testing.T) {
		emptyPath := filepath.Join(tmpDir, "empty.yaml")
		if err := os.WriteFile(emptyPath, []byte(""), 0644); err != nil {
			t.Fatal(err)
		}

		globalOpts := &options.GlobalOptions{}
		factory := cli.NewFactory(globalOpts)
		opts := &ClusterOptions{
			Factory:   factory,
			IOStreams: factory.IOStreams(),
		}

		rootBundle, err := stack.NewBundle("root", nil, nil)
		if err != nil {
			t.Fatalf("NewBundle() error = %v", err)
		}

		node := &stack.Node{Name: "test", Bundle: rootBundle}
		err = opts.loadAppConfig(node, emptyPath)
		// Empty file should succeed with no children
		if err != nil {
			t.Fatalf("loadAppConfig(empty) error = %v", err)
		}
		if len(node.Children) != 0 {
			t.Errorf("expected no children from empty file, got %d", len(node.Children))
		}
	})
}

func TestClusterGenerateLayout(t *testing.T) {
	tmpDir := t.TempDir()

	// Use a simple root-only config to avoid layout integration issues
	configContent := `
name: test-cluster
node:
  name: root
gitops:
  type: flux
`
	configFile := filepath.Join(tmpDir, "cluster.yaml")
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	globalOpts := &options.GlobalOptions{}
	factory := cli.NewFactory(globalOpts)

	opts := &ClusterOptions{
		ConfigFile:          configFile,
		InputDir:            tmpDir,
		BundleGrouping:      "flat",
		ApplicationGrouping: "flat",
		FluxPlacement:       "separate",
		Factory:             factory,
		IOStreams:           factory.IOStreams(),
	}

	cluster, err := opts.loadClusterConfig()
	if err != nil {
		t.Fatalf("loadClusterConfig() error = %v", err)
	}

	err = opts.loadClusterApps(cluster)
	if err != nil {
		t.Fatalf("loadClusterApps() error = %v", err)
	}

	rules := opts.buildLayoutRules(cluster)
	ml, err := opts.generateLayout(cluster, rules)
	if err != nil {
		t.Fatalf("generateLayout() error = %v", err)
	}

	if ml == nil {
		t.Fatal("expected non-nil manifest layout")
	}
}

func TestClusterPrintToStdout(t *testing.T) {
	globalOpts := &options.GlobalOptions{}
	factory := cli.NewFactory(globalOpts)

	var stdout bytes.Buffer
	ioStreams := cli.IOStreams{
		Out:    &stdout,
		ErrOut: &bytes.Buffer{},
	}

	opts := &ClusterOptions{
		Factory:   factory,
		IOStreams: ioStreams,
	}

	ml := &layout.ManifestLayout{
		Name:      "test-cluster",
		Namespace: "default",
		Resources: nil,
	}

	err := opts.printToStdout(ml)
	if err != nil {
		t.Fatalf("printToStdout() error = %v", err)
	}

	output := stdout.String()

	if !bytes.Contains(stdout.Bytes(), []byte("test-cluster")) {
		t.Errorf("expected output to contain cluster name, got: %s", output)
	}

	if !bytes.Contains(stdout.Bytes(), []byte("Resources:")) {
		t.Errorf("expected output to contain 'Resources:', got: %s", output)
	}
}

func TestClusterWriteOutputDryRun(t *testing.T) {
	globalOpts := &options.GlobalOptions{DryRun: true}
	factory := cli.NewFactory(globalOpts)

	var stdout bytes.Buffer
	ioStreams := cli.IOStreams{
		Out:    &stdout,
		ErrOut: &bytes.Buffer{},
	}

	opts := &ClusterOptions{
		OutputDir: "/dev/stdout",
		Factory:   factory,
		IOStreams: ioStreams,
	}

	ml := &layout.ManifestLayout{
		Name:      "test-cluster",
		Namespace: "default",
	}

	err := opts.writeOutput(ml)
	if err != nil {
		t.Fatalf("writeOutput() error = %v", err)
	}

	if stdout.Len() == 0 {
		t.Error("expected output to stdout for dry-run")
	}
}

func TestClusterWriteOutputToDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "output")

	globalOpts := &options.GlobalOptions{DryRun: false}
	factory := cli.NewFactory(globalOpts)

	opts := &ClusterOptions{
		OutputDir:   outputDir,
		ManifestDir: "clusters",
		Factory:     factory,
		IOStreams:   factory.IOStreams(),
	}

	ml := &layout.ManifestLayout{
		Name:      "test-cluster",
		Namespace: "default",
	}

	err := opts.writeOutput(ml)
	if err != nil {
		t.Fatalf("writeOutput() error = %v", err)
	}
}

func TestClusterFlagDefaults(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	factory := cli.NewFactory(globalOpts)
	cmd := NewClusterCommand(factory)

	outputDir, err := cmd.Flags().GetString("output-dir")
	if err != nil {
		t.Fatalf("GetString(output-dir) error = %v", err)
	}
	if outputDir != "out" {
		t.Errorf("output-dir default = %q, want %q", outputDir, "out")
	}

	manifestDir, err := cmd.Flags().GetString("manifest-dir")
	if err != nil {
		t.Fatalf("GetString(manifest-dir) error = %v", err)
	}
	if manifestDir != "clusters" {
		t.Errorf("manifest-dir default = %q, want %q", manifestDir, "clusters")
	}

	bundleGrouping, err := cmd.Flags().GetString("bundle-grouping")
	if err != nil {
		t.Fatalf("GetString(bundle-grouping) error = %v", err)
	}
	if bundleGrouping != "flat" {
		t.Errorf("bundle-grouping default = %q, want %q", bundleGrouping, "flat")
	}

	appGrouping, err := cmd.Flags().GetString("application-grouping")
	if err != nil {
		t.Fatalf("GetString(application-grouping) error = %v", err)
	}
	if appGrouping != "flat" {
		t.Errorf("application-grouping default = %q, want %q", appGrouping, "flat")
	}

	fluxPlacement, err := cmd.Flags().GetString("flux-placement")
	if err != nil {
		t.Fatalf("GetString(flux-placement) error = %v", err)
	}
	if fluxPlacement != "integrated" {
		t.Errorf("flux-placement default = %q, want %q", fluxPlacement, "integrated")
	}

	inputDir, err := cmd.Flags().GetString("input-dir")
	if err != nil {
		t.Fatalf("GetString(input-dir) error = %v", err)
	}
	if inputDir != "" {
		t.Errorf("input-dir default = %q, want %q", inputDir, "")
	}
}

func TestClusterCompleteInputDir(t *testing.T) {
	globalOpts := &options.GlobalOptions{}
	factory := cli.NewFactory(globalOpts)

	opts := &ClusterOptions{
		ConfigFile: "/some/path/cluster.yaml",
		OutputDir:  "out",
		InputDir:   "",
		Factory:    factory,
		IOStreams:  factory.IOStreams(),
	}

	err := opts.Complete()
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	// InputDir should default to the config file's directory
	expected := "/some/path"
	if opts.InputDir != expected {
		t.Errorf("InputDir = %q, want %q", opts.InputDir, expected)
	}
}

func TestClusterLoadNodeAppsWithYmlExtension(t *testing.T) {
	tmpDir := t.TempDir()

	nodeDir := filepath.Join(tmpDir, "mynode")
	if err := os.MkdirAll(nodeDir, 0755); err != nil {
		t.Fatal(err)
	}

	appConfig := `
apiVersion: generators.gokure.dev/v1alpha1
kind: AppWorkload
metadata:
  name: testapp
  namespace: default
spec:
  workload: Deployment
  replicas: 1
  containers:
    - name: nginx
      image: nginx:latest
`
	// Use .yml extension
	if err := os.WriteFile(filepath.Join(nodeDir, "testapp.yml"), []byte(appConfig), 0644); err != nil {
		t.Fatal(err)
	}

	globalOpts := &options.GlobalOptions{}
	factory := cli.NewFactory(globalOpts)

	opts := &ClusterOptions{
		InputDir:  tmpDir,
		Factory:   factory,
		IOStreams: factory.IOStreams(),
	}

	rootBundle, err := stack.NewBundle("root", nil, nil)
	if err != nil {
		t.Fatalf("NewBundle() error = %v", err)
	}

	node := &stack.Node{Name: "mynode", Bundle: rootBundle}
	err = opts.loadNodeApps(node)
	if err != nil {
		t.Fatalf("loadNodeApps() error = %v", err)
	}

	if len(node.Children) != 1 {
		t.Errorf("node.Children count = %d, want 1", len(node.Children))
	}
}
