package layout_test

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// Tests for the checks of a Flux-delivered tree (go-kure/kure#977): a layout
// its parent does not list must be built by a Flux Kustomization, no two Flux
// Kustomizations in a tree share a namespace and name, and no layout's Name or
// Namespace climbs out of the destination.

// fluxParent is Explicit root p (a ConfigMap) with Explicit directory child c
// (a Secret), which p's kustomization.yaml lists.
func fluxParent() *layout.ManifestLayout {
	p := &layout.ManifestLayout{
		Name:      "p",
		Namespace: ".",
		Mode:      layout.KustomizationExplicit,
		Resources: []client.Object{testObj("v1", "ConfigMap", "p")},
	}
	child(p, "c", layout.KustomizationExplicit)
	return p
}

// fluxKs returns a Flux Kustomization at apiVersion in namespace ns.
func fluxKs(apiVersion, ns, name string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetAPIVersion(apiVersion)
	u.SetKind("Kustomization")
	u.SetName(name)
	u.SetNamespace(ns)
	return u
}

// shopKs is the Flux Kustomization flux-system/shop.
func shopKs() *unstructured.Unstructured {
	return fluxKs("kustomize.toolkit.fluxcd.io/v1", "flux-system", "shop")
}

// listOf returns an object of the given kind holding items in its items
// field, as an application may emit one.
func listOf(kind string, items ...client.Object) *unstructured.Unstructured {
	u := &unstructured.Unstructured{Object: map[string]any{"apiVersion": "v1", "kind": kind}}
	raw := make([]any, 0, len(items))
	for _, it := range items {
		raw = append(raw, it.(*unstructured.Unstructured).Object)
	}
	u.Object["items"] = raw
	return u
}

// typedItems is a typed object with an Items field, as a hand-written list
// type has one.
type typedItems struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Items             []unstructured.Unstructured `json:"items"`
}

func (l *typedItems) DeepCopyObject() runtime.Object {
	c := *l
	c.Items = slices.Clone(l.Items)
	return &c
}

func typedListOf(kind, name string, items ...*unstructured.Unstructured) *typedItems {
	l := &typedItems{TypeMeta: metav1.TypeMeta{APIVersion: "example.com/v1", Kind: kind}}
	l.Name, l.Namespace = name, "default"
	for _, it := range items {
		l.Items = append(l.Items, *it)
	}
	return l
}

// TestWriters_FluxTreeRefusesUnappliedLayout: in a tree Flux delivers (its
// root marked with SetFluxBuild) a layout its parent's kustomization.yaml does
// not list is applied only by a Flux Kustomization of its own, so one that is
// not marked as built by one is refused, naming it. The same tree is written
// once the layout is marked, and when the root is not marked.
func TestWriters_FluxTreeRefusesUnappliedLayout(t *testing.T) {
	cases := map[string]struct {
		build   func() *layout.ManifestLayout
		refused string // the layout the refusal names
		writers []string
	}{
		"umbrella child": {
			build: func() *layout.ManifestLayout {
				p := fluxParent()
				p.Children[0].UmbrellaChild = true
				return p
			},
			refused: "p/c",
		},
		"directory child of a FluxIntegratedPerLayout parent": {
			build: func() *layout.ManifestLayout {
				p := fluxParent()
				p.FluxPlacement = layout.FluxIntegratedPerLayout
				return p
			},
			refused: "p/c",
		},
		// The single file lands in p's directory, where p's kustomization.yaml
		// does not list it.
		"AppFileSingle umbrella child that writes its file": {
			build: func() *layout.ManifestLayout {
				p := fluxParent()
				p.Children[0].UmbrellaChild = true
				p.Children[0].ApplicationFileMode = layout.AppFileSingle
				p.Children[0].Mode = layout.KustomizationUnset
				return p
			},
			refused: "p/c",
		},
		// A marked layout does not answer for the layouts below it.
		"umbrella child below a marked umbrella child": {
			build: func() *layout.ManifestLayout {
				p := fluxParent()
				p.Children[0].UmbrellaChild = true
				p.Children[0].SetFluxBuild(true)
				child(p.Children[0], "g", layout.KustomizationExplicit).UmbrellaChild = true
				return p
			},
			refused: "p/c/g",
		},
		// WriteToDisk and WriteToTar do not list a child of another package;
		// WriteManifest lists it, so p's build applies it there.
		"child of another package": {
			build: func() *layout.ManifestLayout {
				p := fluxParent()
				p.PackageRef = &schema.GroupVersionKind{Group: "source.toolkit.fluxcd.io", Version: "v1", Kind: "OCIRepository"}
				p.Children[0].PackageRef = &schema.GroupVersionKind{Group: "source.toolkit.fluxcd.io", Version: "v1", Kind: "GitRepository"}
				return p
			},
			refused: "p/c",
			writers: []string{"WriteToDisk", "WriteToTar"},
		},
	}
	for name, tc := range cases {
		writers := tc.writers
		if writers == nil {
			writers = allWriters
		}
		for _, writer := range writers {
			t.Run(name+"/"+writer, func(t *testing.T) {
				p := tc.build()
				p.SetFluxBuild(true)
				err := writeRefused(t, writer, layout.DefaultLayoutConfig(), p)
				want := `layout "` + tc.refused + `" is not listed by its parent layout`
				if err == nil || !strings.Contains(err.Error(), want) || !strings.Contains(err.Error(), "no Flux Kustomization is recorded as building it") {
					t.Fatalf("err = %v, want it to contain %q", err, want)
				}

				// Marked, the layout is one a Flux Kustomization builds.
				p = tc.build()
				p.SetFluxBuild(true)
				layoutAt(t, p, tc.refused).SetFluxBuild(true)
				writtenFiles(t, writer, layout.DefaultLayoutConfig(), p)

				// An unmarked root is a tree kure does not know Flux delivers.
				writtenFiles(t, writer, layout.DefaultLayoutConfig(), tc.build())
			})
		}
	}
	t.Run("child of another package/WriteManifest lists it", func(t *testing.T) {
		p := cases["child of another package"].build()
		p.SetFluxBuild(true)
		files := writtenFiles(t, "WriteManifest", layout.DefaultLayoutConfig(), p)
		if got := listedResources(t, files, "p/kustomization.yaml"); !slices.Contains(got, "c") {
			t.Errorf("p/kustomization.yaml lists %v, want it to list c", got)
		}
	})
}

