package upstream

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// parseTypes runs the collector over inline source, the way Load runs it over a
// pinned module's files.
func parseTypes(t *testing.T, src string) map[string]Type {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "types.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out := map[string]Type{}
	collectTypes(file, "example.com/api/v1", "example.com", "v1.2.3", out)
	return out
}

// The layout every CRD module kure pins actually uses: a marker block, a blank
// line, then the type's prose doc comment. The blank line detaches the markers
// from GenDecl.Doc, so a collector that reads Doc alone sees none of them.
const detachedLayout = `package v1

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster

// ClusterImageCatalog is the Schema for the clusterimagecatalogs API
type ClusterImageCatalog struct {
	Spec ImageCatalogSpec ` + "`json:\"spec\"`" + `
}
`

func TestDetachedMarkerBlockIsRead(t *testing.T) {
	types := parseTypes(t, detachedLayout)
	got, ok := types[Key("example.com/api/v1", "ClusterImageCatalog")]
	if !ok {
		t.Fatalf("type not collected; got %v", keysOf(types))
	}
	if !strings.Contains(got.Doc, "+kubebuilder:resource:scope=Cluster") {
		t.Errorf("doc = %q, want it to carry the detached marker", got.Doc)
	}
	if !strings.Contains(got.Doc, "is the Schema for") {
		t.Errorf("doc = %q, want it to also carry the prose comment", got.Doc)
	}
}

func TestAttachedMarkersAreRead(t *testing.T) {
	types := parseTypes(t, `package v1

// Issuer is a namespaced issuer.
// +kubebuilder:resource:scope=Namespaced
type Issuer struct{}
`)
	got := types[Key("example.com/api/v1", "Issuer")]
	if !strings.Contains(got.Doc, "+kubebuilder:resource:scope=Namespaced") {
		t.Errorf("doc = %q, want the attached marker", got.Doc)
	}
}

// A file's license header sits before the package clause and must never be
// mistaken for the first type's marker block.
func TestLicenseHeaderIsNotTreatedAsMarkers(t *testing.T) {
	types := parseTypes(t, `/*
Copyright the authors.
+kubebuilder:resource:scope=Cluster
*/

package v1

// Thing is a thing.
type Thing struct{}
`)
	got := types[Key("example.com/api/v1", "Thing")]
	if strings.Contains(got.Doc, "scope=Cluster") {
		t.Errorf("doc = %q, want the license header excluded", got.Doc)
	}
}

// A preceding marker block cannot be attributed to one spec of a grouped
// declaration, so it is dropped rather than applied to all of them.
func TestGroupedDeclarationDoesNotInheritDetachedMarkers(t *testing.T) {
	types := parseTypes(t, `package v1

// +kubebuilder:resource:scope=Cluster

type (
	// A is a thing.
	A struct{}
	// B is another.
	B struct{}
)
`)
	for _, name := range []string{"A", "B"} {
		if got := types[Key("example.com/api/v1", name)]; strings.Contains(got.Doc, "scope=Cluster") {
			t.Errorf("%s doc = %q, want no inherited marker", name, got.Doc)
		}
	}
}

// Markers must not leak from the type declared above to the one below it.
func TestMarkersDoNotLeakToTheNextType(t *testing.T) {
	types := parseTypes(t, `package v1

// +kubebuilder:resource:scope=Cluster

// First is cluster-scoped.
type First struct{}

// Second is namespaced.
type Second struct{}
`)
	if got := types[Key("example.com/api/v1", "First")]; !strings.Contains(got.Doc, "scope=Cluster") {
		t.Errorf("First doc = %q, want the marker", got.Doc)
	}
	if got := types[Key("example.com/api/v1", "Second")]; strings.Contains(got.Doc, "scope=Cluster") {
		t.Errorf("Second doc = %q, want no leaked marker", got.Doc)
	}
}

func TestFieldsAndTags(t *testing.T) {
	types := parseTypes(t, `package v1

type Spec struct {
	TypeMeta `+"`json:\",inline\"`"+`
	// Replicas is the count.
	// +featureGate=Example
	Replicas *int32 `+"`json:\"replicas,omitempty\"`"+`
	Names    []string `+"`json:\"names\"`"+`
	Labels   map[string]string `+"`json:\"labels\"`"+`
	Ref      meta.ObjectReference `+"`json:\"ref\"`"+`
	Untagged string
	A, B     int `+"`json:\"ab\"`"+`
}
`)
	spec := types[Key("example.com/api/v1", "Spec")]
	byName := map[string]Field{}
	for _, f := range spec.Fields {
		byName[f.Name] = f
	}
	for _, tc := range []struct {
		field, jsonName, typeExpr string
		inline                    bool
	}{
		{"TypeMeta", "", "TypeMeta", true},
		{"Replicas", "replicas", "*int32", false},
		{"Names", "names", "[]string", false},
		{"Labels", "labels", "map[string]string", false},
		{"Ref", "ref", "meta.ObjectReference", false},
		{"Untagged", "", "string", false},
		{"A", "ab", "int", false},
		{"B", "ab", "int", false},
	} {
		f, ok := byName[tc.field]
		if !ok {
			t.Errorf("field %s not collected", tc.field)
			continue
		}
		if f.JSONName != tc.jsonName || f.TypeExpr != tc.typeExpr || f.Inline != tc.inline {
			t.Errorf("%s = {json:%q type:%q inline:%v}, want {json:%q type:%q inline:%v}",
				tc.field, f.JSONName, f.TypeExpr, f.Inline, tc.jsonName, tc.typeExpr, tc.inline)
		}
	}
	if !strings.Contains(byName["Replicas"].Doc, "+featureGate=Example") {
		t.Errorf("Replicas doc = %q, want the field marker", byName["Replicas"].Doc)
	}
}

