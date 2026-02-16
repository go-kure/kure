package initialize

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/go-kure/kure/pkg/cli"
	"github.com/go-kure/kure/pkg/cmd/shared/options"
	"github.com/go-kure/kure/pkg/stack"

	// Register generators so ApplicationWrapper can decode AppWorkload specs.
	_ "github.com/go-kure/kure/pkg/stack/generators/appworkload"
)

// newTestOptions creates InitOptions wired to captured IO buffers.
func newTestOptions(t *testing.T, projectName, dir, gitops string) (*InitOptions, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	globalOpts := options.NewGlobalOptions()
	factory := cli.NewFactory(globalOpts)

	var stdout, stderr bytes.Buffer
	streams := cli.IOStreams{
		Out:    &stdout,
		ErrOut: &stderr,
	}

	return &InitOptions{
		ProjectName: projectName,
		OutputDir:   dir,
		GitOpsType:  gitops,
		Factory:     factory,
		IOStreams:   streams,
	}, &stdout, &stderr
}

func TestInitCreatesProjectStructure(t *testing.T) {
	tmpDir := t.TempDir()

	o, _, _ := newTestOptions(t, "my-cluster", tmpDir, "flux")
	if err := o.Complete(); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if err := o.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if err := o.Run(); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// Verify files and directories exist
	for _, path := range []string{
		"cluster.yaml",
		"apps",
		"apps/example.yaml",
		"infra",
	} {
		full := filepath.Join(tmpDir, path)
		if _, err := os.Stat(full); os.IsNotExist(err) {
			t.Errorf("expected %s to exist", path)
		}
	}
}

func TestInitClusterYAMLIsValid(t *testing.T) {
	tmpDir := t.TempDir()

	o, _, _ := newTestOptions(t, "test-cluster", tmpDir, "flux")
	if err := o.Complete(); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if err := o.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if err := o.Run(); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// Parse cluster.yaml into stack.Cluster
	data, err := os.ReadFile(filepath.Join(tmpDir, "cluster.yaml"))
	if err != nil {
		t.Fatalf("failed to read cluster.yaml: %v", err)
	}

	var cluster stack.Cluster
	dec := yaml.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&cluster); err != nil {
		t.Fatalf("failed to decode cluster.yaml: %v", err)
	}

	if cluster.Name != "test-cluster" {
		t.Errorf("cluster.Name = %q, want %q", cluster.Name, "test-cluster")
	}
	if cluster.GitOps == nil {
		t.Fatal("cluster.GitOps is nil")
	}
	if cluster.GitOps.Type != "flux" {
		t.Errorf("cluster.GitOps.Type = %q, want %q", cluster.GitOps.Type, "flux")
	}
	if cluster.Node == nil {
		t.Fatal("cluster.Node is nil")
	}
	if cluster.Node.Name != "flux-system" {
		t.Errorf("cluster.Node.Name = %q, want %q", cluster.Node.Name, "flux-system")
	}
	if len(cluster.Node.Children) != 2 {
		t.Fatalf("len(cluster.Node.Children) = %d, want 2", len(cluster.Node.Children))
	}

	childNames := make(map[string]bool)
	for _, child := range cluster.Node.Children {
		childNames[child.Name] = true
	}
	for _, expected := range []string{"apps", "infra"} {
		if !childNames[expected] {
			t.Errorf("expected child node %q not found", expected)
		}
	}
}

func TestInitAppYAMLIsValid(t *testing.T) {
	tmpDir := t.TempDir()

	o, _, _ := newTestOptions(t, "my-cluster", tmpDir, "flux")
	if err := o.Complete(); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if err := o.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if err := o.Run(); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// Parse apps/example.yaml via ApplicationWrapper
	data, err := os.ReadFile(filepath.Join(tmpDir, "apps", "example.yaml"))
	if err != nil {
		t.Fatalf("failed to read apps/example.yaml: %v", err)
	}

	var wrapper stack.ApplicationWrapper
	dec := yaml.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&wrapper); err != nil {
		t.Fatalf("failed to decode apps/example.yaml: %v", err)
	}

	if wrapper.Metadata.Name != "example" {
		t.Errorf("metadata.name = %q, want %q", wrapper.Metadata.Name, "example")
	}
	if wrapper.Metadata.Namespace != "apps" {
		t.Errorf("metadata.namespace = %q, want %q", wrapper.Metadata.Namespace, "apps")
	}

	// Verify ToApplication succeeds
	app := wrapper.ToApplication()
	if app == nil {
		t.Fatal("ToApplication() returned nil")
	}
	if app.Name != "example" {
		t.Errorf("app.Name = %q, want %q", app.Name, "example")
	}
}

