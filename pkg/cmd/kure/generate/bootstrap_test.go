package generate

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-kure/kure/pkg/cli"
	"github.com/go-kure/kure/pkg/cmd/shared/options"
	"github.com/go-kure/kure/pkg/stack/layout"
)

func TestNewBootstrapCommand(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	factory := cli.NewFactory(globalOpts)
	cmd := NewBootstrapCommand(factory)

	if cmd == nil {
		t.Fatal("expected non-nil bootstrap command")
	}

	if extractCommandName(cmd.Use) != "bootstrap" {
		t.Errorf("expected command name 'bootstrap', got %s", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("expected non-empty short description")
	}

	if cmd.Long == "" {
		t.Error("expected non-empty long description")
	}
}

func TestBootstrapOptionsAddFlags(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	factory := cli.NewFactory(globalOpts)
	cmd := NewBootstrapCommand(factory)

	// Check that expected flags exist
	flags := []string{"output-dir", "manifest-dir", "gitops-type", "flux-mode"}
	for _, flag := range flags {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("expected flag %q to exist", flag)
		}
	}
}

func TestBootstrapOptionsComplete(t *testing.T) {
	tests := []struct {
		name         string
		opts         *BootstrapOptions
		globalOpts   *options.GlobalOptions
		expectOutDir string
	}{
		{
			name: "default options",
			opts: &BootstrapOptions{
				ConfigFile: "/tmp/bootstrap.yaml",
				OutputDir:  "out/bootstrap",
			},
			globalOpts:   &options.GlobalOptions{DryRun: false},
			expectOutDir: "out/bootstrap",
		},
		{
			name: "dry-run with default output",
			opts: &BootstrapOptions{
				ConfigFile: "/tmp/bootstrap.yaml",
				OutputDir:  "out/bootstrap",
			},
			globalOpts:   &options.GlobalOptions{DryRun: true},
			expectOutDir: "/dev/stdout",
		},
		{
			name: "dry-run with custom output",
			opts: &BootstrapOptions{
				ConfigFile: "/tmp/bootstrap.yaml",
				OutputDir:  "custom-out",
			},
			globalOpts:   &options.GlobalOptions{DryRun: true},
			expectOutDir: "custom-out",
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

func TestBootstrapOptionsValidate(t *testing.T) {
	// Create a temp config file
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "bootstrap.yaml")
	if err := os.WriteFile(configFile, []byte("name: test"), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		opts    *BootstrapOptions
		wantErr bool
	}{
		{
			name: "valid options",
			opts: &BootstrapOptions{
				ConfigFile: configFile,
				GitOpsType: "",
				FluxMode:   "",
			},
			wantErr: false,
		},
		{
			name: "valid with flux type",
			opts: &BootstrapOptions{
				ConfigFile: configFile,
				GitOpsType: "flux",
				FluxMode:   "",
			},
			wantErr: false,
		},
		{
			name: "valid with argocd type",
			opts: &BootstrapOptions{
				ConfigFile: configFile,
				GitOpsType: "argocd",
			},
			wantErr: false,
		},
		{
			name: "valid with flux modes",
			opts: &BootstrapOptions{
				ConfigFile: configFile,
				GitOpsType: "flux",
				FluxMode:   "operator",
			},
			wantErr: false,
		},
		{
			name: "valid flux toolkit mode",
			opts: &BootstrapOptions{
				ConfigFile: configFile,
				GitOpsType: "flux",
				FluxMode:   "toolkit",
			},
			wantErr: false,
		},
		{
			name: "missing config file",
			opts: &BootstrapOptions{
				ConfigFile: "/nonexistent/file.yaml",
			},
			wantErr: true,
		},
		{
			name: "invalid gitops type",
			opts: &BootstrapOptions{
				ConfigFile: configFile,
				GitOpsType: "invalid",
			},
			wantErr: true,
		},
		{
			name: "invalid flux mode",
			opts: &BootstrapOptions{
				ConfigFile: configFile,
				GitOpsType: "flux",
				FluxMode:   "invalid",
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

func TestBootstrapDetectGitOpsSettings(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name           string
		configContent  string
		opts           *BootstrapOptions
		expectGitOps   string
		expectFluxMode string
	}{
		{
			name: "detect flux from config",
			configContent: `
name: test-cluster
gitops:
  type: flux
  bootstrap:
    fluxMode: operator
`,
			opts:           &BootstrapOptions{GitOpsType: "", FluxMode: ""},
			expectGitOps:   "flux",
			expectFluxMode: "operator",
		},
		{
			name: "detect argocd from config",
			configContent: `
name: test-cluster
gitops:
  type: argocd
`,
			opts:           &BootstrapOptions{GitOpsType: "", FluxMode: ""},
			expectGitOps:   "argocd",
			expectFluxMode: "",
		},
		{
			name: "default to flux when not specified",
			configContent: `
name: test-cluster
`,
			opts:           &BootstrapOptions{GitOpsType: "", FluxMode: ""},
			expectGitOps:   "flux",
			expectFluxMode: "operator",
		},
		{
			name: "command line overrides config",
			configContent: `
name: test-cluster
gitops:
  type: argocd
`,
			opts:           &BootstrapOptions{GitOpsType: "flux", FluxMode: "toolkit"},
			expectGitOps:   "flux",
			expectFluxMode: "toolkit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configFile := filepath.Join(tmpDir, tt.name+".yaml")
			if err := os.WriteFile(configFile, []byte(tt.configContent), 0644); err != nil {
				t.Fatal(err)
			}

			globalOpts := &options.GlobalOptions{}
			factory := cli.NewFactory(globalOpts)

			tt.opts.ConfigFile = configFile
			tt.opts.Factory = factory
			tt.opts.IOStreams = factory.IOStreams()

			cluster, err := tt.opts.loadClusterConfig()
			if err != nil {
				t.Fatalf("loadClusterConfig() error = %v", err)
			}

			err = tt.opts.detectGitOpsSettings(cluster)
			if err != nil {
				t.Fatalf("detectGitOpsSettings() error = %v", err)
			}

			if tt.opts.GitOpsType != tt.expectGitOps {
				t.Errorf("GitOpsType = %q, want %q", tt.opts.GitOpsType, tt.expectGitOps)
			}

			if tt.opts.FluxMode != tt.expectFluxMode {
				t.Errorf("FluxMode = %q, want %q", tt.opts.FluxMode, tt.expectFluxMode)
			}
		})
	}
}

func TestBootstrapLoadClusterConfig(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("valid config", func(t *testing.T) {
		configContent := `
name: test-cluster
gitops:
  type: flux
  bootstrap:
    enabled: true
    fluxMode: operator
`
		configFile := filepath.Join(tmpDir, "valid.yaml")
		if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
			t.Fatal(err)
		}

		globalOpts := &options.GlobalOptions{}
		factory := cli.NewFactory(globalOpts)

		opts := &BootstrapOptions{
			ConfigFile: configFile,
			Factory:    factory,
			IOStreams:  factory.IOStreams(),
		}

		cluster, err := opts.loadClusterConfig()
		if err != nil {
			t.Fatalf("loadClusterConfig() error = %v", err)
		}

		if cluster.Name != "test-cluster" {
			t.Errorf("cluster.Name = %q, want %q", cluster.Name, "test-cluster")
		}

		if cluster.GitOps == nil {
			t.Fatal("expected non-nil GitOps")
		}

		if cluster.GitOps.Type != "flux" {
			t.Errorf("GitOps.Type = %q, want %q", cluster.GitOps.Type, "flux")
		}
	})

	t.Run("minimal config creates default node", func(t *testing.T) {
		configContent := `
name: minimal-cluster
`
		configFile := filepath.Join(tmpDir, "minimal.yaml")
		if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
			t.Fatal(err)
		}

		globalOpts := &options.GlobalOptions{}
		factory := cli.NewFactory(globalOpts)

		opts := &BootstrapOptions{
			ConfigFile: configFile,
			Factory:    factory,
			IOStreams:  factory.IOStreams(),
		}

		cluster, err := opts.loadClusterConfig()
		if err != nil {
			t.Fatalf("loadClusterConfig() error = %v", err)
		}

		// Should create default node
		if cluster.Node == nil {
			t.Fatal("expected non-nil Node")
		}

		if cluster.Node.Name != "flux-system" {
			t.Errorf("Node.Name = %q, want %q", cluster.Node.Name, "flux-system")
		}

		// Should create default bundle
		if cluster.Node.Bundle == nil {
			t.Fatal("expected non-nil Bundle")
		}

		if cluster.Node.Bundle.Name != "infrastructure" {
			t.Errorf("Bundle.Name = %q, want %q", cluster.Node.Bundle.Name, "infrastructure")
		}
	})
}

func TestBootstrapRun(t *testing.T) {
	t.Run("dry-run flux bootstrap", func(t *testing.T) {
		tmpDir := t.TempDir()

		configContent := `
name: test-cluster
node:
  name: flux-system
gitops:
  type: flux
  bootstrap:
    enabled: true
    fluxMode: flux-operator
`
		configFile := filepath.Join(tmpDir, "bootstrap.yaml")
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

		opts := &BootstrapOptions{
			ConfigFile: configFile,
			OutputDir:  "/dev/stdout",
			GitOpsType: "",
			FluxMode:   "",
			Factory:    factory,
			IOStreams:  ioStreams,
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

		// Verify something was printed
		if stdout.Len() == 0 {
			t.Error("expected output to stdout")
		}
	})
}

func TestBootstrapPrintToStdout(t *testing.T) {
	globalOpts := &options.GlobalOptions{}
	factory := cli.NewFactory(globalOpts)

	var stdout bytes.Buffer
	ioStreams := cli.IOStreams{
		Out:    &stdout,
		ErrOut: &bytes.Buffer{},
	}

	opts := &BootstrapOptions{
		GitOpsType: "flux",
		FluxMode:   "operator",
		Factory:    factory,
		IOStreams:  ioStreams,
	}

	// Create a real manifest layout
	ml := &layout.ManifestLayout{
		Name:      "test-cluster",
		Namespace: "flux-system",
		Resources: nil, // No resources for simplicity
	}

	err := opts.printToStdout(ml)
	if err != nil {
		t.Fatalf("printToStdout() error = %v", err)
	}

	output := stdout.String()

	// Check that output contains expected info
	if !bytes.Contains(stdout.Bytes(), []byte("test-cluster")) {
		t.Error("expected output to contain cluster name")
	}

	if !bytes.Contains(stdout.Bytes(), []byte("flux")) {
		t.Error("expected output to contain gitops type")
	}

	if !bytes.Contains(stdout.Bytes(), []byte("operator")) {
		t.Error("expected output to contain flux mode")
	}

	// Should contain resource count
	if !bytes.Contains(stdout.Bytes(), []byte("Resources:")) {
		t.Errorf("expected output to contain 'Resources:', got: %s", output)
	}
}
