package layout_test

import (
	"errors"
	"io"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kerrors "github.com/go-kure/kure/pkg/errors"
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

// TestWriters_RecursiveRefusesUnlistedSingleChildFile: an AppFileSingle
// umbrella child writes its file into its parent's directory, and its
// parent's Explicit kustomization.yaml does not list it (the umbrella's own
// Kustomization applies it). The Flux build of a marked Recursive parent
// would apply that file too.
func TestWriters_RecursiveRefusesUnlistedSingleChildFile(t *testing.T) {
	for _, writer := range allWriters {
		r := recursiveTree()
		r.SetFluxBuild(true)
		u := child(r, "u", layout.KustomizationUnset)
		u.ApplicationFileMode = layout.AppFileSingle
		u.UmbrellaChild = true
		err := writeRefused(t, writer, layout.DefaultLayoutConfig(), r)
		want := `the Flux build of KustomizationRecursive layout "r" would apply the file "u.yaml" of layout "r/u", which the Explicit mode does not list`
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: err = %v, want it to contain %q", writer, err, want)
		}
	}
}

// TestWriters_RecursiveRefusesUnlistedChildDirectory: WriteToDisk and
// WriteToTar do not list a child directory of another package, but write it
// below its parent. The Flux build of a marked Recursive parent would apply
// it, and the Explicit mode would not. WriteManifest lists that child, so the
// two builds agree there.
func TestWriters_RecursiveRefusesUnlistedChildDirectory(t *testing.T) {
	tree := func() *layout.ManifestLayout {
		r := recursiveTree()
		r.SetFluxBuild(true)
		r.PackageRef = &schema.GroupVersionKind{Group: "source.toolkit.fluxcd.io", Version: "v1", Kind: "OCIRepository"}
		r.Children[0].PackageRef = &schema.GroupVersionKind{Group: "source.toolkit.fluxcd.io", Version: "v1", Kind: "GitRepository"}
		return r
	}
	for _, writer := range []string{"WriteToDisk", "WriteToTar"} {
		err := writeRefused(t, writer, layout.DefaultLayoutConfig(), tree())
		want := `the Flux build of KustomizationRecursive layout "r" would include layout "r/c", which the Explicit mode does not list`
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: err = %v, want it to contain %q", writer, err, want)
		}
	}
	if err := layout.WriteManifest(t.TempDir(), layout.DefaultLayoutConfig(), tree()); err != nil {
		t.Errorf("WriteManifest: %v, want the listed child accepted", err)
	}
}

