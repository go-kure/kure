package layout_test

import (
	"archive/tar"
	"bytes"
	"errors"
	"io"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// Tests for AppFileSingle children (go-kure/kure#860): such a child writes one
// file, <child>.yaml, into its parent's directory. The parent's
// kustomization.yaml must list it next to every one of the parent's own
// files, and the child must never replace that kustomization.yaml with one of
// its own.

// writtenFiles runs one writer and returns every file it wrote, keyed by its
// slash-separated path relative to the writer's root (for WriteManifest, the
// ManifestsDir). A path written twice holds the last content, as on disk.
func writtenFiles(t *testing.T, writer string, cfg layout.Config, ml *layout.ManifestLayout) map[string]string {
	t.Helper()
	out := map[string]string{}
	readDir := func(root string) {
		err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			data, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(root, p)
			if err != nil {
				return err
			}
			out[filepath.ToSlash(rel)] = string(data)
			return nil
		})
		if err != nil {
			t.Fatalf("%s: read output: %v", writer, err)
		}
	}
	switch writer {
	case "WriteToDisk":
		dir := filepath.Join(t.TempDir(), "out")
		if err := ml.WriteToDisk(dir); err != nil {
			t.Fatalf("WriteToDisk: %v", err)
		}
		readDir(dir)
	case "WriteToTar":
		var buf bytes.Buffer
		if err := ml.WriteToTar(&buf); err != nil {
			t.Fatalf("WriteToTar: %v", err)
		}
		tr := tar.NewReader(&buf)
		for {
			hdr, err := tr.Next()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				t.Fatalf("WriteToTar: read archive: %v", err)
			}
			if hdr.Typeflag != tar.TypeReg {
				continue
			}
			data, err := io.ReadAll(tr)
			if err != nil {
				t.Fatalf("WriteToTar: read %s: %v", hdr.Name, err)
			}
			out[hdr.Name] = string(data)
		}
	case "WriteManifest":
		dir := filepath.Join(t.TempDir(), "out")
		if err := layout.WriteManifest(dir, cfg, ml); err != nil {
			t.Fatalf("WriteManifest: %v", err)
		}
		manifests := cfg.ManifestsDir
		if manifests == "" {
			manifests = "clusters"
		}
		readDir(filepath.Join(dir, manifests))
	default:
		t.Fatalf("unknown writer %q", writer)
	}
	return out
}

// listedResources parses a kustomization.yaml and returns its resources.
func listedResources(t *testing.T, files map[string]string, path string) []string {
	t.Helper()
	data, ok := files[path]
	if !ok {
		t.Fatalf("no %s written", path)
	}
	var k struct {
		Resources []string `json:"resources"`
	}
	if err := yaml.Unmarshal([]byte(data), &k); err != nil {
		t.Fatalf("parse %s: %v\n%s", path, err, data)
	}
	slices.Sort(k.Resources)
	return k.Resources
}

// singleChildParent is parent p (a ConfigMap and a Secret of its own) with an
// AppFileSingle child svc, whose Namespace is p's directory.
func singleChildParent(childMode layout.ApplicationFileMode) *layout.ManifestLayout {
	p := &layout.ManifestLayout{
		Name:                "p",
		Namespace:           ".",
		ApplicationFileMode: layout.AppFilePerResource,
		Resources: []client.Object{
			testObj("v1", "ConfigMap", "a"),
			testObj("v1", "Secret", "b"),
		},
	}
	p.Children = []*layout.ManifestLayout{{
		Name:                "svc",
		Namespace:           p.FullRepoPath(),
		ApplicationFileMode: childMode,
		Resources:           []client.Object{testObj("v1", "ConfigMap", "svc-config")},
	}}
	return p
}

func testObj(apiVersion, kind, name string) client.Object {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion(apiVersion)
	obj.SetKind(kind)
	obj.SetName(name)
	obj.SetNamespace("default")
	return obj
}

// TestWriters_AppFileSingleChildKeepsParentKustomization: every writer lists
// the parent's own files and the child's file in the parent's
// kustomization.yaml, and the child's file holds the child's objects.
func TestWriters_AppFileSingleChildKeepsParentKustomization(t *testing.T) {
	want := []string{"default-configmap-a.yaml", "default-secret-b.yaml", "svc.yaml"}
	for _, writer := range []string{"WriteToDisk", "WriteToTar", "WriteManifest"} {
		t.Run(writer, func(t *testing.T) {
			files := writtenFiles(t, writer, layout.Config{}, singleChildParent(layout.AppFileSingle))
			if got := listedResources(t, files, "p/kustomization.yaml"); !slices.Equal(got, want) {
				t.Errorf("p/kustomization.yaml lists %v, want %v", got, want)
			}
			if !strings.Contains(files["p/svc.yaml"], "name: svc-config") {
				t.Errorf("p/svc.yaml does not hold the child's ConfigMap:\n%s", files["p/svc.yaml"])
			}
			if _, ok := files["p/svc/kustomization.yaml"]; ok {
				t.Error("an AppFileSingle child wrote a directory of its own")
			}
		})
	}
}

// TestWriteManifest_ConfigSingleChildIsListedAsFile: under a Config whose
// ApplicationFileMode is AppFileSingle (the Argo profile's), a child with an
// unset mode is written as a single file, so its parent lists that file, not
// a directory that does not exist.
func TestWriteManifest_ConfigSingleChildIsListedAsFile(t *testing.T) {
	cfg := layout.Config{ApplicationFileMode: layout.AppFileSingle}
	files := writtenFiles(t, "WriteManifest", cfg, singleChildParent(layout.AppFileUnset))
	want := []string{"default-configmap-a.yaml", "default-secret-b.yaml", "svc.yaml"}
	if got := listedResources(t, files, "p/kustomization.yaml"); !slices.Equal(got, want) {
		t.Errorf("p/kustomization.yaml lists %v, want %v", got, want)
	}
}

