package kuretest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/go-kure/kure/internal/crds"
	"github.com/go-kure/kure/pkg/kubernetes"
)

// Every registered kind is either validated against the definition its
// pinned module ships, or named here as uncovered with the reason. The table
// is held to the registry in both directions, and an uncovered module is
// held to shipping nothing: the day it ships definitions, this says so.
func TestEveryKindHasOneSource(t *testing.T) {
	v, err := validator()
	if err != nil {
		t.Fatal(err)
	}
	dirs, err := moduleDirs()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, k := range kubernetes.Kinds() {
		seen[k.Module] = true
		src, ok := modules[k.Module]
		if !ok {
			t.Errorf("%s/%s %s: module %s is not in the source table", k.Group, k.Version, k.Kind, k.Module)
			continue
		}
		set := 0
		for _, on := range []bool{src.Dir != "", src.Bundle, src.Uncovered != ""} {
			if on {
				set++
			}
		}
		if set != 1 {
			t.Errorf("%s: the source table entry sets %d of Dir, Bundle and Uncovered, want exactly one", k.Module, set)
		}
		gvk := schema.GroupVersionKind{Group: k.Group, Version: k.Version, Kind: k.Kind}
		if src.Uncovered != "" {
			if k.ScopeSource != kubernetes.ScopeSourceBuiltin {
				if found, err := crds.Definitions(dirs[k.Module]); err != nil || len(found) != 0 {
					t.Errorf("%s is uncovered (%s), but its module ships %d definitions (err %v): read them", gvk, src.Uncovered, len(found), err)
				}
			}
			if v.Definition(gvk.GroupKind()) != nil {
				t.Errorf("%s is uncovered, yet a definition for it was read", gvk)
			}
			continue
		}
		crd := v.Definition(gvk.GroupKind())
		if crd == nil {
			t.Errorf("%s: no definition read from %s", gvk, k.Module)
			continue
		}
		if err := v.Compile(context.Background(), gvk); err != nil {
			t.Errorf("%s: %v", gvk, err)
		}
		if namespaced := crd.Spec.Scope == apiextensionsv1.NamespaceScoped; namespaced != k.Namespaced {
			t.Errorf("%s: the definition says namespaced=%v, the kinds table says %v", gvk, namespaced, k.Namespaced)
		}
	}
	for module := range modules {
		if !seen[module] {
			t.Errorf("the source table names %s, which no registered kind comes from", module)
		}
	}
}

// The two defects the 2026-10-01 investigation found in the shipped
// fixtures, kept as they were: the validator must keep rejecting them.
func TestRejectsTheObjectStoreServerNameTheCRDForbids(t *testing.T) {
	findings := fixture(t, "objectstore.yaml")
	if len(findings) != 1 || !strings.Contains(findings[0].errs.ToAggregate().Error(), "serverName") {
		t.Fatalf("want one finding naming serverName, got:\n%s", format(findings))
	}
}

func TestRejectsTheLocalRedirectPolicyProtocolTheCRDDoesNotAllow(t *testing.T) {
	findings := fixture(t, "ciliumlocalredirectpolicy.yaml")
	if len(findings) != 1 {
		t.Fatalf("want one finding, got:\n%s", format(findings))
	}
	msg := findings[0].errs.ToAggregate().Error()
	for _, want := range []string{`spec.redirectBackend.toPorts[0].protocol: Unsupported value: "ANY"`, `spec.redirectFrontend.addressMatcher.toPorts[0].protocol: Unsupported value: "ANY"`} {
		if !strings.Contains(msg, want) {
			t.Errorf("finding must contain %q:\n%s", want, msg)
		}
	}
}

func fixture(t *testing.T, name string) []finding {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	findings, _, err := validateYAML(data)
	if err != nil {
		t.Fatal(err)
	}
	return findings
}

