package layout_test

import (
	"slices"
	"strings"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// Tests for ConfigMapGenerators: they exist only inside a kustomization.yaml
// the writer writes (go-kure/kure#891), and the ConfigMap each generates is an
// object of that kustomization's build like any resource it lists
// (go-kure/kure#894).

// withGenerator adds a configMapGenerator named name to l, with the extra file
// it reads.
func withGenerator(l *layout.ManifestLayout, name string) *layout.ManifestLayout {
	file := l.Name + "-values.txt"
	if !slices.ContainsFunc(l.ExtraFiles, func(ef layout.ExtraFile) bool { return ef.Name == file }) {
		l.ExtraFiles = append(l.ExtraFiles, layout.ExtraFile{Name: file, Content: []byte("k: v\n")})
	}
	l.ConfigMapGenerators = append(l.ConfigMapGenerators, layout.ConfigMapGeneratorSpec{Name: name, Files: []string{file}})
	return l
}

// genParent is Explicit root p holding Secret default/p, with an Explicit
// directory child c holding Secret default/c, which p's kustomization.yaml
// lists.
func genParent() *layout.ManifestLayout {
	p := &layout.ManifestLayout{
		Name:                "p",
		Namespace:           ".",
		Mode:                layout.KustomizationExplicit,
		ApplicationFileMode: layout.AppFilePerResource,
		Resources:           []client.Object{testObj("v1", "Secret", "p")},
	}
	child(p, "c", layout.KustomizationExplicit)
	return p
}

// singleGenChild is genParent without c, with an AppFileSingle child s
// carrying a configMapGenerator.
func singleGenChild() *layout.ManifestLayout {
	p := genParent()
	p.Children = nil
	s := child(p, "s", layout.KustomizationUnset)
	s.ApplicationFileMode = layout.AppFileSingle
	withGenerator(s, "x")
	return p
}

// TestWriters_RefuseGeneratorsOnSingleChild: an AppFileSingle child writes no
// kustomization.yaml of its own, so every writer refuses one that carries
// ConfigMapGenerators before writing anything, with or without resources,
// instead of dropping them.
func TestWriters_RefuseGeneratorsOnSingleChild(t *testing.T) {
	want := `layout "p/s" is AppFileSingle, which writes no kustomization.yaml, so its ConfigMapGenerators have nowhere to go`
	cases := map[string]func() *layout.ManifestLayout{
		"with resources": singleGenChild,
		"without resources": func() *layout.ManifestLayout {
			p := singleGenChild()
			p.Children[0].Resources = nil
			return p
		},
	}
	for name, build := range cases {
		t.Run(name, func(t *testing.T) {
			for _, writer := range allWriters {
				err := writeRefused(t, writer, layout.DefaultLayoutConfig(), build())
				if err == nil || !strings.Contains(err.Error(), want) {
					t.Errorf("%s: err = %v, want it to contain %q", writer, err, want)
				}
			}
		})
	}
}

// TestWriters_RecursiveGeneratorRefusedAsDropped: a generator counts as an
// object only where a kustomization.yaml holds it, so on a Recursive layout
// that also holds ConfigMap x, the refusal names the dropped generator, not a
// duplicate object.
func TestWriters_RecursiveGeneratorRefusedAsDropped(t *testing.T) {
	want := `layout "p" is KustomizationRecursive, which writes no kustomization.yaml, so its ConfigMapGenerators have nowhere to go`
	for _, writer := range allWriters {
		p := &layout.ManifestLayout{
			Name:                "p",
			Namespace:           ".",
			Mode:                layout.KustomizationRecursive,
			ApplicationFileMode: layout.AppFilePerResource,
			Resources:           []client.Object{testObj("v1", "ConfigMap", "x")},
		}
		withGenerator(p, "x")
		err := writeRefused(t, writer, layout.DefaultLayoutConfig(), p)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: err = %v, want it to contain %q", writer, err, want)
		}
	}
}