// TestWriters_ClusterRootListsSingleChild: the synthetic cluster root
// (no name, no resources of its own) normally gets no kustomization.yaml, but
// one that holds an AppFileSingle child's file lists it.
func TestWriters_ClusterRootListsSingleChild(t *testing.T) {
	root := &layout.ManifestLayout{Namespace: "demo"}
	child := cmLayout("svc", root.FullRepoPath())
	child.ApplicationFileMode = layout.AppFileSingle
	root.Children = []*layout.ManifestLayout{child}
	for _, writer := range []string{"WriteToDisk", "WriteToTar", "WriteManifest"} {
		t.Run(writer, func(t *testing.T) {
			files := writtenFiles(t, writer, layout.Config{}, root)
			if got, want := listedResources(t, files, "demo/kustomization.yaml"), []string{"svc.yaml"}; !slices.Equal(got, want) {
				t.Errorf("demo/kustomization.yaml lists %v, want %v", got, want)
			}
		})
	}
}

// TestWriteManifest_ArgoProfileAppChildrenResolve: under the Argo profile a
// walked bundle layout's application children are single files in the
// bundle's directory; every kustomization.yaml reference resolves.
func TestWriteManifest_ArgoProfileAppChildrenResolve(t *testing.T) {
	ml := walk(t, twoTier("platform", "web-bundle"), groupByName)
	out := t.TempDir()
	if err := layout.WriteManifest(out, layout.DefaultConfigForProfile(layout.ArgoProfile), ml); err != nil {
		t.Fatalf("WriteManifest: %v", err)
	}
	if bad := unresolvedKustomizeRefs(t, out); len(bad) > 0 {
		t.Errorf("unresolved kustomization.yaml references: %v", bad)
	}
	files := writtenFiles(t, "WriteManifest", layout.DefaultConfigForProfile(layout.ArgoProfile), walk(t, twoTier("platform", "web-bundle"), groupByName))
	if got, want := listedResources(t, files, "platform/web/web-bundle/kustomization.yaml"), []string{"frontend.yaml"}; !slices.Equal(got, want) {
		t.Errorf("platform/web/web-bundle/kustomization.yaml lists %v, want %v", got, want)
	}
}

// danglingRefs returns every kustomization.yaml entry among files that names
// neither a written file nor a directory holding one.
func danglingRefs(t *testing.T, files map[string]string) []string {
	t.Helper()
	var bad []string
	for p := range files {
		if path.Base(p) != "kustomization.yaml" {
			continue
		}
		for _, ref := range listedResources(t, files, p) {
			target := path.Join(path.Dir(p), ref)
			if _, ok := files[target]; ok {
				continue
			}
			if !slices.ContainsFunc(slices.Collect(maps.Keys(files)), func(f string) bool {
				return strings.HasPrefix(f, target+"/")
			}) {
				bad = append(bad, target)
			}
		}
	}
	slices.Sort(bad)
	return bad
}

// TestWriters_RefuseSingleChildWithChildren: an AppFileSingle child writes
// one file and no kustomization.yaml, so nothing would list a layout below
// it. Every writer refuses the tree before writing anything.
func TestWriters_RefuseSingleChildWithChildren(t *testing.T) {
	cases := map[string]struct {
		cfg       layout.Config
		childMode layout.ApplicationFileMode
	}{
		"own mode":    {childMode: layout.AppFileSingle},
		"Config mode": {cfg: layout.Config{ApplicationFileMode: layout.AppFileSingle}, childMode: layout.AppFileUnset},
	}
	for name, tc := range cases {
		for _, writer := range []string{"WriteToDisk", "WriteToTar", "WriteManifest"} {
			if tc.childMode == layout.AppFileUnset && writer != "WriteManifest" {
				continue // only WriteManifest takes a mode from Config
			}
			t.Run(name+"/"+writer, func(t *testing.T) {
				p := singleChildParent(tc.childMode)
				svc := p.Children[0]
				svc.Children = []*layout.ManifestLayout{cmLayout("gc", svc.FullRepoPath())}
				dir := filepath.Join(t.TempDir(), "out")
				var buf bytes.Buffer
				var err error
				switch writer {
				case "WriteToDisk":
					err = p.WriteToDisk(dir)
				case "WriteToTar":
					err = p.WriteToTar(&buf)
				case "WriteManifest":
					err = layout.WriteManifest(dir, tc.cfg, p)
				}
				if err == nil || !strings.Contains(err.Error(), `"p/svc"`) || !strings.Contains(err.Error(), "child layouts") {
					t.Fatalf("err = %v, want a refusal naming p/svc and its child layouts", err)
				}
				if _, statErr := os.Stat(dir); !errors.Is(statErr, fs.ErrNotExist) {
					t.Errorf("a refused tree left output at %s (stat: %v)", dir, statErr)
				}
				if buf.Len() > 0 {
					t.Errorf("a refused tree wrote %d tar bytes", buf.Len())
				}
			})
		}
	}
}