func TestValidateYAMLHoldsBuiltInKindsToObjectMeta(t *testing.T) {
	findings, _, err := validateYAML([]byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: Bad_Name\n  namespace: web\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 || !strings.Contains(format(findings), "metadata.name: Invalid value") {
		t.Errorf("want the name rejected, got:\n%s", format(findings))
	}
	findings, _, err = validateYAML([]byte("apiVersion: rbac.authorization.k8s.io/v1\nkind: ClusterRole\nmetadata:\n  name: reader\n  namespace: web\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 || !strings.Contains(format(findings), "metadata.namespace: Forbidden") {
		t.Errorf("want the namespace on a cluster-scoped kind rejected, got:\n%s", format(findings))
	}
}

// A built-in kind's name is held to the rule the apiserver's validation for
// that kind passes to ValidateObjectMeta, not to the custom-resource
// default: a path segment for the RBAC kinds and a PodDisruptionBudget, a
// DNS label for a Namespace, a Service and a StatefulSet, a canonical IP
// for an IPAddress, the subdomain capped at 52 for a CronJob, plural.group
// for a definition.
func TestValidateYAMLHoldsBuiltInKindsToTheirNameRule(t *testing.T) {
	const crdHead = "apiVersion: apiextensions.k8s.io/v1\nkind: CustomResourceDefinition\nmetadata:\n  name: %s\nspec:\n  group: example.com\n  names:\n    kind: Widget\n    plural: widgets\n"
	for name, c := range map[string]struct{ doc, want string }{
		"rbac path segment":          {"apiVersion: rbac.authorization.k8s.io/v1\nkind: ClusterRole\nmetadata:\n  name: system:reader\n", ""},
		"rbac binding path segment":  {"apiVersion: rbac.authorization.k8s.io/v1\nkind: RoleBinding\nmetadata:\n  name: system:reader\n", ""},
		"rbac slash":                 {"apiVersion: rbac.authorization.k8s.io/v1\nkind: Role\nmetadata:\n  name: a/b\n", "metadata.name: Invalid value"},
		"pdb path segment":           {"apiVersion: policy/v1\nkind: PodDisruptionBudget\nmetadata:\n  name: Not_A_Subdomain\n", ""},
		"pdb slash":                  {"apiVersion: policy/v1\nkind: PodDisruptionBudget\nmetadata:\n  name: a/b\n", "metadata.name: Invalid value"},
		"namespace label":            {"apiVersion: v1\nkind: Namespace\nmetadata:\n  name: a.b\n", "metadata.name: Invalid value"},
		"namespace ok":               {"apiVersion: v1\nkind: Namespace\nmetadata:\n  name: a-b\n", ""},
		"service label":              {"apiVersion: v1\nkind: Service\nmetadata:\n  name: a.b\n", "metadata.name: Invalid value"},
		"statefulset label":          {"apiVersion: apps/v1\nkind: StatefulSet\nmetadata:\n  name: a.b\n", "metadata.name: Invalid value"},
		"deployment subdomain":       {"apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: a.b\n", ""},
		"cronjob 52":                 {"apiVersion: batch/v1\nkind: CronJob\nmetadata:\n  name: " + strings.Repeat("a", 53) + "\n", "must be no more than 52 characters"},
		"cronjob ok":                 {"apiVersion: batch/v1\nkind: CronJob\nmetadata:\n  name: " + strings.Repeat("a", 52) + "\n", ""},
		"ipaddress ip":               {"apiVersion: networking.k8s.io/v1\nkind: IPAddress\nmetadata:\n  name: not-an-ip\n", "metadata.name: Invalid value"},
		"ipaddress canonical":        {"apiVersion: networking.k8s.io/v1\nkind: IPAddress\nmetadata:\n  name: 2001:db8::0001\n", "must be in canonical form"},
		"ipaddress ok":               {"apiVersion: networking.k8s.io/v1\nkind: IPAddress\nmetadata:\n  name: 10.0.0.1\n", ""},
		"definition plural.group":    {fmt.Sprintf(crdHead, "widget.example.com"), "metadata.name: Invalid value"},
		"definition plural.group ok": {fmt.Sprintf(crdHead, "widgets.example.com"), ""},
	} {
		t.Run(name, func(t *testing.T) {
			findings, _, err := validateYAML([]byte(c.doc))
			if err != nil {
				t.Fatal(err)
			}
			if c.want == "" && len(findings) != 0 {
				t.Errorf("want no finding, got:\n%s", format(findings))
			}
			if c.want != "" && (len(findings) != 1 || !strings.Contains(format(findings), c.want)) {
				t.Errorf("want one finding with %q, got:\n%s", c.want, format(findings))
			}
		})
	}
}

func TestValidateYAMLReadsEveryDocument(t *testing.T) {
	doc := strings.Join([]string{
		"apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\nresources: [a.yaml]\n",
		"",
		"apiVersion: v1\nkind: List\nitems:\n- apiVersion: v1\n  kind: ConfigMap\n  metadata:\n    name: ok\n- apiVersion: v1\n  kind: ConfigMap\n  metadata:\n    name: Bad_Name\n",
		"apiVersion: example.com/v1\nkind: Gizmo\nmetadata:\n  name: g\n",
	}, "---\n")
	findings, _, err := validateYAML([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	got := format(findings)
	// objects are numbered as decoded: the list's items are 1 and 2
	for _, want := range []string{"object 2: ConfigMap Bad_Name", "object 3: Gizmo g", "is not a kind kure registers"} {
		if !strings.Contains(got, want) {
			t.Errorf("want %q in:\n%s", want, got)
		}
	}
	if len(findings) != 2 {
		t.Errorf("want 2 findings (the kustomization is skipped, the first item is valid), got:\n%s", got)
	}
	if _, _, err := validateYAML([]byte("apiVersion: v1\nkind: ConfigMap\nmetadata: [not a mapping\n")); err == nil {
		t.Error("a document that does not decode must be an error, not a pass")
	}
}

// The server's decoder takes any document with an items field for a list.
// An object is a list only when its kind says so; otherwise items is a
// field of the object, and the object is validated, not skipped.
func TestDecodeKeepsItemsOnAnObject(t *testing.T) {
	findings, _, err := validateYAML([]byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: Bad_Name\nitems: []\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 || !strings.Contains(format(findings), "object 0: ConfigMap Bad_Name") {
		t.Errorf("want the object validated, got:\n%s", format(findings))
	}
	findings, _, err = validateYAML([]byte("apiVersion: v1\nkind: ConfigMapList\nitems:\n- apiVersion: v1\n  kind: ConfigMap\n  metadata:\n    name: Bad_Name\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 || !strings.Contains(format(findings), "object 0: ConfigMap Bad_Name") {
		t.Errorf("want the list's item validated, got:\n%s", format(findings))
	}
}

// kustomize reads a List kind without an items field as a resource of its
// own, which a cluster then refuses; one whose items is null is an empty
// list. This reads both the same way.
func TestDecodeKeepsAListKindWithoutItems(t *testing.T) {
	findings, objects, err := validateYAML([]byte("apiVersion: v1\nkind: ConfigMapList\nmetadata:\n  name: x\n"))
	if err != nil {
		t.Fatal(err)
	}
	if objects != 1 || len(findings) != 1 || !strings.Contains(format(findings), "ConfigMapList is not a kind kure registers") {
		t.Errorf("want the document validated as an object, got %d object(s):\n%s", objects, format(findings))
	}
	findings, objects, err = validateYAML([]byte("apiVersion: v1\nkind: ConfigMapList\nitems: null\n"))
	if err != nil || objects != 0 || len(findings) != 0 {
		t.Errorf("want an empty list read past: %d object(s), %v, err %v", objects, findings, err)
	}
}

// fakeTB records the one fatal failure a helper reports and stops it the
// way testing does, so the helper's behaviour after a failure can be seen.
type fakeTB struct {
	testing.TB
	fatal string
	errs  []string
}

func (f *fakeTB) Helper() {}
func (f *fakeTB) Fatalf(format string, args ...any) {
	f.fatal = fmt.Sprintf(format, args...)
	panic(f)
}
func (f *fakeTB) Errorf(format string, args ...any) {
	f.errs = append(f.errs, fmt.Sprintf(format, args...))
}

func run(fn func(tb testing.TB)) (f *fakeTB) {
	f = &fakeTB{}
	defer func() {
		if r := recover(); r != nil && r != f {
			panic(r)
		}
	}()
	fn(f)
	return f
}

func configMap(name string) *corev1.ConfigMap {
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "web"}}
	cm.SetGroupVersionKind(schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"})
	return cm
}

func TestGoldenNeverWritesAnInvalidFixture(t *testing.T) {
	path := filepath.Join("testdata", "never-written.yaml")
	t.Cleanup(func() { _ = os.Remove(path) })
	*update = true
	t.Cleanup(func() { *update = false })

	f := run(func(tb testing.TB) { Golden(tb, "never-written.yaml", configMap("Bad_Name")) })
	if !strings.Contains(f.fatal, "metadata.name") {
		t.Errorf("Golden did not fail on the invalid object: %q", f.fatal)
	}
	if _, err := os.Stat(path); err == nil {
		t.Errorf("Golden wrote %s under -update for an invalid object", path)
	}
}

func TestGoldenComparesWithTheFixture(t *testing.T) {
	path := filepath.Join("testdata", "configmap.yaml")
	t.Cleanup(func() { _ = os.Remove(path) })
	*update = true
	run(func(tb testing.TB) { Golden(tb, "configmap.yaml", configMap("a")) })
	*update = false
	if f := run(func(tb testing.TB) { Golden(tb, "configmap.yaml", configMap("a")) }); f.fatal != "" || len(f.errs) != 0 {
		t.Errorf("the object it was written from does not match: %q %q", f.fatal, f.errs)
	}
	if f := run(func(tb testing.TB) { Golden(tb, "configmap.yaml", configMap("b")) }); len(f.errs) != 1 || !strings.Contains(f.errs[0], "does not match golden file") {
		t.Errorf("a different object matched: %q %q", f.fatal, f.errs)
	}
}

func TestAssertValid(t *testing.T) {
	if f := run(func(tb testing.TB) { AssertValid(tb, configMap("a"), configMap("b")) }); f.fatal != "" {
		t.Errorf("valid objects failed: %q", f.fatal)
	}
	if f := run(func(tb testing.TB) { AssertValid(tb, configMap("a"), configMap("Bad_Name")) }); !strings.Contains(f.fatal, "object 1: ConfigMap web/Bad_Name") {
		t.Errorf("want the second object named, got %q", f.fatal)
	}
}

func TestAssertValidDir(t *testing.T) {
	dir := t.TempDir()
	if f := run(func(tb testing.TB) { AssertValidDir(tb, dir) }); !strings.Contains(f.fatal, "no objects") {
		t.Errorf("an empty directory must fail, got %q", f.fatal)
	}
	// a manifest with nothing in it is not a manifest a pass could be about
	if err := os.WriteFile(filepath.Join(dir, "empty.yaml"), []byte("# nothing\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if f := run(func(tb testing.TB) { AssertValidDir(tb, dir) }); !strings.Contains(f.fatal, "no objects") {
		t.Errorf("a directory of empty manifests must fail, got %q", f.fatal)
	}
	for name, body := range map[string]string{
		"ok.yaml":            "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: ok\n",
		"kustomization.yaml": "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\nresources: [ok.yaml]\n",
		"sub/bad.yml":        "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: Bad_Name\n",
		"notes.txt":          "not read",
		"sub/README.md":      "not read",
	} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	f := run(func(tb testing.TB) { AssertValidDir(tb, dir) })
	if !strings.Contains(f.fatal, filepath.Join(dir, "sub", "bad.yml")+": object 0: ConfigMap Bad_Name") || strings.Contains(f.fatal, "ok.yaml") {
		t.Errorf("want only bad.yml named, got %q", f.fatal)
	}
}

// What stops validation itself is a failure, never a pass.
func TestAssertValidDirFailsOnWhatItCannotRead(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	if f := run(func(tb testing.TB) { AssertValidDir(tb, missing) }); !strings.Contains(f.fatal, missing) {
		t.Errorf("a directory that does not exist must fail, got %q", f.fatal)
	}
	broken := t.TempDir()
	if err := os.WriteFile(filepath.Join(broken, "broken.yaml"), []byte("metadata: [not a mapping\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if f := run(func(tb testing.TB) { AssertValidDir(tb, broken) }); !strings.Contains(f.fatal, "broken.yaml") || !strings.Contains(f.fatal, "document 0") {
		t.Errorf("a manifest that does not decode must fail, got %q", f.fatal)
	}
	unreadable := t.TempDir()
	if err := os.Symlink(filepath.Join(unreadable, "gone.yaml"), filepath.Join(unreadable, "dangling.yaml")); err != nil {
		t.Fatal(err)
	}
	if f := run(func(tb testing.TB) { AssertValidDir(tb, unreadable) }); !strings.Contains(f.fatal, "dangling.yaml") {
		t.Errorf("a manifest that cannot be read must fail, got %q", f.fatal)
	}
}

func TestAssertValidYAMLFailsOnWhatDoesNotDecode(t *testing.T) {
	if f := run(func(tb testing.TB) { AssertValidYAML(tb, []byte("metadata: [not a mapping\n")) }); !strings.Contains(f.fatal, "document 0") {
		t.Errorf("want the decode error, got %q", f.fatal)
	}
}

// Nothing validated is not a pass: no document, or only kustomize's own
// configuration, which is read past.
func TestAssertValidYAMLFailsOnNoObjects(t *testing.T) {
	for name, doc := range map[string]string{
		"empty":              "",
		"a comment":          "# nothing\n---\n",
		"kustomization only": "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\nresources: [a.yaml]\n",
	} {
		if f := run(func(tb testing.TB) { AssertValidYAML(tb, []byte(doc)) }); !strings.Contains(f.fatal, "no objects") {
			t.Errorf("%s: want no objects reported, got %q", name, f.fatal)
		}
	}
}

func TestGoldenReportsAFixtureItCannotReadOrWrite(t *testing.T) {
	f := run(func(tb testing.TB) { Golden(tb, "no-such-fixture.yaml", configMap("a")) })
	if !strings.Contains(f.fatal, "run with -update to create") {
		t.Errorf("want the missing fixture reported, got %q", f.fatal)
	}
	*update = true
	t.Cleanup(func() { *update = false })
	// a fixture under a path that is a file, not a directory
	f = run(func(tb testing.TB) { Golden(tb, filepath.Join("objectstore.yaml", "x.yaml"), configMap("a")) })
	if !strings.Contains(f.fatal, "creating the golden directory") {
		t.Errorf("want the write failure reported, got %q", f.fatal)
	}
}

// A package's first golden test has no testdata directory yet; -update
// creates it, as the helpers this one replaced did.
func TestGoldenCreatesTheFixtureDirectory(t *testing.T) {
	dir := filepath.Join("testdata", "new-dir")
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	*update = true
	t.Cleanup(func() { *update = false })
	if f := run(func(tb testing.TB) { Golden(tb, filepath.Join("new-dir", "x.yaml"), configMap("a")) }); f.fatal != "" {
		t.Fatalf("Golden did not create the directory under -update: %q", f.fatal)
	}
	if _, err := os.Stat(filepath.Join(dir, "x.yaml")); err != nil {
		t.Error(err)
	}
}

func TestDecodeRejectsADocumentThatIsNotAnObject(t *testing.T) {
	for _, doc := range []string{"k: v\n", "- a\n- b\n"} {
		if _, _, err := validateYAML([]byte(doc)); err == nil {
			t.Errorf("%q passed as an object", doc)
		}
	}
	// a document holding only a comment is nothing to validate
	findings, _, err := validateYAML([]byte("# nothing here\n---\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: ok\n"))
	if err != nil || len(findings) != 0 {
		t.Errorf("a comment-only document: findings %v, err %v", findings, err)
	}
}
