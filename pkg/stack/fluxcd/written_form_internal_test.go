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
				"note":      "a\u0085b c",
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

// Invariant: probeMerges marks exactly the Secrets a secretGenerator entry
// with behavior merge changes, under the namespace and name the build gives
// them. The oracle is kustomize itself: the objects of the build that the same
// tree without the merge entries does not make, identity and content both.
// The cases move the merged Secret every way a build can: the generator's
// namespace, none, another of two same-named Secrets, no content hash, an
// overlay's name suffix and namespace, and a component.
func TestProbeMerges_MarksWhatKustomizeMerges(t *testing.T) {
	const kustomization = "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\n"
	secret := func(namespace string) string {
		return "---\napiVersion: v1\nkind: Secret\nmetadata:\n  name: creds\n  namespace: " + namespace + "\ndata:\n  base: dg==\n"
	}
	merge := func(extra string) string {
		return "secretGenerator:\n- name: creds\n" + extra + "  behavior: merge\n  literals:\n  - k=v\n"
	}
	both := secret("default") + secret("blue")
	base := kustomization + "resources:\n- secrets.yaml\n#MERGE\n"
	cases := map[string]struct {
		files        map[string]string
		merge, build string
	}{
		"in its namespace": {
			files: map[string]string{"base/secrets.yaml": both, "base/kustomization.yaml": base},
			merge: merge("  namespace: default\n"), build: "base",
		},
		"in the other namespace": {
			files: map[string]string{"base/secrets.yaml": both, "base/kustomization.yaml": base},
			merge: merge("  namespace: blue\n"), build: "base",
		},
		"without a namespace": {
			files: map[string]string{"base/secrets.yaml": both, "base/kustomization.yaml": base},
			merge: merge(""), build: "base",
		},
		"without a content hash": {
			files: map[string]string{"base/secrets.yaml": both, "base/kustomization.yaml": base},
			merge: merge("  namespace: blue\n  options:\n    disableNameSuffixHash: true\n"), build: "base",
		},
		"under an overlay's name suffix": {
			files: map[string]string{"base/secrets.yaml": both, "base/kustomization.yaml": base,
				"overlay/kustomization.yaml": kustomization + "nameSuffix: -x\nresources:\n- ../base\n"},
			merge: merge("  namespace: blue\n"), build: "overlay",
		},
		"in an overlay that moves it": {
			files: map[string]string{"base/secrets.yaml": secret("default"), "base/kustomization.yaml": kustomization + "resources:\n- secrets.yaml\n",
				"overlay/kustomization.yaml": kustomization + "namespace: other\nresources:\n- ../base\n#MERGE\n"},
			merge: merge("  namespace: default\n"), build: "overlay",
		},
		"in a component": {
			files: map[string]string{"base/secrets.yaml": both,
				"base/kustomization.yaml": kustomization + "resources:\n- secrets.yaml\ncomponents:\n- ../component\n",
				// A field besides the merge, as kustomize refuses an empty
				// component, which the build without it would be.
				"component/kustomization.yaml": "apiVersion: kustomize.config.k8s.io/v1alpha1\nkind: Component\ncommonAnnotations:\n  component: c\n#MERGE\n"},
			merge: merge("  namespace: blue\n"), build: "base",
		},
	}
	kustomizeschema.Parse()
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			write := func(merge string) string {
				dir := t.TempDir()
				for f, content := range tc.files {
					p := filepath.Join(dir, filepath.FromSlash(f))
					if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(p, []byte(strings.ReplaceAll(content, "#MERGE\n", merge)), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				return filepath.Join(dir, tc.build)
			}
			dir, plainDir := write(tc.merge), write("")
			rm, err := kustomizeBuild(filesys.MakeFsOnDisk(), dir)
			if err != nil {
				t.Fatalf("build: %v", err)
			}
			plain, err := kustomizeBuild(filesys.MakeFsOnDisk(), plainDir)
			if err != nil {
				t.Fatalf("build without the merge: %v", err)
			}
			unmerged := map[string]bool{}
			for _, res := range plain.Resources() {
				m, err := res.Map()
				if err != nil {
					t.Fatal(err)
				}
				unmerged[canonicalJSON(t, m)] = true
			}
			var want []string
			for _, res := range rm.Resources() {
				m, err := res.Map()
				if err != nil {
					t.Fatal(err)
				}
				if !unmerged[canonicalJSON(t, m)] {
					want = append(want, res.CurId().String())
				}
			}
			if len(want) == 0 {
				t.Fatal("the merge changes nothing: the case tests nothing")
			}
			marked, err := probeMerges(filesys.MakeFsOnDisk(), dir, rm)
			if err != nil {
				t.Fatalf("probeMerges: %v", err)
			}
			var got []string
			for i, res := range rm.Resources() {
				if marked[i] {
					got = append(got, res.CurId().String())
				}
			}
			slices.Sort(got)
			slices.Sort(want)
			if !slices.Equal(got, want) {
				t.Errorf("probeMerges marks %v, kustomize merges %v", got, want)
			}
		})
	}
}

// A merged Secret kure's build names web-<hash> may be applied under any
// content hash, as another kustomize version hashes it; its name before the
// hash is the build's, so another name, or the same with no hash, is
// another Secret. One built with no hash is matched by that name only.
func TestMergedName_AnyContentHash(t *testing.T) {
	for _, c := range []struct {
		name, built string
		want        bool
	}{
		{"web-5bt2m8fkgh", "web-5bt2m8fkgh", true},
		{"web-ghk2m8t4bd", "web-5bt2m8fkgh", true},
		{"web", "web-5bt2m8fkgh", false},
		{"webx-ghk2m8t4bd", "web-5bt2m8fkgh", false},
		{"web-foo", "web-5bt2m8fkgh", false},
		{"web-ghk2m8t4ba", "web-5bt2m8fkgh", false},
		{"web", "web", true},
		{"web-ghk2m8t4bd", "web", false},
	} {
		if got := mergedName(c.name, c.built); got != c.want {
			t.Errorf("mergedName(%q, %q) = %v, want %v", c.name, c.built, got, c.want)
		}
	}
}
