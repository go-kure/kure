package kuretest

import (
	"fmt"
	"io/fs"
	"maps"
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
// every want text, none of the absent ones, and the headline's count.
type consistentCase struct {
	docs     []string
	declared []Declared
	want     []string
	absent   []string
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
	for _, a := range c.absent {
		if strings.Contains(f.fatal, a) {
			t.Errorf("want no %q in:\n%s", a, f.fatal)
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

// pathDeclared admits the namespace and the Source every Flux Kustomization
// in the path tests relies on.
var pathDeclared = []Declared{Namespace("flux-system"), External("GitRepository", "flux-system", "src")}

// applying is a Flux Kustomization in flux-system that applies path from the
// GitRepository src; extra holds more spec lines, indented by two spaces.
func applying(name, path, extra string) string {
	return fluxKustomization("flux-system", name, "  path: "+path+"\n  sourceRef:\n    kind: GitRepository\n    name: src\n"+extra)
}

// labelled is a ConfigMap in default that carries the label x.
func labelled(name string) string { return cm("default", name) + "  labels:\n    x: \"1\"\n" }

// removeLabel is a Flux Kustomization's patch that removes the label key
// from every ConfigMap; it fails on one without that label.
func removeLabel(key string) string {
	return "  patches:\n  - patch: '[{\"op\": \"remove\", \"path\": \"/metadata/labels/" + key + "\"}]'\n    target:\n      kind: ConfigMap\n"
}

// builds is a tree whose one Flux Kustomization applies app, which builds.
func builds() map[string]string {
	return map[string]string{
		"flux.yaml":              applying("app", "./app", ""),
		"app/kustomization.yaml": "resources: [cm.yaml]\n",
		"app/cm.yaml":            cm("default", "a"),
	}
}

// failing is a tree whose one Flux Kustomization applies app, which does not
// build though every file decodes: two ConfigMaps share one identity.
func failing() map[string]string {
	return map[string]string{
		"flux.yaml":              applying("app", "./app", ""),
		"app/kustomization.yaml": "resources: [a.yaml, b.yaml]\n",
		"app/a.yaml":             cm("default", "a"),
		"app/b.yaml":             cm("default", "a"),
	}
}

// with is base plus files, which win on a shared path.
func with(base, files map[string]string) map[string]string {
	out := map[string]string{}
	for _, m := range []map[string]string{base, files} {
		for k, v := range m {
			out[k] = v
		}
	}
	return out
}

// writeLinks writes symbolic links, keyed by their path under dir, to their
// target as written.
func writeLinks(t *testing.T, dir string, links map[string]string) {
	t.Helper()
	for name, target := range links {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, p); err != nil {
			t.Fatal(err)
		}
	}
}

// dirCase is a tree written under a temporary directory and what
// AssertConsistentDir on its root, with pathDeclared, must say. A want or
// absent text may name {root}, the root's absolute path.
type dirCase struct {
	files    map[string]string
	links    map[string]string // symbolic links, keyed like files, to their target as written
	root     string            // relative to the temporary directory; empty is the directory itself
	want     []string
	absent   []string
	findings int
}

func runDir(t *testing.T, cases map[string]dirCase) {
	t.Helper()
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			tmp := t.TempDir()
			writeTree(t, tmp, c.files)
			writeLinks(t, tmp, c.links)
			root := filepath.Join(tmp, c.root)
			at := func(texts []string) []string {
				out := make([]string, len(texts))
				for i, s := range texts {
					out[i] = strings.ReplaceAll(s, "{root}", root)
				}
				return out
			}
			f := run(func(tb testing.TB) { AssertConsistentDir(tb, root, pathDeclared...) })
			consistentCase{want: at(c.want), absent: at(c.absent), findings: c.findings}.check(t, f)
		})
	}
}

