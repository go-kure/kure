package layout_test

import (
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/kyaml/filesys"

	"github.com/go-kure/kure/pkg/stack/layout"
)

// Tests for the entry a parent's kustomization.yaml lists for an AppFileSingle
// child whose file lands below the parent's directory (go-kure/kure#879): the
// file's path relative to the parent's directory, so the entry resolves and
// kustomize builds it. A listed file outside the parent's directory is
// refused, since kustomize builds no resource outside a kustomization's
// directory.

// kustomizeBuildFiles writes files, keyed as writtenFiles keys them, into a
// fresh directory and runs a kustomize build of every directory among them
// that holds a kustomization.yaml, returning each build's failure by that
// directory's slash-separated path.
func kustomizeBuildFiles(t *testing.T, files map[string]string) map[string]error {
	t.Helper()
	dir := t.TempDir()
	for p, data := range files {
		fp := filepath.Join(dir, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(fp), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fp, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	failed := map[string]error{}
	for _, p := range slices.Sorted(maps.Keys(files)) {
		if path.Base(p) != "kustomization.yaml" {
			continue
		}
		kdir := path.Dir(p)
		if _, err := krusty.MakeKustomizer(krusty.MakeDefaultOptions()).Run(filesys.MakeFsOnDisk(), filepath.Join(dir, filepath.FromSlash(kdir))); err != nil {
			failed[kdir] = err
		}
	}
	return failed
}

// requireBuilds fails the test for every kustomization.yaml among files that
// kustomize does not build.
func requireBuilds(t *testing.T, writer string, files map[string]string) {
	t.Helper()
	for kdir, err := range kustomizeBuildFiles(t, files) {
		t.Errorf("%s: kustomize build %s: %v\n%s", writer, kdir, err, files[path.Join(kdir, "kustomization.yaml")])
	}
}

// TestWriters_SingleChildBelowParentListedByPath: an AppFileSingle child whose
// directory lies below its parent's is listed by its file's path relative to
// the parent's directory, slash-separated, and every writer's output builds.
// Directories compare case-insensitively, as on default macOS volumes, so a
// child directory that differs only in case from one below the parent's is
// below it; the entry keeps the child directory's own spelling.
func TestWriters_SingleChildBelowParentListedByPath(t *testing.T) {
	below := func(ns string) func() *layout.ManifestLayout {
		return func() *layout.ManifestLayout {
			p := singleChildParent(layout.AppFileSingle)
			p.Children[0].Namespace = ns
			return p
		}
	}
	cases := map[string]struct {
		build func() *layout.ManifestLayout
		entry string // what p/kustomization.yaml lists for svc
		file  string // where svc's file is written
		// caseFolded marks a tree whose entry resolves only on a
		// case-insensitive volume, so kustomize is not run on it here.
		caseFolded bool
	}{
		"one level below":        {build: below("p/sub"), entry: "sub/svc.yaml", file: "p/sub/svc.yaml"},
		"two levels below":       {build: below("p/a/b"), entry: "a/b/svc.yaml", file: "p/a/b/svc.yaml"},
		"below, other case":      {build: below("p/Sub"), entry: "Sub/svc.yaml", file: "p/Sub/svc.yaml"},
		"parent's own directory": {build: below("p"), entry: "svc.yaml", file: "p/svc.yaml"},
		// "P/Sub" is "p/sub" on a case-insensitive volume.
		"parent's directory in other case": {build: below("P/Sub"), entry: "Sub/svc.yaml", file: "P/Sub/svc.yaml", caseFolded: true},
		// The file lands in the directory of p's listed child sub, whose
		// kustomization.yaml does not list it: p's build takes it in once.
		"inside a listed directory child": {
			build: func() *layout.ManifestLayout {
				p := below("p/sub")()
				p.Children = append(p.Children, cmLayout("sub", "p"))
				return p
			},
			entry: "sub/svc.yaml",
			file:  "p/sub/svc.yaml",
		},
	}
	for name, tc := range cases {
		for _, writer := range allWriters {
			t.Run(name+"/"+writer, func(t *testing.T) {
				files := writtenFiles(t, writer, layout.Config{}, tc.build())
				listed := listedResources(t, files, "p/kustomization.yaml")
				if !slices.Contains(listed, tc.entry) {
					t.Errorf("p/kustomization.yaml lists %v, want it to list %s", listed, tc.entry)
				}
				if !strings.Contains(files[tc.file], "name: svc-config") {
					t.Errorf("%s does not hold the child's ConfigMap; wrote %v", tc.file, slices.Sorted(maps.Keys(files)))
				}
				if tc.caseFolded {
					if got := strings.ToLower(path.Join("p", tc.entry)); got != strings.ToLower(tc.file) {
						t.Errorf("entry %s resolves to %s, not to %s even case-insensitively", tc.entry, got, tc.file)
					}
					return
				}
				requireBuilds(t, writer, files)
			})
		}
	}
}

// TestWriteManifest_ConfigSingleChildBelowParentListedByPath: a child that is
// AppFileSingle through Config, not its own mode, is listed by its relative
// path as well.
func TestWriteManifest_ConfigSingleChildBelowParentListedByPath(t *testing.T) {
	cfg := layout.Config{ApplicationFileMode: layout.AppFileSingle}
	p := singleChildParent(layout.AppFileUnset)
	p.Children[0].Namespace = "p/sub"
	files := writtenFiles(t, "WriteManifest", cfg, p)
	want := []string{"default-configmap-a.yaml", "default-secret-b.yaml", "sub/svc.yaml"}
	if got := listedResources(t, files, "p/kustomization.yaml"); !slices.Equal(got, want) {
		t.Errorf("p/kustomization.yaml lists %v, want %v", got, want)
	}
	if _, ok := files["p/sub/svc.yaml"]; !ok {
		t.Errorf("no p/sub/svc.yaml written; wrote %v", slices.Sorted(maps.Keys(files)))
	}
	requireBuilds(t, "WriteManifest", files)
}

// TestWriters_SameNamedSingleChildrenListedApart: two AppFileSingle children
// named s, one in p's directory and one below it, are listed by two entries.
// Holding different objects they build; holding one object, the build check
// refuses them, as kustomize would.
func TestWriters_SameNamedSingleChildrenListedApart(t *testing.T) {
	pair := func(subObj client.Object) *layout.ManifestLayout {
		p := singleChildParent(layout.AppFileSingle)
		sub := singleLayout("s", "p/sub")
		sub.Resources = []client.Object{subObj}
		p.Children = []*layout.ManifestLayout{singleLayout("s", "p"), sub}
		return p
	}
	for _, writer := range allWriters {
		t.Run("different objects/"+writer, func(t *testing.T) {
			files := writtenFiles(t, writer, layout.Config{}, pair(testObj("v1", "ConfigMap", "s-sub")))
			want := []string{"default-configmap-a.yaml", "default-secret-b.yaml", "s.yaml", "sub/s.yaml"}
			if got := listedResources(t, files, "p/kustomization.yaml"); !slices.Equal(got, want) {
				t.Errorf("p/kustomization.yaml lists %v, want %v", got, want)
			}
			requireBuilds(t, writer, files)
		})
		t.Run("one object/"+writer, func(t *testing.T) {
			err := writeRefused(t, writer, layout.Config{}, pair(testObj("v1", "ConfigMap", "s")))
			want := `layouts "p/s" and "p/sub/s" both hold the object v1 ConfigMap default/s, and the kustomization.yaml of layout "p" builds both`
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Errorf("err = %v, want it to contain %q", err, want)
			}
		})
	}
}

// TestWriters_RefuseListedSingleChildOutsideParent: an AppFileSingle child
// whose file lands outside its parent's directory, in a sibling or an
// ancestor directory, is refused before anything is written, naming both
// layouts: kustomize's default load restrictor refuses the "../" entry the
// parent would list ("is not in or below").
func TestWriters_RefuseListedSingleChildOutsideParent(t *testing.T) {
	cases := map[string]struct {
		cfg       layout.Config
		childMode layout.ApplicationFileMode
		ns        string
		writers   []string
		child     string // the refused child's FullRepoPath
	}{
		"sibling directory":  {childMode: layout.AppFileSingle, ns: "q", writers: allWriters, child: "q/svc"},
		"ancestor directory": {childMode: layout.AppFileSingle, ns: ".", writers: allWriters, child: "svc"},
		// "p2" starts with "p" as a string, but is not below it.
		"sibling sharing the parent's prefix": {childMode: layout.AppFileSingle, ns: "p2", writers: allWriters, child: "p2/svc"},
		"sibling, single through Config": {
			cfg: layout.Config{ApplicationFileMode: layout.AppFileSingle}, childMode: layout.AppFileUnset,
			ns: "q", writers: []string{"WriteManifest"}, child: "q/svc",
		},
	}
	for name, tc := range cases {
		for _, writer := range tc.writers {
			t.Run(name+"/"+writer, func(t *testing.T) {
				p := singleChildParent(tc.childMode)
				p.Children[0].Namespace = tc.ns
				err := writeRefused(t, writer, tc.cfg, p)
				want := `layout "` + tc.child + `" is AppFileSingle and its file`
				if err == nil || !strings.Contains(err.Error(), want) || !strings.Contains(err.Error(), `outside the directory`) || !strings.Contains(err.Error(), `of layout "p"`) {
					t.Fatalf("err = %v, want a refusal naming %s outside the directory of layout p", err, tc.child)
				}
			})
		}
	}
}

// TestWriters_UnlistedSingleChildOutsideParentIsWritten: a child its parent's
// kustomization.yaml does not list lands where it lands; nothing lists it, so
// nothing contradicts the output and every writer writes it.
func TestWriters_UnlistedSingleChildOutsideParentIsWritten(t *testing.T) {
	cases := map[string]func() *layout.ManifestLayout{
		"umbrella child": func() *layout.ManifestLayout {
			p := singleChildParent(layout.AppFileSingle)
			p.Children[0].Namespace = "q"
			p.Children[0].UmbrellaChild = true
			return p
		},
		// A Recursive parent writes no kustomization.yaml to list it in.
		"child of a KustomizationRecursive parent": func() *layout.ManifestLayout {
			p := singleChildParent(layout.AppFileSingle)
			p.Mode = layout.KustomizationRecursive
			p.Children[0].Namespace = "q"
			return p
		},
	}
	for name, build := range cases {
		for _, writer := range allWriters {
			t.Run(name+"/"+writer, func(t *testing.T) {
				files := writtenFiles(t, writer, layout.Config{}, build())
				if _, ok := files["q/svc.yaml"]; !ok {
					t.Errorf("no q/svc.yaml written; wrote %v", slices.Sorted(maps.Keys(files)))
				}
				if k, ok := files["p/kustomization.yaml"]; ok && strings.Contains(k, "svc.yaml") {
					t.Errorf("p/kustomization.yaml lists the unlisted child:\n%s", k)
				}
			})
		}
	}
}

// TestWriters_RefuseListedSingleFileInMarkedRecursiveBuild: p lists its
// AppFileSingle child's file d/s.yaml, which lands in the directory of p's
// unlisted child d, a KustomizationRecursive directory a Flux Kustomization
// kure generated builds. That build applies the file too, so its objects would
// be applied twice; Explicit mode would not list it in d. Unmarked, d has no
// such build and the tree is written.
func TestWriters_RefuseListedSingleFileInMarkedRecursiveBuild(t *testing.T) {
	tree := func(marked bool) *layout.ManifestLayout {
		p := singleChildParent(layout.AppFileSingle)
		p.Children[0].Namespace = "p/d"
		d := child(p, "d", layout.KustomizationRecursive)
		d.UmbrellaChild = true
		d.SetFluxBuild(marked)
		return p
	}
	want := `the Flux build of KustomizationRecursive layout "p/d" would apply the file "svc.yaml" of layout "p/d/svc", which the kustomization.yaml of layout "p" lists as well`
	for _, writer := range allWriters {
		t.Run("marked/"+writer, func(t *testing.T) {
			err := writeRefused(t, writer, layout.Config{}, tree(true))
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Errorf("err = %v, want it to contain %q", err, want)
			}
		})
		t.Run("unmarked/"+writer, func(t *testing.T) {
			files := writtenFiles(t, writer, layout.Config{}, tree(false))
			if got := listedResources(t, files, "p/kustomization.yaml"); !slices.Contains(got, "d/svc.yaml") {
				t.Errorf("p/kustomization.yaml lists %v, want it to list d/svc.yaml", got)
			}
			requireBuilds(t, writer, files)
		})
	}
}