// TestWriteManifest_ArgoProfileWalkedTreesAreWritten: the refusal of an
// AppFileSingle layout with children never fires on a walked tree. Under the
// Argo profile the synthetic cluster wrapper (no origin, one child) takes
// Config's AppFileSingle, but it is the root, which the refusal leaves alone.
func TestWriteManifest_ArgoProfileWalkedTreesAreWritten(t *testing.T) {
	cfg := layout.DefaultConfigForProfile(layout.ArgoProfile)
	for name, rules := range map[string]layout.LayoutRules{
		"groupByName":              groupByName,
		"nodeOnly":                 nodeOnly,
		"nodeFlat":                 nodeFlat,
		"groupByName/cluster demo": withClusterName(groupByName, "demo"),
		"nodeOnly/cluster demo":    withClusterName(nodeOnly, "demo"),
		"nodeFlat/cluster demo":    withClusterName(nodeFlat, "demo"),
	} {
		t.Run(name, func(t *testing.T) {
			out := t.TempDir()
			if err := layout.WriteManifest(out, cfg, walk(t, twoTier("platform", "web-bundle"), rules)); err != nil {
				t.Fatalf("WriteManifest: %v", err)
			}
		})
	}
}

// TestWriters_SingleChildWithoutResourcesIsNotListed: an AppFileSingle child
// with no resources writes no file, so no kustomization.yaml lists one, and
// the synthetic cluster root holding only such a child stays without one.
func TestWriters_SingleChildWithoutResourcesIsNotListed(t *testing.T) {
	cases := map[string]struct {
		cfg       layout.Config
		childMode layout.ApplicationFileMode
		writers   []string
	}{
		"own mode":    {childMode: layout.AppFileSingle, writers: []string{"WriteToDisk", "WriteToTar", "WriteManifest"}},
		"Config mode": {cfg: layout.Config{ApplicationFileMode: layout.AppFileSingle}, childMode: layout.AppFileUnset, writers: []string{"WriteManifest"}},
	}
	for name, tc := range cases {
		for _, writer := range tc.writers {
			t.Run(name+"/"+writer+"/parent", func(t *testing.T) {
				p := singleChildParent(tc.childMode)
				p.Children[0].Resources = nil
				files := writtenFiles(t, writer, tc.cfg, p)
				want := []string{"default-configmap-a.yaml", "default-secret-b.yaml"}
				if got := listedResources(t, files, "p/kustomization.yaml"); !slices.Equal(got, want) {
					t.Errorf("p/kustomization.yaml lists %v, want %v", got, want)
				}
				if bad := danglingRefs(t, files); len(bad) > 0 {
					t.Errorf("kustomization.yaml entries name nothing written: %v", bad)
				}
			})
			t.Run(name+"/"+writer+"/cluster root", func(t *testing.T) {
				root := &layout.ManifestLayout{Namespace: "demo"}
				root.Children = []*layout.ManifestLayout{{Name: "svc", Namespace: root.FullRepoPath(), ApplicationFileMode: tc.childMode}}
				files := writtenFiles(t, writer, tc.cfg, root)
				if bad := danglingRefs(t, files); len(bad) > 0 {
					t.Errorf("kustomization.yaml entries name nothing written: %v", bad)
				}
				if _, ok := files["demo/kustomization.yaml"]; ok && writer == "WriteManifest" {
					t.Errorf("WriteManifest wrote the empty cluster root's kustomization.yaml:\n%s", files["demo/kustomization.yaml"])
				}
			})
		}
	}
}

// TestWriters_SingleRootWritesKustomization: an AppFileSingle root has no
// parent to list its file, so it writes a kustomization.yaml next to it.
func TestWriters_SingleRootWritesKustomization(t *testing.T) {
	for _, writer := range []string{"WriteToDisk", "WriteToTar", "WriteManifest"} {
		t.Run(writer, func(t *testing.T) {
			root := cmLayout("svc", "demo")
			root.ApplicationFileMode = layout.AppFileSingle
			files := writtenFiles(t, writer, layout.Config{}, root)
			if got, want := listedResources(t, files, "demo/kustomization.yaml"), []string{"svc.yaml"}; !slices.Equal(got, want) {
				t.Errorf("demo/kustomization.yaml lists %v, want %v", got, want)
			}
		})
	}
}

// TestWriters_SingleRootListsChildrenInItsNamespace: a named AppFileSingle
// root writes its file and kustomization.yaml into its Namespace, so a child
// built with the root's Namespace is listed there and resolves.
func TestWriters_SingleRootListsChildrenInItsNamespace(t *testing.T) {
	for _, writer := range []string{"WriteToDisk", "WriteToTar", "WriteManifest"} {
		t.Run(writer, func(t *testing.T) {
			root := cmLayout("svc", "demo")
			root.ApplicationFileMode = layout.AppFileSingle
			root.Children = []*layout.ManifestLayout{cmLayout("gc", root.Namespace)}
			files := writtenFiles(t, writer, layout.Config{}, root)
			if got, want := listedResources(t, files, "demo/kustomization.yaml"), []string{"gc", "svc.yaml"}; !slices.Equal(got, want) {
				t.Errorf("demo/kustomization.yaml lists %v, want %v", got, want)
			}
			if _, ok := files["demo/gc/kustomization.yaml"]; !ok {
				t.Errorf("listed child directory demo/gc has no kustomization.yaml; wrote %v", slices.Sorted(maps.Keys(files)))
			}
		})
	}
}

