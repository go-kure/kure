package layout_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/kyaml/filesys"

	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// Tests for object identities across the layouts of one kustomize build
// (go-kure/kure#880): kustomize refuses to add an object whose identity the
// build already holds, so a kustomization.yaml kure writes, or the Flux build
// of a marked KustomizationRecursive directory, that takes in two layouts
// holding one object would not build.

// objIn returns an object of the given apiVersion and kind in namespace ns.
func objIn(apiVersion, kind, ns, name string) client.Object {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion(apiVersion)
	obj.SetKind(kind)
	obj.SetName(name)
	obj.SetNamespace(ns)
	return obj
}

// dupParent is Explicit root p holding ConfigMap default/x, with an Explicit
// directory child c holding the same object, which p's kustomization.yaml
// lists: the case go-kure/kure#880 reports.
func dupParent() *layout.ManifestLayout {
	p := &layout.ManifestLayout{
		Name:      "p",
		Namespace: ".",
		Mode:      layout.KustomizationExplicit,
		Resources: []client.Object{testObj("v1", "ConfigMap", "x")},
	}
	c := child(p, "c", layout.KustomizationExplicit)
	c.Resources = []client.Object{testObj("v1", "ConfigMap", "x")}
	return p
}

// TestWriters_RefuseDuplicateAcrossBuild: every writer refuses, before writing
// anything, two layouts that one kustomize build takes in and that hold one
// object, naming both layouts and the build.
func TestWriters_RefuseDuplicateAcrossBuild(t *testing.T) {
	cases := map[string]struct {
		build func() *layout.ManifestLayout
		want  string
	}{
		"Explicit parent lists a child directory": {
			build: dupParent,
			want:  `layouts "p" and "p/c" both hold the object v1 ConfigMap default/x, and the kustomization.yaml of layout "p" builds both`,
		},
		// p lists c, whose kustomization.yaml lists g.
		"grandchild, three levels": {
			build: func() *layout.ManifestLayout {
				p := dupParent()
				c := p.Children[0]
				c.Resources = []client.Object{testObj("v1", "Secret", "c")}
				g := child(c, "g", layout.KustomizationExplicit)
				g.Resources = []client.Object{testObj("v1", "ConfigMap", "x")}
				return p
			},
			want: `layouts "p" and "p/c/g" both hold the object v1 ConfigMap default/x, and the kustomization.yaml of layout "p" builds both`,
		},
		// The innermost build that takes in both is the one named.
		"child and grandchild": {
			build: func() *layout.ManifestLayout {
				p := dupParent()
				p.Resources = []client.Object{testObj("v1", "Secret", "p")}
				g := child(p.Children[0], "g", layout.KustomizationExplicit)
				g.Resources = []client.Object{testObj("v1", "ConfigMap", "x")}
				return p
			},
			want: `layouts "p/c" and "p/c/g" both hold the object v1 ConfigMap default/x, and the kustomization.yaml of layout "p/c" builds both`,
		},
		// p's kustomization.yaml lists s.yaml, s's file in p's directory.
		"AppFileSingle child": {
			build: func() *layout.ManifestLayout {
				p := dupParent()
				p.Children = nil
				s := child(p, "s", layout.KustomizationUnset)
				s.ApplicationFileMode = layout.AppFileSingle
				s.Resources = []client.Object{testObj("v1", "ConfigMap", "x")}
				return p
			},
			want: `layouts "p" and "p/s" both hold the object v1 ConfigMap default/x, and the kustomization.yaml of layout "p" builds both`,
		},
		"siblings both listed": {
			build: func() *layout.ManifestLayout {
				p := dupParent()
				p.Resources = []client.Object{testObj("v1", "Secret", "p")}
				b := child(p, "b", layout.KustomizationExplicit)
				b.Resources = []client.Object{testObj("v1", "ConfigMap", "x")}
				return p
			},
			want: `layouts "p/c" and "p/b" both hold the object v1 ConfigMap default/x, and the kustomization.yaml of layout "p" builds both`,
		},
		// kustomize reads an omitted namespace as "default".
		"omitted namespace is default": {
			build: func() *layout.ManifestLayout {
				p := dupParent()
				p.Children[0].Resources = []client.Object{objIn("v1", "ConfigMap", "", "x")}
				return p
			},
			want: `layouts "p" and "p/c" both hold the object v1 ConfigMap default/x`,
		},
		// kustomize builds a List's items, so an item is an object of the
		// build.
		"object inside a List": {
			build: func() *layout.ManifestLayout {
				p := dupParent()
				list := &unstructured.Unstructured{Object: map[string]any{
					"apiVersion": "v1",
					"kind":       "List",
					"items": []any{map[string]any{
						"apiVersion": "v1", "kind": "ConfigMap",
						"metadata": map[string]any{"name": "x", "namespace": "default"},
					}},
				}}
				p.Children[0].Resources = []client.Object{list}
				return p
			},
			want: `layouts "p" and "p/c" both hold the object v1 ConfigMap default/x`,
		},
		// The Flux build of a marked Recursive directory takes in every
		// directory below it that no kustomization.yaml shields, and each
		// directory with one as a whole.
		"marked Recursive directory over two children": {
			build: func() *layout.ManifestLayout {
				r := recursiveTree()
				r.SetFluxBuild(true)
				r.Resources = []client.Object{testObj("v1", "Secret", "r")}
				r.Children[0].Resources = []client.Object{testObj("v1", "ConfigMap", "x")}
				u := child(r, "u", layout.KustomizationRecursive)
				u.Resources = []client.Object{testObj("v1", "ConfigMap", "x")}
				return r
			},
			want: `layouts "r/c" and "r/u" both hold the object v1 ConfigMap default/x, and the Flux build of KustomizationRecursive layout "r" includes both`,
		},
		"marked Recursive directory over its own object": {
			build: func() *layout.ManifestLayout {
				r := recursiveTree()
				r.SetFluxBuild(true)
				g := child(child(r, "u", layout.KustomizationRecursive), "g", layout.KustomizationExplicit)
				g.Resources = []client.Object{testObj("v1", "ConfigMap", "a")}
				return r
			},
			want: `layouts "r" and "r/u/g" both hold the object v1 ConfigMap default/a, and the Flux build of KustomizationRecursive layout "r" includes both`,
		},
		// r's Flux build takes in c as c's own build, which lists g.
		"marked Recursive directory over a listed grandchild directory": {
			build: func() *layout.ManifestLayout {
				r := recursiveTree()
				r.SetFluxBuild(true)
				g := child(r.Children[0], "g", layout.KustomizationExplicit)
				g.Resources = []client.Object{testObj("v1", "ConfigMap", "a")}
				return r
			},
			want: `layouts "r" and "r/c/g" both hold the object v1 ConfigMap default/a, and the Flux build of KustomizationRecursive layout "r" includes both`,
		},
		// The same through the file of an AppFileSingle grandchild that c's
		// kustomization.yaml lists.
		"marked Recursive directory over a listed AppFileSingle grandchild": {
			build: func() *layout.ManifestLayout {
				r := recursiveTree()
				r.SetFluxBuild(true)
				g := child(r.Children[0], "g", layout.KustomizationUnset)
				g.ApplicationFileMode = layout.AppFileSingle
				g.Resources = []client.Object{testObj("v1", "ConfigMap", "a")}
				return r
			},
			want: `layouts "r" and "r/c/g" both hold the object v1 ConfigMap default/a, and the Flux build of KustomizationRecursive layout "r" includes both`,
		},
		// kustomize knows Namespace is cluster-scoped and ignores the
		// namespace field of one: both are the object Namespace foo.
		"cluster-scoped kind with a namespace field": {
			build: func() *layout.ManifestLayout {
				p := dupParent()
				p.Resources = []client.Object{objIn("v1", "Namespace", "", "foo")}
				p.Children[0].Resources = []client.Object{objIn("v1", "Namespace", "bar", "foo")}
				return p
			},
			want: `layouts "p" and "p/c" both hold the object v1 Namespace foo, and the kustomization.yaml of layout "p" builds both`,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			for _, writer := range allWriters {
				err := writeRefused(t, writer, layout.DefaultLayoutConfig(), tc.build())
				if err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Errorf("%s: err = %v, want it to contain %q", writer, err, tc.want)
				}
			}
		})
	}
}

