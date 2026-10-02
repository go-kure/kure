package kuretest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// meta is a document's head: apiVersion, kind and metadata, with no
// namespace line when namespace is empty.
func meta(apiVersion, kind, namespace, name string) string {
	s := "apiVersion: " + apiVersion + "\nkind: " + kind + "\nmetadata:\n  name: " + name + "\n"
	if namespace != "" {
		s += "  namespace: " + namespace + "\n"
	}
	return s
}

// fluxKustomization is a Flux Kustomization whose spec is the given block,
// already indented by two spaces.
func fluxKustomization(namespace, name, spec string) string {
	return meta("kustomize.toolkit.fluxcd.io/v1", "Kustomization", namespace, name) + "spec:\n" + spec
}

func helmRelease(namespace, name, spec string) string {
	return meta("helm.toolkit.fluxcd.io/v2", "HelmRelease", namespace, name) + "spec:\n" + spec
}

func gitRepository(namespace, name string) string {
	return meta("source.toolkit.fluxcd.io/v1", "GitRepository", namespace, name)
}

func cm(namespace, name string) string { return meta("v1", "ConfigMap", namespace, name) }

func ns(name string) string { return meta("v1", "Namespace", "", name) }

func docs(d ...string) []byte { return []byte(strings.Join(d, "---\n")) }

// consistentCase is one call and what its failure must say: nothing, or
// every listed text plus the headline's count.
type consistentCase struct {
	docs     []string
	declared []Declared
	want     []string
	findings int
}

func (c consistentCase) check(t *testing.T, f *fakeTB) {
	t.Helper()
	if len(c.want) == 0 {
		if f.fatal != "" {
			t.Errorf("want a pass, got:\n%s", f.fatal)
		}
		return
	}
	for _, w := range c.want {
		if !strings.Contains(f.fatal, w) {
			t.Errorf("want %q in:\n%s", w, f.fatal)
		}
	}
	n := c.findings
	if n == 0 {
		n = 1
	}
	if head := strings.SplitN(f.fatal, "\n", 2)[0]; head != fmt.Sprintf(disagreeHeadline, n) {
		t.Errorf("want %d finding(s), got:\n%s", n, f.fatal)
	}
}

func runConsistent(t *testing.T, cases map[string]consistentCase) {
	t.Helper()
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			c.check(t, run(func(tb testing.TB) { AssertConsistentYAML(tb, docs(c.docs...), c.declared...) }))
		})
	}
}

func TestAssertConsistentYAMLResolvesASourceRef(t *testing.T) {
	flux := Namespace("flux-system")
	ref := "  sourceRef:\n    kind: GitRepository\n    name: src\n"
	runConsistent(t, map[string]consistentCase{
		"in the tree": {docs: []string{fluxKustomization("flux-system", "app", ref), gitRepository("flux-system", "src")}, declared: []Declared{flux}},
		"declared":    {docs: []string{fluxKustomization("flux-system", "app", ref)}, declared: []Declared{flux, External("GitRepository", "flux-system", "src")}},
		"missing": {
			docs:     []string{fluxKustomization("flux-system", "app", ref)},
			declared: []Declared{flux},
			want:     []string{"object 0: Kustomization flux-system/app", `spec.sourceRef: Not found: "GitRepository/flux-system/src"`},
		},
		"of another kind": {
			docs:     []string{fluxKustomization("flux-system", "app", ref), meta("source.toolkit.fluxcd.io/v1", "OCIRepository", "flux-system", "src")},
			declared: []Declared{flux},
			want:     []string{`spec.sourceRef: Not found: "GitRepository/flux-system/src"`},
		},
		"names nothing": {
			docs:     []string{fluxKustomization("flux-system", "app", "  sourceRef:\n    kind: GitRepository\n"), gitRepository("flux-system", "src")},
			declared: []Declared{flux},
			want:     []string{"spec.sourceRef.name: Required value: the reference names nothing"},
		},
	})
}

