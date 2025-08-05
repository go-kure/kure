package patch

import (
	"fmt"
	"strings"
	"testing"
)

func TestParseTOMLHeader(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected *TOMLHeader
		wantErr  bool
	}{
		{
			name:  "simple kind.name",
			input: "[deployment.app]",
			expected: &TOMLHeader{
				Kind: "deployment",
				Name: "app",
			},
		},
		{
			name:  "with single section",
			input: "[deployment.app.containers]",
			expected: &TOMLHeader{
				Kind:     "deployment",
				Name:     "app",
				Sections: []string{"containers"},
			},
		},
		{
			name:  "with key-value selector",
			input: "[deployment.app.containers.name=main]",
			expected: &TOMLHeader{
				Kind:     "deployment",
				Name:     "app",
				Sections: []string{"containers"},
				Selector: &Selector{
					Type:  "key-value",
					Key:   "name",
					Value: "main",
				},
			},
		},
		{
			name:  "with index selector",
			input: "[service.app.ports.0]",
			expected: &TOMLHeader{
				Kind:     "service",
				Name:     "app",
				Sections: []string{"ports"},
				Selector: &Selector{
					Type:  "index",
					Index: func() *int { i := 0; return &i }(),
				},
			},
		},
		{
			name:  "with bracketed selector",
			input: "[deployment.app.containers[image.name=main]]",
			expected: &TOMLHeader{
				Kind:     "deployment",
				Name:     "app",
				Sections: []string{"containers"},
				Selector: &Selector{
					Type:      "bracketed",
					Bracketed: "image.name=main",
				},
			},
		},
		{
			name:  "multiple sections",
			input: "[ingress.web.tls.0]",
			expected: &TOMLHeader{
				Kind:     "ingress",
				Name:     "web",
				Sections: []string{"tls"},
				Selector: &Selector{
					Type:  "index",
					Index: func() *int { i := 0; return &i }(),
				},
			},
		},
		{
			name:    "empty header",
			input:   "[]",
			wantErr: true,
		},
		{
			name:    "missing closing bracket",
			input:   "[deployment.app",
			wantErr: true,
		},
		{
			name:    "missing kind.name",
			input:   "[containers]",
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ParseTOMLHeader(tc.input)
			
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}
			
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			
			if result.Kind != tc.expected.Kind {
				t.Errorf("Kind: expected %s, got %s", tc.expected.Kind, result.Kind)
			}
			
			if result.Name != tc.expected.Name {
				t.Errorf("Name: expected %s, got %s", tc.expected.Name, result.Name)
			}
			
			if len(result.Sections) != len(tc.expected.Sections) {
				t.Errorf("Sections length: expected %d, got %d", len(tc.expected.Sections), len(result.Sections))
			} else {
				for i, section := range tc.expected.Sections {
					if result.Sections[i] != section {
						t.Errorf("Section[%d]: expected %s, got %s", i, section, result.Sections[i])
					}
				}
			}
			
			// Check selector
			if tc.expected.Selector == nil && result.Selector != nil {
				t.Errorf("expected no selector but got one")
			} else if tc.expected.Selector != nil && result.Selector == nil {
				t.Errorf("expected selector but got none")
			} else if tc.expected.Selector != nil && result.Selector != nil {
				if result.Selector.Type != tc.expected.Selector.Type {
					t.Errorf("Selector type: expected %s, got %s", tc.expected.Selector.Type, result.Selector.Type)
				}
				
				switch tc.expected.Selector.Type {
				case "index":
					if tc.expected.Selector.Index != nil && result.Selector.Index != nil {
						if *result.Selector.Index != *tc.expected.Selector.Index {
							t.Errorf("Selector index: expected %d, got %d", *tc.expected.Selector.Index, *result.Selector.Index)
						}
					}
				case "key-value":
					if result.Selector.Key != tc.expected.Selector.Key {
						t.Errorf("Selector key: expected %s, got %s", tc.expected.Selector.Key, result.Selector.Key)
					}
					if result.Selector.Value != tc.expected.Selector.Value {
						t.Errorf("Selector value: expected %s, got %s", tc.expected.Selector.Value, result.Selector.Value)
					}
				case "bracketed":
					if result.Selector.Bracketed != tc.expected.Selector.Bracketed {
						t.Errorf("Selector bracketed: expected %s, got %s", tc.expected.Selector.Bracketed, result.Selector.Bracketed)
					}
				}
			}
		})
	}
}