func TestAssertConsistentDirResolvesEveryEntry(t *testing.T) {
	runDir(t, map[string]dirCase{
		"every entry resolves": {files: map[string]string{
			"kustomization.yaml":      "resources: [cm.yaml, sub]\ncomponents: [comp]\n",
			"cm.yaml":                 cm("default", "a"),
			"sub/kustomization.yaml":  "resources: [cm.yaml]\n",
			"sub/cm.yaml":             cm("default", "b"),
			"comp/kustomization.yaml": "apiVersion: kustomize.config.k8s.io/v1alpha1\nkind: Component\n",
		}},
		"a resources entry that is missing": {
			files: map[string]string{"kustomization.yaml": "resources: [cm.yaml, missing.yaml]\n", "cm.yaml": cm("default", "a")},
			want:  []string{"{root}/kustomization.yaml", `resources[1]: Not found: "missing.yaml"`},
		},
		"a resources directory without a kustomization file": {
			files: map[string]string{"kustomization.yaml": "resources: [cm.yaml, sub]\n", "cm.yaml": cm("default", "a"), "sub/cm.yaml": cm("default", "b")},
			want:  []string{`resources[1]: Invalid value: "sub": a directory without a kustomization file`},
		},
		"a components entry that is missing": {
			files: map[string]string{"kustomization.yaml": "resources: [cm.yaml]\ncomponents: [comp]\n", "cm.yaml": cm("default", "a")},
			want:  []string{`components[0]: Not found: "comp"`},
		},
		"a component that is a file": {
			files: map[string]string{"kustomization.yaml": "resources: [cm.yaml]\ncomponents: [cm.yaml]\n", "cm.yaml": cm("default", "a")},
			want:  []string{`components[0]: Invalid value: "cm.yaml": a file, where a component is a directory`},
		},
		"a patch file that is missing": {
			files: map[string]string{"kustomization.yaml": "resources: [cm.yaml]\npatches:\n- path: patch.yaml\n  target: {kind: ConfigMap}\n", "cm.yaml": cm("default", "a")},
			want:  []string{`patches[0].path: Not found: "patch.yaml"`},
		},
		"generator sources that are missing": {
			files: map[string]string{
				"kustomization.yaml": `resources: [cm.yaml]
configMapGenerator:
- name: v
  files: [values.yaml, key=other.yaml]
  envs: [env.yaml]
secretGenerator:
- name: s
  files: [s.yaml, k=t.yaml]
  env: secret.env
`,
				"cm.yaml": cm("default", "a"),
			},
			want: []string{
				`configMapGenerator[0].files[0]: Not found: "values.yaml"`,
				`configMapGenerator[0].files[1]: Not found: "other.yaml"`,
				`configMapGenerator[0].envs[0]: Not found: "env.yaml"`,
				`secretGenerator[0].files[0]: Not found: "s.yaml"`,
				`secretGenerator[0].files[1]: Not found: "t.yaml"`,
				`secretGenerator[0].env: Not found: "secret.env"`,
			},
		},
		"a generator source that is a directory": {
			files: map[string]string{"kustomization.yaml": "resources: [cm.yaml]\nconfigMapGenerator:\n- name: v\n  files: [sub]\n", "cm.yaml": cm("default", "a"), "sub/cm.yaml": cm("default", "b")},
			want:  []string{`configMapGenerator[0].files[0]: Invalid value: "sub": a directory, where a file is named`},
		},
		// a file entry stays in or below its kustomization file's directory,
		// as kustomize's default load restriction has it; a directory entry
		// may name any directory in the tree
		"a file entry that leaves its directory beside a directory entry that may": {
			files: map[string]string{
				"app/kustomization.yaml":  "resources: [../shared/cm.yaml, ../base]\npatches:\n- path: ../patch.yaml\n  target: {kind: ConfigMap}\n",
				"shared/cm.yaml":          cm("default", "a"),
				"base/kustomization.yaml": "resources: [cm.yaml]\n",
				"base/cm.yaml":            cm("default", "b"),
				"patch.yaml":              "- op: remove\n  path: /metadata/labels\n",
			},
			want: []string{
				"{root}/app/kustomization.yaml",
				`resources[0]: Invalid value: "../shared/cm.yaml": a file outside the kustomization file's directory`,
				`patches[0].path: Invalid value: "../patch.yaml": a file outside the kustomization file's directory`,
			},
			absent: []string{"resources[1]", "{root}/base/kustomization.yaml"},
		},
		"an entry outside the tree": {
			root: "tree",
			files: map[string]string{
				"tree/kustomization.yaml":    "resources: [cm.yaml, ../outside]\n",
				"tree/cm.yaml":               cm("default", "a"),
				"outside/kustomization.yaml": "resources: [cm.yaml]\n",
				"outside/cm.yaml":            cm("default", "b"),
			},
			want: []string{`resources[1]: Invalid value: "../outside": outside the tree`},
		},
	})
}