// A reference without a namespace is in its referrer's; an object, a
// declaration and a reference without one are all in default.
func TestAssertConsistentYAMLReadsAReferenceNamespace(t *testing.T) {
	declared := []Declared{Namespace("flux-system"), Namespace("apps")}
	implicit := "  sourceRef:\n    kind: GitRepository\n    name: src\n"
	explicit := "  sourceRef:\n    kind: GitRepository\n    name: src\n    namespace: flux-system\n"
	runConsistent(t, map[string]consistentCase{
		"the referrer's, resolving": {docs: []string{fluxKustomization("apps", "app", implicit), gitRepository("apps", "src")}, declared: declared},
		"the referrer's, missing": {
			docs:     []string{fluxKustomization("apps", "app", implicit), gitRepository("flux-system", "src")},
			declared: declared,
			want:     []string{`spec.sourceRef: Not found: "GitRepository/apps/src"`},
		},
		"explicit, resolving": {docs: []string{fluxKustomization("apps", "app", explicit), gitRepository("flux-system", "src")}, declared: declared},
		"explicit, missing": {
			docs:     []string{fluxKustomization("apps", "app", explicit), gitRepository("apps", "src")},
			declared: declared,
			want:     []string{`spec.sourceRef: Not found: "GitRepository/flux-system/src"`},
		},
		"omitted everywhere":       {docs: []string{fluxKustomization("", "app", implicit), gitRepository("", "src")}},
		"omitted, written default": {docs: []string{fluxKustomization("", "app", implicit), gitRepository("default", "src")}},
		"omitted, declared":        {docs: []string{fluxKustomization("", "app", implicit)}, declared: []Declared{External("GitRepository", "", "src")}},
		"omitted, elsewhere": {
			docs:     []string{fluxKustomization("", "app", implicit), gitRepository("flux-system", "src")},
			declared: declared,
			want:     []string{`spec.sourceRef: Not found: "GitRepository/default/src"`},
		},
	})
}

func TestAssertConsistentYAMLResolvesDependsOn(t *testing.T) {
	flux := []Declared{Namespace("flux-system"), External("GitRepository", "flux-system", "src")}
	ref := "  sourceRef:\n    kind: GitRepository\n    name: src\n"
	chart := "  chart:\n    spec:\n      chart: web\n      sourceRef:\n        kind: HelmRepository\n        name: charts\n"
	charts := meta("source.toolkit.fluxcd.io/v1", "HelmRepository", "flux-system", "charts")
	runConsistent(t, map[string]consistentCase{
		"Kustomization on Kustomization": {
			docs:     []string{fluxKustomization("flux-system", "app", ref+"  dependsOn:\n  - name: base\n"), fluxKustomization("flux-system", "base", ref)},
			declared: flux,
		},
		"Kustomization on a missing Kustomization": {
			docs:     []string{fluxKustomization("flux-system", "app", ref+"  dependsOn:\n  - name: base\n  - name: infra\n    namespace: infra\n"), fluxKustomization("flux-system", "base", ref)},
			declared: flux,
			want:     []string{`spec.dependsOn[1]: Not found: "Kustomization/infra/infra"`},
		},
		"Kustomization on a dependency that names nothing": {
			docs:     []string{fluxKustomization("flux-system", "app", ref+"  dependsOn:\n  - namespace: flux-system\n")},
			declared: flux,
			want:     []string{"spec.dependsOn[0].name: Required value: the reference names nothing"},
		},
		"HelmRelease on HelmRelease": {
			docs:     []string{helmRelease("flux-system", "web", chart+"  dependsOn:\n  - name: db\n"), helmRelease("flux-system", "db", chart), charts},
			declared: flux,
		},
		// dependsOn on a HelmRelease names a HelmRelease: a Kustomization of
		// that name does not satisfy it.
		"HelmRelease on a Kustomization": {
			docs:     []string{helmRelease("flux-system", "web", chart+"  dependsOn:\n  - name: db\n"), fluxKustomization("flux-system", "db", ref), charts},
			declared: flux,
			want:     []string{"object 0: HelmRelease flux-system/web", `spec.dependsOn[0]: Not found: "HelmRelease/flux-system/db"`},
		},
	})
}