// TestWriters_RefuseDuplicateWithListedOtherPackageChild: WriteManifest lists
// a child of another package, so its build takes in the child; WriteToDisk
// and WriteToTar do not list it, and accept the tree.
func TestWriters_RefuseDuplicateWithListedOtherPackageChild(t *testing.T) {
	tree := func() *layout.ManifestLayout {
		p := dupParent()
		p.PackageRef = &schema.GroupVersionKind{Group: "source.toolkit.fluxcd.io", Version: "v1", Kind: "OCIRepository"}
		p.Children[0].PackageRef = &schema.GroupVersionKind{Group: "source.toolkit.fluxcd.io", Version: "v1", Kind: "GitRepository"}
		return p
	}
	err := writeRefused(t, "WriteManifest", layout.DefaultLayoutConfig(), tree())
	want := `layouts "p" and "p/c" both hold the object v1 ConfigMap default/x`
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("WriteManifest: err = %v, want it to contain %q", err, want)
	}
	for _, writer := range []string{"WriteToDisk", "WriteToTar"} {
		_ = writtenFiles(t, writer, layout.DefaultLayoutConfig(), tree())
	}
	kustomizeBuildsAll(t, tree())
}

// TestWriters_AcceptSameObjectInSeparateBuilds: one object in two layouts
// that no build the writers produce takes in together, or two objects whose
// identities differ, are written; kustomize builds every kustomization.yaml
// WriteToDisk writes.
func TestWriters_AcceptSameObjectInSeparateBuilds(t *testing.T) {
	cases := map[string]func() *layout.ManifestLayout{
		// Applied by its own Flux Kustomization; the parent does not list it.
		"umbrella child": func() *layout.ManifestLayout {
			p := dupParent()
			p.Children[0].UmbrellaChild = true
			return p
		},
		// The parent lists the child's Kustomization CR, not its directory.
		"child of a FluxIntegratedPerLayout parent": func() *layout.ManifestLayout {
			p := dupParent()
			p.FluxPlacement = layout.FluxIntegratedPerLayout
			return p
		},
		"different namespace": func() *layout.ManifestLayout {
			p := dupParent()
			p.Children[0].Resources = []client.Object{objIn("v1", "ConfigMap", "other", "x")}
			return p
		},
		"different kind": func() *layout.ManifestLayout {
			p := dupParent()
			p.Children[0].Resources = []client.Object{testObj("v1", "Secret", "x")}
			return p
		},
		"different group": func() *layout.ManifestLayout {
			p := dupParent()
			p.Children[0].Resources = []client.Object{testObj("example.com/v1", "ConfigMap", "x")}
			return p
		},
		// No Flux Kustomization kure generated builds r, and each Explicit
		// child is its own build.
		"unmarked Recursive root over two children": func() *layout.ManifestLayout {
			r := recursiveTree()
			r.Children[0].Resources = []client.Object{testObj("v1", "ConfigMap", "x")}
			child(r, "d", layout.KustomizationExplicit).Resources = []client.Object{testObj("v1", "ConfigMap", "x")}
			return r
		},
		// r's Flux build takes in c as a whole, and c's kustomization.yaml
		// does not list its umbrella child u.
		"marked Recursive build shielded by an Explicit directory": func() *layout.ManifestLayout {
			r := recursiveTree()
			r.SetFluxBuild(true)
			u := child(r.Children[0], "u", layout.KustomizationExplicit)
			u.UmbrellaChild = true
			u.Resources = []client.Object{testObj("v1", "ConfigMap", "a")}
			return r
		},
		// g's file lands in c's directory, which c's kustomization.yaml
		// shields from r's Flux build, and c does not list an umbrella
		// child.
		"marked Recursive build shielded from an unlisted AppFileSingle file": func() *layout.ManifestLayout {
			r := recursiveTree()
			r.SetFluxBuild(true)
			g := child(r.Children[0], "g", layout.KustomizationUnset)
			g.ApplicationFileMode = layout.AppFileSingle
			g.UmbrellaChild = true
			g.Resources = []client.Object{testObj("v1", "ConfigMap", "a")}
			return r
		},
		// kustomize's identity includes the version, so two versions of
		// one object build.
		"different version": func() *layout.ManifestLayout {
			p := dupParent()
			p.Resources = []client.Object{objIn("autoscaling/v1", "HorizontalPodAutoscaler", "default", "x")}
			p.Children[0].Resources = []client.Object{objIn("autoscaling/v2", "HorizontalPodAutoscaler", "default", "x")}
			return p
		},
	}
	for name, build := range cases {
		t.Run(name, func(t *testing.T) {
			for _, writer := range allWriters {
				_ = writtenFiles(t, writer, layout.DefaultLayoutConfig(), build())
			}
			kustomizeBuildsAll(t, build())
		})
	}
}