// writeRefused runs one writer on ml and returns its error, failing the test
// when the writer left any output behind: a refusal comes before anything is
// written.
func writeRefused(t *testing.T, writer string, cfg layout.Config, ml *layout.ManifestLayout) error {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "out")
	var buf bytes.Buffer
	var err error
	switch writer {
	case "WriteToDisk":
		err = ml.WriteToDisk(dir)
	case "WriteToTar":
		err = ml.WriteToTar(&buf)
	case "WriteManifest":
		err = layout.WriteManifest(dir, cfg, ml)
	default:
		t.Fatalf("unknown writer %q", writer)
	}
	if _, statErr := os.Stat(dir); !errors.Is(statErr, fs.ErrNotExist) {
		t.Errorf("%s: a refused tree left output at %s (stat: %v)", writer, dir, statErr)
	}
	if buf.Len() > 0 {
		t.Errorf("%s: a refused tree wrote %d tar bytes", writer, buf.Len())
	}
	return err
}

// TestWriters_RefuseSingleFileOverParentFile: an AppFileSingle layout's file,
// <Name>.yaml, and its extra files land in the directory of another layout,
// usually its parent, so every writer refuses one that is that layout's
// kustomization.yaml or one of its resource or extra files, or another
// AppFileSingle layout's file there, before writing anything
// (go-kure/kure#871). Without the refusal the file replaced the other,
// dropping its objects or its whole kustomization.
func TestWriters_RefuseSingleFileOverParentFile(t *testing.T) {
	allWriters := []string{"WriteToDisk", "WriteToTar", "WriteManifest"}
	cases := map[string]struct {
		cfg     layout.Config
		build   func() *layout.ManifestLayout
		writers []string
		want    string // what the refusal says the file would replace
	}{
		"child named kustomization": {
			build: func() *layout.ManifestLayout {
				p := singleChildParent(layout.AppFileSingle)
				p.Children[0].Name = "kustomization"
				return p
			},
			writers: allWriters,
			want:    `would replace the kustomization.yaml of layout "p"`,
		},
		// Names are compared as checkLayoutTree compares directories:
		// case-insensitively, as on default macOS volumes, where
		// Kustomization.yaml and kustomization.yaml are one file.
		"child named Kustomization": {
			build: func() *layout.ManifestLayout {
				p := singleChildParent(layout.AppFileSingle)
				p.Children[0].Name = "Kustomization"
				return p
			},
			writers: allWriters,
			want:    `would replace the kustomization.yaml of layout "p"`,
		},
		"child named like a generated file": {
			build: func() *layout.ManifestLayout {
				p := singleChildParent(layout.AppFileSingle)
				p.Children[0].Name = "default-configmap-a"
				return p
			},
			writers: allWriters,
			want:    `would replace the resource file "default-configmap-a.yaml" of layout "p"`,
		},
		"child named like a kind file, layout naming": {
			build: func() *layout.ManifestLayout {
				p := singleChildParent(layout.AppFileSingle)
				p.FileNaming = layout.FileNamingKindName
				p.FilePer = layout.FilePerKind
				p.Children[0].Name = "configmap"
				return p
			},
			writers: []string{"WriteToDisk", "WriteToTar"},
			want:    `would replace the resource file "configmap.yaml" of layout "p"`,
		},
		// WriteManifest names files from Config, not the layout.
		"child named like a kind file, Config naming": {
			cfg: layout.Config{FileNaming: layout.FileNamingKindName, FilePer: layout.FilePerKind},
			build: func() *layout.ManifestLayout {
				p := singleChildParent(layout.AppFileSingle)
				p.Children[0].Name = "configmap"
				return p
			},
			writers: []string{"WriteManifest"},
			want:    `would replace the resource file "configmap.yaml" of layout "p"`,
		},
		"child single through Config": {
			cfg: layout.Config{ApplicationFileMode: layout.AppFileSingle},
			build: func() *layout.ManifestLayout {
				p := singleChildParent(layout.AppFileUnset)
				p.Children[0].Name = "kustomization"
				return p
			},
			writers: []string{"WriteManifest"},
			want:    `would replace the kustomization.yaml of layout "p"`,
		},
		// A root has no parent, but writes its file and its own
		// kustomization.yaml into one directory, its Namespace.
		"root named kustomization": {
			build: func() *layout.ManifestLayout {
				root := cmLayout("kustomization", "demo")
				root.ApplicationFileMode = layout.AppFileSingle
				return root
			},
			writers: allWriters,
			want:    `would replace the kustomization.yaml of layout "demo/kustomization"`,
		},
		// The directory's owner need not be the single layout's parent: x
		// is p's child but lands in sib's directory, next to sib's extra
		// file x.yaml.
		"single file over another layout's extra file": {
			build: func() *layout.ManifestLayout {
				sib := cmLayout("sib", "p")
				sib.ExtraFiles = []layout.ExtraFile{{Name: "x.yaml", Content: []byte("k: v\n")}}
				x := cmLayout("x", "p/sib")
				x.ApplicationFileMode = layout.AppFileSingle
				return &layout.ManifestLayout{Name: "p", Namespace: ".", Children: []*layout.ManifestLayout{sib, x}}
			},
			writers: allWriters,
			want:    `would replace the extra file "x.yaml" of layout "p/sib"`,
		},
		// The parent's own extra file is checkExtraFiles', which now also
		// runs before anything is written, including the files of the
		// layouts above the parent.
		"single file over its parent's extra file": {
			build: func() *layout.ManifestLayout {
				root := cmLayout("r", ".")
				p := singleChildParent(layout.AppFileSingle)
				p.Namespace = root.FullRepoPath()
				p.Children[0].Namespace = p.FullRepoPath()
				p.ExtraFiles = []layout.ExtraFile{{Name: "svc.yaml", Content: []byte("k: v\n")}}
				root.Children = []*layout.ManifestLayout{p}
				return root
			},
			writers: allWriters,
			want:    `extra file "svc.yaml" would replace the child file "svc.yaml"`,
		},
		// An AppFileSingle layout's extra files land in the same directory
		// as its file.
		"single layout's extra file over a resource file": {
			build: func() *layout.ManifestLayout {
				p := singleChildParent(layout.AppFileSingle)
				p.Children[0].ExtraFiles = []layout.ExtraFile{{Name: "default-configmap-a.yaml", Content: []byte("k: v\n")}}
				return p
			},
			writers: allWriters,
			want:    `layout "p/svc" is AppFileSingle and its extra file "default-configmap-a.yaml" would replace the resource file "default-configmap-a.yaml" of layout "p"`,
		},
		"single layout's extra file over a sibling's file": {
			build: func() *layout.ManifestLayout {
				p := singleChildParent(layout.AppFileSingle)
				p.Children = append(p.Children, &layout.ManifestLayout{
					Name:                "a",
					Namespace:           p.FullRepoPath(),
					ApplicationFileMode: layout.AppFileSingle,
					Resources:           []client.Object{testObj("v1", "ConfigMap", "a-config")},
					ExtraFiles:          []layout.ExtraFile{{Name: "svc.yaml", Content: []byte("k: v\n")}},
				})
				return p
			},
			writers: allWriters,
			want:    `layout "p/a" is AppFileSingle and its extra file "svc.yaml" would replace the file "svc.yaml" of AppFileSingle layout "p/svc"`,
		},
	}
	for name, tc := range cases {
		for _, writer := range tc.writers {
			t.Run(name+"/"+writer, func(t *testing.T) {
				err := writeRefused(t, writer, tc.cfg, tc.build())
				if err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("err = %v, want a refusal naming %s", err, tc.want)
				}
			})
		}
	}
}