// The entries of every file kustomize recognizes are resolved, the
// extension-less one included, which the object walk never decodes.
func TestAssertConsistentDirResolvesUnderEveryKustomizationFileName(t *testing.T) {
	cases := map[string]dirCase{}
	for _, name := range []string{"kustomization.yaml", "kustomization.yml", "Kustomization"} {
		cases[name] = dirCase{
			files: map[string]string{"app/" + name: "resources: [cm.yaml, missing.yaml]\n", "app/cm.yaml": cm("default", "a")},
			want:  []string{"{root}/app/" + name, `resources[1]: Not found: "missing.yaml"`},
		}
	}
	runDir(t, cases)
}

func TestAssertConsistentDirBuildsWhatFluxApplies(t *testing.T) {
	runDir(t, map[string]dirCase{
		"a path that builds": {files: builds()},
		"a path that is missing": {
			files: with(builds(), map[string]string{"flux.yaml": applying("app", "./missing", "")}),
			want:  []string{"{root}/flux.yaml: object 0: Kustomization flux-system/app", `spec.path: Not found: "./missing"`},
		},
		"a path that is not a directory": {
			files: with(builds(), map[string]string{"flux.yaml": applying("app", "./app/cm.yaml", "")}),
			want:  []string{`spec.path: Invalid value: "./app/cm.yaml": not a directory`},
		},
		"a path that leaves the tree": {
			files: with(builds(), map[string]string{"flux.yaml": applying("app", "../app", "")}),
			want:  []string{`spec.path: Invalid value: "../app": not a path inside the tree`},
		},
		"an absolute path": {
			files: with(builds(), map[string]string{"flux.yaml": applying("app", "/app", "")}),
			want:  []string{`spec.path: Invalid value: "/app": not a path inside the tree`},
		},
		"a path that does not build": {
			files: failing(),
			want:  []string{"{root}/flux.yaml: object 0: Kustomization flux-system/app", `spec.path: Invalid value: "./app": the build fails: `, "already registered id"},
		},
		// Flux generates the kustomization file: every manifest in the
		// directory, and a subdirectory holding a kustomization file whole —
		// were dup.yaml read on its own, ConfigMap a would be there twice
		"a directory without a kustomization file": {files: map[string]string{
			"flux.yaml":                  applying("app", "./app", ""),
			"app/cm.yaml":                cm("default", "a"),
			"app/sub/kustomization.yaml": "resources: [b.yaml]\n",
			"app/sub/b.yaml":             cm("default", "b"),
			"app/sub/dup.yaml":           cm("default", "a"),
		}},
		// the Flux Kustomization's own fields reach the build
		"a component the Kustomization names and the tree lacks": {
			files: with(builds(), map[string]string{"flux.yaml": applying("app", "./app", "  components:\n  - absent-component\n")}),
			want:  []string{`spec.path: Invalid value: "./app": the build fails: `, "absent-component"},
		},
		// patchesJson6902 is a field v1 dropped and the generator still reads;
		// the next Kustomization is still built
		"a Kustomization Flux cannot generate a kustomization file for": {
			files: with(failing(), map[string]string{
				"flux.yaml":   string(docs(applying("gen", "./gen", "  patchesJson6902: not-a-list\n"), applying("app", "./app", ""))),
				"gen/cm.yaml": cm("default", "g"),
			}),
			want: []string{
				"{root}/flux.yaml: object 0: Kustomization flux-system/gen",
				`spec.path: Invalid value: "./gen": Flux cannot generate its kustomization file: `, "patchesJson6902",
				"{root}/flux.yaml: object 1: Kustomization flux-system/app", `spec.path: Invalid value: "./app": the build fails: `,
			},
			findings: 2,
		},
	})
}

