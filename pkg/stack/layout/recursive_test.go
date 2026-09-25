package layout_test

import (
	"io"
	"maps"
	"slices"
	"strings"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack/layout"
)

// Tests for KustomizationRecursive (go-kure/kure#868): a layout whose
// resolved mode is Recursive gets every file except kustomization.yaml, and
// kure refuses only what then contradicts its own output.

var allWriters = []string{"WriteToDisk", "WriteToTar", "WriteManifest"}

// recursiveTree is root r (Recursive, a ConfigMap) with child c (Explicit, a
// Secret), whose directory r's Flux build includes.
func recursiveTree() *layout.ManifestLayout {
	r := &layout.ManifestLayout{
		Name:      "r",
		Namespace: ".",
		Mode:      layout.KustomizationRecursive,
		Resources: []client.Object{testObj("v1", "ConfigMap", "a")},
	}
	r.Children = []*layout.ManifestLayout{{
		Name:      "c",
		Namespace: r.FullRepoPath(),
		Mode:      layout.KustomizationExplicit,
		Resources: []client.Object{testObj("v1", "Secret", "b")},
	}}
	return r
}

// child returns a child layout of parent named name, with a Secret of its own.
func child(parent *layout.ManifestLayout, name string, mode layout.KustomizationMode) *layout.ManifestLayout {
	c := &layout.ManifestLayout{
		Name:      name,
		Namespace: parent.FullRepoPath(),
		Mode:      mode,
		Resources: []client.Object{testObj("v1", "Secret", name)},
	}
	parent.Children = append(parent.Children, c)
	return c
}

// TestWriters_RecursiveWritesNoKustomization: a Recursive layout's directory
// gets its resource and extra files and no kustomization.yaml; an Explicit
// child below it keeps its own.
func TestWriters_RecursiveWritesNoKustomization(t *testing.T) {
	for _, writer := range allWriters {
		t.Run(writer, func(t *testing.T) {
			r := recursiveTree()
			r.ExtraFiles = []layout.ExtraFile{{Name: "notes.txt", Content: []byte("hello\n")}}
			files := writtenFiles(t, writer, layout.DefaultLayoutConfig(), r)
			got := slices.Sorted(maps.Keys(files))
			want := []string{"r/c/default-secret-b.yaml", "r/c/kustomization.yaml", "r/default-configmap-a.yaml", "r/notes.txt"}
			if !slices.Equal(got, want) {
				t.Fatalf("files = %v, want %v", got, want)
			}
			if got := listedResources(t, files, "r/c/kustomization.yaml"); !slices.Equal(got, []string{"default-secret-b.yaml"}) {
				t.Errorf("r/c/kustomization.yaml lists %v", got)
			}
		})
	}
}

// TestWriteManifest_ConfigRecursiveWritesNoKustomization: the mode WriteManifest
// resolves from Config counts, not only the layout's own.
func TestWriteManifest_ConfigRecursiveWritesNoKustomization(t *testing.T) {
	r := recursiveTree()
	r.Mode = layout.KustomizationUnset
	r.Children[0].Mode = layout.KustomizationUnset
	cfg := layout.DefaultLayoutConfig()
	cfg.KustomizationMode = layout.KustomizationRecursive
	files := writtenFiles(t, "WriteManifest", cfg, r)
	got := slices.Sorted(maps.Keys(files))
	want := []string{"r/c/default-secret-b.yaml", "r/default-configmap-a.yaml"}
	if !slices.Equal(got, want) {
		t.Fatalf("files = %v, want %v", got, want)
	}
}