func TestAssertConsistentYAMLResolvesAChart(t *testing.T) {
	flux := []Declared{Namespace("flux-system")}
	chartRef := "  chartRef:\n    kind: OCIRepository\n    name: chart\n"
	chart := "  chart:\n    spec:\n      chart: web\n      sourceRef:\n        kind: HelmRepository\n        name: charts\n"
	runConsistent(t, map[string]consistentCase{
		"chartRef in the tree": {docs: []string{helmRelease("flux-system", "web", chartRef), meta("source.toolkit.fluxcd.io/v1", "OCIRepository", "flux-system", "chart")}, declared: flux},
		"chartRef missing": {
			docs:     []string{helmRelease("flux-system", "web", chartRef)},
			declared: flux,
			want:     []string{`spec.chartRef: Not found: "OCIRepository/flux-system/chart"`},
		},
		"chart.spec.sourceRef in the tree": {docs: []string{helmRelease("flux-system", "web", chart), meta("source.toolkit.fluxcd.io/v1", "HelmRepository", "flux-system", "charts")}, declared: flux},
		"chart.spec.sourceRef missing": {
			docs:     []string{helmRelease("flux-system", "web", chart)},
			declared: flux,
			want:     []string{`spec.chart.spec.sourceRef: Not found: "HelmRepository/flux-system/charts"`},
		},
	})
}

func TestAssertConsistentYAMLChecksNamespaces(t *testing.T) {
	target := "  targetNamespace: apps\n  sourceRef:\n    kind: GitRepository\n    name: src\n"
	flux := []Declared{Namespace("flux-system"), External("GitRepository", "flux-system", "src")}
	runConsistent(t, map[string]consistentCase{
		"in the tree":     {docs: []string{cm("web", "a"), ns("web")}},
		"declared":        {docs: []string{cm("web", "a")}, declared: []Declared{Namespace("web")}},
		"default":         {docs: []string{cm("default", "a"), cm("", "b")}},
		"kube-system":     {docs: []string{cm("kube-system", "a")}},
		"kube-public":     {docs: []string{cm("kube-public", "a")}},
		"kube-node-lease": {docs: []string{cm("kube-node-lease", "a")}},
		"missing": {
			docs: []string{cm("web", "a"), cm("web", "b"), ns("cache")},
			want: []string{"object 0: ConfigMap web/a", "object 1: ConfigMap web/b", `metadata.namespace: Not found: "web"`},
			// one finding per object
			findings: 2,
		},
		"targetNamespace in the tree": {docs: []string{fluxKustomization("flux-system", "app", target), ns("apps")}, declared: flux},
		"targetNamespace declared":    {docs: []string{fluxKustomization("flux-system", "app", target)}, declared: append([]Declared{Namespace("apps")}, flux...)},
		"targetNamespace missing": {
			docs:     []string{fluxKustomization("flux-system", "app", target)},
			declared: flux,
			want:     []string{`spec.targetNamespace: Not found: "apps"`},
		},
		// a namespace on a cluster-scoped kind is AssertValid's finding
		"cluster-scoped": {docs: []string{meta("rbac.authorization.k8s.io/v1", "ClusterRole", "nowhere", "reader")}},
		// a namespace object is cluster-scoped: no namespace makes one
		"a namespace's namespace": {docs: []string{meta("v1", "Namespace", "nowhere", "web"), cm("web", "a")}},
		// a kind kure does not register reads as namespaced, as kustomize
		// reads it
		"an unregistered kind": {
			docs: []string{meta("example.com/v1", "Gizmo", "web", "g")},
			want: []string{"object 0: Gizmo web/g", `metadata.namespace: Not found: "web"`},
		},
	})
}

// Every error of one object is one finding; every object with one is listed.
func TestAssertConsistentYAMLGroupsFindingsPerObject(t *testing.T) {
	runConsistent(t, map[string]consistentCase{
		"two errors, two objects": {
			docs: []string{
				fluxKustomization("apps", "app", "  targetNamespace: web\n  sourceRef:\n    kind: GitRepository\n    name: src\n"),
				cm("cache", "c"),
			},
			want:     []string{`metadata.namespace: Not found: "apps"`, `spec.sourceRef: Not found: "GitRepository/apps/src"`, `spec.targetNamespace: Not found: "web"`, "object 1: ConfigMap cache/c"},
			findings: 2,
		},
	})
}

