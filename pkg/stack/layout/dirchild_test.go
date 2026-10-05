package layout_test

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// Tests for directory children (go-kure/kure#979): a parent's
// kustomization.yaml lists a directory child by its Name, so the child is
// expected at <parent directory>/<Name>. A child whose Namespace is that path,
// its own, instead of its parent's is written one level below it, and the
// entry names a directory that holds no kustomization.yaml.

// namedChild is a layout of one ConfigMap, named name, in ns.
func namedChild(name, ns string) *layout.ManifestLayout {
	child := cmLayout("payload-"+strings.ReplaceAll(name, "/", "-"), ns)
	child.Name = name
	return child
}

func fluxSourceKind(kind string) *schema.GroupVersionKind {
	return &schema.GroupVersionKind{Group: "source.toolkit.fluxcd.io", Version: "v1", Kind: kind}
}

// TestWriters_RefuseDirectoryChildNestedBelowItsEntry: every writer whose
// parent kustomization.yaml would list the child refuses the tree before
// writing anything, naming both layouts, the directory written and the
// directory the parent lists.
func TestWriters_RefuseDirectoryChildNestedBelowItsEntry(t *testing.T) {
	cases := map[string]struct {
		tree    func() *layout.ManifestLayout
		writers []string
		// want are fragments every refusal holds, on top of the advice.
		want []string
	}{
		// The probe: a root "." whose child sets Namespace to its own full
		// path. WriteManifest used to write no kustomization.yaml for a root
		// that holds nothing itself, so it had no entry to dangle and wrote
		// the nested tree; it writes that file now (go-kure/kure#979) and
		// refuses like the other two.
		"unnamed root": {
			tree: func() *layout.ManifestLayout {
				return &layout.ManifestLayout{Name: "", Namespace: ".", Children: []*layout.ManifestLayout{
					namedChild("flux-system", "flux-system"),
				}}
			},
			writers: allWriters,
			want:    []string{`layout "flux-system/flux-system"`, `flux-system/flux-system", not to "`, `parent layout "."`, `lists as "flux-system"`, `Namespace "flux-system"`},
		},
		"named parent": {
			tree: func() *layout.ManifestLayout {
				parent := cmLayout("apps", ".")
				parent.Children = []*layout.ManifestLayout{namedChild("web", "apps/web")}
				return parent
			},
			writers: allWriters,
			want:    []string{`layout "apps/web/web"`, `apps/web/web", not to "`, `apps/web", the directory`, `parent layout "apps"`, `lists as "web"`, `Namespace "apps/web" is the child's own path`},
		},
		// The same mistake in another case: the entry Web names apps/Web, which
		// does not exist on a case-sensitive volume and is apps/web, holding
		// only the child's directory, on a case-insensitive one.
		"Namespace is the child's own path in another case": {
			tree: func() *layout.ManifestLayout {
				child := namedChild("web", "apps/web")
				child.Name = "Web"
				parent := cmLayout("apps", ".")
				parent.Children = []*layout.ManifestLayout{child}
				return parent
			},
			writers: allWriters,
			want:    []string{`layout "apps/web/Web"`, `apps/web/Web", not to "`, `apps/Web", the directory`, `parent layout "apps"`, `lists as "Web"`, `Namespace "apps/web" differs only in case from the child's own path`},
		},
		"grandchild": {
			tree: func() *layout.ManifestLayout {
				mid := cmLayout("apps", "root")
				mid.Children = []*layout.ManifestLayout{namedChild("web", "root/apps/web")}
				root := cmLayout("root", ".")
				root.Children = []*layout.ManifestLayout{mid}
				return root
			},
			writers: allWriters,
			want:    []string{`layout "root/apps/web/web"`, `parent layout "root/apps"`},
		},
		"Name of two segments": {
			tree: func() *layout.ManifestLayout {
				parent := cmLayout("apps", ".")
				parent.Children = []*layout.ManifestLayout{namedChild("team/web", "apps/team/web")}
				return parent
			},
			writers: allWriters,
			want:    []string{`layout "apps/team/web/team/web"`, `lists as "team/web"`},
		},
		// A rooted Namespace resolves under the writer's root, for the parent
		// and the child alike.
		"rooted Namespaces": {
			tree: func() *layout.ManifestLayout {
				parent := cmLayout("apps", "/")
				parent.Children = []*layout.ManifestLayout{namedChild("web", "/apps/web")}
				return parent
			},
			writers: allWriters,
			want:    []string{`layout "/apps/web/web"`, `parent layout "/apps"`},
		},
		// An AppFileSingle root writes its kustomization.yaml into its
		// Namespace and lists its children there: the directories compared are
		// the writer's, not FullRepoPath's.
		"AppFileSingle root": {
			tree: func() *layout.ManifestLayout {
				root := cmLayout("svc", "demo")
				root.ApplicationFileMode = layout.AppFileSingle
				root.Children = []*layout.ManifestLayout{namedChild("gc", "demo/gc")}
				return root
			},
			writers: allWriters,
			want:    []string{`layout "demo/gc/gc"`, `demo/gc", the directory`, `parent layout "demo/svc"`},
		},
		// WriteManifest lists a child of another package; WriteToDisk and
		// WriteToTar do not (see TestWriters_UnlistedDirectoryChildMayNest).
		"child of another package, where the parent lists it": {
			tree: func() *layout.ManifestLayout {
				child := namedChild("web", "apps/web")
				child.PackageRef = fluxSourceKind("GitRepository")
				parent := cmLayout("apps", ".")
				parent.PackageRef = fluxSourceKind("OCIRepository")
				parent.Children = []*layout.ManifestLayout{child}
				return parent
			},
			writers: []string{"WriteManifest"},
			want:    []string{`layout "apps/web/web"`, `parent layout "apps"`},
		},
	}
	for name, tc := range cases {
		for _, writer := range tc.writers {
			t.Run(name+"/"+writer, func(t *testing.T) {
				err := writeRefused(t, writer, layout.Config{}, tc.tree())
				if err == nil {
					t.Fatal("the tree was written: its parent's kustomization.yaml lists a directory above the child's")
				}
				for _, want := range append(tc.want, "set Namespace to the parent's path") {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("refusal does not hold %s:\n%v", want, err)
					}
				}
			})
		}
	}
}

