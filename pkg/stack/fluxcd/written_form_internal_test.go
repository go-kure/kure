package fluxcd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kustomize/api/resource"
	"sigs.k8s.io/kustomize/kyaml/filesys"

	"github.com/go-kure/kure/internal/kustomizeschema"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// lastApplied is the annotation kubectl writes, which the tree's writers
// strip from a resource's top-level object and nowhere else.
const lastApplied = "kubectl.kubernetes.io/last-applied-configuration"

// Invariant: every object the view reads as the tree holds it (heldBuild) is
// the object kustomize reads from the file the tree's writers write for it.
// The corpus holds each field the writers strip or rewrite (server fields, a
// last-applied annotation carrying a line break YAML reads, a null creation
// timestamp, an empty status, nested template metadata) and each value whose
// YAML form differs from its Go form (NEL, U+2028, numeric-looking strings,
// integers, floats), and a List, whose items the writers write as they
// marshal. A field the view reads that the file does not hold, or the
// reverse, fails here, whichever check reads it.
func TestHeldBuild_ReadsTheWrittenTree(t *testing.T) {
	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
		ObjectMeta: metav1.ObjectMeta{Name: "cm", Namespace: "default", ResourceVersion: "7", UID: "u", Generation: 2,
			ManagedFields: []metav1.ManagedFieldsEntry{{Manager: "m"}},
			Annotations: map[string]string{
				lastApplied: "{}\r  kustomize.toolkit.fluxcd.io/ssa: Ignore",
				"note":      "a\u0085b c",
			}},
		Data: map[string]string{"n": "0123", "b": "yes", "f": "1.50"},
	}
	item := &corev1.ConfigMap{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
		ObjectMeta: metav1.ObjectMeta{Name: "item", Namespace: "default", Annotations: map[string]string{lastApplied: "{}"}},
	}
	dep := &appsv1.Deployment{
		TypeMeta:   metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"},
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
		Spec: appsv1.DeploymentSpec{Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{lastApplied: "{}"}},
			Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "c", Image: "i",
				Ports: []corev1.ContainerPort{{ContainerPort: 80}}}}},
		}},
	}
	ks := &kustv1.Kustomization{
		TypeMeta:   metav1.TypeMeta{APIVersion: kustv1.GroupVersion.String(), Kind: kustv1.KustomizationKind},
		ObjectMeta: metav1.ObjectMeta{Name: "k", Namespace: "flux-system"},
		Spec: kustv1.KustomizationSpec{Path: "./x", Interval: metav1.Duration{Duration: time.Minute}, Prune: true,
			SourceRef: kustv1.CrossNamespaceSourceReference{Kind: "GitRepository", Name: "flux-system"}},
		Status: kustv1.KustomizationStatus{ObservedGeneration: 3},
	}
	thing := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "example.com/v1", "kind": "Thing",
		"metadata": map[string]any{"name": "t", "namespace": "default", "creationTimestamp": nil},
		"status":   map[string]any{},
		"spec":     map[string]any{"count": int64(3), "ratio": 1.5, "empty": ""},
	}}
	itemContent, err := runtime.DefaultUnstructuredConverter.ToUnstructured(item)
	if err != nil {
		t.Fatal(err)
	}
	list := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1", "kind": "List",
		"metadata": map[string]any{"name": "holder", "namespace": "default"},
		"items":    []any{itemContent},
	}}
	l := &layout.ManifestLayout{Name: "app", Namespace: ".", Resources: []client.Object{cm, dep, ks, thing, list}}

	held, err := heldBuild(l)
	if err != nil {
		t.Fatalf("heldBuild: %v", err)
	}
	disk := t.TempDir()
	if err := l.WriteToDisk(disk); err != nil {
		t.Fatalf("WriteToDisk: %v", err)
	}
	kustomizeschema.Parse()
	rm, err := kustomizeBuild(filesys.MakeFsOnDisk(), filepath.Join(disk, filepath.FromSlash(l.FullRepoPath())))
	if err != nil {
		t.Fatalf("kustomize build of the written tree: %v", err)
	}
	var got, want []string
	for _, obj := range held {
		content, err := writtenContent(obj)
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, canonicalJSON(t, content))
	}
	for _, res := range rm.Resources() {
		m, err := res.Map()
		if err != nil {
			t.Fatal(err)
		}
		want = append(want, canonicalJSON(t, m))
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("heldBuild reads another tree than the writers write:\n got %s\nwant %s", strings.Join(got, "\n    "), strings.Join(want, "\n     "))
	}
}