// TestWriteManifest_RefuseGeneratorsOnSingleChildFromConfig: WriteManifest
// writes a child whose own mode is unset AppFileSingle when Config says so, and
// refuses its ConfigMapGenerators; WriteToDisk and WriteToTar take the child's
// own mode, write it a directory and list the generator there.
func TestWriteManifest_RefuseGeneratorsOnSingleChildFromConfig(t *testing.T) {
	tree := func() *layout.ManifestLayout {
		p := singleGenChild()
		p.Children[0].ApplicationFileMode = layout.AppFileUnset
		return p
	}
	cfg := layout.DefaultLayoutConfig()
	cfg.ApplicationFileMode = layout.AppFileSingle
	err := writeRefused(t, "WriteManifest", cfg, tree())
	want := `layout "p/s" is AppFileSingle, which writes no kustomization.yaml, so its ConfigMapGenerators have nowhere to go`
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("WriteManifest: err = %v, want it to contain %q", err, want)
	}
	for _, writer := range []string{"WriteToDisk", "WriteToTar"} {
		files := writtenFiles(t, writer, cfg, tree())
		if k := files["p/s/kustomization.yaml"]; !strings.Contains(k, "- name: x") {
			t.Errorf("%s: p/s/kustomization.yaml does not generate x:\n%s", writer, k)
		}
	}
	kustomizeBuildsAll(t, tree())
}

// TestWriteManifest_RefuseWalkedSingleAppGenerators: the same from a walked
// tree. Under ArgoProfile the root bundle's application monitoring is
// AppFileSingle, and its augmenter's configMapGenerator is refused, not
// dropped.
func TestWriteManifest_RefuseWalkedSingleAppGenerators(t *testing.T) {
	aug := &fakeAugmentingConfig{objs: []*client.Object{makeCM("mon")}, extraFileName: "dash.txt", cmgName: "dash"}
	child := &stack.Node{Name: "monitoring", Bundle: &stack.Bundle{Name: "monitoring", Applications: []*stack.Application{
		stack.NewApplication("agent", "ns", &fakeConfig{objs: []*client.Object{makeCM("agent")}}),
	}}}
	root := &stack.Node{Name: "root", Bundle: &stack.Bundle{Name: "root", Applications: []*stack.Application{
		stack.NewApplication("monitoring", "ns", aug),
	}}, Children: []*stack.Node{child}}
	child.SetParent(root)
	ml, err := layout.WalkCluster(&stack.Cluster{Name: "demo", Node: root}, layout.DefaultLayoutRules())
	if err != nil {
		t.Fatalf("walk cluster: %v", err)
	}
	err = writeRefused(t, "WriteManifest", layout.DefaultConfigForProfile(layout.ArgoProfile), ml)
	want := `layout "root/monitoring" is AppFileSingle, which writes no kustomization.yaml, so its ConfigMapGenerators have nowhere to go`
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("WriteManifest: err = %v, want it to contain %q", err, want)
	}
}