// TestWriters_ResourcelessSingleChildNamedLikeParentExtraFile: an
// AppFileSingle child without resources writes no <Name>.yaml, so a parent's
// extra file of that name replaces nothing and the tree is written.
func TestWriters_ResourcelessSingleChildNamedLikeParentExtraFile(t *testing.T) {
	for _, writer := range []string{"WriteToDisk", "WriteToTar", "WriteManifest"} {
		t.Run(writer, func(t *testing.T) {
			p := singleChildParent(layout.AppFileSingle)
			p.Children[0].Resources = nil
			p.ExtraFiles = []layout.ExtraFile{{Name: "svc.yaml", Content: []byte("k: v\n")}}
			files := writtenFiles(t, writer, layout.Config{}, p)
			if files["p/svc.yaml"] != "k: v\n" {
				t.Errorf("p/svc.yaml = %q, want the parent's extra file", files["p/svc.yaml"])
			}
		})
	}
}

// TestWriters_ResourcelessSingleChildNamedLikeParentFile: an AppFileSingle
// child without resources writes no file, so its name replaces nothing and
// the tree is written.
func TestWriters_ResourcelessSingleChildNamedLikeParentFile(t *testing.T) {
	for _, writer := range []string{"WriteToDisk", "WriteToTar", "WriteManifest"} {
		t.Run(writer, func(t *testing.T) {
			p := singleChildParent(layout.AppFileSingle)
			p.Children[0].Name = "kustomization"
			p.Children[0].Resources = nil
			files := writtenFiles(t, writer, layout.Config{}, p)
			want := []string{"default-configmap-a.yaml", "default-secret-b.yaml"}
			if got := listedResources(t, files, "p/kustomization.yaml"); !slices.Equal(got, want) {
				t.Errorf("p/kustomization.yaml lists %v, want %v", got, want)
			}
		})
	}
}

// TestWriters_RefuseExtraFileOverResourcelessSingleChildDir: the writers
// create an AppFileSingle child's directory even when the child writes no
// file, so a parent's extra file at that path is refused before anything is
// written.
func TestWriters_RefuseExtraFileOverResourcelessSingleChildDir(t *testing.T) {
	for _, writer := range []string{"WriteToDisk", "WriteToTar", "WriteManifest"} {
		t.Run(writer, func(t *testing.T) {
			p := singleChildParent(layout.AppFileSingle)
			p.Children[0].Resources = nil
			p.Children[0].Namespace = "p/sub"
			p.ExtraFiles = []layout.ExtraFile{{Name: "sub", Content: []byte("k: v\n")}}
			err := writeRefused(t, writer, layout.Config{}, p)
			if err == nil || !strings.Contains(err.Error(), `extra file "sub" would take a directory that child layout "svc" needs`) {
				t.Errorf("got %v, want the extra file refused for the child's directory", err)
			}
		})
	}
}

// TestWriters_ExtraFileRefusalNamesOneOwner: when several files need the
// directory an extra file would take, the refusal names the same one every
// time.
func TestWriters_ExtraFileRefusalNamesOneOwner(t *testing.T) {
	for range 50 {
		p := singleChildParent(layout.AppFileSingle)
		p.Children[0].Namespace = "p/sub"
		second := *p.Children[0]
		second.Name = "svc2"
		p.Children = append(p.Children, &second)
		p.ExtraFiles = []layout.ExtraFile{{Name: "sub", Content: []byte("k: v\n")}}
		err := writeRefused(t, "WriteToTar", layout.Config{}, p)
		if err == nil || !strings.Contains(err.Error(), `extra file "sub" would take a directory that the child file "sub/svc.yaml" needs`) {
			t.Fatalf("got %v, want the refusal to name the first file in path order", err)
		}
	}
}