// TestWriters_FluxTreeRefusesUnappliedBundleLayout: a child layout that
// renders a bundle is a unit its parent does not list, so in a marked tree it
// must be marked too.
func TestWriters_FluxTreeRefusesUnappliedBundleLayout(t *testing.T) {
	tree := func() *layout.ManifestLayout {
		web := &stack.Node{Name: "web", Bundle: &stack.Bundle{Name: "web", Applications: []*stack.Application{configMapApp("w")}}}
		root := &stack.Node{Name: "platform", Bundle: &stack.Bundle{Name: "platform", Applications: []*stack.Application{configMapApp("x")}}, Children: []*stack.Node{web}}
		web.SetParent(root)
		ml := walk(t, &stack.Cluster{Name: "demo", Node: root}, nodeOnly)
		if len(layoutAt(t, ml, "platform/web").OriginBundles()) == 0 {
			t.Fatal("platform/web renders no bundle: the case needs one")
		}
		ml.SetFluxBuild(true)
		return ml
	}
	for _, writer := range allWriters {
		t.Run(writer, func(t *testing.T) {
			err := writeRefused(t, writer, layout.DefaultLayoutConfig(), tree())
			want := `layout "platform/web" is not listed by its parent layout "platform"`
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("err = %v, want it to contain %q", err, want)
			}
			ml := tree()
			layoutAt(t, ml, "platform/web").SetFluxBuild(true)
			writtenFiles(t, writer, layout.DefaultLayoutConfig(), ml)
		})
	}
}

// TestWriters_FluxTreeAcceptsWhatNothingNeedsToApply: a marked tree whose
// parents list every child is written, and so is one whose only unlisted
// child writes nothing: an AppFileSingle child without resources.
func TestWriters_FluxTreeAcceptsWhatNothingNeedsToApply(t *testing.T) {
	cases := map[string]func() *layout.ManifestLayout{
		"every child listed": fluxParent,
		"AppFileSingle umbrella child without resources": func() *layout.ManifestLayout {
			p := fluxParent()
			c := p.Children[0]
			c.UmbrellaChild, c.ApplicationFileMode, c.Mode, c.Resources = true, layout.AppFileSingle, layout.KustomizationUnset, nil
			return p
		},
	}
	for name, build := range cases {
		for _, writer := range allWriters {
			t.Run(name+"/"+writer, func(t *testing.T) {
				p := build()
				p.SetFluxBuild(true)
				writtenFiles(t, writer, layout.DefaultLayoutConfig(), p)
			})
		}
	}
}

