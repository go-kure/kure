package fluxcd_test

import (
	"bytes"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// The files under testdata/unset-kustomization-name were written by the code
// as it was before Bundle.KustomizationName existed. A cluster that does not
// set the field must keep producing them byte for byte.

// unnamedCluster is platform -> {apps, ops}, and sets no KustomizationName.
// apps holds the umbrella bundle shop with children shop-db and shop-api
// (which depends on shop-db); ops holds monitoring, which depends on shop and
// names shop-db and a Kustomization outside the cluster in NamedDependsOn.
func unnamedCluster() *stack.Cluster {
	bundle := func(name string) *stack.Bundle { return srBundle(name, cmApp(name+"-app")) }
	db := bundle("shop-db")
	api := bundle("shop-api")
	api.DependsOn = []*stack.Bundle{db}
	shop := bundle("shop")
	shop.Children = []*stack.Bundle{db, api}
	monitoring := bundle("monitoring")
	monitoring.DependsOn = []*stack.Bundle{shop}
	monitoring.NamedDependsOn = []string{"shop-db", "external"}
	apps := &stack.Node{Name: "apps", Bundle: shop}
	ops := &stack.Node{Name: "ops", Bundle: monitoring}
	root := &stack.Node{Name: "platform", Bundle: bundle("platform"), Children: []*stack.Node{apps, ops}}
	apps.SetParent(root)
	ops.SetParent(root)
	return &stack.Cluster{Name: "demo", Node: root}
}

// TestKustomizationName_UnsetOutputUnchanged writes the cluster under each
// placement, and once with every node merged into one directory, and compares
// every written file with the stored tree.
func TestKustomizationName_UnsetOutputUnchanged(t *testing.T) {
	cases := map[string]layout.LayoutRules{"merged": allFlat}
	for _, placement := range allPlacements {
		rules := propertyGroupings["GroupByName"]
		rules.FluxPlacement = placement
		cases[string(placement)] = rules
	}
	for name, rules := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			if err := integrated(t, unnamedCluster(), rules).WriteToDisk(dir); err != nil {
				t.Fatalf("WriteToDisk: %v", err)
			}
			compareWithStored(t, filepath.Join("testdata", "unset-kustomization-name", name+".txt"), treeBytes(t, dir))
		})
	}
}

// treeBytes returns every file below dir as one text: a line naming the file
// by its path below dir, then its content, in path order.
func treeBytes(t *testing.T, dir string) []byte {
	t.Helper()
	var out bytes.Buffer
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(p) //nolint:gosec // a file this test wrote
		if err != nil {
			return err
		}
		fmt.Fprintf(&out, "=== %s ===\n", filepath.ToSlash(rel))
		out.Write(data)
		return nil
	})
	if err != nil {
		t.Fatalf("reading the written tree: %v", err)
	}
	if out.Len() == 0 {
		t.Fatal("nothing was written")
	}
	return out.Bytes()
}

// compareWithStored compares got with the stored file, or rewrites the file
// under -update, the flag internal/kuretest registers for golden files.
func compareWithStored(t *testing.T, stored string, got []byte) {
	t.Helper()
	if f := flag.Lookup("update"); f != nil && f.Value.String() == "true" {
		if err := os.MkdirAll(filepath.Dir(stored), 0o755); err != nil { //nolint:gosec // a test fixture directory, world-readable by design
			t.Fatalf("creating the fixture directory: %v", err)
		}
		if err := os.WriteFile(stored, got, 0o644); err != nil { //nolint:gosec // a test fixture, world-readable by design
			t.Fatalf("updating %s: %v", stored, err)
		}
		return
	}
	want, err := os.ReadFile(stored) //nolint:gosec // a test fixture named by the test
	if err != nil {
		t.Fatalf("reading the stored output (run with -update to create): %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("output differs from %s\n\ngot:\n%s\nwant:\n%s", stored, got, want)
	}
}
