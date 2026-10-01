package main

import (
	"testing"

	"github.com/go-kure/kure/internal/kuretest"
	"github.com/go-kure/kure/pkg/stack"
)

// TestRun exercises the complete getting-started pipeline end-to-end and
// holds every manifest it writes to what a cluster would accept.
func TestRun(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OUT_DIR", dir)
	if err := run(); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	kuretest.AssertValidDir(t, dir)
}

// TestRedisConfigGenerate exercises the RedisConfig.Generate method.
func TestRedisConfigGenerate(t *testing.T) {
	cfg := &RedisConfig{
		Namespace: "cache",
		Image:     "redis:7-alpine",
	}
	app := stack.NewApplication("redis", "cache", cfg)
	objs, err := cfg.Generate(app)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if len(objs) != 1 {
		t.Fatalf("expected 1 object, got %d", len(objs))
	}
}

// TestWebAppConfigGenerate exercises the WebAppConfig.Generate method.
func TestWebAppConfigGenerate(t *testing.T) {
	cfg := &WebAppConfig{
		Namespace: "web",
		Image:     "nginx:1.27-alpine",
		Replicas:  2,
		Port:      80,
	}
	app := stack.NewApplication("web-app", "web", cfg)
	objs, err := cfg.Generate(app)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if len(objs) != 2 {
		t.Fatalf("expected 2 objects (Deployment+Service), got %d", len(objs))
	}
}

// TestOutputDirectory_TempDir exercises the tempdir branch of outputDirectory.
func TestOutputDirectory_TempDir(t *testing.T) {
	t.Setenv("OUT_DIR", "")
	dir, err := outputDirectory()
	if err != nil {
		t.Fatalf("outputDirectory() error = %v", err)
	}
	if dir == "" {
		t.Fatal("expected non-empty directory path")
	}
}

// TestOutputDirectory_EnvVar exercises the OUT_DIR environment variable branch.
func TestOutputDirectory_EnvVar(t *testing.T) {
	tmpDir := t.TempDir()
	outDir := tmpDir + "/manifests"
	t.Setenv("OUT_DIR", outDir)

	dir, err := outputDirectory()
	if err != nil {
		t.Fatalf("outputDirectory() error = %v", err)
	}
	if dir != outDir {
		t.Errorf("expected dir %q, got %q", outDir, dir)
	}
}