// TestWriters_DirectoryChildInParentPathIsListed is the control: with the
// child's Namespace set to its parent's path, every writer writes the tree and
// every kustomization.yaml entry names a directory that holds one. Layouts of
// one name nest, and are not the refused shape.
func TestWriters_DirectoryChildInParentPathIsListed(t *testing.T) {
	cases := map[string]struct {
		tree      func() *layout.ManifestLayout
		childKust []string
	}{
		"unnamed root": {
			tree: func() *layout.ManifestLayout {
				return &layout.ManifestLayout{Name: "", Namespace: ".", Children: []*layout.ManifestLayout{
					namedChild("flux-system", "."),
				}}
			},
			childKust: []string{"flux-system/kustomization.yaml"},
		},
		"named parent": {
			tree: func() *layout.ManifestLayout {
				parent := cmLayout("apps", ".")
				parent.Children = []*layout.ManifestLayout{namedChild("web", parent.FullRepoPath())}
				return parent
			},
			childKust: []string{"apps/web/kustomization.yaml"},
		},
		"rooted Namespaces": {
			tree: func() *layout.ManifestLayout {
				parent := cmLayout("apps", "/")
				parent.Children = []*layout.ManifestLayout{namedChild("web", parent.FullRepoPath())}
				return parent
			},
			childKust: []string{"apps/web/kustomization.yaml"},
		},
		"Name of two segments": {
			tree: func() *layout.ManifestLayout {
				parent := cmLayout("apps", ".")
				parent.Children = []*layout.ManifestLayout{namedChild("team/web", parent.FullRepoPath())}
				return parent
			},
			childKust: []string{"apps/team/web/kustomization.yaml"},
		},
		// web/web/web: each child's Namespace is its parent's path, which ends
		// in the child's own Name.
		"parent, child and grandchild of one name": {
			tree: func() *layout.ManifestLayout {
				root := cmLayout("web", ".")
				child := namedChild("web", root.FullRepoPath())
				grandchild := cmLayout("deepest", child.FullRepoPath())
				grandchild.Name = "web"
				child.Children = []*layout.ManifestLayout{grandchild}
				root.Children = []*layout.ManifestLayout{child}
				return root
			},
			childKust: []string{"web/web/kustomization.yaml", "web/web/web/kustomization.yaml"},
		},
		// An umbrella child is not listed by its parent at all; the walkers
		// refuse one named like its umbrella, a hand-built tree may hold one.
		"umbrella child of its parent's name": {
			tree: func() *layout.ManifestLayout {
				root := cmLayout("web", ".")
				child := namedChild("web", root.FullRepoPath())
				child.UmbrellaChild = true
				root.Children = []*layout.ManifestLayout{child}
				return root
			},
			childKust: []string{"web/web/kustomization.yaml"},
		},
		"AppFileSingle root, child of the root's name": {
			tree: func() *layout.ManifestLayout {
				root := cmLayout("svc", "demo")
				root.ApplicationFileMode = layout.AppFileSingle
				root.Children = []*layout.ManifestLayout{namedChild("svc", root.Namespace)}
				return root
			},
			childKust: []string{"demo/svc/kustomization.yaml"},
		},
	}
	for name, tc := range cases {
		for _, writer := range allWriters {
			t.Run(name+"/"+writer, func(t *testing.T) {
				files := writtenFiles(t, writer, layout.Config{}, tc.tree())
				for _, want := range tc.childKust {
					if _, ok := files[want]; !ok {
						t.Errorf("no %s written; wrote %v", want, slices.Sorted(maps.Keys(files)))
					}
				}
				if bad := danglingRefs(t, files); len(bad) > 0 {
					t.Errorf("entries name nothing written: %v", bad)
				}
			})
		}
	}
}

