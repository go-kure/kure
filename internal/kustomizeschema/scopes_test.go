package kustomizeschema

import (
	"testing"

	"sigs.k8s.io/kustomize/kyaml/openapi"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// addPaths adds a schema that holds paths and no definition, as a caller adds
// the paths of an API server's discovery document.
func addPaths(t *testing.T, paths string) {
	t.Helper()
	doc := `{"swagger":"2.0","info":{"title":"x","version":"v1"},"paths":` + paths + `,"definitions":{}}`
	if err := openapi.AddSchema([]byte(doc)); err != nil {
		t.Fatalf("add a schema: %v", err)
	}
}

func pathOf(path, group, version, kind string) string {
	return `{"` + path + `":{"get":{"x-kubernetes-group-version-kind":{"group":"` + group + `","version":"` + version + `","kind":"` + kind + `"}}}}`
}

// TestBuiltInScopes_AgreeWithKyamlsAnswerForTheBuiltInSchema: every kind the
// paths of the built-in schema declare is answered by kyaml as they say, so the
// baseline ScopeIsBuiltIn compares with is kyaml's own for the untouched schema.
func TestBuiltInScopes_AgreeWithKyamlsAnswerForTheBuiltInSchema(t *testing.T) {
	Parse()
	scopes := builtInScopes()
	for _, k := range []struct {
		apiVersion, kind string
		namespaced       bool
	}{{"v1", "ConfigMap", true}, {"apps/v1", "Deployment", true}, {"v1", "Node", false}, {"v1", "Namespace", false}} {
		if got, ok := scopes[yaml.TypeMeta{APIVersion: k.apiVersion, Kind: k.kind}]; !ok || got != k.namespaced {
			t.Errorf("the baseline says %s %s is (namespaced %v, held %v), want namespaced %v", k.apiVersion, k.kind, got, ok, k.namespaced)
		}
	}
	for tm, want := range scopes {
		got, found := openapi.IsNamespaceScoped(tm)
		if !found || got != want {
			t.Errorf("%s %s: kyaml answers (%v, found %v), the paths of the built-in schema say %v", tm.APIVersion, tm.Kind, got, found, want)
		}
		if !ScopeIsBuiltIn(tm.APIVersion, tm.Kind) {
			t.Errorf("%s %s: ScopeIsBuiltIn is false on the untouched schema", tm.APIVersion, tm.Kind)
		}
	}
	if !ScopeIsBuiltIn("example.com/v1", "Unknown") {
		t.Error("a kind no schema declares is not answered differently: ScopeIsBuiltIn wants true")
	}
}

// TestScopeIsBuiltIn_AKindANewSchemaNamesByPathIsDetected: a schema added with
// paths alone names a kind the built-in one lacks, as cluster-scoped or as
// namespaced, and kyaml answers for it from then on, which neither Definitions
// nor BuiltIn shows (go-kure/kure#1166). A reset restores the built-in answer.
func TestScopeIsBuiltIn_AKindANewSchemaNamesByPathIsDetected(t *testing.T) {
	if !inFreshProcess(t, "TestScopeIsBuiltIn_AKindANewSchemaNamesByPathIsDetected") {
		return
	}
	Parse()
	kinds := []struct{ apiVersion, group, version, kind, path string }{
		{"example.com/v1", "example.com", "v1", "Widget", "/apis/example.com/v1/widgets"},
		{"example.com/v1", "example.com", "v1", "Gadget", "/apis/example.com/v1/namespaces/{namespace}/gadgets"},
	}
	for _, k := range kinds {
		if !ScopeIsBuiltIn(k.apiVersion, k.kind) {
			t.Fatalf("%s %s before the schema is added: ScopeIsBuiltIn is false", k.apiVersion, k.kind)
		}
	}
	if got, ok := Definitions(); !ok {
		t.Fatalf("Definitions() = %s before the paths are added, want the pin", got)
	}
	for _, k := range kinds {
		addPaths(t, pathOf(k.path, k.group, k.version, k.kind))
	}
	if got, ok := Definitions(); !ok {
		t.Fatalf("Definitions() = %s after paths alone were added: the test needs a change only the scope shows", got)
	}
	for _, k := range kinds {
		if ScopeIsBuiltIn(k.apiVersion, k.kind) {
			t.Errorf("%s %s after its path was added: ScopeIsBuiltIn is true, want false", k.apiVersion, k.kind)
		}
	}
	if !ScopeIsBuiltIn("v1", "ConfigMap") {
		t.Error("v1 ConfigMap, which the added paths do not name: ScopeIsBuiltIn is false")
	}
	openapi.ResetOpenAPI()
	Parse()
	for _, k := range kinds {
		if !ScopeIsBuiltIn(k.apiVersion, k.kind) {
			t.Errorf("%s %s after a reset: ScopeIsBuiltIn is false, want the built-in answer", k.apiVersion, k.kind)
		}
	}
}

// TestDeclaredKind_ReadsAKindAsKyamlDoes: the group-version-kind extension of a
// path, read as kyaml's toTypeMeta reads it.
func TestDeclaredKind_ReadsAKindAsKyamlDoes(t *testing.T) {
	for _, tc := range []struct {
		name     string
		ext      any
		apiVer   string
		kind     string
		declared bool
	}{
		{"grouped", map[string]any{"group": "example.com", "version": "v1", "kind": "Widget"}, "example.com/v1", "Widget", true},
		{"core group", map[string]any{"group": "", "version": "v1", "kind": "Pod"}, "v1", "Pod", true},
		{"no group key", map[string]any{"version": "v1", "kind": "Pod"}, "v1", "Pod", true},
		{"not a map", "Widget", "", "", false},
		{"no kind", map[string]any{"group": "example.com", "version": "v1"}, "", "", false},
		{"no version", map[string]any{"group": "example.com", "kind": "Widget"}, "", "", false},
		{"nil", nil, "", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tm, ok := declaredKind(tc.ext)
			if ok != tc.declared || tm.APIVersion != tc.apiVer || tm.Kind != tc.kind {
				t.Errorf("declaredKind(%v) = %s %s, %v; want %s %s, %v", tc.ext, tm.APIVersion, tm.Kind, ok, tc.apiVer, tc.kind, tc.declared)
			}
		})
	}
}
