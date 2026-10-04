package fluxcd_test

import (
	"strings"
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
)

// TestBootstrap_RootNodeName: the bootstrap generator takes a root node, not a
// cluster, so stack.ValidateCluster has not seen its name. The name becomes a
// path segment of the gotk Kustomization's spec.path and of the FluxInstance's
// sync.path, so every entry point checks it as a directory name. An unnamed
// root and no root at all stay valid.
func TestBootstrap_RootNodeName(t *testing.T) {
	config := func(mode string) *stack.BootstrapConfig {
		return &stack.BootstrapConfig{
			Enabled:     true,
			FluxMode:    mode,
			FluxVersion: "v2.4.0",
			Registry:    "registry.example.com",
			SourceURL:   "https://github.com/example/fleet.git",
			SourceRef:   "main",
			SourceKind:  "GitRepository",
		}
	}
	tests := []struct {
		name    string
		root    *stack.Node
		wantErr string // substring of the error; "" means generated
	}{
		{"named root", &stack.Node{Name: "production"}, ""},
		{"dotted root", &stack.Node{Name: "prod.eu"}, ""},
		{"unnamed root", &stack.Node{}, ""},
		{"no root", nil, ""},
		{"parent traversal", &stack.Node{Name: "../outside"}, "path separator"},
		{"backslash traversal", &stack.Node{Name: `..\outside`}, "path separator"},
		{"nested", &stack.Node{Name: "clusters/prod"}, "path separator"},
		{"parent directory", &stack.Node{Name: ".."}, `".."`},
	}
	check := func(t *testing.T, err error, wantErr, rootName string) {
		t.Helper()
		if wantErr == "" {
			if err != nil {
				t.Fatalf("got %v, want nil", err)
			}
			return
		}
		if err == nil {
			t.Fatal("got nil, want an error")
		}
		if !strings.Contains(err.Error(), wantErr) || !strings.Contains(err.Error(), "'"+rootName+"'") {
			t.Fatalf("got %v, want it to name the node and contain %q", err, wantErr)
		}
	}
	for _, tt := range tests {
		name := ""
		if tt.root != nil {
			name = tt.root.Name
		}
		for _, mode := range []string{fluxstack.ModeGotk, fluxstack.DefaultFluxMode} {
			t.Run(tt.name+"/GenerateBootstrap/"+mode, func(t *testing.T) {
				objs, err := fluxstack.NewBootstrapGenerator().GenerateBootstrap(config(mode), tt.root)
				check(t, err, tt.wantErr, name)
				if tt.wantErr != "" && objs != nil {
					t.Fatalf("got %d objects next to the error", len(objs))
				}
			})
		}
		t.Run(tt.name+"/GenerateFluxInstance", func(t *testing.T) {
			fi, err := fluxstack.NewBootstrapGenerator().GenerateFluxInstance(config(fluxstack.DefaultFluxMode), tt.root)
			check(t, err, tt.wantErr, name)
			if tt.wantErr != "" && fi != nil {
				t.Fatal("got a FluxInstance next to the error")
			}
		})
	}

	// The accepted name is the path segment, unchanged.
	objs, err := fluxstack.NewBootstrapGenerator().GenerateBootstrap(config(fluxstack.ModeGotk), &stack.Node{Name: "prod.eu"})
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, o := range objs {
		if k, ok := o.(*kustv1.Kustomization); ok {
			paths = append(paths, k.Spec.Path)
		}
	}
	want := fluxstack.DefaultBootstrapPathRoot + "/prod.eu"
	if len(paths) != 1 || paths[0] != want {
		t.Fatalf("bootstrap Kustomization spec.path = %v, want [%s]", paths, want)
	}
}