// TestWriters_RefuseGeneratedConfigMapTwice:the ConfigMap a configMapGenerator
// generates has the generator's name before the content-hash suffix and no
// namespace, which kustomize reads as "default"; every writer refuses, before
// writing anything, a build that also holds that ConfigMap, whether as a
// resource or from another generator.
func TestWriters_RefuseGeneratedConfigMapTwice(t *testing.T) {
	cases := map[string]struct {
		build func() *layout.ManifestLayout
		want  string
	}{
		"ConfigMap in the generator's layout": {
			build: func() *layout.ManifestLayout {
				p := withGenerator(genParent(), "x")
				p.Resources = append(p.Resources, testObj("v1", "ConfigMap", "x"))
				return p
			},
			want: `layout "p" holds the same object /ConfigMap default/x twice, one generated by a configMapGenerator`,
		},
		"namespace-less ConfigMap in the generator's layout": {
			build: func() *layout.ManifestLayout {
				p := withGenerator(genParent(), "x")
				p.Resources = append(p.Resources, objIn("v1", "ConfigMap", "", "x"))
				return p
			},
			want: `layout "p" holds the same object /ConfigMap default/x twice, one generated by a configMapGenerator`,
		},
		"two generators with one name in one layout": {
			build: func() *layout.ManifestLayout {
				return withGenerator(withGenerator(genParent(), "x"), "x")
			},
			want: `layout "p" holds the same object /ConfigMap default/x twice, both generated by configMapGenerators`,
		},
		"generator in the parent, ConfigMap in a listed child": {
			build: func() *layout.ManifestLayout {
				p := withGenerator(genParent(), "x")
				c := p.Children[0]
				c.Resources = append(c.Resources, testObj("v1", "ConfigMap", "x"))
				return p
			},
			want: `layouts "p" and "p/c" both hold the object v1 ConfigMap default/x, one generated by a configMapGenerator, and the kustomization.yaml of layout "p" builds both`,
		},
		"ConfigMap in the parent, generator in a listed child": {
			build: func() *layout.ManifestLayout {
				p := genParent()
				p.Resources = append(p.Resources, testObj("v1", "ConfigMap", "x"))
				withGenerator(p.Children[0], "x")
				return p
			},
			want: `layouts "p" and "p/c" both hold the object v1 ConfigMap default/x, one generated by a configMapGenerator, and the kustomization.yaml of layout "p" builds both`,
		},
		// p's kustomization.yaml lists s.yaml, s's file in p's directory.
		"generator in the parent, ConfigMap in a listed AppFileSingle child": {
			build: func() *layout.ManifestLayout {
				p := withGenerator(genParent(), "x")
				p.Children = nil
				s := child(p, "s", layout.KustomizationUnset)
				s.ApplicationFileMode = layout.AppFileSingle
				s.Resources = []client.Object{testObj("v1", "ConfigMap", "x")}
				return p
			},
			want: `layouts "p" and "p/s" both hold the object v1 ConfigMap default/x, one generated by a configMapGenerator, and the kustomization.yaml of layout "p" builds both`,
		},
		// The hash suffix differs with the content, but kustomize compares
		// the names before it.
		"generator in the parent and a listed child": {
			build: func() *layout.ManifestLayout {
				p := withGenerator(genParent(), "x")
				c := withGenerator(p.Children[0], "x")
				c.ExtraFiles[0].Content = []byte("k: other\n")
				return p
			},
			want: `layouts "p" and "p/c" both hold the object v1 ConfigMap default/x, both generated by configMapGenerators, and the kustomization.yaml of layout "p" builds both`,
		},
		// The Flux build of a marked Recursive directory takes in each
		// Explicit directory below it as that directory's build.
		"marked Recursive directory over two children's generators": {
			build: func() *layout.ManifestLayout {
				r := recursiveTree()
				r.SetFluxBuild(true)
				withGenerator(r.Children[0], "x")
				withGenerator(child(r, "d", layout.KustomizationExplicit), "x")
				return r
			},
			want: `layouts "r/c" and "r/d" both hold the object v1 ConfigMap default/x, both generated by configMapGenerators, and the Flux build of KustomizationRecursive layout "r" includes both`,
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

// TestWriters_AcceptGeneratedConfigMapElsewhere: a generated ConfigMap and an
// object whose identity differs, or one that no build the writers produce
// takes in together with it, are written; kustomize builds every
// kustomization.yaml WriteToDisk writes.
func TestWriters_AcceptGeneratedConfigMapElsewhere(t *testing.T) {
	cases := map[string]func() *layout.ManifestLayout{
		"ConfigMap in another namespace, same layout": func() *layout.ManifestLayout {
			p := withGenerator(genParent(), "x")
			p.Resources = append(p.Resources, objIn("v1", "ConfigMap", "other", "x"))
			return p
		},
		"ConfigMap in another namespace, listed child": func() *layout.ManifestLayout {
			p := withGenerator(genParent(), "x")
			c := p.Children[0]
			c.Resources = append(c.Resources, objIn("v1", "ConfigMap", "other", "x"))
			return p
		},
		"Secret named like the generator": func() *layout.ManifestLayout {
			p := withGenerator(genParent(), "x")
			p.Resources = append(p.Resources, testObj("v1", "Secret", "x"))
			return p
		},
		"generators with different names": func() *layout.ManifestLayout {
			p := withGenerator(genParent(), "x")
			withGenerator(p.Children[0], "other")
			return p
		},
		// Applied by its own Flux Kustomization; the parent does not list it.
		"generator in an umbrella child": func() *layout.ManifestLayout {
			p := withGenerator(genParent(), "x")
			c := withGenerator(p.Children[0], "x")
			c.UmbrellaChild = true
			return p
		},
		// The parent lists the child's Kustomization CR, not its directory.
		"generator in a child of a FluxIntegratedPerLayout parent": func() *layout.ManifestLayout {
			p := withGenerator(genParent(), "x")
			p.FluxPlacement = layout.FluxIntegratedPerLayout
			withGenerator(p.Children[0], "x")
			return p
		},
		// No Flux Kustomization kure generated builds r, and each Explicit
		// child is its own build.
		"unmarked Recursive root over two children's generators": func() *layout.ManifestLayout {
			r := recursiveTree()
			withGenerator(r.Children[0], "x")
			withGenerator(child(r, "d", layout.KustomizationExplicit), "x")
			return r
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
