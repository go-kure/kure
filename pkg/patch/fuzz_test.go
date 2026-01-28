package patch

import (
	"strings"
	"testing"
)

// FuzzParsePatchLine tests the ParsePatchLine function with fuzzy inputs
// Run with: go test -fuzz=FuzzParsePatchLine -fuzztime=30s ./pkg/patch/...
func FuzzParsePatchLine(f *testing.F) {
	// Add seed corpus from common patch patterns
	seedCorpus := []struct {
		key   string
		value string
	}{
		// Basic paths
		{"spec.replicas", "3"},
		{"metadata.name", "my-app"},
		{"metadata.labels.app", "nginx"},

		// Array selectors
		{"spec.containers[name=main].image", "nginx:latest"},
		{"spec.containers[0].image", "nginx:1.21"},
		{"spec.ports[port=80].targetPort", "8080"},

		// Append operations
		{"spec.containers[-]", "name: sidecar"},
		{"spec.volumes[-]", "name: data"},

		// Delete operations
		{"spec.containers[delete]", ""},
		{"spec.containers[delete=name=sidecar]", ""},

		// Insert operations
		{"spec.containers[-name=main]", "name: init-container"},
		{"spec.containers[+name=main]", "name: after-main"},
		{"spec.containers[-0]", "name: first"},
		{"spec.containers[+0]", "name: second"},

		// Nested paths
		{"spec.template.spec.containers[name=app].resources.limits.cpu", "100m"},
		{"spec.template.spec.containers[name=app].resources.limits.memory", "128Mi"},

		// Complex key-value selectors
		{"spec.containers[name=main].env[name=DEBUG]", "true"},
		{"spec.volumes[name=config].configMap.name", "my-config"},

		// Deep nesting
		{"a.b.c.d.e.f.g", "value"},

		// Empty and edge cases
		{"", "value"},
		{"key", ""},
		{".", "value"},
		{"..", "value"},

		// Special characters in values
		{"spec.image", "gcr.io/project/image:v1.2.3-beta+build.123"},
		{"metadata.annotations.description", "This is a \"quoted\" value with 'apostrophes'"},
		{"spec.command", "['/bin/sh', '-c', 'echo hello']"},

		// Numeric values
		{"spec.replicas", "0"},
		{"spec.replicas", "-1"},
		{"spec.replicas", "9999999"},

		// Boolean-like strings
		{"spec.enabled", "true"},
		{"spec.enabled", "false"},
		{"spec.enabled", "True"},
		{"spec.enabled", "FALSE"},

		// YAML-like values
		{"spec.config", "{key: value}"},
		{"spec.list", "[a, b, c]"},

		// Unicode
		{"metadata.labels.unicode", "日本語"},
		{"spec.name", "émoji-🚀"},
	}

	for _, tc := range seedCorpus {
		f.Add(tc.key, tc.value)
	}

	f.Fuzz(func(t *testing.T, key string, valueStr string) {
		// ParsePatchLine should not panic on any input
		_, err := ParsePatchLine(key, valueStr)

		// We don't check err because many invalid inputs are expected to fail
		// The important thing is that it doesn't panic
		_ = err
	})
}

// FuzzLoadPatchFile tests the LoadPatchFile function with fuzzy inputs
// Run with: go test -fuzz=FuzzLoadPatchFile -fuzztime=30s ./pkg/patch/...
func FuzzLoadPatchFile(f *testing.F) {
	// Add seed corpus from common patch file formats

	// YAML format patches
	f.Add(`spec.replicas: 3
metadata.name: my-app
`)

	f.Add(`spec.containers[name=main].image: nginx:latest
spec.containers[name=main].resources.limits.cpu: 100m
`)

	f.Add(`- target: deployment.my-app
  patch:
    spec.replicas: 5
`)

	f.Add(`- target: service.my-service
  patch:
    spec.ports[port=80].targetPort: 8080
    spec.type: LoadBalancer
`)

	// TOML format patches
	f.Add(`[deployment.my-app]
replicas: 3
`)

	f.Add(`[deployment.my-app.containers.name=main]
image: nginx:latest
`)

	f.Add(`[service.my-service.ports.0]
port: 80
targetPort: 8080
`)

	// Complex TOML patches
	f.Add(`[deployment.my-app.containers.name=main]
image: nginx:latest
imagePullPolicy: Always

[deployment.my-app.containers.name=sidecar]
image: envoy:latest
`)

	// Edge cases
	f.Add(``)
	f.Add(`# Just a comment`)
	f.Add(`
# Multiple
# Comments
`)

	// Malformed inputs (to test error handling)
	f.Add(`{invalid json}`)
	f.Add(`[incomplete`)
	f.Add(`key without value`)
	f.Add(`:::`)

	// Mixed content
	f.Add(`spec.replicas: 3
# comment in the middle
metadata.labels.app: nginx
`)

	// Large values
	f.Add(`spec.data: ` + strings.Repeat("a", 1000))

	// Unicode content
	f.Add(`metadata.labels.japanese: 日本語
metadata.labels.emoji: 🚀
`)

	// Nested YAML
	f.Add(`spec.template.spec.containers[name=main].env:
  - name: DEBUG
    value: "true"
`)

	f.Fuzz(func(t *testing.T, content string) {
		reader := strings.NewReader(content)

		// LoadPatchFile should not panic on any input
		_, err := LoadPatchFile(reader)

		// We don't check err because many invalid inputs are expected to fail
		// The important thing is that it doesn't panic
		_ = err
	})
}

