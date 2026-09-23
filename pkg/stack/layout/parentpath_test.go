package layout_test

import (
	"archive/tar"
	"bytes"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// Tests for parent-aware layout paths (go-kure/kure#771): Namespace is always
// the parent path, FullRepoPath is Namespace/Name, and every writer refuses a
// tree in which two layouts resolve to the same directory.

func TestFullRepoPath_IsNamespaceSlashName(t *testing.T) {
	for _, tc := range []struct {
		ns, name, want string
	}{
		{"applications/foo", "foo", "applications/foo/foo"},
		{"applications/foo", "oo", "applications/foo/oo"},
		{"applications/foo", "o", "applications/foo/o"},
		{"applications/foo", "bar", "applications/foo/bar"},
		{".", "web", "web"},
		{".", "", "."},
		{"", "web", "cluster/web"},
		{"", "", "cluster"},
	} {
		got := (&layout.ManifestLayout{Namespace: tc.ns, Name: tc.name}).FullRepoPath()
		if got != tc.want {
			t.Errorf("Namespace %q Name %q: FullRepoPath() = %q, want %q", tc.ns, tc.name, got, tc.want)
		}
	}
}

func configMapApp(appName string) *stack.Application {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind("ConfigMap")
	obj.SetName(appName + "-cm")
	obj.SetNamespace("default")
	var o client.Object = obj
	return stack.NewApplication(appName, "default", &fakeConfig{objs: []*client.Object{&o}})
}

// unresolvedKustomizeRefs returns every `- <ref>` entry of every
// kustomization.yaml under dir that names no existing file or directory.
func unresolvedKustomizeRefs(t *testing.T, dir string) []string {
	t.Helper()
	var bad []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() != "kustomization.yaml" {
			return err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		for _, line := range strings.Split(string(data), "\n") {
			trimmed := strings.TrimSpace(line)
			if !strings.HasPrefix(trimmed, "- ") {
				continue
			}
			ref := strings.TrimPrefix(trimmed, "- ")
			if _, err := os.Stat(filepath.Join(filepath.Dir(p), ref)); err != nil {
				rel, _ := filepath.Rel(dir, filepath.Join(filepath.Dir(p), ref))
				bad = append(bad, rel)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(bad)
	return bad
}

func writeTreeToDisk(t *testing.T, ml *layout.ManifestLayout) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "out")
	if err := ml.WriteToDisk(dir); err != nil {
		t.Fatalf("WriteToDisk: %v", err)
	}
	return dir
}

func TestWalkCluster_SameNameBundleAndAppNest(t *testing.T) {
	// A bundle and an application with the same name nest instead of
	// collapsing: before go-kure/kure#771 web/web resolved onto web, and the
	// two kustomization.yaml files at one path dropped api from the graph.
	// The bundle sits on a child node: the ClusterName walk always flattens the
	// root node's own bundle into the root layout.
	bundle := &stack.Bundle{Name: "web", Applications: []*stack.Application{configMapApp("web"), configMapApp("api")}}
	store := &stack.Node{Name: "store", Bundle: bundle}
	root := &stack.Node{Name: "shop", Children: []*stack.Node{store}}
	store.SetParent(root)
	cluster := &stack.Cluster{Name: "c", Node: root}
	ml, err := layout.WalkCluster(cluster, layout.LayoutRules{
		ClusterName:         "c",
		BundleGrouping:      layout.GroupByName,
		ApplicationGrouping: layout.GroupByName,
	})
	if err != nil {
		t.Fatalf("WalkCluster: %v", err)
	}
	paths := collectRepoPaths(ml)
	for _, want := range []string{"c/shop/store/web", "c/shop/store/web/web", "c/shop/store/web/api"} {
		if !containsString(paths, want) {
			t.Errorf("missing layout path %q in %v", want, paths)
		}
	}
	dir := writeTreeToDisk(t, ml)
	if bad := unresolvedKustomizeRefs(t, dir); len(bad) > 0 {
		t.Errorf("unresolved kustomize references: %v", bad)
	}
	for _, f := range []string{"c/shop/store/web/web", "c/shop/store/web/api"} {
		if _, err := os.Stat(filepath.Join(dir, f, "kustomization.yaml")); err != nil {
			t.Errorf("missing %s/kustomization.yaml: %v", f, err)
		}
	}
}

func TestWalkCluster_NoClusterNameChildNodesNestUnderRoot(t *testing.T) {
	// Without a ClusterName the root node sits at <root> and its child nodes
	// at <root>/<child>, the paths the Flux generator derives from the node
	// hierarchy. Before go-kure/kure#771 the root alone resolved to
	// cluster/<root>, so its references to the children dangled.
	child := &stack.Node{Name: "apps", Bundle: &stack.Bundle{Name: "apps-bundle", Applications: []*stack.Application{configMapApp("a")}}}
	root := &stack.Node{Name: "platform", Children: []*stack.Node{child}}
	child.SetParent(root)
	ml, err := layout.WalkCluster(&stack.Cluster{Name: "platform", Node: root}, layout.LayoutRules{})
	if err != nil {
		t.Fatalf("WalkCluster: %v", err)
	}
	if got := ml.FullRepoPath(); got != "platform" {
		t.Fatalf("root FullRepoPath() = %q, want platform", got)
	}
	if paths := collectRepoPaths(ml); !containsString(paths, "platform/apps") {
		t.Errorf("child node not under the root directory: %v", paths)
	}
	if bad := unresolvedKustomizeRefs(t, writeTreeToDisk(t, ml)); len(bad) > 0 {
		t.Errorf("unresolved kustomize references: %v", bad)
	}
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// writers runs each of the three writers against ml and returns their errors.
func writers(t *testing.T, ml *layout.ManifestLayout) map[string]error {
	t.Helper()
	var buf bytes.Buffer
	return map[string]error{
		"WriteToDisk":   ml.WriteToDisk(filepath.Join(t.TempDir(), "out")),
		"WriteToTar":    ml.WriteToTar(&buf),
		"WriteManifest": layout.WriteManifest(filepath.Join(t.TempDir(), "out"), layout.Config{}, ml),
	}
}

func cmLayout(name, ns string) *layout.ManifestLayout {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind("ConfigMap")
	obj.SetName(name)
	obj.SetNamespace("default")
	return &layout.ManifestLayout{Name: name, Namespace: ns, Resources: []client.Object{obj}}
}

func TestWriters_RefuseTwoLayoutsInOneDirectory(t *testing.T) {
	// Two sibling layouts resolving to one directory would each write a
	// kustomization.yaml there; the later one silently wins.
	parent := &layout.ManifestLayout{Name: "p", Namespace: ".", Children: []*layout.ManifestLayout{
		cmLayout("x", "p"),
		cmLayout("x", "p"),
	}}
	for name, err := range writers(t, parent) {
		if err == nil || !strings.Contains(err.Error(), "same directory") {
			t.Errorf("%s: err = %v, want a same-directory refusal", name, err)
		}
	}
}

func TestWriters_RefuseTwoLayoutsInOneDirectoryAcrossLevels(t *testing.T) {
	// The check covers the whole tree, not only siblings: grandchild y under
	// child x has Namespace "p" instead of "p/x", so it resolves to p/y, the
	// directory of p's other child y.
	grandchild := cmLayout("y", "p")
	child := cmLayout("x", "p")
	child.Children = []*layout.ManifestLayout{grandchild}
	parent := &layout.ManifestLayout{Name: "p", Namespace: ".", Children: []*layout.ManifestLayout{child, cmLayout("y", "p")}}
	for name, err := range writers(t, parent) {
		if err == nil || !strings.Contains(err.Error(), "same directory") {
			t.Errorf("%s: err = %v, want a same-directory refusal", name, err)
		}
	}
}

func TestWriters_RefuseTwoSingleFileLayoutsInOneFile(t *testing.T) {
	single := func() *layout.ManifestLayout {
		l := cmLayout("x", "p")
		l.ApplicationFileMode = layout.AppFileSingle
		return l
	}
	parent := &layout.ManifestLayout{Name: "p", Namespace: ".", Children: []*layout.ManifestLayout{single(), single()}}
	for name, err := range writers(t, parent) {
		if err == nil || !strings.Contains(err.Error(), "same file") {
			t.Errorf("%s: err = %v, want a same-file refusal", name, err)
		}
	}
}

func TestWriters_RefusalWritesNothing(t *testing.T) {
	parent := &layout.ManifestLayout{Name: "p", Namespace: ".", Children: []*layout.ManifestLayout{cmLayout("x", "p"), cmLayout("x", "p")}}
	dir := filepath.Join(t.TempDir(), "out")
	if err := parent.WriteToDisk(dir); err == nil {
		t.Fatal("WriteToDisk: want refusal")
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("refused WriteToDisk created %s (err %v)", dir, err)
	}
	var buf bytes.Buffer
	if err := parent.WriteToTar(&buf); err == nil {
		t.Fatal("WriteToTar: want refusal")
	}
	tr := tar.NewReader(&buf)
	if h, err := tr.Next(); err != io.EOF {
		t.Errorf("refused WriteToTar wrote an entry (%v, %v)", h, err)
	}
}

func TestWriters_AcceptParentPathTree(t *testing.T) {
	// The correct shape: every child's Namespace is its parent's FullRepoPath.
	p := &layout.ManifestLayout{Name: "p", Namespace: "."}
	p.Children = []*layout.ManifestLayout{cmLayout("x", p.FullRepoPath()), cmLayout("y", p.FullRepoPath())}
	for name, err := range writers(t, p) {
		if err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	if bad := unresolvedKustomizeRefs(t, writeTreeToDisk(t, p)); len(bad) > 0 {
		t.Errorf("unresolved kustomize references: %v", bad)
	}
}

func TestWalkClusterByPackage_NestedExcludedAncestors_WriteAll(t *testing.T) {
	// root -> middle -> leaf with only leaf in a package: the package walk
	// wraps each excluded ancestor in an unnamed layout, and both wrappers sat
	// at the same directory. They are coalesced into one, so the package can
	// be written (the same-directory check refused it otherwise).
	pkg := &schema.GroupVersionKind{Group: "source.toolkit.fluxcd.io", Version: "v1", Kind: "OCIRepository"}
	leaf := &stack.Node{Name: "leaf", PackageRef: pkg, Bundle: &stack.Bundle{Name: "leaf", Applications: []*stack.Application{configMapApp("a")}}}
	middle := &stack.Node{Name: "middle", Children: []*stack.Node{leaf}}
	root := &stack.Node{Name: "root", Children: []*stack.Node{middle}}
	leaf.SetParent(middle)
	middle.SetParent(root)
	layouts, err := layout.WalkClusterByPackage(&stack.Cluster{Name: "c", Node: root}, layout.LayoutRules{})
	if err != nil {
		t.Fatalf("WalkClusterByPackage: %v", err)
	}
	dir := filepath.Join(t.TempDir(), "out")
	if err := layout.WritePackagesToDisk(layouts, dir); err != nil {
		t.Fatalf("WritePackagesToDisk: %v", err)
	}
	var files []string
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".yaml") && d.Name() != "kustomization.yaml" {
			rel, _ := filepath.Rel(dir, p)
			files = append(files, rel)
		}
		return nil
	})
	if len(files) == 0 {
		t.Errorf("leaf package wrote no resource files")
	}
	if bad := unresolvedKustomizeRefs(t, dir); len(bad) > 0 {
		t.Errorf("unresolved kustomize references: %v", bad)
	}
}

func TestWriters_RefuseEquivalentSingleFilePaths(t *testing.T) {
	// "x" and "./x" name the same file <dir>/x.yaml once the path is cleaned,
	// as the writers do; the second would silently replace the first.
	single := func(name string) *layout.ManifestLayout {
		l := cmLayout(name, "p")
		l.ApplicationFileMode = layout.AppFileSingle
		return l
	}
	parent := &layout.ManifestLayout{Name: "p", Namespace: ".", Children: []*layout.ManifestLayout{single("x"), single("./x")}}
	for name, err := range writers(t, parent) {
		if err == nil || !strings.Contains(err.Error(), "same file") {
			t.Errorf("%s: err = %v, want a same-file refusal", name, err)
		}
	}
	dir := filepath.Join(t.TempDir(), "out")
	if err := parent.WriteToDisk(dir); err == nil {
		t.Fatal("WriteToDisk: want refusal")
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("refused WriteToDisk created %s (err %v)", dir, err)
	}
}
