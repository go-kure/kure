package layout_test

import (
	"archive/tar"
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
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