// A path the pass does not resolve could take the build past the tree, so
// it builds nothing: failing's Kustomization would add a finding of its own.
func TestAssertConsistentDirBuildsNothingItCannotResolve(t *testing.T) {
	notLocal := "not a local path, so nothing is built"
	cases := map[string]dirCase{
		"a remote resources entry": {
			files: with(failing(), map[string]string{"other/kustomization.yaml": "resources: [cm.yaml, https://example.com/app.yaml]\n", "other/cm.yaml": cm("default", "b")}),
			want:  []string{"{root}/other/kustomization.yaml", `resources[1]: Invalid value: "https://example.com/app.yaml": ` + notLocal},
		},
		"an scp-style resources entry": {
			files: with(failing(), map[string]string{"other/kustomization.yaml": "resources: [cm.yaml, deploy@example.com:org/repo]\n", "other/cm.yaml": cm("default", "b")}),
			want:  []string{`resources[1]: Invalid value: "deploy@example.com:org/repo": ` + notLocal},
		},
		"a remote generator source": {
			files: with(failing(), map[string]string{"other/kustomization.yaml": "resources: [cm.yaml]\nconfigMapGenerator:\n- name: v\n  files: [https://example.com/values.yaml]\n", "other/cm.yaml": cm("default", "b")}),
			want:  []string{`configMapGenerator[0].files[0]: Invalid value: "https://example.com/values.yaml": ` + notLocal},
		},
		"a remote component on a Kustomization": {
			files: with(failing(), map[string]string{
				"flux.yaml":     string(docs(applying("app", "./app", ""), applying("remote", "./other", "  components:\n  - https://example.com/component\n"))),
				"other/cm.yaml": cm("default", "b"),
			}),
			want: []string{"{root}/flux.yaml: object 1: Kustomization flux-system/remote", `spec.components[0]: Invalid value: "https://example.com/component": ` + notLocal},
		},
	}
	for name, body := range map[string]string{
		"openapi":                     "openapi:\n  path: schema.json\n",
		"patchesStrategicMerge":       "patchesStrategicMerge: [patch.yaml]\n",
		"patchesJson6902":             "patchesJson6902:\n- path: patch.yaml\n  target: {kind: ConfigMap, name: b}\n",
		"replacements":                "replacements:\n- path: replacement.yaml\n",
		"crds":                        "crds: [crd.yaml]\n",
		"bases":                       "bases: [../app]\n",
		"helmGlobals":                 "helmGlobals:\n  chartHome: charts\n",
		"helmCharts":                  "helmCharts:\n- name: web\n",
		"helmChartInflationGenerator": "helmChartInflationGenerator:\n- chartName: web\n",
		"configurations":              "configurations: [config.yaml]\n",
		"generators":                  "generators: [generator.yaml]\n",
		"transformers":                "transformers: [transformer.yaml]\n",
		"validators":                  "validators: [validator.yaml]\n",
	} {
		cases["a "+name+" field"] = dirCase{
			files: with(failing(), map[string]string{"other/kustomization.yaml": "resources: [cm.yaml]\n" + body, "other/cm.yaml": cm("default", "b")}),
			want:  []string{"{root}/other/kustomization.yaml", name + ": Forbidden: the pass does not resolve what this field loads, so nothing is built"},
		}
	}
	for name, c := range cases {
		c.absent = append(c.absent, "the build fails")
		cases[name] = c
	}
	runDir(t, cases)
}

// Every Flux Kustomization is built on the one copy as it was before any
// other: an inner one built first, whose own patch would fail the outer
// build were it left behind, and one whose build fails, which would fail
// the outer had its patch stayed. Each with the kustomization files the tree
// holds, then with none, which Flux generates and removes.
func TestAssertConsistentDirRestoresTheTreeBetweenBuilds(t *testing.T) {
	cases := map[string]dirCase{}
	for variant, kfiles := range map[string]map[string]string{
		"with kustomization files": {"outer/kustomization.yaml": "resources: [inner]\n", "outer/inner/kustomization.yaml": "resources: [cm.yaml]\n"},
		"without":                  {},
	} {
		cases[variant+": a patch built before"] = dirCase{files: with(kfiles, map[string]string{
			"flux.yaml":           string(docs(applying("inner", "./outer/inner", removeLabel("x")), applying("outer", "./outer", removeLabel("x")))),
			"outer/inner/cm.yaml": labelled("c"),
		})}
		cases[variant+": a build that failed before"] = dirCase{
			files: with(kfiles, map[string]string{
				"flux.yaml":           string(docs(applying("inner", "./outer/inner", removeLabel("missing")), applying("outer", "./outer", ""))),
				"outer/inner/cm.yaml": labelled("c"),
			}),
			want:   []string{"{root}/flux.yaml: object 0: Kustomization flux-system/inner", `spec.path: Invalid value: "./outer/inner": the build fails: `},
			absent: []string{"flux-system/outer"},
		}
	}
	runDir(t, cases)
}

