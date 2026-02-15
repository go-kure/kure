package stack

import (
	"errors"
	"strings"
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

// TestApplicationValidation exercises the optional Validator interface.
func TestApplicationValidation(t *testing.T) {
	pod := kubernetes.ToClientObject(&corev1.Pod{})
	objs := []*client.Object{pod}

	tests := []struct {
		name           string
		config         ApplicationConfig
		wantErr        bool
		errContains    string
		generateCalled bool
	}{
		{
			name:           "config without Validator generates normally",
			config:         &fakeConfig{objs: objs},
			wantErr:        false,
			generateCalled: true,
		},
		{
			name:           "config with Validator returning nil generates normally",
			config:         &validatingConfig{objs: objs, validateErr: nil},
			wantErr:        false,
			generateCalled: true,
		},
		{
			name:           "config with Validator returning error stops generation",
			config:         &validatingConfig{validateErr: errors.New("port is required")},
			wantErr:        true,
			errContains:    "validation failed",
			generateCalled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := NewApplication("my-app", "my-ns", tt.config)
			_, err := app.Generate()

			if tt.wantErr && err == nil {
				t.Fatal("expected error but got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.errContains)
			}

			if vc, ok := tt.config.(*validatingConfig); ok {
				if vc.generateCalled != tt.generateCalled {
					t.Errorf("generateCalled = %v, want %v", vc.generateCalled, tt.generateCalled)
				}
			}
		})
	}

	t.Run("error message includes app name and namespace", func(t *testing.T) {
		cfg := &validatingConfig{validateErr: errors.New("missing field")}
		app := NewApplication("web-server", "production", cfg)
		_, err := app.Generate()
		if err == nil {
			t.Fatal("expected error")
		}
		msg := err.Error()
		if !strings.Contains(msg, "web-server") {
			t.Errorf("error %q does not contain app name %q", msg, "web-server")
		}
		if !strings.Contains(msg, "production") {
			t.Errorf("error %q does not contain namespace %q", msg, "production")
		}
	})

	t.Run("error wraps original validation error", func(t *testing.T) {
		origErr := errors.New("replicas must be positive")
		cfg := &validatingConfig{validateErr: origErr}
		app := NewApplication("app", "ns", cfg)
		_, err := app.Generate()
		if err == nil {
			t.Fatal("expected error")
		}
		if !errors.Is(err, origErr) {
			t.Errorf("error chain does not contain original error; got %v", err)
		}
	})
}