// TestWriters_SamePackageChildListed: a parent and child whose PackageRefs
// are separately allocated but equal are one package, so the parent's
// kustomization.yaml lists the child and a marked Recursive parent over it
// is accepted.
func TestWriters_SamePackageChildListed(t *testing.T) {
	gvk := func() *schema.GroupVersionKind {
		return &schema.GroupVersionKind{Group: "source.toolkit.fluxcd.io", Version: "v1", Kind: "OCIRepository"}
	}
	for _, writer := range []string{"WriteToDisk", "WriteToTar"} {
		t.Run(writer, func(t *testing.T) {
			r := recursiveTree()
			r.Mode = layout.KustomizationExplicit
			r.PackageRef, r.Children[0].PackageRef = gvk(), gvk()
			files := writtenFiles(t, writer, layout.DefaultLayoutConfig(), r)
			if got := listedResources(t, files, "r/kustomization.yaml"); !slices.Contains(got, "c") {
				t.Errorf("r/kustomization.yaml lists %v, want it to list c", got)
			}

			r = recursiveTree()
			r.SetFluxBuild(true)
			r.PackageRef, r.Children[0].PackageRef = gvk(), gvk()
			writtenFiles(t, writer, layout.DefaultLayoutConfig(), r)
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

// emptyRecursiveMsg is the refusal of a marked Recursive directory that
// holds no file (go-kure/kure#904).
const emptyRecursiveMsg = "a Flux Kustomization kure generated builds its directory, but it holds no file"

// vacant is a marked Recursive root named vacant with no resource, extra
// file or child.
func vacant() *layout.ManifestLayout {
	r := &layout.ManifestLayout{Name: "vacant", Namespace: ".", Mode: layout.KustomizationRecursive}
	r.SetFluxBuild(true)
	return r
}

// TestWriters_RecursiveRefusesEmptyFluxBuild: a Flux Kustomization kure
// generated names a Recursive directory, which gets no kustomization.yaml; when
// nothing else lands at or below it, every writer would leave it an empty
// directory, which a Git tree drops, so every writer refuses it before
// writing anything, naming the directory.
func TestWriters_RecursiveRefusesEmptyFluxBuild(t *testing.T) {
	cases := map[string]struct {
		build func() *layout.ManifestLayout
		dir   string
	}{
		"no resource, extra file or child": {build: vacant, dir: "vacant"},
		// A descendant directory that itself holds no file adds none.
		"only an empty unmarked Recursive child": {
			build: func() *layout.ManifestLayout {
				r := vacant()
				c := child(r, "c", layout.KustomizationRecursive)
				c.Resources = nil
				return r
			},
			dir: "vacant",
		},
		// An AppFileSingle child with no resource writes no file.
		"only an AppFileSingle child with no resource": {
			build: func() *layout.ManifestLayout {
				r := vacant()
				s := child(r, "s", layout.KustomizationUnset)
				s.ApplicationFileMode = layout.AppFileSingle
				s.Resources = nil
				return r
			},
			dir: "vacant",
		},
		// An AppFileSingle root with no resource writes no file, and a
		// Recursive one no kustomization.yaml either. Its directory is the
		// writer's output directory itself, which differs per writer, so
		// the error's path is not compared.
		"an AppFileSingle root with no resource": {
			build: func() *layout.ManifestLayout {
				r := vacant()
				r.ApplicationFileMode = layout.AppFileSingle
				return r
			},
		},
		// Its file lands in "Vacant", which is not the directory the
		// Kustomization names in a Git tree or on a case-sensitive volume.
		"only an AppFileSingle child in a directory that differs in case": {
			build: func() *layout.ManifestLayout {
				r := vacant()
				s := child(r, "s", layout.KustomizationUnset)
				s.ApplicationFileMode = layout.AppFileSingle
				s.Namespace = "Vacant"
				return r
			},
			dir: "vacant",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			for _, writer := range allWriters {
				err := writeRefused(t, writer, layout.DefaultLayoutConfig(), tc.build())
				want := `layout "vacant" is KustomizationRecursive and ` + emptyRecursiveMsg
				if err == nil || !strings.Contains(err.Error(), want) {
					t.Errorf("%s: err = %v, want it to contain %q", writer, err, want)
					continue
				}
				var fe *kerrors.FileError
				if !errors.As(err, &fe) {
					t.Errorf("%s: err is %T, want a *errors.FileError", writer, err)
					continue
				}
				if p := filepath.ToSlash(fe.Path); tc.dir != "" && p != tc.dir && !strings.HasSuffix(p, "/"+tc.dir) {
					t.Errorf("%s: error names %q, want the directory %q", writer, fe.Path, tc.dir)
				}
			}
		})
	}
}

// TestWriters_RecursiveEmptyFluxBuildCounterparts: a marked Recursive
// directory with any file at or below it, an unmarked empty Recursive
// directory, and a marked empty Explicit directory (which gets "resources:
// []") are written.
func TestWriters_RecursiveEmptyFluxBuildCounterparts(t *testing.T) {
	cases := map[string]struct {
		build func() *layout.ManifestLayout
		want  []string // every file written
	}{
		"marked Recursive with one resource": {
			build: func() *layout.ManifestLayout {
				r := vacant()
				r.Resources = []client.Object{testObj("v1", "ConfigMap", "a")}
				return r
			},
			want: []string{"vacant/default-configmap-a.yaml"},
		},
		// The child's resource file lands below the marked directory; the
		// child writes no kustomization.yaml either.
		"marked Recursive whose only content is a Recursive child with a resource": {
			build: func() *layout.ManifestLayout {
				r := vacant()
				child(r, "c", layout.KustomizationRecursive)
				return r
			},
			want: []string{"vacant/c/default-secret-c.yaml"},
		},
		// A descendant's own kustomization.yaml is a file too.
		"marked Recursive whose only content is an empty Explicit child": {
			build: func() *layout.ManifestLayout {
				r := vacant()
				child(r, "c", layout.KustomizationExplicit).Resources = nil
				return r
			},
			want: []string{"vacant/c/kustomization.yaml"},
		},
		"marked Recursive whose only content is an extra file": {
			build: func() *layout.ManifestLayout {
				r := vacant()
				r.ExtraFiles = []layout.ExtraFile{{Name: "notes.txt", Content: []byte("x")}}
				return r
			},
			want: []string{"vacant/notes.txt"},
		},
		"marked Recursive whose only content is an AppFileSingle child's file": {
			build: func() *layout.ManifestLayout {
				r := vacant()
				child(r, "s", layout.KustomizationUnset).ApplicationFileMode = layout.AppFileSingle
				return r
			},
			want: []string{"vacant/s.yaml"},
		},
		// No Flux Kustomization kure generated names it.
		"unmarked empty Recursive": {
			build: func() *layout.ManifestLayout {
				r := vacant()
				r.SetFluxBuild(false)
				return r
			},
			want: nil,
		},
		"marked empty Explicit": {
			build: func() *layout.ManifestLayout {
				r := vacant()
				r.Mode = layout.KustomizationExplicit
				return r
			},
			want: []string{"vacant/kustomization.yaml"},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			for _, writer := range allWriters {
				files := writtenFiles(t, writer, layout.DefaultLayoutConfig(), tc.build())
				if got := slices.Sorted(maps.Keys(files)); !slices.Equal(got, tc.want) {
					t.Errorf("%s: files = %v, want %v", writer, got, tc.want)
				}
				if slices.Contains(tc.want, "vacant/kustomization.yaml") {
					if got := listedResources(t, files, "vacant/kustomization.yaml"); len(got) != 0 {
						t.Errorf("%s: vacant/kustomization.yaml lists %v, want resources: []", writer, got)
					}
				}
			}
		})
	}
}

// TestWriteManifest_RecursiveFromConfigRefusesEmptyFluxBuild: a layout with
// no Mode of its own is Recursive through Config.KustomizationMode, which only
// WriteManifest resolves; its empty marked directory is refused all the same.
func TestWriteManifest_RecursiveFromConfigRefusesEmptyFluxBuild(t *testing.T) {
	r := vacant()
	r.Mode = layout.KustomizationUnset
	cfg := layout.DefaultLayoutConfig()
	cfg.KustomizationMode = layout.KustomizationRecursive
	err := writeRefused(t, "WriteManifest", cfg, r)
	if err == nil || !strings.Contains(err.Error(), emptyRecursiveMsg) {
		t.Errorf("err = %v, want it to contain %q", err, emptyRecursiveMsg)
	}
}

// TestWriters_RecursiveFilledFromElsewhere: a marked Recursive directory is
// filled by a file another layout writes into it, and is written.
func TestWriters_RecursiveFilledFromElsewhere(t *testing.T) {
	cases := map[string]struct {
		build func() *layout.ManifestLayout
		want  []string // files that must be written
	}{
		// The AppFileSingle child's file lands in its own Namespace, p/d, and
		// not in its parent's directory p.
		"an AppFileSingle child's file in a directory below its parent's": {
			build: func() *layout.ManifestLayout {
				p := &layout.ManifestLayout{Name: "p", Namespace: ".", Mode: layout.KustomizationRecursive}
				s := child(p, "s", layout.KustomizationUnset)
				s.ApplicationFileMode = layout.AppFileSingle
				s.Namespace = "p/d"
				d := child(p, "d", layout.KustomizationRecursive)
				d.Resources = nil
				d.SetFluxBuild(true)
				return p
			},
			want: []string{"p/d/s.yaml"},
		},
		// An AppFileSingle root writes its file and a kustomization.yaml into
		// its Namespace, apps/web, which is its umbrella child's directory.
		"an AppFileSingle root's files": {
			build: func() *layout.ManifestLayout {
				r := &layout.ManifestLayout{
					Name: "r", Namespace: "apps/web", ApplicationFileMode: layout.AppFileSingle,
					Resources: []client.Object{testObj("v1", "ConfigMap", "a")},
				}
				d := &layout.ManifestLayout{Name: "web", Namespace: "apps", Mode: layout.KustomizationRecursive, UmbrellaChild: true}
				d.SetFluxBuild(true)
				r.Children = []*layout.ManifestLayout{d}
				return r
			},
			want: []string{"apps/web/kustomization.yaml", "apps/web/r.yaml"},
		},
		// With no resource, its kustomization.yaml ("resources: []") is the
		// one file there.
		"an AppFileSingle root's kustomization.yaml": {
			build: func() *layout.ManifestLayout {
				r := &layout.ManifestLayout{Name: "r", Namespace: "apps/web", ApplicationFileMode: layout.AppFileSingle}
				d := &layout.ManifestLayout{Name: "web", Namespace: "apps", Mode: layout.KustomizationRecursive, UmbrellaChild: true}
				d.SetFluxBuild(true)
				r.Children = []*layout.ManifestLayout{d}
				return r
			},
			want: []string{"apps/web/kustomization.yaml"},
		},
		// A Recursive one writes no kustomization.yaml: its resource file is
		// the one file there.
		"a Recursive AppFileSingle root's resource file": {
			build: func() *layout.ManifestLayout {
				r := &layout.ManifestLayout{
					Name: "r", Namespace: "apps/web", ApplicationFileMode: layout.AppFileSingle,
					Mode:      layout.KustomizationRecursive,
					Resources: []client.Object{testObj("v1", "ConfigMap", "a")},
				}
				d := &layout.ManifestLayout{Name: "web", Namespace: "apps", Mode: layout.KustomizationRecursive, UmbrellaChild: true}
				d.SetFluxBuild(true)
				r.Children = []*layout.ManifestLayout{d}
				return r
			},
			want: []string{"apps/web/r.yaml"},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			for _, writer := range allWriters {
				files := writtenFiles(t, writer, layout.DefaultLayoutConfig(), tc.build())
				for _, f := range tc.want {
					if _, ok := files[f]; !ok {
						t.Errorf("%s: %s not written; files = %v", writer, f, slices.Sorted(maps.Keys(files)))
					}
				}
			}
		})
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
