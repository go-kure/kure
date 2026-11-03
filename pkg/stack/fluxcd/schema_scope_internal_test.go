package fluxcd

import (
	"os"
	"path/filepath"
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	"sigs.k8s.io/kustomize/kyaml/openapi"
)

// scopeTree writes a tree of one directory app, which builds a Widget of
// example.com/v1, a kind kustomize's built-in schema does not know, and returns
// the treeBuilds over it and a Kustomization of app that sets the target
// namespace apps.
func scopeTree(t *testing.T) (*treeBuilds, *kustv1.Kustomization) {
	t.Helper()
	tree := t.TempDir()
	files := map[string]string{
		"app/kustomization.yaml": "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\nresources:\n  - widget.yaml\n",
		"app/widget.yaml":        "apiVersion: example.com/v1\nkind: Widget\nmetadata:\n  name: w\n",
	}
	for name, content := range files {
		p := filepath.Join(tree, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	k := &kustv1.Kustomization{}
	k.Name, k.Namespace = "app", "flux-system"
	k.Spec.Path, k.Spec.TargetNamespace = "app", "apps"
	return &treeBuilds{dir: tree, written: true}, k
}

// TestTreeBuilds_APathOnlySchemaIsNotRead: openapi.AddSchema also reads the
// paths of a schema and records which kinds are cluster-scoped, which neither
// the definitions nor the version show. A schema holding one path of the kind
// Widget and no definition makes kustomize give a Widget no namespace, where
// Flux's build gives it apps (go-kure/kure#1166). The control is the unchanged
// schema: it builds the same tree and puts the Widget in apps.
func TestTreeBuilds_APathOnlySchemaIsNotRead(t *testing.T) {
	if !inDefinitionsProcess(t, "TestTreeBuilds_APathOnlySchemaIsNotRead") {
		return
	}
	b, k := scopeTree(t)
	objs, err := b.build(k)
	if err != nil {
		t.Fatalf("with the built-in schema: got %v, want the build read", err)
	}
	if len(objs) != 1 || objs[0].GetNamespace() != "apps" {
		t.Fatalf("with the built-in schema: got %v, want one Widget in apps", objs)
	}

	paths := `{"swagger":"2.0","info":{"title":"x","version":"v1"},"paths":{"/apis/example.com/v1/widgets":{"get":{"x-kubernetes-group-version-kind":{"group":"example.com","version":"v1","kind":"Widget"}}}},"definitions":{}}`
	if err := openapi.AddSchema([]byte(paths)); err != nil {
		t.Fatalf("add a schema: %v", err)
	}
	changed, k := scopeTree(t)
	_, err = changed.build(k)
	mustBeUnread(t, err, "scope of example.com/v1 Widget", "not its built-in one")
}

// deletedWidgetTree is a tree of one directory app that loads a Widget and
// then deletes it with a patch, so the final resmap holds no Widget: kustomize
// read the scope of the kind on the way (name references, vars), where a
// schema added with a path alone changes the answer.
func deletedWidgetTree(t *testing.T, files map[string]string) (*treeBuilds, *kustv1.Kustomization) {
	t.Helper()
	tree := t.TempDir()
	for name, content := range files {
		p := filepath.Join(tree, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	k := &kustv1.Kustomization{}
	k.Name, k.Namespace = "app", "flux-system"
	k.Spec.Path = "app"
	return &treeBuilds{dir: tree, written: true}, k
}

const widgetPathOnlySchema = `{"swagger":"2.0","info":{"title":"x","version":"v1"},"paths":{"/apis/example.com/v1/widgets":{"get":{"x-kubernetes-group-version-kind":{"group":"example.com","version":"v1","kind":"Widget"}}}},"definitions":{}}`

// TestTreeBuilds_AKindALaterOverlayDeletesIsAsked: the scope of a kind is read
// when the build loads it, so a kind an overlay deletes later is asked about
// too, from the files and the inline patches the build reads, not only from
// the objects it ends with (go-kure/kure#1235). The control builds the same
// trees with the built-in schema.
func TestTreeBuilds_AKindALaterOverlayDeletesIsAsked(t *testing.T) {
	if !inDefinitionsProcess(t, "TestTreeBuilds_AKindALaterOverlayDeletesIsAsked") {
		return
	}
	const widget = "apiVersion: example.com/v1\nkind: Widget\nmetadata:\n  name: w\n"
	const deletion = "apiVersion: example.com/v1\nkind: Widget\nmetadata:\n  name: w\n$patch: delete\n"
	trees := map[string]map[string]string{
		"a deleting patch file": {
			"app/kustomization.yaml": "resources:\n  - widget.yaml\npatches:\n  - path: delete.yaml\n",
			"app/widget.yaml":        widget,
			"app/delete.yaml":        deletion,
		},
		"an inline deleting patch": {
			"app/kustomization.yaml": "resources:\n  - widget.yaml\npatches:\n  - patch: |-\n      apiVersion: example.com/v1\n      kind: Widget\n      metadata:\n        name: w\n      $patch: delete\n",
			"app/widget.yaml":        widget,
		},
		"a patch file naming one object twice": {
			"app/kustomization.yaml": "resources:\n  - widget.yaml\npatches:\n  - path: twice.yaml\n",
			"app/widget.yaml":        widget,
			"app/twice.yaml":         "apiVersion: example.com/v1\nkind: Widget\nmetadata:\n  name: w\n---\n" + deletion,
		},
		"a deleting strategic merge entry": {
			"app/kustomization.yaml": "resources:\n  - widget.yaml\npatchesStrategicMerge:\n  - |-\n    apiVersion: example.com/v1\n    kind: Widget\n    metadata:\n      name: w\n    $patch: delete\n",
			"app/widget.yaml":        widget,
		},
	}
	for name, files := range trees {
		b, k := deletedWidgetTree(t, files)
		objs, err := b.build(k)
		if err != nil || len(objs) != 0 {
			t.Fatalf("%s, with the built-in schema: got %v, %v, want a read build with no object", name, objs, err)
		}
	}
	if err := openapi.AddSchema([]byte(widgetPathOnlySchema)); err != nil {
		t.Fatalf("add a schema: %v", err)
	}
	for name, files := range trees {
		b, k := deletedWidgetTree(t, files)
		_, err := b.build(k)
		t.Run(name, func(t *testing.T) {
			mustBeUnread(t, err, "scope of example.com/v1 Widget", "not its built-in one")
		})
	}
}
