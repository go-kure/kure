package crds

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// write puts one file in dir, creating parent directories as needed, and
// returns dir so a test reads as a single expression.
func write(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

const clusterCRD = `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: widgets.example.com
spec:
  group: example.com
  names:
    kind: Widget
    plural: widgets
  scope: Cluster
`

const namespacedCRD = `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: gadgets.example.com
spec:
  group: example.com
  names:
    kind: Gadget
  scope: Namespaced
`

func TestLoadIndexesTheShippedDefinitions(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "config/crd/bases/widget.yaml", clusterCRD)
	write(t, dir, "config/crd/bases/gadget.yml", namespacedCRD)

	index, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := index["example.com/Widget"]; got != ScopeCluster {
		t.Errorf("Widget = %q, want %q", got, ScopeCluster)
	}
	if got := index["example.com/Gadget"]; got != ScopeNamespaced {
		t.Errorf("Gadget = %q, want %q", got, ScopeNamespaced)
	}
	if len(index) != 2 {
		t.Errorf("index has %d entries, want 2", len(index))
	}
}

// A multi-document manifest is how a module that ships one file for all its
// CRDs is laid out.
func TestLoadReadsEveryDocumentInAManifest(t *testing.T) {
	dir := write(t, t.TempDir(), "crds.yaml", clusterCRD+"---\n"+namespacedCRD)
	index, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(index) != 2 {
		t.Errorf("index = %v, want both definitions", index)
	}
}

// A kustomize patch fragment carries the CRD's apiVersion, kind and name and
// nothing else. It defines nothing, so it is neither indexed nor an error —
// cnpg ships one per kind, and rejecting them broke the whole walk.
func TestLoadIgnoresPatchFragments(t *testing.T) {
	dir := write(t, t.TempDir(), "config/crd/patches/cainjection.yaml", `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  annotations:
    cert-manager.io/inject-ca-from: $(NS)/$(NAME)
  name: widgets.example.com
`)
	index, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(index) != 0 {
		t.Errorf("index = %v, want nothing from a patch fragment", index)
	}
}

// A definition that does name a kind but declares no scope is a manifest this
// package must not guess about.
func TestLoadRejectsADefinitionWithNoScope(t *testing.T) {
	dir := write(t, t.TempDir(), "crd.yaml", `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: widgets.example.com
spec:
  group: example.com
  names:
    kind: Widget
`)
	_, err := Load(dir)
	if err == nil {
		t.Fatal("Load accepted a definition with no scope")
	}
	if !strings.Contains(err.Error(), "unrecognised spec.scope") {
		t.Errorf("error = %v, want it to name the scope", err)
	}
}

func TestLoadRejectsConflictingScopes(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "a.yaml", clusterCRD)
	write(t, dir, "b.yaml", strings.Replace(clusterCRD, "scope: Cluster", "scope: Namespaced", 1))
	_, err := Load(dir)
	if err == nil {
		t.Fatal("Load accepted two definitions disagreeing about scope")
	}
	if !strings.Contains(err.Error(), "example.com/Widget") {
		t.Errorf("error = %v, want it to name the kind", err)
	}
}

// Two definitions agreeing is ordinary: a module can ship the same CRD in a
// bundle and under config/crd/bases.
func TestLoadAcceptsDuplicateAgreeingDefinitions(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "bundle.yaml", clusterCRD)
	write(t, dir, "config/crd/bases/widget.yaml", clusterCRD)
	index, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(index) != 1 || index["example.com/Widget"] != ScopeCluster {
		t.Errorf("index = %v, want one cluster-scoped Widget", index)
	}
}

func TestLoadSkipsVendorAndTestdata(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "vendor/other/crd.yaml", clusterCRD)
	write(t, dir, "testdata/broken.yaml", `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
spec:
  group: example.com
  names:
    kind: Widget
  scope: Nonsense
`)
	index, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(index) != 0 {
		t.Errorf("index = %v, want nothing from vendor or testdata", index)
	}
}

