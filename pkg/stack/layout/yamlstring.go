package layout

import (
	"strconv"

	"sigs.k8s.io/yaml"
)

// yamlString renders s as a YAML scalar that kustomize reads back as the
// string s, for the entries of a kustomization.yaml the writers write
// (go-kure/kure#896). kustomize reads that file with sigs.k8s.io/yaml, which
// follows YAML 1.1: a plain y, on, null or 1e3 is a bool, null or number, and
// kustomize then fails to build it; " #" starts a comment and ": " a mapping.
// Such a value is double-quoted; for valid UTF-8, Go's escapes are a subset of
// YAML's double-quoted escapes. A string that is not valid UTF-8 has no YAML
// form: its \xHH escape reads back as the character U+00HH. A value read back
// unchanged is written plain, as before.
func yamlString(s string) string {
	var v map[string]any
	if err := yaml.Unmarshal([]byte("v: "+s), &v); err == nil {
		if got, ok := v["v"].(string); ok && got == s {
			return s
		}
	}
	return strconv.Quote(s)
}
