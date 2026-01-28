package generate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-kure/kure/pkg/cli"
	"github.com/go-kure/kure/pkg/cmd/shared/options"
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
