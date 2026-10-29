package fluxcd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
)

// TestTreeBuilds_SpecPathStaysInTheTree: build writes the kustomization.yaml
// Flux's generator makes into spec.path's directory, so a spec.path that
// leaves the tree is refused before anything is written, whatever its caller
// checked.
func TestTreeBuilds_SpecPathStaysInTheTree(t *testing.T) {
	parent := t.TempDir()
	tree := filepath.Join(parent, "tree")
	if err := os.Mkdir(tree, 0o750); err != nil {
		t.Fatal(err)
	}
	b := &treeBuilds{dir: tree, written: true}
	for _, p := range []string{"..", "../outside", "./../outside", "demo/../../outside"} {
		t.Run(p, func(t *testing.T) {
			k := &kustv1.Kustomization{}
			k.Name, k.Namespace = "escape", "flux-system"
			k.Spec.Path = p
			_, _, err := b.build(k)
			if err == nil || !strings.Contains(err.Error(), "is outside the tree") {
				t.Fatalf("got %v, want spec.path %q refused as outside the tree", err, p)
			}
			for _, f := range []string{"kustomization.yaml", filepath.Join("outside", "kustomization.yaml")} {
				if _, err := os.Stat(filepath.Join(parent, f)); !os.IsNotExist(err) {
					t.Errorf("%s exists beside the tree (stat: %v)", f, err)
				}
			}
		})
	}
}