func TestResolveTOMLPath(t *testing.T) {
	testCases := []struct {
		name               string
		header             *TOMLHeader
		expectedTarget     string
		expectedFieldPath  string
	}{
		{
			name: "deployment with containers",
			header: &TOMLHeader{
				Kind:     "deployment",
				Name:     "app",
				Sections: []string{"containers"},
				Selector: &Selector{
					Type:  "key-value",
					Key:   "name",
					Value: "main",
				},
			},
			expectedTarget:    "deployment.app",
			expectedFieldPath: "spec.template.spec.containers[name=main]",
		},
		{
			name: "service with ports",
			header: &TOMLHeader{
				Kind:     "service",
				Name:     "app",
				Sections: []string{"ports"},
				Selector: &Selector{
					Type:  "index",
					Index: func() *int { i := 0; return &i }(),
				},
			},
			expectedTarget:    "service.app",
			expectedFieldPath: "spec.ports[0]",
		},
		{
			name: "simple deployment",
			header: &TOMLHeader{
				Kind: "deployment",
				Name: "app",
			},
			expectedTarget:    "deployment.app",
			expectedFieldPath: "",
		},
		{
			name: "ingress with complex path",
			header: &TOMLHeader{
				Kind:     "ingress",
				Name:     "web",
				Sections: []string{"rules", "paths"},
				Selector: &Selector{
					Type:  "index",
					Index: func() *int { i := 0; return &i }(),
				},
			},
			expectedTarget:    "ingress.web",
			expectedFieldPath: "spec.rules.http.paths[0]",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			target, fieldPath, err := tc.header.ResolveTOMLPath()
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			
			if target != tc.expectedTarget {
				t.Errorf("Target: expected %s, got %s", tc.expectedTarget, target)
			}
			
			if fieldPath != tc.expectedFieldPath {
				t.Errorf("FieldPath: expected %s, got %s", tc.expectedFieldPath, fieldPath)
			}
		})
	}
}

func TestSubstituteVariables(t *testing.T) {
	ctx := &VariableContext{
		Values: map[string]interface{}{
			"version":    "1.2.3",
			"replicas":   3,
			"cpu_limit":  "500m",
		},
		Features: map[string]bool{
			"enable_debug": true,
			"use_ssl":      false,
		},
	}

	testCases := []struct {
		name     string
		input    interface{}
		expected interface{}
	}{
		{
			name:     "simple string value",
			input:    "nginx",
			expected: "nginx",
		},
		{
			name:     "values substitution",
			input:    "${values.version}",
			expected: "1.2.3",
		},
		{
			name:     "features substitution",
			input:    "${features.enable_debug}",
			expected: "true",
		},
		{
			name:     "mixed substitution",
			input:    "image:${values.version}",
			expected: "image:1.2.3",
		},
		{
			name:     "multiple substitutions",
			input:    "${values.cpu_limit}-${features.use_ssl}",
			expected: "500m-false",
		},
		{
			name:     "undefined variable",
			input:    "${values.undefined}",
			expected: "${values.undefined}",
		},
		{
			name:     "numeric value",
			input:    123,
			expected: 123,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := SubstituteVariables(fmt.Sprintf("%v", tc.input), ctx)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			
			if fmt.Sprintf("%v", result) != fmt.Sprintf("%v", tc.expected) {
				t.Errorf("expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestIsTOMLFormat(t *testing.T) {
	testCases := []struct {
		name     string
		content  string
		expected bool
	}{
		{
			name: "TOML format",
			content: `# Comment
[deployment.app]
replicas: 3`,
			expected: true,
		},
		{
			name: "YAML list format",
			content: `- target: app
  patch:
    replicas: 3`,
			expected: false,
		},
		{
			name: "YAML map format",
			content: `spec.replicas: 3
spec.image: nginx`,
			expected: false,
		},
		{
			name: "empty content",
			content: ``,
			expected: false,
		},
		{
			name: "only comments",
			content: `# This is a comment
# Another comment`,
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := IsTOMLFormat(tc.content)
			if result != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestLoadTOMLPatchFile(t *testing.T) {
	tomlContent := `# TOML patch example
[deployment.app]
replicas: 3
metadata.labels.env: production

[deployment.app.containers.name=main]
image.repository: nginx
image.tag: 1.20
resources.requests.cpu: 100m`

	patches, err := LoadTOMLPatchFile(strings.NewReader(tomlContent), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(patches) != 5 {
		t.Errorf("expected 5 patches, got %d", len(patches))
	}

	// Check first patch
	if patches[0].Target != "deployment.app" {
		t.Errorf("first patch target: expected 'deployment.app', got '%s'", patches[0].Target)
	}

	if patches[0].Patch.Path != "replicas" {
		t.Errorf("first patch path: expected 'replicas', got '%s'", patches[0].Patch.Path)
	}

	// Check container-specific patch
	found := false
	for _, patch := range patches {
		if strings.Contains(patch.Patch.Path, "containers") && patch.Patch.Selector == "name=main" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find container-specific patch")
	}
}

func TestLoadTOMLPatchFileWithVariables(t *testing.T) {
	tomlContent := `[deployment.app]
replicas: ${values.replica_count}
image: nginx:${values.version}
debug: ${features.enable_debug}`

	ctx := &VariableContext{
		Values: map[string]interface{}{
			"replica_count": 3,
			"version":       "1.20",
		},
		Features: map[string]bool{
			"enable_debug": true,
		},
	}

	patches, err := LoadTOMLPatchFile(strings.NewReader(tomlContent), ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(patches) != 3 {
		t.Errorf("expected 3 patches, got %d", len(patches))
	}

	// Check variable substitution worked
	for _, patch := range patches {
		switch patch.Patch.Path {
		case "replicas":
			if patch.Patch.Value != 3 {
				t.Errorf("replicas value: expected 3, got %v (%T)", patch.Patch.Value, patch.Patch.Value)
			}
		case "image":
			if patch.Patch.Value != "nginx:1.20" {
				t.Errorf("image value: expected 'nginx:1.20', got %v", patch.Patch.Value)
			}
		case "debug":
			if patch.Patch.Value != true {
				t.Errorf("debug value: expected bool(true), got %v (%T)", patch.Patch.Value, patch.Patch.Value)
			}
		}
	}
}