// FuzzParseTOMLHeader tests the ParseTOMLHeader function with fuzzy inputs
// Run with: go test -fuzz=FuzzParseTOMLHeader -fuzztime=30s ./pkg/patch/...
func FuzzParseTOMLHeader(f *testing.F) {
	// Add seed corpus
	f.Add("[deployment.my-app]")
	f.Add("[deployment.my-app.containers]")
	f.Add("[deployment.my-app.containers.name=main]")
	f.Add("[deployment.my-app.containers.0]")
	f.Add("[deployment.my-app.spec.template.spec]")
	f.Add("[service.my-service.ports.port=80]")
	f.Add("[configmap.my-config.data]")
	f.Add("[secret.my-secret.stringData]")

	// Bracketed selectors
	f.Add("[deployment.my-app.containers[image.name=main]]")

	// Edge cases
	f.Add("[]")
	f.Add("[a]")
	f.Add("[a.b]")
	f.Add("[.]")
	f.Add("[..]")
	f.Add("[[double]]")
	f.Add("[unbalanced")
	f.Add("unbalanced]")
	f.Add("no brackets")

	// Deep nesting
	f.Add("[a.b.c.d.e.f.g.h.i.j]")

	// Special characters
	f.Add("[deployment.my-app-name-with-dashes]")
	f.Add("[deployment.my_app_with_underscores]")

	f.Fuzz(func(t *testing.T, header string) {
		// ParseTOMLHeader should not panic on any input
		_, err := ParseTOMLHeader(header)

		// We don't check err because many invalid inputs are expected to fail
		// The important thing is that it doesn't panic
		_ = err
	})
}

// FuzzParsePatchPath tests the ParsePatchPath function with fuzzy inputs
// Run with: go test -fuzz=FuzzParsePatchPath -fuzztime=30s ./pkg/patch/...
func FuzzParsePatchPath(f *testing.F) {
	// Add seed corpus
	f.Add("spec.replicas")
	f.Add("metadata.name")
	f.Add("spec.containers[0]")
	f.Add("spec.containers[name=main]")
	f.Add("spec.containers[0].image")
	f.Add("spec.containers[name=main].resources.limits.cpu")
	f.Add("spec.template.spec.containers[name=app].env[name=DEBUG].value")

	// Edge cases
	f.Add("")
	f.Add(".")
	f.Add("..")
	f.Add("...")
	f.Add("a")
	f.Add("[0]")
	f.Add("a[0]")
	f.Add("a[]")
	f.Add("a[")
	f.Add("a]")
	f.Add("a[b")
	f.Add("a[0")
	f.Add("a[=value]")
	f.Add("a[key=]")
	f.Add("a[key=value")

	// Deep paths
	f.Add("a.b.c.d.e.f.g.h.i.j.k.l.m.n.o.p")

	// Complex selectors
	f.Add("spec.containers[name=main].ports[containerPort=8080]")
	f.Add("a[0].b[1].c[2]")

	f.Fuzz(func(t *testing.T, path string) {
		// ParsePatchPath should not panic on any input
		_, err := ParsePatchPath(path)

		// We don't check err because many invalid inputs are expected to fail
		// The important thing is that it doesn't panic
		_ = err
	})
}

// FuzzSubstituteVariables tests the SubstituteVariables function with fuzzy inputs
// Run with: go test -fuzz=FuzzSubstituteVariables -fuzztime=30s ./pkg/patch/...
func FuzzSubstituteVariables(f *testing.F) {
	// Add seed corpus
	f.Add("${values.name}")
	f.Add("${features.enabled}")
	f.Add("prefix-${values.name}-suffix")
	f.Add("${values.a}${values.b}")
	f.Add("no variables here")
	f.Add("")
	f.Add("${}")
	f.Add("${values.}")
	f.Add("${.name}")
	f.Add("${values.nested.key}")
	f.Add("unclosed ${values.name")
	f.Add("extra } brace")

	f.Fuzz(func(t *testing.T, value string) {
		// Create a context with some values
		ctx := &VariableContext{
			Values: map[string]interface{}{
				"name":       "test",
				"replicas":   3,
				"enabled":    true,
				"nested.key": "value",
			},
			Features: map[string]bool{
				"enabled":  true,
				"disabled": false,
			},
		}

		// SubstituteVariables should not panic on any input
		_, err := SubstituteVariables(value, ctx)
		_ = err

		// Also test with nil context
		_, err = SubstituteVariables(value, nil)
		_ = err
	})
}

// FuzzIsTOMLFormat tests the IsTOMLFormat function with fuzzy inputs
// Run with: go test -fuzz=FuzzIsTOMLFormat -fuzztime=30s ./pkg/patch/...
func FuzzIsTOMLFormat(f *testing.F) {
	// TOML-like content
	f.Add("[section]\nkey = value")
	f.Add("[deployment.app]\nreplicas = 3")
	f.Add("key = value")

	// YAML-like content
	f.Add("key: value")
	f.Add("- item1\n- item2")
	f.Add("spec:\n  replicas: 3")

	// Edge cases
	f.Add("")
	f.Add("# comment only")
	f.Add("[")
	f.Add("]")
	f.Add("[]")
	f.Add("=")
	f.Add(":")
	f.Add("{}")

	f.Fuzz(func(t *testing.T, content string) {
		// IsTOMLFormat should not panic on any input
		_ = IsTOMLFormat(content)
	})
}