func TestInitRefusesOverwrite(t *testing.T) {
	tmpDir := t.TempDir()

	// First run succeeds
	o1, _, _ := newTestOptions(t, "my-cluster", tmpDir, "flux")
	if err := o1.Complete(); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if err := o1.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if err := o1.Run(); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// Second run must fail at Validate
	o2, _, _ := newTestOptions(t, "my-cluster", tmpDir, "flux")
	if err := o2.Complete(); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	err := o2.Validate()
	if err == nil {
		t.Fatal("expected error on second init, got nil")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' in error, got %q", err.Error())
	}
}

func TestInitValidatesProjectName(t *testing.T) {
	tests := []struct {
		name    string
		project string
		wantErr bool
	}{
		{"valid lowercase", "my-cluster", false},
		{"valid single word", "cluster", false},
		{"valid with numbers", "cluster-01", false},
		{"uppercase rejected", "My-Cluster", true},
		{"underscores rejected", "my_cluster", true},
		{"dots rejected", "my.cluster", true},
		{"starts with hyphen", "-cluster", true},
		{"ends with hyphen", "cluster-", true},
		{"empty string", "", true},
		{"spaces rejected", "my cluster", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			o, _, _ := newTestOptions(t, tt.project, tmpDir, "flux")
			// Skip Complete to test Validate directly with the exact project name
			o.OutputDir = tmpDir
			err := o.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestInitValidatesGitOpsType(t *testing.T) {
	tests := []struct {
		name    string
		gitops  string
		wantErr bool
	}{
		{"flux accepted", "flux", false},
		{"argocd accepted", "argocd", false},
		{"invalid rejected", "invalid", true},
		{"empty rejected", "", true},
		{"capitalize rejected", "Flux", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			o, _, _ := newTestOptions(t, "my-cluster", tmpDir, tt.gitops)
			o.OutputDir = tmpDir
			err := o.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestInitArgoCD(t *testing.T) {
	tmpDir := t.TempDir()

	o, _, _ := newTestOptions(t, "my-cluster", tmpDir, "argocd")
	if err := o.Complete(); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if err := o.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if err := o.Run(); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, "cluster.yaml"))
	if err != nil {
		t.Fatalf("failed to read cluster.yaml: %v", err)
	}

	var cluster stack.Cluster
	dec := yaml.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&cluster); err != nil {
		t.Fatalf("failed to decode cluster.yaml: %v", err)
	}

	if cluster.GitOps == nil {
		t.Fatal("cluster.GitOps is nil")
	}
	if cluster.GitOps.Type != "argocd" {
		t.Errorf("cluster.GitOps.Type = %q, want %q", cluster.GitOps.Type, "argocd")
	}
}

func TestInitCompleteDefaultsProjectName(t *testing.T) {
	tmpDir := t.TempDir()

	o, _, _ := newTestOptions(t, "", tmpDir, "flux")
	if err := o.Complete(); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	expected := filepath.Base(tmpDir)
	if o.ProjectName != expected {
		t.Errorf("ProjectName = %q, want %q", o.ProjectName, expected)
	}
}

func TestInitCompleteResolvesAbsolutePath(t *testing.T) {
	o, _, _ := newTestOptions(t, "test", ".", "flux")
	if err := o.Complete(); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	if !filepath.IsAbs(o.OutputDir) {
		t.Errorf("OutputDir %q is not absolute", o.OutputDir)
	}
}

func TestNewInitCommand(t *testing.T) {
	globalOpts := options.NewGlobalOptions()
	cmd := NewInitCommand(globalOpts)

	if cmd == nil {
		t.Fatal("expected non-nil init command")
	}

	if cmd.Use != "init [PROJECT_NAME] [flags]" {
		t.Errorf("unexpected Use: %s", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("expected non-empty short description")
	}

	// Check flags exist
	for _, flag := range []string{"dir", "gitops"} {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("expected flag %q to exist", flag)
		}
	}
}

func TestInitSummaryOutput(t *testing.T) {
	tmpDir := t.TempDir()

	o, _, stderr := newTestOptions(t, "my-cluster", tmpDir, "flux")
	if err := o.Complete(); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if err := o.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if err := o.Run(); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	output := stderr.String()
	for _, expected := range []string{
		"Initialized kure project",
		"cluster.yaml",
		"apps/example.yaml",
		"kure generate cluster",
	} {
		if !strings.Contains(output, expected) {
			t.Errorf("expected stderr to contain %q, got:\n%s", expected, output)
		}
	}
}