// singleLayout is an AppFileSingle layout name in directory ns, with one
// ConfigMap and the named extra files.
func singleLayout(name, ns string, extras ...string) *layout.ManifestLayout {
	l := cmLayout(name, ns)
	l.ApplicationFileMode = layout.AppFileSingle
	for _, e := range extras {
		l.ExtraFiles = append(l.ExtraFiles, layout.ExtraFile{Name: e, Content: []byte("k: v\n")})
	}
	return l
}

// TestWriters_RefuseSingleFileDirectoryClash: an AppFileSingle layout's file
// and extra files land in another layout's directory, so every writer
// refuses one that needs a directory where another written path is a file,
// or is a file where another written path needs a directory, before writing
// anything (go-kure/kure#878); so, too, the directory the writers create for
// a single layout without resources. Without the refusal WriteToDisk and
// WriteManifest failed partway with "not a directory", and WriteToTar wrote
// an archive tar could not extract; names that differ only in case clash
// that way only on a case-insensitive volume.
func TestWriters_RefuseSingleFileDirectoryClash(t *testing.T) {
	parent := func(children ...*layout.ManifestLayout) *layout.ManifestLayout {
		p := singleChildParent(layout.AppFileSingle)
		p.Children = children
		return p
	}
	cases := map[string]struct {
		build func() *layout.ManifestLayout
		want  string
	}{
		"extra file beneath the parent's resource file": {
			build: func() *layout.ManifestLayout {
				return parent(singleLayout("svc", "p", "default-configmap-a.yaml/v.yaml"))
			},
			want: `layout "p/svc" is AppFileSingle and its extra file "default-configmap-a.yaml/v.yaml" would use the resource file "default-configmap-a.yaml" of layout "p" as a directory`,
		},
		"extra file over the directory of a directory child": {
			build: func() *layout.ManifestLayout {
				return parent(singleLayout("svc", "p", "sub"), cmLayout("sub", "p"))
			},
			want: `layout "p/svc" is AppFileSingle and its extra file "sub" would take the directory of layout "p/sub"`,
		},
		"extra file over the directory of a directory child, other case": {
			build: func() *layout.ManifestLayout {
				return parent(singleLayout("svc", "p", "SUB"), cmLayout("sub", "p"))
			},
			want: `layout "p/svc" is AppFileSingle and its extra file "SUB" would take the directory of layout "p/sub"`,
		},
		"extra file above the directory of a directory child": {
			build: func() *layout.ManifestLayout {
				return parent(singleLayout("svc", "p", "sub"), cmLayout("deeper", "p/sub"))
			},
			want: `layout "p/svc" is AppFileSingle and its extra file "sub" would take a directory that layout "p/sub/deeper" needs`,
		},
		"extra file over the directory of a resourceless single layout": {
			build: func() *layout.ManifestLayout {
				b := singleLayout("b", "p/sub")
				b.Resources = nil
				return parent(singleLayout("a", "p", "sub"), b)
			},
			want: `layout "p/a" is AppFileSingle and its extra file "sub" would take a directory that layout "p/sub/b" needs`,
		},
		"extra file over the directory of the parent's extra file": {
			build: func() *layout.ManifestLayout {
				p := parent(singleLayout("svc", "p", "sub"))
				p.ExtraFiles = []layout.ExtraFile{{Name: "sub/v.yaml", Content: []byte("k: v\n")}}
				return p
			},
			want: `layout "p/svc" is AppFileSingle and its extra file "sub" would take a directory that the extra file "sub/v.yaml" of layout "p" needs`,
		},
		"extra file beneath another single layout's extra file": {
			build: func() *layout.ManifestLayout {
				return parent(singleLayout("a", "p", "x"), singleLayout("b", "p", "x/y.yaml"))
			},
			want: `layout "p/b" is AppFileSingle and its extra file "x/y.yaml" would use the extra file "x" of AppFileSingle layout "p/a" as a directory`,
		},
		"extra file over the directory of another single layout's extra file": {
			build: func() *layout.ManifestLayout {
				return parent(singleLayout("a", "p", "x/y.yaml"), singleLayout("b", "p", "x"))
			},
			want: `layout "p/b" is AppFileSingle and its extra file "x" would take a directory that the extra file "x/y.yaml" of AppFileSingle layout "p/a" needs`,
		},
		"extra file beneath another single layout's file": {
			build: func() *layout.ManifestLayout {
				return parent(singleLayout("x", "p"), singleLayout("b", "p", "x.yaml/v.yaml"))
			},
			want: `layout "p/b" is AppFileSingle and its extra file "x.yaml/v.yaml" would use the file "x.yaml" of AppFileSingle layout "p/x" as a directory`,
		},
		"file over the directory of another single layout's extra file": {
			build: func() *layout.ManifestLayout {
				return parent(singleLayout("b", "p", "x.yaml/v.yaml"), singleLayout("x", "p"))
			},
			want: `layout "p/x" is AppFileSingle and its file "x.yaml" would take a directory that the extra file "x.yaml/v.yaml" of AppFileSingle layout "p/b" needs`,
		},
		// A single layout without resources writes no file, but the
		// writers still create its directory.
		"resourceless single layout's directory over the parent's resource file": {
			build: func() *layout.ManifestLayout {
				s := singleLayout("s", "p/default-configmap-a.yaml")
				s.Resources = nil
				return parent(s)
			},
			want: `the directory of layout "p/default-configmap-a.yaml/s" would replace the resource file "default-configmap-a.yaml" of layout "p"`,
		},
		"resourceless single layout's directory over a sibling's extra file": {
			build: func() *layout.ManifestLayout {
				c := cmLayout("c", "p")
				c.ExtraFiles = []layout.ExtraFile{{Name: "sub", Content: []byte("k: v\n")}}
				s := singleLayout("s", "p/c/sub")
				s.Resources = nil
				return parent(c, s)
			},
			want: `the directory of layout "p/c/sub/s" would replace the extra file "sub" of layout "p/c"`,
		},
		"resourceless single layout's directory beneath a sibling's extra file": {
			build: func() *layout.ManifestLayout {
				c := cmLayout("c", "p")
				c.ExtraFiles = []layout.ExtraFile{{Name: "sub", Content: []byte("k: v\n")}}
				s := singleLayout("s", "p/c/sub/deeper")
				s.Resources = nil
				return parent(c, s)
			},
			want: `the directory of layout "p/c/sub/deeper/s" would use the extra file "sub" of layout "p/c" as a directory`,
		},
		"directory layout's directory over the parent's resource file": {
			build: func() *layout.ManifestLayout {
				return parent(cmLayout("default-configmap-a.yaml", "p"))
			},
			want: `the directory of layout "p/default-configmap-a.yaml" would replace the resource file "default-configmap-a.yaml" of layout "p"`,
		},
		// Compared case-insensitively, as on default macOS volumes.
		"directory layout's directory over the parent's resource file, other case": {
			build: func() *layout.ManifestLayout {
				return parent(cmLayout("Default-ConfigMap-A.yaml", "p"))
			},
			want: `the directory of layout "p/Default-ConfigMap-A.yaml" would replace the resource file "default-configmap-a.yaml" of layout "p"`,
		},
		// kustomization.yaml is the one kustomize control file the
		// writers write.
		"directory layout's directory over the parent's kustomization.yaml": {
			build: func() *layout.ManifestLayout {
				return parent(cmLayout("kustomization.yaml", "p"))
			},
			want: `the directory of layout "p/kustomization.yaml" would replace the kustomization.yaml of layout "p"`,
		},
	}
	for name, tc := range cases {
		for _, writer := range []string{"WriteToDisk", "WriteToTar", "WriteManifest"} {
			t.Run(name+"/"+writer, func(t *testing.T) {
				err := writeRefused(t, writer, layout.Config{}, tc.build())
				if err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("err = %v, want a refusal naming %s", err, tc.want)
				}
			})
		}
	}
}

