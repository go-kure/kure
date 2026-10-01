package kuretest

import (
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"

	"github.com/go-kure/kure/internal/gotk"
	"github.com/go-kure/kure/pkg/kubernetes"
)

// moduleDir holds a resolved module to the registry's pin: the version must
// be the one the kinds table was generated from, and the directory must be
// that version's. A replace directive keeps the version and swaps the
// directory, so one that changes path or version is refused too.
func TestModuleDirHoldsTheDirectoryToThePin(t *testing.T) {
	pin := kubernetes.KindInfo{ImportPath: "github.com/backube/volsync/api/v1alpha1", Module: "github.com/backube/volsync", ModuleVersion: "v0.16.0"}
	for name, c := range map[string]struct {
		m    *packages.Module
		want string // "" for the directory
	}{
		"pinned":              {&packages.Module{Path: pin.Module, Version: pin.ModuleVersion, Dir: "/cache/volsync@v0.16.0"}, ""},
		"no module":           {nil, "GOWORK=off"},
		"no directory":        {&packages.Module{Path: pin.Module, Version: pin.ModuleVersion}, "GOWORK=off"},
		"another version":     {&packages.Module{Path: pin.Module, Version: "v0.17.0", Dir: "/cache/volsync@v0.17.0"}, "generated from github.com/backube/volsync v0.16.0"},
		"replaced by a path":  {&packages.Module{Path: pin.Module, Version: pin.ModuleVersion, Dir: "/src/volsync", Replace: &packages.Module{Path: "../volsync", Dir: "/src/volsync"}}, "replaced by ../volsync"},
		"replaced by a fork":  {&packages.Module{Path: pin.Module, Version: pin.ModuleVersion, Dir: "/cache/fork@v0.16.0", Replace: &packages.Module{Path: "example.com/fork", Version: "v0.16.0", Dir: "/cache/fork@v0.16.0"}}, "replaced by example.com/fork v0.16.0"},
		"replaced by itself":  {&packages.Module{Path: pin.Module, Version: pin.ModuleVersion, Dir: "/cache/volsync@v0.16.0", Replace: &packages.Module{Path: pin.Module, Version: pin.ModuleVersion, Dir: "/cache/volsync@v0.16.0"}}, ""},
		"replaced by a newer": {&packages.Module{Path: pin.Module, Version: pin.ModuleVersion, Dir: "/cache/volsync@v0.17.0", Replace: &packages.Module{Path: pin.Module, Version: "v0.17.0", Dir: "/cache/volsync@v0.17.0"}}, "replaced by github.com/backube/volsync v0.17.0"},
	} {
		t.Run(name, func(t *testing.T) {
			dir, err := moduleDir(pin.ImportPath, c.m, pin)
			switch {
			case c.want == "" && (err != nil || dir != c.m.Dir):
				t.Errorf("moduleDir = %q, %v; want %q", dir, err, c.m.Dir)
			case c.want != "" && (err == nil || !strings.Contains(err.Error(), c.want)):
				t.Errorf("moduleDir = %q, %v; want an error naming %q", dir, err, c.want)
			}
		})
	}
}

// ciliumNetworkPolicyCRD is the smallest definition of a kind another module
// already defines.
const ciliumNetworkPolicyCRD = `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: ciliumnetworkpolicies.cilium.io
spec:
  group: cilium.io
  names:
    kind: CiliumNetworkPolicy
    plural: ciliumnetworkpolicies
  scope: Namespaced
  versions:
  - name: v2
    served: true
    storage: true
    schema:
      openAPIV3Schema:
        type: object
`

// build refuses what it cannot read in full: a module directory with no
// definitions, one it cannot decode, a kind two modules define, and a bundle
// that is not an archive. A validator built from less than the whole table
// would pass what it never read.
func TestBuildRefusesAnIncompleteRead(t *testing.T) {
	dirs, err := moduleDirs()
	if err != nil {
		t.Fatal(err)
	}
	// the module whose directory each case replaces, and the subdirectory
	// the table reads its definitions from
	const module = "github.com/backube/volsync"
	with := func(root string) map[string]string {
		out := maps.Clone(dirs)
		out[module] = root
		return out
	}
	moduleRoot := func(t *testing.T, name, body string) string {
		t.Helper()
		root := t.TempDir()
		dir := filepath.Join(root, modules[module].Dir)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if name != "" {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return root
	}
	for name, c := range map[string]struct{ root, want string }{
		"no definitions":                    {moduleRoot(t, "", ""), "ships no definitions"},
		"a definition that does not decode": {moduleRoot(t, "broken.yaml", "apiVersion: apiextensions.k8s.io/v1\nkind: CustomResourceDefinition\nspec: [not a mapping\n"), "broken.yaml"},
		"a kind another module defines":     {moduleRoot(t, "copy.yaml", ciliumNetworkPolicyCRD), "defined twice"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := build(with(c.root), gotk.Open())
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("build: %v, want an error naming %q", err, c.want)
			}
		})
	}
	t.Run("a bundle that is not an archive", func(t *testing.T) {
		_, err := build(dirs, strings.NewReader("not a gzip"))
		if err == nil || !strings.Contains(err.Error(), "the Flux bundle") {
			t.Errorf("build: %v, want the bundle named", err)
		}
	})
}