// spec.path ./ is the root, and so is a root of "." passed relative to the
// working directory.
func TestAssertConsistentDirTakesDotAsTheRoot(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{"flux.yaml": applying("root", "./", ""), "cm.yaml": cm("default", "a")})
	t.Chdir(dir)
	if f := run(func(tb testing.TB) { AssertConsistentDir(tb, ".", pathDeclared...) }); f.fatal != "" {
		t.Errorf("want a pass, got:\n%s", f.fatal)
	}
	writeTree(t, dir, map[string]string{"dup.yaml": cm("default", "a")})
	f := run(func(tb testing.TB) { AssertConsistentDir(tb, ".", pathDeclared...) })
	consistentCase{want: []string{filepath.Join(dir, "flux.yaml") + ": object 0: Kustomization flux-system/root", `spec.path: Invalid value: "./": the build fails: `}}.check(t, f)
}

// snapshot reads every directory, link and file under dir; a link is read
// as its target, not through it.
func snapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.Type()&fs.ModeSymlink != 0:
			target, err := os.Readlink(p)
			out[p] = "a link to " + target
			return err
		case d.IsDir():
			out[p] = "a directory"
			return nil
		}
		b, err := os.ReadFile(p)
		out[p] = string(b)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// The builds write into a copy; the caller's tree is as it was.
func TestAssertConsistentDirLeavesTheTreeAsItWas(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, with(failing(), map[string]string{
		"flux.yaml": string(docs(
			applying("app", "./app", removeLabel("x")),
			applying("inner", "./outer/inner", removeLabel("missing")),
			applying("outer", "./outer", ""),
			applying("plain", "./plain", removeLabel("x")),
		)),
		"outer/kustomization.yaml":       "resources: [inner]\n",
		"outer/inner/kustomization.yaml": "resources: [cm.yaml]\n",
		"outer/inner/cm.yaml":            labelled("c"),
		"plain/cm.yaml":                  labelled("p"),
	}))
	before := snapshot(t, dir)
	f := run(func(tb testing.TB) { AssertConsistentDir(tb, dir, pathDeclared...) })
	if !strings.Contains(f.fatal, "the build fails") {
		t.Fatalf("want the builds run and a failing one reported, got %q", f.fatal)
	}
	if after := snapshot(t, dir); !maps.Equal(before, after) {
		t.Errorf("the tree changed:\nbefore %v\nafter  %v", before, after)
	}
}

// A symbolic link anywhere under root, or a file named as Flux's generator
// names the backup it keeps, is a finding on the tree and nothing is built:
// the copy the builds run on would keep the link and a build would write
// through it, and the generator overwrites and then consumes the backup.
// failing's Kustomization would add a finding of its own were anything
// built. A link is still read as what its name says, so the other checks
// run.
func TestAssertConsistentDirBuildsNothingThroughALink(t *testing.T) {
	cases := map[string]dirCase{
		// Codex's trace: building a would write its patch through the link
		// into b's kustomization file, which the restore leaves amended, so
		// b's own build would apply the removal twice and fail — though Flux
		// reconciles both.
		"a kustomization file that is a link": {
			files: map[string]string{
				"flux.yaml":            string(docs(applying("a", "./a", removeLabel("x")), applying("b", "./b", removeLabel("x")))),
				"a/cm.yaml":            labelled("a"),
				"b/kustomization.yaml": "resources: [cm.yaml]\n",
				"b/cm.yaml":            labelled("b"),
			},
			links: map[string]string{"a/kustomization.yaml": "../b/kustomization.yaml"},
			want:  []string{"  {root}\n    a/kustomization.yaml: Invalid value: \"../b/kustomization.yaml\": " + aLink},
		},
		"a kustomization file that is a link is still read, from where it is": {
			files: with(failing(), map[string]string{
				"a/cm.yaml":            cm("default", "c"),
				"b/kustomization.yaml": "resources: [cm.yaml, missing.yaml]\n",
				"b/cm.yaml":            cm("default", "b"),
			}),
			links: map[string]string{"a/kustomization.yaml": "../b/kustomization.yaml"},
			want: []string{
				"  {root}/a/kustomization.yaml\n    resources[1]: Not found: \"missing.yaml\"",
				"  {root}/b/kustomization.yaml\n    resources[1]: Not found: \"missing.yaml\"",
				"  {root}\n    a/kustomization.yaml: Invalid value: \"../b/kustomization.yaml\": " + aLink,
			},
			findings: 3,
		},
		"a directory that is a link": {
			files: failing(),
			links: map[string]string{"linked": "app"},
			want:  []string{"  {root}\n    linked: Invalid value: \"app\": " + aLink},
		},
		"a manifest that is a link": {
			files: failing(),
			links: map[string]string{"app/c.yaml": "a.yaml"},
			want:  []string{"  {root}\n    app/c.yaml: Invalid value: \"a.yaml\": " + aLink},
		},
	}
	for _, name := range []string{"kustomization.yaml", "kustomization.yml", "Kustomization"} {
		cases["a "+name+".original"] = dirCase{
			files: with(failing(), map[string]string{"app/" + name + ".original": "resources: [a.yaml]\n"}),
			want:  []string{"  {root}\n    app/" + name + ".original: Forbidden: " + anOriginal},
		}
	}
	for name, c := range cases {
		c.absent = append(c.absent, "the build fails")
		cases[name] = c
	}
	runDir(t, cases)
}