// TestWriters_DirectoryChildPlacedElsewhereIsWritten: a caller may place a
// child elsewhere than below its parent on purpose. The shape here is an
// artifact whose root lists sibling layers, one of which lives under a group
// directory that has no layout of its own. It is not the refused shape (its
// Namespace is not its own path) and is written as before; the root's
// kustomization.yaml is then not a valid kustomize entry point, which is the
// caller's choice. A group directory that differs from the child's Name by
// more than case is such a placement too, and is not the refused shape.
func TestWriters_DirectoryChildPlacedElsewhereIsWritten(t *testing.T) {
	cases := map[string]struct {
		tree func() *layout.ManifestLayout
		want []string
	}{
		"sibling layers under a group directory": {
			tree: func() *layout.ManifestLayout {
				return &layout.ManifestLayout{Name: "", Namespace: ".", Children: []*layout.ManifestLayout{
					namedChild("flux-system", "."),
					namedChild("flux-system-platform", "."),
					namedChild("cert-manager", "platform"),
				}}
			},
			want: []string{
				"flux-system/kustomization.yaml",
				"flux-system-platform/kustomization.yaml",
				"platform/cert-manager/kustomization.yaml",
			},
		},
		"group directory that differs from the Name by more than case": {
			tree: func() *layout.ManifestLayout {
				child := namedChild("web", "apps/webs")
				child.Name = "Web"
				parent := cmLayout("apps", ".")
				parent.Children = []*layout.ManifestLayout{child}
				return parent
			},
			want: []string{"apps/kustomization.yaml", "apps/webs/Web/kustomization.yaml"},
		},
	}
	for name, tc := range cases {
		for _, writer := range allWriters {
			t.Run(name+"/"+writer, func(t *testing.T) {
				files := writtenFiles(t, writer, layout.Config{}, tc.tree())
				for _, want := range tc.want {
					if _, ok := files[want]; !ok {
						t.Errorf("no %s written; wrote %v", want, slices.Sorted(maps.Keys(files)))
					}
				}
			})
		}
	}
}