// TestWriters_SingleExtraFileInSubdirectory: an AppFileSingle layout's extra
// file in a subdirectory of the directory it lands in is written when no
// other path there needs that subdirectory to be a file, or a file in it to
// be a directory; two such layouts may share the subdirectory.
func TestWriters_SingleExtraFileInSubdirectory(t *testing.T) {
	for _, writer := range []string{"WriteToDisk", "WriteToTar", "WriteManifest"} {
		t.Run(writer, func(t *testing.T) {
			p := singleChildParent(layout.AppFileSingle)
			p.Children = []*layout.ManifestLayout{
				singleLayout("svc", "p", "sub/v.yaml"),
				singleLayout("w", "p", "sub/w.yaml"),
				cmLayout("other", "p"),
			}
			files := writtenFiles(t, writer, layout.Config{}, p)
			for _, f := range []string{"p/sub/v.yaml", "p/sub/w.yaml"} {
				if files[f] != "k: v\n" {
					t.Errorf("%s = %q, want the single layout's extra file; wrote %v", f, files[f], slices.Sorted(maps.Keys(files)))
				}
			}
			want := []string{"default-configmap-a.yaml", "default-secret-b.yaml", "other", "svc.yaml", "w.yaml"}
			if got := listedResources(t, files, "p/kustomization.yaml"); !slices.Equal(got, want) {
				t.Errorf("p/kustomization.yaml lists %v, want %v", got, want)
			}
		})
	}
}

// TestWriters_DirectoryNamedLikeUnwrittenControlFile: the writers write
// kustomization.yaml and no other kustomize control file, so a directory
// named kustomization, Kustomization or kustomization.yml next to it clashes
// with no written file, and every writer writes the tree.
func TestWriters_DirectoryNamedLikeUnwrittenControlFile(t *testing.T) {
	cases := map[string]struct {
		build func() *layout.ManifestLayout
		want  string // a file the writers must write
		// listed, when set, is what p/kustomization.yaml must list, and
		// kustomize must build the written tree.
		listed []string
	}{
		"directory child kustomization": {
			build: func() *layout.ManifestLayout {
				p := singleChildParent(layout.AppFileSingle)
				p.Children = []*layout.ManifestLayout{cmLayout("kustomization", "p")}
				return p
			},
			want: "p/kustomization/kustomization.yaml",
		},
		"directory child Kustomization": {
			build: func() *layout.ManifestLayout {
				p := singleChildParent(layout.AppFileSingle)
				p.Children = []*layout.ManifestLayout{cmLayout("Kustomization", "p")}
				return p
			},
			want: "p/Kustomization/kustomization.yaml",
		},
		"directory child kustomization.yml": {
			build: func() *layout.ManifestLayout {
				p := singleChildParent(layout.AppFileSingle)
				p.Children = []*layout.ManifestLayout{cmLayout("kustomization.yml", "p")}
				return p
			},
			want: "p/kustomization.yml/kustomization.yaml",
		},
		// p lists s's file by its path below p's directory
		// (go-kure/kure#879), and kustomize builds p.
		"single layout's file in directory kustomization": {
			build: func() *layout.ManifestLayout {
				p := singleChildParent(layout.AppFileSingle)
				p.Children = []*layout.ManifestLayout{singleLayout("s", "p/kustomization")}
				return p
			},
			want:   "p/kustomization/s.yaml",
			listed: []string{"default-configmap-a.yaml", "default-secret-b.yaml", "kustomization/s.yaml"},
		},
	}
	for name, tc := range cases {
		for _, writer := range []string{"WriteToDisk", "WriteToTar", "WriteManifest"} {
			t.Run(name+"/"+writer, func(t *testing.T) {
				files := writtenFiles(t, writer, layout.Config{}, tc.build())
				if _, ok := files[tc.want]; !ok {
					t.Errorf("no %s written; wrote %v", tc.want, slices.Sorted(maps.Keys(files)))
				}
				if _, ok := files["p/kustomization.yaml"]; !ok {
					t.Errorf("no p/kustomization.yaml written; wrote %v", slices.Sorted(maps.Keys(files)))
				}
				if tc.listed == nil {
					return
				}
				if got := listedResources(t, files, "p/kustomization.yaml"); !slices.Equal(got, tc.listed) {
					t.Errorf("p/kustomization.yaml lists %v, want %v", got, tc.listed)
				}
				requireBuilds(t, writer, files)
			})
		}
	}
}

