package gvk

import (
	"testing"
)

func TestGVK(t *testing.T) {
	t.Run("String representation", func(t *testing.T) {
		gvk := GVK{
			Group:   "example.com",
			Version: "v1",
			Kind:    "TestType",
		}

		expected := "example.com/v1, Kind=TestType"
		if gvk.String() != expected {
			t.Errorf("Expected %s, got %s", expected, gvk.String())
		}
	})

	t.Run("APIVersion", func(t *testing.T) {
		tests := []struct {
			name     string
			gvk      GVK
			expected string
		}{
			{
				name: "with group",
				gvk: GVK{
					Group:   "example.com",
					Version: "v1",
					Kind:    "TestType",
				},
				expected: "example.com/v1",
			},
			{
				name: "without group",
				gvk: GVK{
					Group:   "",
					Version: "v1",
					Kind:    "ConfigMap",
				},
				expected: "v1",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if got := tt.gvk.APIVersion(); got != tt.expected {
					t.Errorf("APIVersion() = %s, want %s", got, tt.expected)
				}
			})
		}
	})
}

func TestParseAPIVersion(t *testing.T) {
	tests := []struct {
		name       string
		apiVersion string
		kind       string
		expected   GVK
	}{
		{
			name:       "full GVK",
			apiVersion: "example.com/v1",
			kind:       "TestType",
			expected: GVK{
				Group:   "example.com",
				Version: "v1",
				Kind:    "TestType",
			},
		},
		{
			name:       "core type",
			apiVersion: "v1",
			kind:       "ConfigMap",
			expected: GVK{
				Group:   "",
				Version: "v1",
				Kind:    "ConfigMap",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseAPIVersion(tt.apiVersion, tt.kind)
			if got != tt.expected {
				t.Errorf("ParseAPIVersion() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestBaseMetadata(t *testing.T) {
	metadata := &BaseMetadata{}

	// Test name
	metadata.SetName("test-name")
	if got := metadata.GetName(); got != "test-name" {
		t.Errorf("GetName() = %s, want test-name", got)
	}

	// Test namespace
	metadata.SetNamespace("test-namespace")
	if got := metadata.GetNamespace(); got != "test-namespace" {
		t.Errorf("GetNamespace() = %s, want test-namespace", got)
	}
}

func TestValidateGVK(t *testing.T) {
	tests := []struct {
		name    string
		gvk     GVK
		wantErr bool
	}{
		{
			name: "valid GVK",
			gvk: GVK{
				Group:   "example.com",
				Version: "v1",
				Kind:    "TestType",
			},
			wantErr: false,
		},
		{
			name: "missing kind",
			gvk: GVK{
				Group:   "example.com",
				Version: "v1",
				Kind:    "",
			},
			wantErr: true,
		},
		{
			name: "missing version",
			gvk: GVK{
				Group:   "example.com",
				Version: "",
				Kind:    "TestType",
			},
			wantErr: true,
		},
		{
			name: "missing group is ok",
			gvk: GVK{
				Group:   "",
				Version: "v1",
				Kind:    "ConfigMap",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateGVK(tt.gvk)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateGVK() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