// TestWriters_UnlistedDirectoryChildMayNest: a child its parent's
// kustomization.yaml does not list has no entry to dangle, so the nested shape
// is written. Each writer decides by its own plan.
func TestWriters_UnlistedDirectoryChildMayNest(t *testing.T) {
	cases := map[string]struct {
		tree    func() *layout.ManifestLayout
		writers []string
		written string
	}{
		// No writer writes a kustomization.yaml for a KustomizationRecursive
		// root, so none lists its child.
		"root that writes no kustomization.yaml": {
			tree: func() *layout.ManifestLayout {
				return &layout.ManifestLayout{Name: "", Namespace: ".", Mode: layout.KustomizationRecursive, Children: []*layout.ManifestLayout{
					namedChild("flux-system", "flux-system"),
				}}
			},
			writers: allWriters,
			written: "flux-system/flux-system/kustomization.yaml",
		},
		// WriteToDisk and WriteToTar do not list a child of another package.
		"child of another package": {
			tree: func() *layout.ManifestLayout {
				child := namedChild("web", "apps/web")
				child.PackageRef = fluxSourceKind("GitRepository")
				parent := cmLayout("apps", ".")
				parent.PackageRef = fluxSourceKind("OCIRepository")
				parent.Children = []*layout.ManifestLayout{child}
				return parent
			},
			writers: []string{"WriteToDisk", "WriteToTar"},
			written: "apps/web/web/kustomization.yaml",
		},
	}
	for name, tc := range cases {
		for _, writer := range tc.writers {
			t.Run(name+"/"+writer, func(t *testing.T) {
				files := writtenFiles(t, writer, layout.Config{}, tc.tree())
				if _, ok := files[tc.written]; !ok {
					t.Fatalf("no %s written; wrote %v", tc.written, slices.Sorted(maps.Keys(files)))
				}
				if bad := danglingRefs(t, files); len(bad) > 0 {
					t.Errorf("entries name nothing written: %v", bad)
				}
			})
		}
	}
}

// TestWalkers_SameNameLevelsAreWritten: a walked tree never meets the refusal,
// also where a level carries the name of the one above it, so that a child's
// Namespace, its parent's path, ends in the child's own Name: a bundle named
// like its node, an application named like its bundle, a child node named
// like its parent node. Under every grouping, with and without a ClusterName,
// both walkers build a tree every writer writes.
func TestWalkers_SameNameLevelsAreWritten(t *testing.T) {
	apps := func(names ...string) []*stack.Application {
		var out []*stack.Application
		for _, name := range names {
			out = append(out, configMapApp(name))
		}
		return out
	}
	clusters := map[string]func() *stack.Cluster{
		"bundle named like its node": func() *stack.Cluster {
			root := &stack.Node{Name: "platform", Bundle: &stack.Bundle{Name: "platform", Applications: apps("api", "db")}}
			return &stack.Cluster{Name: "demo", Node: root}
		},
		"application named like its bundle": func() *stack.Cluster {
			store := &stack.Node{Name: "store", Bundle: &stack.Bundle{Name: "web", Applications: apps("web", "api")}}
			root := &stack.Node{Name: "shop", Children: []*stack.Node{store}}
			store.SetParent(root)
			return &stack.Cluster{Name: "demo", Node: root}
		},
		"child node named like its parent node": func() *stack.Cluster {
			child := &stack.Node{Name: "platform", Bundle: &stack.Bundle{Name: "core", Applications: apps("api")}}
			root := &stack.Node{Name: "platform", Children: []*stack.Node{child}}
			child.SetParent(root)
			return &stack.Cluster{Name: "demo", Node: root}
		},
	}
	groupings := []layout.GroupingMode{layout.GroupByName, layout.GroupFlat}
	for clusterCase, cluster := range clusters {
		for _, clusterName := range []string{"", ".", "prod", "platform", "clusters/platform"} {
			for _, nodes := range groupings {
				for _, bundles := range groupings {
					for _, appGrouping := range groupings {
						rules := layout.LayoutRules{
							ClusterName:         clusterName,
							NodeGrouping:        nodes,
							BundleGrouping:      bundles,
							ApplicationGrouping: appGrouping,
						}
						name := fmt.Sprintf("%s/ClusterName=%q/nodes=%v/bundles=%v/apps=%v", clusterCase, clusterName, nodes, bundles, appGrouping)
						t.Run(name, func(t *testing.T) {
							trees := map[string]*layout.ManifestLayout{}
							ml, err := layout.WalkCluster(cluster(), rules)
							if err != nil {
								t.Fatalf("WalkCluster: %v", err)
							}
							trees["WalkCluster"] = ml
							byPackage, err := layout.WalkClusterByPackage(cluster(), rules)
							if err != nil {
								t.Fatalf("WalkClusterByPackage: %v", err)
							}
							for key, pkg := range byPackage {
								trees["WalkClusterByPackage/"+key] = pkg
							}
							for walker, tree := range trees {
								for writer, err := range writers(t, tree) {
									if err != nil {
										t.Errorf("%s, %s: %v", walker, writer, err)
									}
								}
							}
						})
					}
				}
			}
		}
	}
}
