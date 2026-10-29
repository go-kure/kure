package fluxcd

import (
	"encoding/json"
	stderrors "errors"
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
	"sigs.k8s.io/kustomize/api/provider"
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
// overlay's name suffix and namespace, and a component. Each also pins that
// a merged Secret is read as hashed exactly where kustomize appends a hash:
// into a generated Secret, unless its options say not, and never into a
// Secret a file holds, whatever shape its name has, a patch copying a data key
// into it included. A probe that cannot tell, a build whose replacements may
// move a mark, or a marked file a generator reads as data, does not read the
// build.
func TestProbeMerges_MarksWhatKustomizeMerges(t *testing.T) {
	const kustomization = "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\n"
	named := func(name, namespace string) string {
		return "---\napiVersion: v1\nkind: Secret\nmetadata:\n  name: " + name + "\n  namespace: " + namespace + "\ndata:\n  base: dg==\n"
	}
	secret := func(namespace string) string { return named("creds", namespace) }
	annotated := func(name, namespace string) string {
		return "---\napiVersion: v1\nkind: Secret\nmetadata:\n  name: " + name + "\n  namespace: " + namespace + "\n  annotations:\n    a: b\ndata:\n  base: dg==\n"
	}
	mergeNamed := func(name, extra string) string {
		return "secretGenerator:\n- name: " + name + "\n" + extra + "  behavior: merge\n  literals:\n  - k=v\n"
	}
	merge := func(extra string) string { return mergeNamed("creds", extra) }
	both := secret("default") + secret("blue")
	base := kustomization + "resources:\n- secrets.yaml\n#MERGE\n"
	unhashed := "  options:\n    disableNameSuffixHash: true\n"
	cases := map[string]struct {
		files        map[string]string
		merge, build string
		// hashed: kustomize appends a hash to the merged Secret's name;
		// unread: the probe does not read the build, its error holding
		// reason.
		hashed, unread bool
		reason         string
	}{
		"into a generated Secret": {
			files: map[string]string{"base/kustomization.yaml": kustomization + "secretGenerator:\n- name: creds\n  namespace: blue\n  literals:\n  - a=b\n",
				"overlay/kustomization.yaml": kustomization + "resources:\n- ../base\n#MERGE\n"},
			merge: merge("  namespace: blue\n"), build: "overlay", hashed: true,
		},
		"into a generated Secret, without a content hash": {
			files: map[string]string{"base/kustomization.yaml": kustomization + "secretGenerator:\n- name: creds\n  namespace: blue\n  literals:\n  - a=b\n" + unhashed,
				"overlay/kustomization.yaml": kustomization + "resources:\n- ../base\n#MERGE\n"},
			merge: merge("  namespace: blue\n" + unhashed), build: "overlay",
		},
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
			merge: merge("  namespace: blue\n" + unhashed), build: "base",
		},
		"under a literal name of a hash's shape": {
			files: map[string]string{"base/secrets.yaml": named("web-bcdfghkmt2", "default") + named("web-2456789bcd", "default"), "base/kustomization.yaml": base},
			merge: mergeNamed("web-bcdfghkmt2", "  namespace: default\n"), build: "base",
		},
		"beside a Secret carrying the annotation": {
			files: map[string]string{"base/secrets.yaml": both + "---\napiVersion: v1\nkind: Secret\nmetadata:\n  name: ready\n  namespace: default\n  annotations:\n    " + mergeProbe + ": \"false\"\ndata:\n  base: dg==\n",
				"base/kustomization.yaml": base},
			merge: merge("  namespace: blue\n"), build: "base",
		},
		"with a replacement copying its annotations": {
			files: map[string]string{"base/secrets.yaml": secret("default") + named("ready", "default") +
				"---\napiVersion: v1\nkind: Secret\nmetadata:\n  name: creds\n  namespace: blue\n  annotations:\n    x: y\ndata:\n  base: dg==\n",
				"base/kustomization.yaml": base + "replacements:\n- source:\n    kind: Secret\n    name: creds\n    namespace: blue\n    fieldPath: metadata.annotations\n  targets:\n  - select:\n      kind: Secret\n      name: ready\n    fieldPaths:\n    - metadata.annotations\n    options:\n      create: true\n"},
			merge: merge("  namespace: blue\n  options:\n    annotations:\n      a: b\n"), build: "base", unread: true, reason: "whose replacements",
		},
		// The mark leaves creds for ready and clean's annotations take its
		// place: one mark on one object, and the marked build the build but
		// for it, yet on a Secret the merge did not change.
		"with replacements moving its mark to another Secret": {
			files: map[string]string{"base/secrets.yaml": annotated("creds", "blue") + annotated("ready", "default") + annotated("clean", "default"),
				"base/kustomization.yaml": base + "replacements:\n" +
					"- source:\n    kind: Secret\n    name: creds\n    fieldPath: metadata.annotations\n  targets:\n  - select:\n      kind: Secret\n      name: ready\n    fieldPaths:\n    - metadata.annotations\n" +
					"- source:\n    kind: Secret\n    name: clean\n    fieldPath: metadata.annotations\n  targets:\n  - select:\n      kind: Secret\n      name: creds\n    fieldPaths:\n    - metadata.annotations\n"},
			merge: merge("  namespace: blue\n"), build: "base", unread: true, reason: "whose replacements",
		},
		// A patch copies a data key the file holds into the name: the probe's
		// literal, under a key no file holds, leaves it and the name as they
		// are, so the unhashed Secret is not read as hashed.
		"with a patch copying a data key into its name": {
			files: map[string]string{"base/secrets.yaml": "---\napiVersion: v1\nkind: Secret\nmetadata:\n  name: web\n  namespace: default\ndata:\n  " + mergeProbeKey + ": webxwebx\n",
				"base/kustomization.yaml": base + "patches:\n- target:\n    kind: Secret\n    name: web\n  patch: |-\n    - op: copy\n      from: /data/" + mergeProbeKey + "\n      path: /metadata/name\n"},
			merge: mergeNamed("web", "  namespace: default\n"), build: "base",
		},
		"beside a generator reading a kustomization file as data": {
			files: map[string]string{"base/secrets.yaml": both,
				"base/kustomization.yaml":      base + "configMapGenerator:\n- name: data\n  files:\n  - data/kustomization.yaml\n",
				"base/data/kustomization.yaml": mergeNamed("other", "")},
			merge: merge("  namespace: blue\n"), build: "base", unread: true,
		},
		"beside an unhashed generator reading a kustomization file as data": {
			files: map[string]string{"base/secrets.yaml": both,
				"base/kustomization.yaml":      base + "configMapGenerator:\n- name: data\n  options:\n    disableNameSuffixHash: true\n  files:\n  - data/kustomization.yaml\n",
				"base/data/kustomization.yaml": mergeNamed("other", "")},
			merge: merge("  namespace: blue\n"), build: "base", unread: true,
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
			if tc.unread {
				var u unreadBuild
				if !stderrors.As(err, &u) || !strings.Contains(err.Error(), tc.reason) {
					t.Fatalf("probeMerges: got %v, want the build not read, for %q", err, tc.reason)
				}
				return
			}
			if err != nil {
				t.Fatalf("probeMerges: %v", err)
			}
			var got []string
			for i, res := range rm.Resources() {
				if !marked[i].merged {
					if marked[i].hashed {
						t.Errorf("%s is read as hashed, not merged", res.CurId())
					}
					continue
				}
				got = append(got, res.CurId().String())
				if marked[i].hashed != tc.hashed {
					t.Errorf("%s is read as hashed %v, kustomize hashes it %v", res.CurId(), marked[i].hashed, tc.hashed)
				}
				if name := res.GetName(); tc.hashed && (len(name) < 11 || name[len(name)-11] != '-' || !contentHash(name[len(name)-10:])) {
					t.Errorf("%s: the case expects a hashed name", res.CurId())
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

// A kustomization file with a field the merge probe does not model is named
// by that field, whatever it is, a field nobody listed included; a file of
// modelled fields only (each writes the values its file holds, a patch only
// inside its object) names none.
func TestUnmodelledField(t *testing.T) {
	const head = "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\nresources:\n- s.yaml\n"
	for file, want := range map[string]string{
		head + "replacements:\n- source:\n    kind: Secret\n    name: a\n  targets:\n  - select:\n      name: b\n": "replacements",
		head + "vars:\n- name: V\n  objref:\n    kind: Secret\n    name: a\n    apiVersion: v1\n":                  "vars",
		head + "transformers:\n- t.yaml\n":     "transformers",
		head + "validators:\n- v.yaml\n":       "validators",
		head + "generators:\n- g.yaml\n":       "generators",
		head + "configurations:\n- c.yaml\n":   "configurations",
		head + "crds:\n- c.json\n":             "crds",
		head + "sortOptions:\n  order: fifo\n": "sortOptions",
		head + "replacements: []\n":            "replacements",
		"- a\n":                                "content",
		head + "patches:\n- target:\n    kind: Secret\n  patch: |-\n    - op: copy\n      from: /data/a\n      path: /data/b\n" +
			"commonAnnotations:\n  a: b\nnamePrefix: x-\nnamespace: n\nsecretGenerator:\n- name: s\n  behavior: merge\n": "",
		"apiVersion: kustomize.config.k8s.io/v1alpha1\nkind: Component\ncomponents:\n- ../c\nimages:\n- name: a\nlabels:\n- pairs:\n    a: b\n": "",
		head + "metadata:\n  name: k\nbases:\n- ../b\ncommonLabels:\n  a: b\nreplicas:\n- name: d\n  count: 2\n" +
			"patchesStrategicMerge:\n- p.yaml\npatchesJson6902:\n- path: j.yaml\n  target:\n    kind: Secret\n    name: s\n" +
			"configMapGenerator:\n- name: c\n  literals:\n  - a=b\ngeneratorOptions:\n  disableNameSuffixHash: true\n": "",
	} {
		if got := unmodelledField([]byte(file)); got != want {
			t.Errorf("unmodelledField(%q) = %q, want %q", file, got, want)
		}
	}
}

// A builtin SecretGenerator with behavior merge listed under generators
// merges into a Secret where no secretGenerator field shows it: the build
// counts as merged, and the probe does not read it.
func TestOpenAPIGuard_GeneratorsEntryMayMerge(t *testing.T) {
	dir := t.TempDir()
	for f, content := range map[string]string{
		"secrets.yaml":       "apiVersion: v1\nkind: Secret\nmetadata:\n  name: creds\n  namespace: default\ndata:\n  base: dg==\n",
		"gen.yaml":           "apiVersion: builtin\nkind: SecretGenerator\nmetadata:\n  name: creds\n  namespace: default\nbehavior: merge\nliterals:\n- k=v\n",
		"kustomization.yaml": "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\nresources:\n- secrets.yaml\ngenerators:\n- gen.yaml\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, f), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	kustomizeschema.Parse()
	guard := &openAPIGuard{FileSystem: filesys.MakeFsOnDisk()}
	rm, err := kustomizeBuild(guard, dir)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if res := rm.Resources(); len(res) != 1 || res[0].GetDataMap()["k"] == "" {
		t.Fatalf("the generators entry does not merge into creds: the case tests nothing")
	}
	if !guard.merged() {
		t.Error("a build loading a generators entry is not read as one that may merge")
	}
	_, err = probeMerges(filesys.MakeFsOnDisk(), dir, rm)
	var u unreadBuild
	if !stderrors.As(err, &u) || !strings.Contains(err.Error(), "whose generators") {
		t.Errorf("probeMerges: got %v, want the build not read, for its generators", err)
	}
}

// A mark is only one the probe issued: its nonce, "-" and the number of an
// entry it marked, written as it writes it.
func TestMergeMarker_ValueIsAnIssuedMark(t *testing.T) {
	m := &mergeMarker{nonce: "n0", n: 2}
	for value, want := range map[string]bool{
		"n0-0": true, "n0-1": true,
		"n0-2": false, "n0--1": false, "n0-01": false, "n0-+1": false, "n0-": false, "n0-x": false, "n1-0": false, "": false,
	} {
		res, err := provider.NewDefaultDepProvider().GetResourceFactory().FromMap(map[string]any{
			"apiVersion": "v1", "kind": "Secret",
			"metadata": map[string]any{"name": "s", "annotations": map[string]any{mergeProbe: value}},
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, got := m.value(res); got != want {
			t.Errorf("value of %q: got %v, want %v", value, got, want)
		}
	}
}

// A merged Secret kure's build names web-<hash>, kustomize having hashed it,
// may be applied under any content hash, as another kustomize version hashes
// it; its name before the hash is the build's, so another name, or the same
// with no hash, is another Secret. One built with no hash is matched by that
// name only, a literal suffix of a hash's shape included.
func TestMergedName_AnyContentHash(t *testing.T) {
	for _, c := range []struct {
		name, built string
		hashed      bool
		want        bool
	}{
		{"web-5bt2m8fkgh", "web-5bt2m8fkgh", true, true},
		{"web-ghk2m8t4bd", "web-5bt2m8fkgh", true, true},
		{"web", "web-5bt2m8fkgh", true, false},
		{"webx-ghk2m8t4bd", "web-5bt2m8fkgh", true, false},
		{"web-foo", "web-5bt2m8fkgh", true, false},
		{"web-ghk2m8t4ba", "web-5bt2m8fkgh", true, false},
		{"web", "web", false, true},
		{"web-ghk2m8t4bd", "web", false, false},
		{"web-5bt2m8fkgh", "web-5bt2m8fkgh", false, true},
		{"web-ghk2m8t4bd", "web-5bt2m8fkgh", false, false},
	} {
		if got := mergedName(c.name, c.built, c.hashed); got != c.want {
			t.Errorf("mergedName(%q, %q, hashed %v) = %v, want %v", c.name, c.built, c.hashed, got, c.want)
		}
	}
}