// TestWriters_RefuseTraversalWithoutExtraFiles: a layout whose Name or
// Namespace has a ".." segment is refused whether or not it has extra files,
// the root and a child alike, and nothing is written.
func TestWriters_RefuseTraversalWithoutExtraFiles(t *testing.T) {
	cases := map[string]struct {
		build func() *layout.ManifestLayout
		want  string
	}{
		"root name": {
			build: func() *layout.ManifestLayout {
				p := fluxParent()
				p.Children = nil
				p.Name = "../outside"
				return p
			},
			want: `layout name "../outside" must not contain a ".." path segment`,
		},
		"root namespace": {
			build: func() *layout.ManifestLayout {
				p := fluxParent()
				p.Children = nil
				p.Namespace = "../outside"
				return p
			},
			want: `layout namespace "../outside" must not contain a ".." path segment`,
		},
		// As an augmenter may rename its layout after the walk.
		"child name": {
			build: func() *layout.ManifestLayout {
				p := fluxParent()
				p.Children[0].Name = "../outside"
				return p
			},
			want: `layout name "../outside" must not contain a ".." path segment`,
		},
		"child namespace": {
			build: func() *layout.ManifestLayout {
				p := fluxParent()
				p.Children[0].Namespace = "p/../../outside"
				return p
			},
			want: `layout namespace "p/../../outside" must not contain a ".." path segment`,
		},
	}
	for name, tc := range cases {
		t.Run(name+"/WriteToDisk", func(t *testing.T) {
			root := t.TempDir()
			err := tc.build().WriteToDisk(filepath.Join(root, "out"))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to contain %q", err, tc.want)
			}
			if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
				t.Errorf("the refused tree left %v behind (err %v), want nothing written", entries, err)
			}
		})
		t.Run(name+"/WriteManifest", func(t *testing.T) {
			root := t.TempDir()
			err := layout.WriteManifest(filepath.Join(root, "out"), layout.Config{ManifestsDir: "."}, tc.build())
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to contain %q", err, tc.want)
			}
			if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
				t.Errorf("the refused tree left %v behind (err %v), want nothing written", entries, err)
			}
		})
		t.Run(name+"/WriteToTar", func(t *testing.T) {
			var buf bytes.Buffer
			err := tc.build().WriteToTar(&buf)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to contain %q", err, tc.want)
			}
			if hdr, err := tar.NewReader(&buf).Next(); err != io.EOF {
				t.Errorf("WriteToTar emitted an entry before refusing: %v (err %v)", hdr, err)
			}
		})
	}
	// The control: the same tree with its own names is written.
	for _, writer := range allWriters {
		writtenFiles(t, writer, layout.DefaultLayoutConfig(), fluxParent())
	}
}

// separately is p with its child c made an umbrella child, so each is applied
// on its own; with marked set the tree is one Flux delivers.
func separately(marked bool, inParent, inChild client.Object) *layout.ManifestLayout {
	p := fluxParent()
	c := p.Children[0]
	c.UmbrellaChild = true
	p.Resources = append(p.Resources, inParent)
	c.Resources = append(c.Resources, inChild)
	if marked {
		p.SetFluxBuild(true)
		c.SetFluxBuild(true)
	}
	return p
}

// TestWriters_RefuseDuplicateFluxKustomization: two Flux Kustomizations with
// one namespace and name are one object in the cluster wherever each is
// applied, so every writer refuses them, in a marked tree and an unmarked one,
// naming both layouts. A List is opened as kustomize opens it.
func TestWriters_RefuseDuplicateFluxKustomization(t *testing.T) {
	cases := map[string]func() (inParent, inChild client.Object){
		"top-level": func() (client.Object, client.Object) { return shopKs(), shopKs() },
		// The version is not part of what the cluster keeps apart.
		"at two versions": func() (client.Object, client.Object) {
			return shopKs(), fluxKs("kustomize.toolkit.fluxcd.io/v1beta2", "flux-system", "shop")
		},
		"omitted namespace is default": func() (client.Object, client.Object) {
			return fluxKs("kustomize.toolkit.fluxcd.io/v1", "", "shop"), fluxKs("kustomize.toolkit.fluxcd.io/v1", "default", "shop")
		},
		"inside a List":              func() (client.Object, client.Object) { return shopKs(), listOf("List", shopKs()) },
		"inside a List in a List":    func() (client.Object, client.Object) { return shopKs(), listOf("List", listOf("List", shopKs())) },
		"inside a KustomizationList": func() (client.Object, client.Object) { return shopKs(), listOf("KustomizationList", shopKs()) },
		"inside a typed List": func() (client.Object, client.Object) {
			return shopKs(), typedListOf("KustomizationList", "l", shopKs())
		},
		// Not a List: the object is a Kustomization itself, whatever fields
		// it has.
		"a Kustomization with an items field": func() (client.Object, client.Object) {
			k := shopKs()
			k.Object["items"] = []any{}
			return shopKs(), k
		},
	}
	for name, objs := range cases {
		for _, marked := range []bool{false, true} {
			for _, writer := range allWriters {
				t.Run(name+"/"+map[bool]string{false: "unmarked", true: "marked"}[marked]+"/"+writer, func(t *testing.T) {
					inParent, inChild := objs()
					err := writeRefused(t, writer, layout.DefaultLayoutConfig(), separately(marked, inParent, inChild))
					want := `layouts "p" and "p/c" both hold the Flux Kustomization `
					if err == nil || !strings.Contains(err.Error(), want) || !strings.Contains(err.Error(), "/shop") {
						t.Fatalf("err = %v, want it to contain %q and the name", err, want)
					}
				})
			}
		}
	}
}