// canonicalJSON returns content as JSON with sorted keys and numbers as JSON
// writes them, so that one value read two ways compares equal.
func canonicalJSON(t *testing.T, content map[string]any) string {
	t.Helper()
	data, err := json.Marshal(content)
	if err != nil {
		t.Fatal(err)
	}
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatal(err)
	}
	if data, err = json.Marshal(v); err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// Invariant: a build that merges into a Secret is never read as one that does
// not. Each case is a component, loaded by base, that kustomize reads as a
// merge into base's Secret creds, whatever its YAML spells: the merge in a
// second entry, a key in another case, the value through an alias, the entry through a merge key, an empty
// list before the real one, the field in two cases, and a builtin
// SecretGenerator listed under generators. kustomize is the oracle: its build
// must change creds, or the case tests nothing. The guard must then report
// the component as a merge the build loads (buildGuard.merged). A
// component base does not load merges nothing.
func TestBuildGuard_FindsEveryMerge(t *testing.T) {
	const (
		head    = "apiVersion: kustomize.config.k8s.io/v1alpha1\nkind: Component\n"
		entry   = "  name: creds\n  namespace: default\n"
		literal = "  literals:\n  - k=v\n"
		merge   = "- name: creds\n  namespace: default\n  behavior: merge\n" + literal
	)
	cases := map[string]struct {
		component, gen string
		unloaded       bool
	}{
		"behavior merge":                 {component: head + "secretGenerator:\n" + merge},
		"a second entry":                 {component: head + "secretGenerator:\n- name: other\n  namespace: default\n" + literal + merge},
		"a key in another case":          {component: head + "secretGenerator:\n-" + entry[1:] + "  Behavior: merge\n" + literal},
		"the field in another case":      {component: head + "SecretGenerator:\n" + merge},
		"the value through an alias":     {component: head + "metadata:\n  annotations:\n    b: &m merge\nsecretGenerator:\n-" + entry[1:] + "  behavior: *m\n" + literal},
		"the entry through a merge key":  {component: head + "metadata:\n  annotations: &e\n" + "  " + strings.ReplaceAll(entry, "\n  ", "\n    ") + "    behavior: merge\nsecretGenerator:\n- <<: *e\n" + literal},
		"an empty list, then the real":   {component: head + "secretGenerator: []\nsecretGenerator:\n" + merge},
		"the field in two cases":         {component: head + "SecretGenerator: []\nsecretGenerator:\n" + merge},
		"a builtin generator":            {component: head + "generators:\n- gen.yaml\n", gen: "apiVersion: builtin\nkind: SecretGenerator\nmetadata:\n  name: creds\n  namespace: default\nbehavior: merge\nliterals:\n- k=v\n"},
		"a component base does not load": {component: head + "secretGenerator:\n" + merge, unloaded: true},
	}
	kustomizeschema.Parse()
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			components := "components:\n- ../c\n"
			if tc.unloaded {
				components = ""
			}
			files := map[string]string{
				"base/secrets.yaml":       "apiVersion: v1\nkind: Secret\nmetadata:\n  name: creds\n  namespace: default\ndata:\n  base: dg==\n",
				"base/kustomization.yaml": "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\nresources:\n- secrets.yaml\n" + components,
				"c/kustomization.yaml":    tc.component,
			}
			if tc.gen != "" {
				files["c/gen.yaml"] = tc.gen
			}
			for f, content := range files {
				p := filepath.Join(dir, filepath.FromSlash(f))
				if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			base := filepath.Join(dir, "base")
			rm, err := kustomizeBuild(filesys.MakeFsOnDisk(), base)
			if err != nil {
				t.Fatalf("build: %v", err)
			}
			merged := slices.ContainsFunc(rm.Resources(), func(r *resource.Resource) bool {
				return r.GetName() == "creds" && r.GetDataMap()["k"] != ""
			})
			if merged == tc.unloaded {
				t.Fatalf("kustomize merges into creds: %v, the case expects %v: it tests nothing", merged, !tc.unloaded)
			}
			guard := newBuildGuard(filesys.MakeFsOnDisk(), dir)
			if _, err := kustomizeBuild(guard, base); err != nil {
				t.Fatalf("build through the guard: %v", err)
			}
			file, found := guard.merged()
			if found != merged {
				t.Fatalf("the guard finds a merge: %v (%s), kustomize merges: %v", found, file, merged)
			}
			if found && file != filepath.Join(dir, "c", "kustomization.yaml") {
				t.Errorf("the guard names %s, not the component", file)
			}
		})
	}
}
