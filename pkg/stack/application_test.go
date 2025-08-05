package stack

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/kubernetes"
)

// TestNewApplicationAndSetters verifies constructor and setter behaviour.
func TestNewApplicationAndSetters(t *testing.T) {
	cfg1 := &fakeConfig{}
	app := NewApplication("name", "ns", cfg1)
	if app.Name != "name" || app.Namespace != "ns" || app.Config != cfg1 {
		t.Fatalf("application fields not set correctly: %#v", app)
	}

	app.SetName("new-name")
	if app.Name != "new-name" {
		t.Errorf("SetName did not update name")
	}
	app.SetNamespace("new-ns")
	if app.Namespace != "new-ns" {
		t.Errorf("SetNamespace did not update namespace")
	}

	cfg2 := &fakeConfig{}
	app.SetConfig(cfg2)
	if app.Config != cfg2 {
		t.Errorf("SetConfig did not replace config")
	}
}

// TestGenerate exercises the Generate method with and without configuration.
func TestGenerate(t *testing.T) {
	app := NewApplication("app", "ns", nil)
	if _, err := app.Generate(); err == nil {
		t.Fatalf("expected error when ApplicationConfig is nil")
	}

	pod := kubernetes.ToClientObject(&corev1.Pod{})
	objs := []*client.Object{pod}
	cfg := &fakeConfig{objs: objs}
	app.SetConfig(cfg)

	res, err := app.Generate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res) != 1 || res[0] != pod {
		t.Fatalf("unexpected result: %#v", res)
	}
}