// TestWriters_RefuseDuplicateFluxKustomizationInOneLayout: a duplicate the
// per-layout check does not see, one of the two two Lists deep, is refused as
// held twice by the one layout.
func TestWriters_RefuseDuplicateFluxKustomizationInOneLayout(t *testing.T) {
	for _, writer := range allWriters {
		p := fluxParent()
		p.Resources = append(p.Resources, shopKs(), listOf("List", listOf("List", shopKs())))
		err := writeRefused(t, writer, layout.DefaultLayoutConfig(), p)
		want := `layout "p" holds the Flux Kustomization flux-system/shop twice`
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: err = %v, want it to contain %q", writer, err, want)
		}
	}
}

// TestWriters_AcceptDistinctFluxKustomizations: Kustomizations that differ in
// namespace or name, objects of another group or kind, and what kustomize does
// not open as a List, are written.
func TestWriters_AcceptDistinctFluxKustomizations(t *testing.T) {
	cases := map[string]func() (inParent, inChild client.Object){
		"different name": func() (client.Object, client.Object) {
			return shopKs(), fluxKs("kustomize.toolkit.fluxcd.io/v1", "flux-system", "cart")
		},
		"different namespace": func() (client.Object, client.Object) {
			return shopKs(), fluxKs("kustomize.toolkit.fluxcd.io/v1", "tenant", "shop")
		},
		// A kustomize.config.k8s.io Kustomization is not a Flux object.
		"another group": func() (client.Object, client.Object) {
			return shopKs(), fluxKs("example.com/v1", "flux-system", "shop")
		},
		"another kind": func() (client.Object, client.Object) {
			return shopKs(), objIn("kustomize.toolkit.fluxcd.io/v1", "Other", "flux-system", "shop")
		},
		// A kind that does not end in List is one object, whatever its items
		// field holds: kustomize does not open it.
		"a non-List kind with items": func() (client.Object, client.Object) {
			holder := listOf("Holder", shopKs())
			holder.SetName("holder")
			return shopKs(), holder
		},
		"a typed non-List kind with items": func() (client.Object, client.Object) {
			return shopKs(), typedListOf("Holder", "holder", shopKs())
		},
		// A kind ending in List without an items field is one object too.
		"a List kind without items": func() (client.Object, client.Object) {
			u := objIn("example.com/v1", "AllowList", "default", "a")
			return shopKs(), u
		},
		// kustomize reads a null items field as an empty List.
		"a List with null items": func() (client.Object, client.Object) {
			u := &unstructured.Unstructured{Object: map[string]any{"apiVersion": "v1", "kind": "List", "items": nil}}
			return shopKs(), u
		},
		// kustomize fails on it; there is nothing in it for this check to read.
		"a List whose items field is no array": func() (client.Object, client.Object) {
			u := objIn("example.com/v1", "AllowList", "default", "a")
			u.(*unstructured.Unstructured).Object["items"] = "none"
			return shopKs(), u
		},
		// The tree as it was written before: no Flux Kustomization at all.
		"no Flux Kustomization": func() (client.Object, client.Object) {
			return testObj("v1", "ConfigMap", "a"), testObj("v1", "ConfigMap", "a")
		},
	}
	for name, objs := range cases {
		for _, marked := range []bool{false, true} {
			for _, writer := range allWriters {
				t.Run(name+"/"+map[bool]string{false: "unmarked", true: "marked"}[marked]+"/"+writer, func(t *testing.T) {
					inParent, inChild := objs()
					writtenFiles(t, writer, layout.DefaultLayoutConfig(), separately(marked, inParent, inChild))
				})
			}
		}
	}
}
