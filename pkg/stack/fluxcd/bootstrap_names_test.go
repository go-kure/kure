package fluxcd_test

import (
	"path"
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

	// The accepted name is the last path segment, unchanged. What precedes it
	// is the bootstrap path's own rule, tested with that rule.
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
	const want = "prod.eu"
	if len(paths) != 1 || path.Base(paths[0]) != want {
		t.Fatalf("bootstrap Kustomization spec.path = %v, want one path whose last segment is %s", paths, want)
	}
}

// TestBootstrap_RootNodeName_NoSync: the name is checked only where a path is
// built from it. Without a SourceURL the FluxInstance carries no sync, so the
// root name is not used and any name generates. The gotk Kustomization's
// spec.path is built with or without a SourceURL, so that mode still refuses.
func TestBootstrap_RootNodeName_NoSync(t *testing.T) {
	config := func(mode string) *stack.BootstrapConfig {
		return &stack.BootstrapConfig{
			Enabled:     true,
			FluxMode:    mode,
			FluxVersion: "v2.4.0",
			Registry:    "registry.example.com",
		}
	}
	root := &stack.Node{Name: "../outside"}

	fi, err := fluxstack.NewBootstrapGenerator().GenerateFluxInstance(config(fluxstack.DefaultFluxMode), root)
	if err != nil {
		t.Fatalf("GenerateFluxInstance without a source: got %v, want nil", err)
	}
	if fi == nil || fi.Spec.Sync != nil {
		t.Fatalf("GenerateFluxInstance without a source: got %+v, want a FluxInstance without sync", fi)
	}

	objs, err := fluxstack.NewBootstrapGenerator().GenerateBootstrap(config(fluxstack.DefaultFluxMode), root)
	if err != nil {
		t.Fatalf("GenerateBootstrap, flux-operator mode without a source: got %v, want nil", err)
	}
	if len(objs) == 0 {
		t.Fatal("GenerateBootstrap, flux-operator mode without a source: got no objects")
	}

	objs, err = fluxstack.NewBootstrapGenerator().GenerateBootstrap(config(fluxstack.ModeGotk), root)
	if err == nil || !strings.Contains(err.Error(), "path separator") {
		t.Fatalf("GenerateBootstrap, gotk mode without a source: got %v, want the name refused", err)
	}
	if objs != nil {
		t.Fatalf("got %d objects next to the error", len(objs))
	}
}

// TestBootstrap_RootNodeName_NilConfig: a nil config still means "nothing to
// generate", whatever the root is named. The name check runs before the nil
// check in GenerateFluxInstance, so it must accept a nil config itself.
func TestBootstrap_RootNodeName_NilConfig(t *testing.T) {
	for _, root := range []*stack.Node{nil, {}, {Name: "production"}, {Name: "../outside"}} {
		fi, err := fluxstack.NewBootstrapGenerator().GenerateFluxInstance(nil, root)
		if err != nil || fi != nil {
			t.Fatalf("GenerateFluxInstance(nil, %+v) = %v, %v; want nil, nil", root, fi, err)
		}
		objs, err := fluxstack.NewBootstrapGenerator().GenerateBootstrap(nil, root)
		if err != nil || objs != nil {
			t.Fatalf("GenerateBootstrap(nil, %+v) = %v, %v; want nil, nil", root, objs, err)
		}
	}
}

// TestBootstrap_RootNodeName_SourceName: in gotk mode a named root also names
// the generated GitRepository or OCIRepository and the bootstrap
// Kustomization's sourceRef, so it has to be a DNS-1123 subdomain as well as
// a directory name. It is refused with or without a SourceURL: the sourceRef
// is written either way. Flux-operator mode uses the name only in sync.path
// and keeps accepting it.
func TestBootstrap_RootNodeName_SourceName(t *testing.T) {
	config := func(mode, url string) *stack.BootstrapConfig {
		return &stack.BootstrapConfig{
			Enabled:     true,
			FluxMode:    mode,
			FluxVersion: "v2.4.0",
			Registry:    "registry.example.com",
			SourceURL:   url,
			SourceRef:   "main",
			SourceKind:  "GitRepository",
		}
	}
	const url = "https://github.com/example/fleet.git"
	for _, name := range []string{"Prod", "prod_root"} {
		root := &stack.Node{Name: name}
		for _, u := range []string{url, ""} {
			objs, err := fluxstack.NewBootstrapGenerator().GenerateBootstrap(config(fluxstack.ModeGotk, u), root)
			if err == nil {
				t.Fatalf("gotk mode, root %q, SourceURL %q: generated, want the name refused", name, u)
			}
			if objs != nil {
				t.Errorf("gotk mode, root %q: got %d objects next to the error", name, len(objs))
			}
			for _, want := range []string{"'" + name + "'", "not a valid name for the bootstrap source"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("gotk mode, root %q: got %v, want it to contain %q", name, err, want)
				}
			}
		}

		objs, err := fluxstack.NewBootstrapGenerator().GenerateBootstrap(config(fluxstack.DefaultFluxMode, url), root)
		if err != nil || len(objs) == 0 {
			t.Fatalf("flux-operator mode, root %q: got %d objects, %v; want the bootstrap", name, len(objs), err)
		}
		fi, err := fluxstack.NewBootstrapGenerator().GenerateFluxInstance(config(fluxstack.DefaultFluxMode, url), root)
		if err != nil || fi == nil || fi.Spec.Sync == nil || path.Base(fi.Spec.Sync.Path) != name {
			t.Fatalf("GenerateFluxInstance, root %q: got %+v, %v; want a sync path ending in the name", name, fi, err)
		}
	}
}
