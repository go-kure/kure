package argocd

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"

	// Registers -update, the flag that rewrites stored test output.
	_ "github.com/go-kure/kure/internal/kuretest"
	kureio "github.com/go-kure/kure/pkg/io"
	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// TestGenerateFromCluster_UnsetOutputUnchanged: a cluster that sets no
// Bundle.KustomizationName keeps producing the Applications stored under
// testdata, which the code wrote before the field existed, byte for byte.
func TestGenerateFromCluster_UnsetOutputUnchanged(t *testing.T) {
	db := &stack.Bundle{Name: "db"}
	web := &stack.Bundle{Name: "web", DependsOn: []*stack.Bundle{db}}
	dbn := &stack.Node{Name: "dbn", Bundle: db}
	webn := &stack.Node{Name: "webn", Bundle: web}
	r := &stack.Node{Name: "r", Children: []*stack.Node{dbn, webn}}
	dbn.SetParent(r)
	webn.SetParent(r)

	objs, err := Engine().GenerateFromCluster(&stack.Cluster{Name: "demo", Node: r}, layout.DefaultLayoutRules())
	if err != nil {
		t.Fatalf("GenerateFromCluster: %v", err)
	}
	ptrs := make([]*client.Object, len(objs))
	for i := range objs {
		ptrs[i] = &objs[i]
	}
	got, err := kureio.EncodeObjectsToYAML(ptrs)
	if err != nil {
		t.Fatalf("encoding to YAML: %v", err)
	}

	stored := filepath.Join("testdata", "unset-kustomization-name.yaml")
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