// What stops the pass itself is a failure, never a pass.
func TestAssertConsistentYAMLFailsOnWhatItCannotRead(t *testing.T) {
	for name, c := range map[string]struct{ doc, want string }{
		"empty":                 {"", "no objects"},
		"kustomization only":    {"apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\nresources: [a.yaml]\n", "no objects"},
		"not a document":        {"metadata: [not a mapping\n", "document 0"},
		"not an object":         {"k: v\n", "document 0"},
		"a Kustomization spec":  {fluxKustomization("flux-system", "app", "  dependsOn: base\n"), "object 0: Kustomization flux-system/app"},
		"a HelmRelease spec":    {helmRelease("flux-system", "web", "  chartRef: chart\n"), "object 0: HelmRelease flux-system/web"},
		"a second document":     {cm("default", "a") + "---\nk: v\n", "document 1"},
		"only an empty comment": {"# nothing\n---\n", "no objects"},
	} {
		t.Run(name, func(t *testing.T) {
			f := run(func(tb testing.TB) { AssertConsistentYAML(tb, []byte(c.doc)) })
			if !strings.Contains(f.fatal, "kuretest: ") || !strings.Contains(f.fatal, c.want) {
				t.Errorf("want a kuretest failure naming %q, got %q", c.want, f.fatal)
			}
		})
	}
}

func TestAssertConsistent(t *testing.T) {
	if f := run(func(tb testing.TB) { AssertConsistent(tb, []client.Object{configMap("a")}, Namespace("web")) }); f.fatal != "" {
		t.Errorf("a declared namespace failed: %q", f.fatal)
	}
	f := run(func(tb testing.TB) { AssertConsistent(tb, []client.Object{configMap("a"), configMap("b")}) })
	if !strings.Contains(f.fatal, "2 object(s) that do not agree with the tree") || !strings.Contains(f.fatal, "object 1: ConfigMap web/b") || !strings.Contains(f.fatal, `metadata.namespace: Not found: "web"`) {
		t.Errorf("want both objects named, got %q", f.fatal)
	}
	if f := run(func(tb testing.TB) { AssertConsistent(tb, nil) }); !strings.Contains(f.fatal, "no objects") {
		t.Errorf("no objects must fail, got %q", f.fatal)
	}
}

// writeTree writes files, keyed by their path under dir.
func writeTree(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, body := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestAssertConsistentDir(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{
		"kustomization.yaml":    "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\nresources: [cm.yaml, sub]\n",
		"cm.yaml":               cm("default", "a"),
		"sub/kustomization.yml": "resources: [cm.yml]\n",
		"sub/cm.yml":            cm("web", "b"),
		"sub/README.md":         "not read",
	})
	f := run(func(tb testing.TB) { AssertConsistentDir(tb, dir) })
	if want := filepath.Join(dir, "sub", "cm.yml") + ": object 0: ConfigMap web/b"; !strings.Contains(f.fatal, want) || !strings.Contains(f.fatal, `metadata.namespace: Not found: "web"`) || strings.Contains(f.fatal, "cm.yaml") {
		t.Errorf("want only sub/cm.yml named, got %q", f.fatal)
	}
	if f := run(func(tb testing.TB) { AssertConsistentDir(tb, dir, Namespace("web")) }); f.fatal != "" {
		t.Errorf("a consistent tree failed: %q", f.fatal)
	}
}

// A file a kustomization file names as a generator's source or a patch is
// data, not an object: it is not decoded, so a kind-less one passes. Every
// other manifest still decodes or fails.
func TestAssertConsistentDirSkipsTheDataAKustomizationNames(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{
		"kustomization.yaml": `resources: [cm.yaml]
configMapGenerator:
- name: values
  files: [values.yaml, other=sub/other.yml]
  envs: [env.yaml]
secretGenerator:
- name: secret
  env: secret.yaml
patches:
- path: patch.yaml
  target: {kind: ConfigMap}
- patch: '[{"op": "add", "path": "/data", "value": {}}]'
`,
		"cm.yaml":       cm("default", "a"),
		"values.yaml":   "k: v\n",
		"sub/other.yml": "k: v\n",
		"env.yaml":      "k: v\n",
		"secret.yaml":   "k: v\n",
		"patch.yaml":    "- op: remove\n  path: /metadata/labels\n",
	})
	if f := run(func(tb testing.TB) { AssertConsistentDir(tb, dir) }); f.fatal != "" {
		t.Errorf("the data files were read as objects: %q", f.fatal)
	}
	writeTree(t, dir, map[string]string{"stray.yaml": "k: v\n"})
	if f := run(func(tb testing.TB) { AssertConsistentDir(tb, dir) }); !strings.Contains(f.fatal, filepath.Join(dir, "stray.yaml")) || !strings.Contains(f.fatal, "document 0") {
		t.Errorf("an unlisted kind-less file must fail, got %q", f.fatal)
	}
}