// A link named as Flux's generator names its backup, pointing outside the
// tree, is what the generator's save would write through: nothing is built,
// and neither the tree nor the link's target is touched.
func TestAssertConsistentDirLeavesALinkedTreeAsItWas(t *testing.T) {
	outside := t.TempDir()
	target := filepath.Join(outside, "kustomization.yaml")
	body := "resources: [outside.yaml]\n"
	if err := os.WriteFile(target, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	writeTree(t, dir, failing())
	writeLinks(t, dir, map[string]string{"app/kustomization.yaml.original": target})
	before := snapshot(t, dir)
	f := run(func(tb testing.TB) { AssertConsistentDir(tb, dir, pathDeclared...) })
	consistentCase{
		want:   []string{"  " + dir + "\n    app/kustomization.yaml.original: Invalid value: " + fmt.Sprintf("%q", target) + ": " + aLink},
		absent: []string{"the build fails"},
	}.check(t, f)
	if after := snapshot(t, dir); !maps.Equal(before, after) {
		t.Errorf("the tree changed:\nbefore %v\nafter  %v", before, after)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != body {
		t.Errorf("the link's target changed:\nbefore %q\nafter  %q", body, got)
	}
}

// The copy is made under TMPDIR, as kuretest-consistent-*, and is gone once
// the call returns, whether the tree passes or not.
func TestAssertConsistentDirRemovesItsCopy(t *testing.T) {
	for name, c := range map[string]struct {
		files map[string]string
		want  string
	}{
		"a tree that builds":         {builds(), ""},
		"a tree that does not build": {failing(), "the build fails"},
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			writeTree(t, dir, c.files)
			copies := t.TempDir()
			t.Setenv("TMPDIR", copies)
			f := run(func(tb testing.TB) { AssertConsistentDir(tb, dir, pathDeclared...) })
			if !strings.Contains(f.fatal, c.want) || (c.want == "") != (f.fatal == "") {
				t.Fatalf("want %q, got %q", c.want, f.fatal)
			}
			left, err := os.ReadDir(copies)
			if err != nil {
				t.Fatal(err)
			}
			for _, e := range left {
				t.Errorf("left behind under TMPDIR: %s", e.Name())
			}
		})
	}
	// where TMPDIR cannot hold the copy, the call fails rather than passing
	// unbuilt
	dir := t.TempDir()
	writeTree(t, dir, builds())
	missing := filepath.Join(t.TempDir(), "missing")
	t.Setenv("TMPDIR", missing)
	if f := run(func(tb testing.TB) { AssertConsistentDir(tb, dir, pathDeclared...) }); !strings.Contains(f.fatal, "kuretest: ") || !strings.Contains(f.fatal, missing) {
		t.Errorf("want a kuretest failure naming %s, got %q", missing, f.fatal)
	}
}
