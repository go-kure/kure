package layout

import (
	"testing"

	"sigs.k8s.io/yaml"
)

// TestYAMLString: yamlString writes a value plain when kustomize's reader
// reads it back unchanged, quotes it otherwise, and every value it writes
// reads back as the string it was given (go-kure/kure#896).
func TestYAMLString(t *testing.T) {
	for _, tc := range []struct {
		in, want string
	}{
		{"svc", "svc"},
		{"svc.yaml", "svc.yaml"},
		{"sub/svc.yaml", "sub/svc.yaml"},
		{"default-configmap-a.yaml", "default-configmap-a.yaml"},
		{"y", `"y"`},
		{"on", `"on"`},
		{"null", `"null"`},
		{"~", `"~"`},
		{"1e3", `"1e3"`},
		{"0x1f", `"0x1f"`},
		// Read as a string, but not as this one.
		{"a #b", `"a #b"`},
		{" svc", `" svc"`},
		{"", `""`},
	} {
		if got := yamlString(tc.in); got != tc.want {
			t.Errorf("yamlString(%q) = %s, want %s", tc.in, got, tc.want)
		}
	}
	for _, in := range []string{"y", "a #b", " svc", "", "tab\there", "é", "\x01", `back\slash`, `"quoted"`, "a: b", "- x", "*alias"} {
		var v map[string]any
		if err := yaml.Unmarshal([]byte("v: "+yamlString(in)), &v); err != nil {
			t.Errorf("yamlString(%q) = %s does not parse: %v", in, yamlString(in), err)
			continue
		}
		if got, ok := v["v"].(string); !ok || got != in {
			t.Errorf("yamlString(%q) = %s reads back as %#v", in, yamlString(in), v["v"])
		}
	}
}