// TestWriters_RecursiveRefusals: what a Recursive layout makes contradictory
// in kure's own output is refused before anything is written.
func TestWriters_RecursiveRefusals(t *testing.T) {
	cases := map[string]struct {
		build func() *layout.ManifestLayout
		want  string
	}{
		// ConfigMapGenerators exist only inside a kustomization.yaml.
		"ConfigMapGenerators": {
			build: func() *layout.ManifestLayout {
				r := recursiveTree()
				r.ConfigMapGenerators = []layout.ConfigMapGeneratorSpec{{Name: "g", Files: []string{"notes.txt"}}}
				return r
			},
			want: `layout "r" is KustomizationRecursive, which writes no kustomization.yaml, so its ConfigMapGenerators have nowhere to go`,
		},
		// An Explicit parent lists its directory child, which then has no
		// kustomization.yaml for kustomize to build.
		"Explicit parent lists a Recursive child": {
			build: func() *layout.ManifestLayout {
				r := recursiveTree()
				r.Mode = layout.KustomizationExplicit
				r.Children[0].Mode = layout.KustomizationRecursive
				return r
			},
			want: `layout "r/c" is KustomizationRecursive and writes no kustomization.yaml, but the kustomization.yaml of layout "r" lists its directory`,
		},
		// A single-file child named kustomization would turn the Recursive
		// directory into a kustomization after all.
		"single child named kustomization": {
			build: func() *layout.ManifestLayout {
				r := recursiveTree()
				s := child(r, "kustomization", layout.KustomizationUnset)
				s.Namespace = r.FullRepoPath()
				s.ApplicationFileMode = layout.AppFileSingle
				return r
			},
			want: `its file "kustomization.yaml" would make the directory of KustomizationRecursive layout "r" a kustomization`,
		},
		// F1: the root build (the bootstrap's) includes c, which its own
		// generated Kustomization also builds. c's own kustomization.yaml
		// does not keep it out: Flux adds such a directory as a resource.
		"marked Recursive root over a marked Explicit target": {
			build: func() *layout.ManifestLayout {
				r := recursiveTree()
				r.SetFluxBuild(true)
				r.Children[0].SetFluxBuild(true)
				return r
			},
			want: `the Flux build of KustomizationRecursive layout "r" would include layout "r/c"`,
		},
		// F1 with a kustomization-less target.
		"marked Recursive root over a marked Recursive target": {
			build: func() *layout.ManifestLayout {
				r := recursiveTree()
				r.SetFluxBuild(true)
				r.Children[0].Mode = layout.KustomizationRecursive
				r.Children[0].SetFluxBuild(true)
				return r
			},
			want: `the Flux build of KustomizationRecursive layout "r" would include layout "r/c"`,
		},
		// F1 through an unmarked Recursive directory in between, which
		// shields nothing.
		"marked target below an unmarked Recursive directory": {
			build: func() *layout.ManifestLayout {
				r := recursiveTree()
				r.SetFluxBuild(true)
				r.Children[0].Mode = layout.KustomizationRecursive
				child(r.Children[0], "t", layout.KustomizationExplicit).SetFluxBuild(true)
				return r
			},
			want: `the Flux build of KustomizationRecursive layout "r" would include layout "r/c/t"`,
		},
		// F2: a YAML extra file is applied by the Flux build and never
		// listed by an Explicit kustomization.yaml.
		"decodable YAML extra file": {
			build: func() *layout.ManifestLayout {
				r := recursiveTree()
				r.SetFluxBuild(true)
				r.ExtraFiles = []layout.ExtraFile{{Name: "x.yaml", Content: []byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: x\n")}}
				return r
			},
			want: `the Flux build of KustomizationRecursive layout "r" would apply the extra file "x.yaml" of layout "r"`,
		},
		// F2: an undecodable one fails the whole Flux build.
		"undecodable YML extra file": {
			build: func() *layout.ManifestLayout {
				r := recursiveTree()
				r.SetFluxBuild(true)
				r.ExtraFiles = []layout.ExtraFile{{Name: "values.yml", Content: []byte("replicas: [\n")}}
				return r
			},
			want: `would apply the extra file "values.yml" of layout "r"`,
		},
		// F2 in an unmarked Recursive directory inside the build.
		"YAML extra file in an unmarked Recursive child": {
			build: func() *layout.ManifestLayout {
				r := recursiveTree()
				r.SetFluxBuild(true)
				r.Children[0].Mode = layout.KustomizationRecursive
				r.Children[0].ExtraFiles = []layout.ExtraFile{{Name: "x.yaml", Content: []byte("a: b\n")}}
				return r
			},
			want: `would apply the extra file "x.yaml" of layout "r/c"`,
		},
		// F2 for a file Flux reads as a kustomization rather than by its
		// extension: an extensionless Kustomization makes Flux add its
		// directory to the build as a kustomization of its own.
		"Kustomization extra file in a subdirectory": {
			build: func() *layout.ManifestLayout {
				r := recursiveTree()
				r.SetFluxBuild(true)
				r.ExtraFiles = []layout.ExtraFile{{Name: "sub/Kustomization", Content: []byte("configMapGenerator:\n- name: sneaky\n  literals: [a=b]\n")}}
				return r
			},
			want: `the Flux build of KustomizationRecursive layout "r" would build the extra file "sub/Kustomization" of layout "r" as a kustomization`,
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

// TestWriteManifest_RecursiveRefusesFilesFluxDoesNotScan: a Config's
// ManifestFileName can name resource files the Flux build of a marked
// Recursive directory skips (not *.yaml or *.yml) or reads as the
// directory's kustomization. Explicit mode lists either kind by name, so the
// two builds would differ. A file in a directory with a kustomization.yaml
// of its own is listed there and stays accepted.
func TestWriteManifest_RecursiveRefusesFilesFluxDoesNotScan(t *testing.T) {
	naming := func(name func(kind string) string) layout.Config {
		cfg := layout.DefaultLayoutConfig()
		cfg.ManifestFileName = func(_, kind, _ string, _ layout.FileExportMode) string { return name(kind) }
		return cfg
	}
	refused := map[string]struct {
		cfg  layout.Config
		want string
	}{
		"JSON resource file": {
			cfg:  naming(func(kind string) string { return kind + ".json" }),
			want: `the Flux build of KustomizationRecursive layout "r" would skip the resource file "configmap.json" of layout "r"`,
		},
		"resource file named like a kustomization": {
			cfg:  naming(func(string) string { return "kustomization.yml" }),
			want: `the Flux build of KustomizationRecursive layout "r" would read the resource file "kustomization.yml" of layout "r" as a kustomization`,
		},
	}
	for name, tc := range refused {
		t.Run(name, func(t *testing.T) {
			r := recursiveTree()
			r.SetFluxBuild(true)
			err := writeRefused(t, "WriteManifest", tc.cfg, r)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want it to contain %q", err, tc.want)
			}
		})
	}
	t.Run("JSON file in a directory with its own kustomization.yaml", func(t *testing.T) {
		r := recursiveTree()
		r.SetFluxBuild(true)
		cfg := naming(func(kind string) string {
			if kind == "secret" {
				return "secret.json"
			}
			return kind + ".yaml"
		})
		if err := layout.WriteManifest(t.TempDir(), cfg, r); err != nil {
			t.Fatalf("WriteManifest: %v", err)
		}
	})
}

// TestWriters_RecursiveAccepted: shapes whose Recursive output is kure's own
// and consistent are written.
func TestWriters_RecursiveAccepted(t *testing.T) {
	cases := map[string]func() *layout.ManifestLayout{
		// No marker: no Flux Kustomization kure generated builds r.
		"unmarked Recursive root over a marked target": func() *layout.ManifestLayout {
			r := recursiveTree()
			r.Children[0].SetFluxBuild(true)
			return r
		},
		"marked Recursive root over an unmarked Explicit child": func() *layout.ManifestLayout {
			r := recursiveTree()
			r.SetFluxBuild(true)
			return r
		},
		// c's kustomization.yaml does not list t (an umbrella child), so
		// the Flux build of r, which adds c as a resource, never reaches t.
		"marked target shielded by an intermediate Explicit directory": func() *layout.ManifestLayout {
			r := recursiveTree()
			r.SetFluxBuild(true)
			t := child(r.Children[0], "t", layout.KustomizationExplicit)
			t.UmbrellaChild = true
			t.SetFluxBuild(true)
			return r
		},
		// An extra file in a shielded directory is never listed there.
		"YAML extra file in an Explicit child": func() *layout.ManifestLayout {
			r := recursiveTree()
			r.SetFluxBuild(true)
			r.Children[0].ExtraFiles = []layout.ExtraFile{{Name: "x.yaml", Content: []byte("a: b\n")}}
			return r
		},
		// Flux takes only .yaml and .yml files.
		"non-YAML extra file": func() *layout.ManifestLayout {
			r := recursiveTree()
			r.SetFluxBuild(true)
			r.ExtraFiles = []layout.ExtraFile{{Name: "notes.txt", Content: []byte("x")}}
			return r
		},
		// A parent that does not list its child directory: an umbrella
		// child is applied by its own CR.
		"Explicit parent over an umbrella Recursive child": func() *layout.ManifestLayout {
			r := recursiveTree()
			r.Mode = layout.KustomizationExplicit
			r.Children[0].Mode = layout.KustomizationRecursive
			r.Children[0].UmbrellaChild = true
			return r
		},
		// Recursive parent over an Explicit child needs no listing at all.
		"Recursive parent over an Explicit child": recursiveTree,
	}
	for name, build := range cases {
		t.Run(name, func(t *testing.T) {
			for _, writer := range allWriters {
				_ = writtenFiles(t, writer, layout.DefaultLayoutConfig(), build())
			}
		})
	}
}

// TestWriters_RecursiveKeepsControlFileReservation: an extra file still may
// take no kustomize control file name in a Recursive directory, where it would
// make the directory a kustomization after all. (checkExtraFiles runs as each
// layout is written, so the tar writer's end-of-archive trailer is out.)
func TestWriters_RecursiveKeepsControlFileReservation(t *testing.T) {
	for _, writer := range allWriters {
		r := recursiveTree()
		r.ExtraFiles = []layout.ExtraFile{{Name: "kustomization.yaml", Content: []byte("resources: []\n")}}
		var err error
		switch writer {
		case "WriteToDisk":
			err = r.WriteToDisk(t.TempDir())
		case "WriteToTar":
			err = r.WriteToTar(io.Discard)
		case "WriteManifest":
			err = layout.WriteManifest(t.TempDir(), layout.DefaultLayoutConfig(), r)
		}
		if err == nil || !strings.Contains(err.Error(), "kustomize control file") {
			t.Errorf("%s: err = %v, want the control file refused", writer, err)
		}
	}
}

// TestSetFluxBuild: the marker reads back as set.
func TestSetFluxBuild(t *testing.T) {
	l := &layout.ManifestLayout{Name: "x"}
	if l.FluxBuild() {
		t.Fatal("a new layout is marked")
	}
	l.SetFluxBuild(true)
	if !l.FluxBuild() {
		t.Fatal("SetFluxBuild(true) did not mark the layout")
	}
	l.SetFluxBuild(false)
	if l.FluxBuild() {
		t.Fatal("SetFluxBuild(false) did not unmark the layout")
	}
}