// The data a Kustomization file names as such is skipped wherever that file
// sits, under every name kustomize recognizes, the extension-less one included.
func TestAssertConsistentDirReadsEveryKustomizationFileName(t *testing.T) {
	for _, name := range []string{"kustomization.yaml", "kustomization.yml", "Kustomization"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			writeTree(t, dir, map[string]string{
				"app/" + name:       "resources: [cm.yaml]\nconfigMapGenerator:\n- name: v\n  files: [values.yaml]\n",
				"app/cm.yaml":       cm("default", "a"),
				"app/values.yaml":   "k: v\n",
				"other/values.yaml": "k: v\n",
			})
			f := run(func(tb testing.TB) { AssertConsistentDir(tb, dir) })
			if !strings.Contains(f.fatal, filepath.Join(dir, "other", "values.yaml")) || strings.Contains(f.fatal, filepath.Join(dir, "app", "values.yaml")) {
				t.Errorf("want only other/values.yaml named, got %q", f.fatal)
			}
		})
	}
}

func TestAssertConsistentDirFailsOnWhatItCannotRead(t *testing.T) {
	empty := t.TempDir()
	if f := run(func(tb testing.TB) { AssertConsistentDir(tb, empty) }); !strings.Contains(f.fatal, "no objects") {
		t.Errorf("an empty directory must fail, got %q", f.fatal)
	}
	missing := filepath.Join(t.TempDir(), "missing")
	if f := run(func(tb testing.TB) { AssertConsistentDir(tb, missing) }); !strings.Contains(f.fatal, missing) {
		t.Errorf("a directory that does not exist must fail, got %q", f.fatal)
	}
	broken := t.TempDir()
	writeTree(t, broken, map[string]string{"broken.yaml": "metadata: [not a mapping\n"})
	if f := run(func(tb testing.TB) { AssertConsistentDir(tb, broken) }); !strings.Contains(f.fatal, "broken.yaml") || !strings.Contains(f.fatal, "document 0") {
		t.Errorf("a manifest that does not decode must fail, got %q", f.fatal)
	}
	badKust := t.TempDir()
	writeTree(t, badKust, map[string]string{"kustomization.yaml": "resources: {not: a list}\n", "cm.yaml": cm("default", "a")})
	if f := run(func(tb testing.TB) { AssertConsistentDir(tb, badKust) }); !strings.Contains(f.fatal, filepath.Join(badKust, "kustomization.yaml")) {
		t.Errorf("a kustomization file that does not parse must fail, got %q", f.fatal)
	}
	for _, name := range []string{"dangling.yaml", "kustomization.yaml"} {
		unreadable := t.TempDir()
		writeTree(t, unreadable, map[string]string{"cm.yaml": cm("default", "a")})
		if err := os.Symlink(filepath.Join(unreadable, "gone"), filepath.Join(unreadable, name)); err != nil {
			t.Fatal(err)
		}
		if f := run(func(tb testing.TB) { AssertConsistentDir(tb, unreadable) }); !strings.Contains(f.fatal, name) {
			t.Errorf("%s: a file that cannot be read must fail, got %q", name, f.fatal)
		}
	}
	notADoc := t.TempDir()
	writeTree(t, notADoc, map[string]string{"app.yaml": fluxKustomization("flux-system", "app", "  dependsOn: base\n")})
	if f := run(func(tb testing.TB) { AssertConsistentDir(tb, notADoc) }); !strings.Contains(f.fatal, "kuretest: ") || !strings.Contains(f.fatal, "Kustomization flux-system/app") {
		t.Errorf("a Kustomization the pass cannot read must fail, got %q", f.fatal)
	}
}
