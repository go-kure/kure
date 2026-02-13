package generators_test

import (
	"testing"

	"github.com/go-kure/kure/pkg/stack/generators"
)

func TestRegistry(t *testing.T) {
	t.Run("ParseAPIVersion", func(t *testing.T) {
		tests := []struct {
			name       string
			apiVersion string
			kind       string
			expected   generators.GVK
		}{
			{
				name:       "full GVK",
				apiVersion: "generators.gokure.dev/v1alpha1",
				kind:       "AppWorkload",
				expected: generators.GVK{
					Group:   "generators.gokure.dev",
					Version: "v1alpha1",
					Kind:    "AppWorkload",
				},
			},
			{
				name:       "core version",
				apiVersion: "v1",
				kind:       "ConfigMap",
				expected: generators.GVK{
					Group:   "",
					Version: "v1",
					Kind:    "ConfigMap",
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				gvk := generators.ParseAPIVersion(tt.apiVersion, tt.kind)
				if gvk != tt.expected {
					t.Errorf("ParseAPIVersion() = %v, want %v", gvk, tt.expected)
				}
			})
		}
	})
}
