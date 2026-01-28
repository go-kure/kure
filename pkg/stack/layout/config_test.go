package layout_test

import (
	"testing"

	"github.com/go-kure/kure/pkg/stack/layout"
)

func TestDefaultKustomizationFileName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple name",
			input:    "test",
			expected: "kustomization-test.yaml",
		},
		{
			name:     "name with dashes",
			input:    "my-cluster",
			expected: "kustomization-my-cluster.yaml",
		},
		{
			name:     "empty name",
			input:    "",
			expected: "kustomization-.yaml",
		},
		{
			name:     "name with underscores",
			input:    "test_kust",
			expected: "kustomization-test_kust.yaml",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := layout.DefaultKustomizationFileName(test.input)
			if result != test.expected {
				t.Errorf("expected %q, got %q", test.expected, result)
			}
		})
	}
}