// An embedded field's Go name is its type's name, without pointer or package
// qualifier — that is the name a maturity walk has to report.
func TestEmbeddedName(t *testing.T) {
	for _, tc := range []struct{ expr, want string }{
		{"TypeMeta", "TypeMeta"},
		{"*TypeMeta", "TypeMeta"},
		{"metav1.ObjectMeta", "ObjectMeta"},
		{"*metav1.ObjectMeta", "ObjectMeta"},
	} {
		if got := embeddedName(tc.expr); got != tc.want {
			t.Errorf("embeddedName(%q) = %q, want %q", tc.expr, got, tc.want)
		}
	}
}

func TestJSONTagEdgeCases(t *testing.T) {
	for _, tc := range []struct {
		name, tag, wantName string
		wantInline          bool
	}{
		{"no tag", "", "", false},
		{"no json key", "`yaml:\"x\"`", "", false},
		{"unterminated json value", "`json:\"x", "", false},
		{"name only", "`json:\"spec\"`", "spec", false},
		{"omitempty is not inline", "`json:\"spec,omitempty\"`", "spec", false},
		{"inline after omitempty", "`json:\"spec,omitempty,inline\"`", "spec", true},
		{"empty name inline", "`json:\",inline\"`", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var lit *ast.BasicLit
			if tc.tag != "" {
				lit = &ast.BasicLit{Kind: token.STRING, Value: tc.tag}
			}
			name, inline := jsonTag(lit)
			if name != tc.wantName || inline != tc.wantInline {
				t.Errorf("jsonTag(%q) = %q,%v want %q,%v", tc.tag, name, inline, tc.wantName, tc.wantInline)
			}
		})
	}
}

func TestExprStringUnsupportedYieldsEmpty(t *testing.T) {
	// A channel type is not something an API struct field uses; the renderer
	// returns "" rather than inventing a spelling.
	if got := exprString(&ast.ChanType{Value: ast.NewIdent("int")}); got != "" {
		t.Errorf("exprString(chan) = %q, want %q", got, "")
	}
	if got := exprString(&ast.InterfaceType{Methods: &ast.FieldList{}}); got != "interface{}" {
		t.Errorf("exprString(interface) = %q, want %q", got, "interface{}")
	}
}

func TestModuleProvenanceIsRecorded(t *testing.T) {
	types := parseTypes(t, "package v1\n\ntype Thing struct{}\n")
	got := types[Key("example.com/api/v1", "Thing")]
	if got.Module != "example.com" || got.Version != "v1.2.3" || got.ImportPath != "example.com/api/v1" {
		t.Errorf("provenance = %s@%s (%s), want example.com@v1.2.3 (example.com/api/v1)", got.Module, got.Version, got.ImportPath)
	}
}

// Non-type declarations advance the scan bound; a const block between two types
// must not let the first type's markers reach the second.
func TestNonTypeDeclarationsAdvanceTheBound(t *testing.T) {
	types := parseTypes(t, `package v1

// +kubebuilder:resource:scope=Cluster

// First is cluster-scoped.
type First struct{}

const X = 1

// Second is namespaced.
type Second struct{}
`)
	if got := types[Key("example.com/api/v1", "Second")]; strings.Contains(got.Doc, "scope=Cluster") {
		t.Errorf("Second doc = %q, want no leaked marker", got.Doc)
	}
}

func TestLoadNoPathsIsNotAnError(t *testing.T) {
	got, err := Load(nil)
	if err != nil {
		t.Fatalf("Load(nil): %v", err)
	}
	if len(got) != 0 {
		t.Errorf("Load(nil) = %d types, want 0", len(got))
	}
}

func TestLoadRejectsAMissingPackage(t *testing.T) {
	if _, err := Load([]string{"example.invalid/does/not/exist"}); err == nil {
		t.Fatal("Load accepted a package that does not exist")
	}
}

// Load against a real pinned module: the path this package exists to serve.
func TestLoadReadsAPinnedModule(t *testing.T) {
	const path = "k8s.io/api/core/v1"
	types, err := Load([]string{path})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	pod, ok := types[Key(path, "Pod")]
	if !ok {
		t.Fatalf("Pod not found; loaded %d types", len(types))
	}
	if pod.Module != "k8s.io/api" || pod.Version == "" {
		t.Errorf("Pod provenance = %s@%s, want k8s.io/api@<version>", pod.Module, pod.Version)
	}
	if len(pod.Fields) == 0 {
		t.Error("Pod has no fields")
	}
}

// The import map is what lets a field written as metav1.ObjectMeta be resolved
// to the package that declares it. Aliases are file-scoped, and an import with
// no alias is referred to by its last path segment.
func TestFileImportsRecordsAliases(t *testing.T) {
	types := parseTypes(t, `package v1

import (
	"strings"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/apps/v1"
)

type Thing struct {
	Meta metav1.ObjectMeta ` + "`json:\"metadata\"`" + `
}
`)
	got := types[Key("example.com/api/v1", "Thing")].Imports
	for _, tc := range []struct{ alias, want string }{
		{"metav1", "k8s.io/apimachinery/pkg/apis/meta/v1"},
		{"corev1", "k8s.io/api/core/v1"},
		{"v1", "k8s.io/api/apps/v1"},
		{"strings", "strings"},
	} {
		if got[tc.alias] != tc.want {
			t.Errorf("imports[%q] = %q, want %q", tc.alias, got[tc.alias], tc.want)
		}
	}
}

func keysOf(m map[string]Type) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
