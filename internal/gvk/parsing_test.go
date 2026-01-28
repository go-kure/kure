package gvk_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/go-kure/kure/internal/gvk"
)

// TestConfig is a simple test type for parsing
type TestConfig struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

func TestParseSingle(t *testing.T) {
	registry := gvk.NewRegistry[TestConfig]()
	registry.Register(gvk.GVK{Group: "test.io", Version: "v1", Kind: "TestConfig"}, func() TestConfig {
		return TestConfig{}
	})

	yaml := `apiVersion: test.io/v1
kind: TestConfig
metadata:
  name: test-config
spec:
  name: my-test
  value: test-value
`

	wrapper, err := gvk.ParseSingle([]byte(yaml), registry, nil)
	if err != nil {
		t.Fatalf("failed to parse single: %v", err)
	}

	if wrapper == nil {
		t.Fatal("expected non-nil wrapper")
	}

	if wrapper.APIVersion != "test.io/v1" {
		t.Errorf("expected apiVersion 'test.io/v1', got %s", wrapper.APIVersion)
	}
	if wrapper.Kind != "TestConfig" {
		t.Errorf("expected kind 'TestConfig', got %s", wrapper.Kind)
	}
}

func TestParseMultiple(t *testing.T) {
	registry := gvk.NewRegistry[TestConfig]()
	registry.Register(gvk.GVK{Group: "test.io", Version: "v1", Kind: "TestConfig"}, func() TestConfig {
		return TestConfig{}
	})

	yaml := `apiVersion: test.io/v1
kind: TestConfig
metadata:
  name: config1
spec:
  name: first
  value: value1
---
apiVersion: test.io/v1
kind: TestConfig
metadata:
  name: config2
spec:
  name: second
  value: value2
`

	wrappers, err := gvk.ParseMultiple([]byte(yaml), registry, nil)
	if err != nil {
		t.Fatalf("failed to parse multiple: %v", err)
	}

	if len(wrappers) != 2 {
		t.Fatalf("expected 2 wrappers, got %d", len(wrappers))
	}

	if wrappers[0].GetName() != "config1" {
		t.Errorf("expected first name 'config1', got %s", wrappers[0].GetName())
	}
	if wrappers[1].GetName() != "config2" {
		t.Errorf("expected second name 'config2', got %s", wrappers[1].GetName())
	}
}

func TestParseMultiple_EmptyDocuments(t *testing.T) {
	registry := gvk.NewRegistry[TestConfig]()
	registry.Register(gvk.GVK{Group: "test.io", Version: "v1", Kind: "TestConfig"}, func() TestConfig {
		return TestConfig{}
	})

	yaml := `---
apiVersion: test.io/v1
kind: TestConfig
metadata:
  name: config1
spec:
  name: first
---
---
apiVersion: test.io/v1
kind: TestConfig
metadata:
  name: config2
spec:
  name: second
`

	wrappers, err := gvk.ParseMultiple([]byte(yaml), registry, nil)
	if err != nil {
		t.Fatalf("failed to parse multiple with empty docs: %v", err)
	}

	if len(wrappers) != 2 {
		t.Fatalf("expected 2 wrappers (empty docs skipped), got %d", len(wrappers))
	}
}

func TestParseStream(t *testing.T) {
	registry := gvk.NewRegistry[TestConfig]()
	registry.Register(gvk.GVK{Group: "test.io", Version: "v1", Kind: "TestConfig"}, func() TestConfig {
		return TestConfig{}
	})

	yaml := `apiVersion: test.io/v1
kind: TestConfig
metadata:
  name: config1
spec:
  name: first
---
apiVersion: test.io/v1
kind: TestConfig
metadata:
  name: config2
spec:
  name: second
`

	reader := bytes.NewReader([]byte(yaml))
	wrappers, err := gvk.ParseStream(reader, registry, nil)
	if err != nil {
		t.Fatalf("failed to parse stream: %v", err)
	}

	if len(wrappers) != 2 {
		t.Fatalf("expected 2 wrappers, got %d", len(wrappers))
	}
}

func TestParseStream_EmptyStream(t *testing.T) {
	registry := gvk.NewRegistry[TestConfig]()
	reader := strings.NewReader("")
	wrappers, err := gvk.ParseStream(reader, registry, nil)
	if err != nil {
		t.Fatalf("failed to parse empty stream: %v", err)
	}

	if len(wrappers) != 0 {
		t.Fatalf("expected 0 wrappers for empty stream, got %d", len(wrappers))
	}
}

func TestValidateGVK(t *testing.T) {
	tests := []struct {
		name    string
		gvk     gvk.GVK
		wantErr bool
	}{
		{
			name: "valid GVK with group",
			gvk: gvk.GVK{
				Group:   "apps",
				Version: "v1",
				Kind:    "Deployment",
			},
			wantErr: false,
		},
		{
			name: "valid GVK without group (core)",
			gvk: gvk.GVK{
				Group:   "",
				Version: "v1",
				Kind:    "Pod",
			},
			wantErr: false,
		},
		{
			name: "missing kind",
			gvk: gvk.GVK{
				Group:   "apps",
				Version: "v1",
				Kind:    "",
			},
			wantErr: true,
		},
		{
			name: "missing version",
			gvk: gvk.GVK{
				Group:   "apps",
				Version: "",
				Kind:    "Deployment",
			},
			wantErr: true,
		},
		{
			name: "missing both",
			gvk: gvk.GVK{
				Group:   "apps",
				Version: "",
				Kind:    "",
			},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := gvk.ValidateGVK(test.gvk)
			if test.wantErr && err == nil {
				t.Error("expected validation error, got nil")
			}
			if !test.wantErr && err != nil {
				t.Errorf("unexpected validation error: %v", err)
			}
		})
	}
}

func TestParseSingle_InvalidYAML(t *testing.T) {
	registry := gvk.NewRegistry[TestConfig]()
	registry.Register(gvk.GVK{Group: "test.io", Version: "v1", Kind: "TestConfig"}, func() TestConfig {
		return TestConfig{}
	})

	invalidYAML := `this is not: [valid yaml`

	_, err := gvk.ParseSingle([]byte(invalidYAML), registry, nil)
	if err == nil {
		t.Error("expected error for invalid YAML, got nil")
	}
}

func TestParseSingle_UnknownType(t *testing.T) {
	registry := gvk.NewRegistry[TestConfig]()
	// Don't register any types

	yaml := `apiVersion: unknown/v1
kind: Unknown
metadata:
  name: test
spec:
  value: test
`

	_, err := gvk.ParseSingle([]byte(yaml), registry, nil)
	if err == nil {
		t.Error("expected error for unknown type, got nil")
	}
}

func TestParseMultiple_InvalidDocument(t *testing.T) {
	registry := gvk.NewRegistry[TestConfig]()
	registry.Register(gvk.GVK{Group: "test.io", Version: "v1", Kind: "TestConfig"}, func() TestConfig {
		return TestConfig{}
	})

	yaml := `apiVersion: test.io/v1
kind: TestConfig
metadata:
  name: valid
spec:
  name: good
---
this is: [invalid
`

	_, err := gvk.ParseMultiple([]byte(yaml), registry, nil)
	if err == nil {
		t.Error("expected error for invalid document in multi-doc, got nil")
	}
}