// TestWriters_SingleExtraFileInChildDirectory: an AppFileSingle layout's
// extra file may land inside the directory of a sibling directory layout,
// its parent's child. That directory's kustomization.yaml does not list it,
// which contradicts nothing the writers produce, so every writer writes it.
// (A layout's own extra file inside its child's directory is refused; see
// checkExtraFiles.)
func TestWriters_SingleExtraFileInChildDirectory(t *testing.T) {
	for _, writer := range []string{"WriteToDisk", "WriteToTar", "WriteManifest"} {
		t.Run(writer, func(t *testing.T) {
			p := singleChildParent(layout.AppFileSingle)
			p.Children = []*layout.ManifestLayout{singleLayout("svc", "p", "sub/v.yaml"), cmLayout("sub", "p")}
			files := writtenFiles(t, writer, layout.Config{}, p)
			if files["p/sub/v.yaml"] != "k: v\n" {
				t.Errorf("p/sub/v.yaml = %q, want the single layout's extra file; wrote %v", files["p/sub/v.yaml"], slices.Sorted(maps.Keys(files)))
			}
			if got, want := listedResources(t, files, "p/sub/kustomization.yaml"), []string{"default-configmap-sub.yaml"}; !slices.Equal(got, want) {
				t.Errorf("p/sub/kustomization.yaml lists %v, want %v", got, want)
			}
		})
	}
}

// TestWriteManifest_WalkedSingleAppExtraFileInChildNodeDirectory: the same
// shape from a walked tree. Under ArgoProfile the root bundle's application
// monitoring is AppFileSingle and lands in the root's directory, and its
// augmenter's extra file monitoring/dash.yaml lands in the directory of the
// root's child node monitoring. The tree is written.
func TestWriteManifest_WalkedSingleAppExtraFileInChildNodeDirectory(t *testing.T) {
	aug := &fakeAugmentingConfig{objs: []*client.Object{makeCM("mon")}, extraFileName: "monitoring/dash.yaml", cmgName: "dash"}
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
	files := writtenFiles(t, "WriteManifest", layout.DefaultConfigForProfile(layout.ArgoProfile), ml)
	var dash string
	for f := range files {
		if strings.HasSuffix(f, "/monitoring/dash.yaml") {
			dash = f
		}
	}
	if dash == "" || files[dash] != "k: v\n" {
		t.Fatalf("no monitoring/dash.yaml written with the augmenter's content; wrote %v", slices.Sorted(maps.Keys(files)))
	}
	kust := path.Join(path.Dir(dash), "kustomization.yaml")
	if slices.Contains(listedResources(t, files, kust), "dash.yaml") {
		t.Errorf("%s lists dash.yaml, the parent application's extra file", kust)
	}
}

// TestWriters_SingleThroughConfigFollowsEachWritersPlan: a layout that is
// AppFileSingle only through Config is single in WriteManifest, which refuses
// its extra file beneath the parent's resource file, and a directory layout
// in WriteToDisk and WriteToTar, which ignore Config and write the extra file
// into its own directory. Each writer checks the tree against its own plan.
func TestWriters_SingleThroughConfigFollowsEachWritersPlan(t *testing.T) {
	cfg := layout.Config{ApplicationFileMode: layout.AppFileSingle}
	build := func() *layout.ManifestLayout {
		p := singleChildParent(layout.AppFileUnset)
		p.Children[0].ExtraFiles = []layout.ExtraFile{{Name: "default-configmap-a.yaml/v.yaml", Content: []byte("k: v\n")}}
		return p
	}
	t.Run("WriteManifest", func(t *testing.T) {
		err := writeRefused(t, "WriteManifest", cfg, build())
		want := `layout "p/svc" is AppFileSingle and its extra file "default-configmap-a.yaml/v.yaml" would use the resource file "default-configmap-a.yaml" of layout "p" as a directory`
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("err = %v, want a refusal naming %s", err, want)
		}
	})
	for _, writer := range []string{"WriteToDisk", "WriteToTar"} {
		t.Run(writer, func(t *testing.T) {
			files := writtenFiles(t, writer, cfg, build())
			if f := "p/svc/default-configmap-a.yaml/v.yaml"; files[f] != "k: v\n" {
				t.Errorf("%s = %q, want the extra file in the layout's own directory; wrote %v", f, files[f], slices.Sorted(maps.Keys(files)))
			}
		})
	}
}
