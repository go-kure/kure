package fluxcd_test

import (
	"os"
	"path"
	"path/filepath"
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// Tests for the directory the bootstrap points Flux at (go-kure/kure#979): the
// gotk bootstrap Kustomization's spec.path and the FluxInstance's sync.path
// name the same directory for the same root node, the one a walk without a
// ClusterName writes a named root node to.

// bootstrapPaths returns the gotk bootstrap Kustomization's spec.path and the
// FluxInstance's sync.path for root. gotk builds from the vendored bundle
// (GotkVersion), so neither call touches the network.
func bootstrapPaths(t *testing.T, root *stack.Node) (gotk, sync string) {
	t.Helper()
	bg := fluxstack.NewBootstrapGenerator()

	objs, err := bg.GenerateBootstrap(&stack.BootstrapConfig{
		Enabled:     true,
		FluxMode:    fluxstack.ModeGotk,
		FluxVersion: fluxstack.GotkVersion,
		SourceURL:   "oci://registry.example.com/fleet",
	}, root)
	if err != nil {
		t.Fatalf("GenerateBootstrap (gotk): %v", err)
	}
	var kust *kustv1.Kustomization
	for _, obj := range objs {
		if k, ok := obj.(*kustv1.Kustomization); ok {
			if kust != nil {
				t.Fatalf("gotk bootstrap emitted two Kustomizations: %q and %q", kust.Name, k.Name)
			}
			kust = k
		}
	}
	if kust == nil {
		t.Fatal("gotk bootstrap emitted no Kustomization")
	}

	fi, err := bg.GenerateFluxInstance(&stack.BootstrapConfig{
		Enabled:     true,
		FluxVersion: "v2.8.2",
		Registry:    "ghcr.io/fluxcd",
		SourceURL:   "oci://registry.example.com/fleet",
	}, root)
	if err != nil {
		t.Fatalf("GenerateFluxInstance: %v", err)
	}
	if fi.Spec.Sync == nil {
		t.Fatal("FluxInstance has no sync block")
	}
	return kust.Spec.Path, fi.Spec.Sync.Path
}

// TestBootstrapModesNameTheSameDirectory pins both spellings and that they are
// one directory: the gotk Kustomization's spec.path is the directory itself,
// with no leading "./" and "." for the root of the source (as FullRepoPath
// spells every other spec.path the package writes); the FluxInstance's
// sync.path is "./" followed by the directory.
func TestBootstrapModesNameTheSameDirectory(t *testing.T) {
	for name, tc := range map[string]struct {
		root     *stack.Node
		wantGotk string
		wantSync string
	}{
		"named root":   {&stack.Node{Name: "prod"}, "prod", "./prod"},
		"unnamed root": {&stack.Node{}, ".", "./"},
		"no root node": {nil, ".", "./"},
	} {
		t.Run(name, func(t *testing.T) {
			gotk, sync := bootstrapPaths(t, tc.root)
			if gotk != tc.wantGotk {
				t.Errorf("gotk spec.path = %q, want %q", gotk, tc.wantGotk)
			}
			if sync != tc.wantSync {
				t.Errorf("FluxInstance sync.path = %q, want %q", sync, tc.wantSync)
			}
			if g, s := path.Clean(gotk), path.Clean(sync); g != s {
				t.Errorf("the two modes name different directories: gotk %q, FluxInstance %q", g, s)
			}
		})
	}
}

// TestBootstrapPathIsTheWalkedRootDirectory renders a cluster with a named
// root and no ClusterName and checks, in every writer's output, that the
// directory each bootstrap mode names is the one that holds the root's
// kustomization.yaml. An unnamed root, or a walk with a ClusterName, is not
// covered: the bootstrap is given neither the layout rules nor the tree.
func TestBootstrapPathIsTheWalkedRootDirectory(t *testing.T) {
	web := &stack.Node{Name: "web", Bundle: srBundle("web", cmApp("web-app"))}
	root := &stack.Node{Name: "prod", Bundle: srBundle("prod", cmApp("prod-app")), Children: []*stack.Node{web}}
	c := &stack.Cluster{Name: "demo", Node: root}

	ml, err := layout.WalkCluster(c, layout.DefaultLayoutRules())
	if err != nil {
		t.Fatalf("WalkCluster: %v", err)
	}
	if got := ml.FullRepoPath(); got != "prod" {
		t.Fatalf("walked root is at %q, want %q", got, "prod")
	}

	gotk, sync := bootstrapPaths(t, root)
	for writer, tree := range writeAll(t, ml) {
		for mode, p := range map[string]string{"gotk spec.path": gotk, "FluxInstance sync.path": sync} {
			file := filepath.Join(tree.root, filepath.FromSlash(p), "kustomization.yaml")
			if _, err := os.Stat(file); err != nil {
				t.Errorf("%s: %s %q names no directory with a kustomization.yaml: %v", writer, mode, p, err)
			}
		}
	}
}
