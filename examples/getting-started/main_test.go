package main

import (
	"maps"
	"path/filepath"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/internal/kuretest"
	"github.com/go-kure/kure/pkg/stack"
)

// TestRun exercises the complete getting-started pipeline end-to-end, holds
// every manifest it writes to what a cluster would accept, and holds the tree
// to itself: every reference, namespace and path resolves, and what Flux
// applies builds.
func TestRun(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OUT_DIR", dir)
	if err := run(); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	kuretest.AssertValidDir(t, dir)
	// The Kustomizations live in flux-system and name the OCIRepository the
	// bootstrap creates; the example creates every other namespace itself.
	kuretest.AssertConsistentDir(t, filepath.Join(dir, "clusters"),
		kuretest.Namespace("flux-system"),
		kuretest.External("OCIRepository", "flux-system", "manifests"))
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
	if len(objs) != 2 {
		t.Fatalf("expected 2 objects (Namespace+Deployment), got %d", len(objs))
	}
	if ns, ok := (*objs[0]).(*corev1.Namespace); !ok || ns.Name != "cache" {
		t.Fatalf("first object = %T %q, want the Namespace %q", *objs[0], (*objs[0]).GetName(), "cache")
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
	if len(objs) != 3 {
		t.Fatalf("expected 3 objects (Namespace+Deployment+Service), got %d", len(objs))
	}
	if ns, ok := (*objs[0]).(*corev1.Namespace); !ok || ns.Name != "web" {
		t.Fatalf("first object = %T %q, want the Namespace %q", *objs[0], (*objs[0]).GetName(), "web")
	}
}

// TestGenerate_SharesNothing: an edit of any map or pointer a config returned
// reaches no other field, no other object, no other call's objects and not
// the config.
func TestGenerate_SharesNothing(t *testing.T) {
	web := &WebAppConfig{Namespace: "web", Image: "nginx:1.27-alpine", Replicas: 2, Port: 80}
	redis := &RedisConfig{Namespace: "cache", Image: "redis:7-alpine"}
	for name, cfg := range map[string]stack.ApplicationConfig{"web": web, "redis": redis} {
		t.Run(name, func(t *testing.T) {
			app := stack.NewApplication(name, "default", cfg)
			first, err := cfg.Generate(app)
			if err != nil {
				t.Fatalf("Generate() error = %v", err)
			}
			second, err := cfg.Generate(app)
			if err != nil {
				t.Fatalf("Generate() error = %v", err)
			}
			want := map[string]string{"app": name}
			edited := labelMaps("first", first)
			unedited := labelMaps("second", second)
			if len(edited) == 0 || len(unedited) != len(edited) {
				t.Fatalf("found %d label maps in the first call and %d in the second", len(edited), len(unedited))
			}
			// Add a key naming the field to every map of the first call: each
			// must then hold its own key only, and every map of the second
			// call must keep its value.
			for field, m := range edited {
				m[field] = "edited"
			}
			dep := (*first[1]).(*appsv1.Deployment)
			*dep.Spec.Replicas = 9
			for field, got := range edited {
				exp := map[string]string{"app": name, field: "edited"}
				if !maps.Equal(got, exp) {
					t.Errorf("%s = %v after editing every map of the first call, want %v", field, got, exp)
				}
			}
			for field, got := range unedited {
				if !maps.Equal(got, want) {
					t.Errorf("%s = %v after editing every map of the first call, want %v", field, got, want)
				}
			}
			if *(*second[1]).(*appsv1.Deployment).Spec.Replicas == 9 {
				t.Error("editing one call's replicas changed the next call's")
			}
			if web.Replicas != 2 {
				t.Errorf("config replicas = %d after editing the Deployment's, want 2", web.Replicas)
			}
		})
	}
}

// labelMaps names every label map of the Deployments and Services in objs.
func labelMaps(call string, objs []*client.Object) map[string]map[string]string {
	out := map[string]map[string]string{}
	for _, o := range objs {
		switch obj := (*o).(type) {
		case *appsv1.Deployment:
			out[call+" Deployment labels"] = obj.Labels
			out[call+" Deployment selector"] = obj.Spec.Selector.MatchLabels
			out[call+" Deployment pod template labels"] = obj.Spec.Template.Labels
		case *corev1.Service:
			out[call+" Service labels"] = obj.Labels
			out[call+" Service selector"] = obj.Spec.Selector
		}
	}
	return out
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