// TestWriters_AcceptSameObjectInChildUnit: a child layout that renders a
// bundle is a unit of its own, which its parent does not list, so it may hold
// an object its parent holds too.
func TestWriters_AcceptSameObjectInChildUnit(t *testing.T) {
	cluster := func() *stack.Cluster {
		web := &stack.Node{Name: "web", Bundle: &stack.Bundle{Name: "web", Applications: []*stack.Application{configMapApp("x")}}}
		root := &stack.Node{Name: "platform", Bundle: &stack.Bundle{Name: "platform", Applications: []*stack.Application{configMapApp("x")}}, Children: []*stack.Node{web}}
		web.SetParent(root)
		return &stack.Cluster{Name: "demo", Node: root}
	}
	tree := func() *layout.ManifestLayout { return walk(t, cluster(), nodeOnly) }
	web := layoutAt(t, tree(), "platform/web")
	if len(web.OriginBundles()) == 0 || len(web.Resources) == 0 {
		t.Fatalf("platform/web renders no bundle or holds no object: the case needs both")
	}
	for _, writer := range allWriters {
		_ = writtenFiles(t, writer, layout.DefaultLayoutConfig(), tree())
	}
	kustomizeBuildsAll(t, tree())
}

// kustomizeBuildsAll writes ml with WriteToDisk and runs a kustomize build of
// every directory that holds a kustomization.yaml, as Flux's
// kustomize-controller builds a Kustomization's spec.path.
func kustomizeBuildsAll(t *testing.T, ml *layout.ManifestLayout) {
	t.Helper()
	dir := t.TempDir()
	if err := ml.WriteToDisk(dir); err != nil {
		t.Fatalf("WriteToDisk: %v", err)
	}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() != "kustomization.yaml" {
			return err
		}
		kdir := filepath.Dir(p)
		if _, err := krusty.MakeKustomizer(krusty.MakeDefaultOptions()).Run(filesys.MakeFsOnDisk(), kdir); err != nil {
			data, _ := os.ReadFile(p)
			t.Errorf("kustomize build %s: %v\n%s", kdir, err, data)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