// A Helm template is not a manifest. It must neither fail the walk nor be read
// as a definition.
func TestLoadIgnoresAHelmTemplate(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "templates/crd.yaml", "{{- if .Values.crds }}\nkind: CustomResourceDefinition\n{{- end }}\n")
	write(t, dir, "good.yaml", namespacedCRD)
	index, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(index) != 1 {
		t.Errorf("index = %v, want only the readable definition", index)
	}
}

// The shape metallb ships: a templated file whose leading documents are
// ordinary CRDs and whose later one is not yaml at all. Reading it would stop
// mid-file and take the leading documents as authoritative, which is a scope
// sourced from a file that could only be read in part. The whole file is
// skipped instead — and it must be skipped without an error, since a module
// shipping a Helm chart beside its real manifests is ordinary.
func TestLoadSkipsATemplatedFileEvenWhereItsFirstDocumentsAreValid(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "charts/x/templates/crds.yaml", namespacedCRD+"---\nkind: Namespace\nmetadata:\n  name: {{ .Release.Namespace }}\n")
	index, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(index) != 0 {
		t.Errorf("index = %v, want nothing read out of a templated file", index)
	}
}

// A document that does not decode, in a file that really does declare a CRD and
// is not a template, is an error naming the file and the document's index.
// Stopping quietly there drops every definition after it — which loses a kind's
// only answer, or one half of a scope conflict — with nothing to show that
// anything was skipped.
func TestLoadRejectsAnUndecodableDocument(t *testing.T) {
	const bad = "apiVersion: apiextensions.k8s.io/v1\nkind: CustomResourceDefinition\nspec: [this: is, not: a, mapping\n"
	cases := []struct {
		name, body string
		wantIndex  int
	}{
		{"ahead of a good definition", bad + "---\n" + namespacedCRD, 0},
		{"behind a good definition", namespacedCRD + "---\n" + bad, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			write(t, dir, "crd.yaml", c.body)
			_, err := Load(dir)
			if err == nil {
				t.Fatal("an undecodable document must fail the walk, not truncate it")
			}
			for _, want := range []string{"crd.yaml", "document " + strconv.Itoa(c.wantIndex)} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error must name %q: %v", want, err)
				}
			}
		})
	}
}

// The prefilter selects on the document's own kind key, not the bare word. A
// substring match also selects every template and doc that mentions it — files
// that cannot decode and would now fail the walk.
func TestDeclaresCRD(t *testing.T) {
	cases := map[string]bool{
		"kind: CustomResourceDefinition\n":                      true,
		"apiVersion: v1\nkind: CustomResourceDefinition\n":      true,
		"apiVersion: v1\nkind: \"CustomResourceDefinition\"\n":  true,
		"apiVersion: v1\nkind: 'CustomResourceDefinition'\n":    true,
		"# applies to every CustomResourceDefinition\n":         false,
		"spec:\n  names:\n    kind: CustomResourceDefinition\n": false,
		"": false,
	}
	for body, want := range cases {
		if got := declaresCRD([]byte(body)); got != want {
			t.Errorf("declaresCRD(%q) = %v, want %v", body, got, want)
		}
	}
}

// Non-yaml files, and yaml that never mentions a CRD, are skipped without
// being decoded.
func TestLoadIgnoresIrrelevantFiles(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "README.md", "kind: CustomResourceDefinition\n")
	write(t, dir, "config.yaml", "apiVersion: v1\nkind: ConfigMap\n")
	index, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(index) != 0 {
		t.Errorf("index = %v, want nothing", index)
	}
}

func TestLoadOnNoDirectory(t *testing.T) {
	index, err := Load("")
	if err != nil {
		t.Fatalf("Load(\"\"): %v", err)
	}
	if len(index) != 0 {
		t.Errorf("index = %v, want empty", index)
	}
	if _, err := Load(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Error("Load accepted a directory that does not exist")
	}
}

// An apiextensions kind is required: something else's CustomResourceDefinition
// is not this API's.
func TestLoadIgnoresAForeignAPIGroup(t *testing.T) {
	dir := write(t, t.TempDir(), "crd.yaml", strings.Replace(clusterCRD,
		"apiVersion: apiextensions.k8s.io/v1", "apiVersion: example.io/v1", 1))
	index, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(index) != 0 {
		t.Errorf("index = %v, want nothing", index)
	}
}
