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
