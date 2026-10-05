package fluxcd_test

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// Tests for the directory the bootstrap points Flux at (go-kure/kure#979): the
// gotk bootstrap Kustomization's spec.path and the FluxInstance's sync.path
// name the same directory, the top directory of the tree a walk with the same
// rules writes for the same root node.

// bootstrapPaths returns the gotk bootstrap Kustomization's spec.path and the
// FluxInstance's sync.path for root under rules. gotk builds from the vendored
// bundle (GotkVersion), so neither call touches the network.
func bootstrapPaths(t *testing.T, root *stack.Node, rules layout.LayoutRules) (gotk, sync string) {
	t.Helper()
	bg := fluxstack.NewBootstrapGenerator()

	objs, err := bg.GenerateBootstrap(&stack.BootstrapConfig{
		Enabled:     true,
		FluxMode:    fluxstack.ModeGotk,
		FluxVersion: fluxstack.GotkVersion,
		SourceURL:   "oci://registry.example.com/fleet",
	}, root, rules)
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
	}, root, rules)
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
// sync.path is "./" followed by the directory. The directory is the cluster
// directory under a ClusterName, whatever the root node is; without one it is
// the root node's name, "cluster" for an unnamed root node and the root of the
// source when there is no root node.
func TestBootstrapModesNameTheSameDirectory(t *testing.T) {
	named, unnamed := &stack.Node{Name: "prod"}, &stack.Node{}
	for name, tc := range map[string]struct {
		root        *stack.Node
		clusterName string
		wantGotk    string
		wantSync    string
	}{
		"named root":   {named, "", "prod", "./prod"},
		"unnamed root": {unnamed, "", "cluster", "./cluster"},
		"no root node": {nil, "", ".", "./"},

		"named root under a ClusterName":   {named, "clusters/eu", "clusters/eu", "./clusters/eu"},
		"unnamed root under a ClusterName": {unnamed, "clusters/eu", "clusters/eu", "./clusters/eu"},
		"no root node under a ClusterName": {nil, "clusters/eu", "clusters/eu", "./clusters/eu"},

		// The root node is the cluster directory when the directory's last
		// segment is its name.
		"ClusterName is the root's name":      {named, "prod", "prod", "./prod"},
		"ClusterName ends in the root's name": {named, "clusters/prod", "clusters/prod", "./clusters/prod"},

		"ClusterName .":                {named, ".", ".", "./"},
		"ClusterName with a dot slash": {named, "./fleet", "fleet", "./fleet"},
		"ClusterName with a slash":     {named, "fleet/", "fleet", "./fleet"},
		// The writers resolve a rooted directory under the one they write to.
		"rooted ClusterName": {named, "/fleet", "fleet", "./fleet"},
	} {
		t.Run(name, func(t *testing.T) {
			gotk, sync := bootstrapPaths(t, tc.root, layout.LayoutRules{ClusterName: tc.clusterName})
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

// bootstrapConfigs are the configs the rules are checked under: the two that
// generate nothing and one per mode.
func bootstrapConfigs() map[string]*stack.BootstrapConfig {
	enabled := func(mode string) *stack.BootstrapConfig {
		return &stack.BootstrapConfig{
			Enabled:     true,
			FluxMode:    mode,
			FluxVersion: fluxstack.GotkVersion,
			Registry:    "ghcr.io/fluxcd",
			SourceURL:   "oci://registry.example.com/fleet",
		}
	}
	return map[string]*stack.BootstrapConfig{
		"nil config":    nil,
		"disabled":      {Enabled: false},
		"gotk":          enabled(fluxstack.ModeGotk),
		"flux-operator": enabled(fluxstack.DefaultFluxMode),
	}
}

// TestBootstrap_RefusesInvalidRules: the rules name the directory, so rules a
// walk refuses are refused here, by the same check and before anything else:
// whatever the config is, a nil or disabled one included, and whatever the
// root node is. Nothing is returned next to the error.
func TestBootstrap_RefusesInvalidRules(t *testing.T) {
	rules := layout.LayoutRules{ClusterName: "clusters/../prod"}
	bg := fluxstack.NewBootstrapGenerator()
	we := fluxstack.NewWorkflowEngine()
	for configName, config := range bootstrapConfigs() {
		for rootName, root := range map[string]*stack.Node{"named root": {Name: "prod"}, "unnamed root": {}, "no root node": nil} {
			check := func(t *testing.T, returned bool, err error) {
				t.Helper()
				if err == nil || !strings.Contains(err.Error(), "ClusterName") || !strings.Contains(err.Error(), rules.ClusterName) {
					t.Fatalf("err = %v, want one naming ClusterName and %q", err, rules.ClusterName)
				}
				if returned {
					t.Error("something was returned next to the error")
				}
			}
			t.Run(configName+"/"+rootName+"/GenerateBootstrap", func(t *testing.T) {
				objs, err := bg.GenerateBootstrap(config, root, rules)
				check(t, objs != nil, err)
			})
			t.Run(configName+"/"+rootName+"/GenerateFluxInstance", func(t *testing.T) {
				fi, err := bg.GenerateFluxInstance(config, root, rules)
				check(t, fi != nil, err)
			})
			t.Run(configName+"/"+rootName+"/WorkflowEngine", func(t *testing.T) {
				objs, err := we.GenerateBootstrap(config, root, rules)
				check(t, objs != nil, err)
			})
		}
	}
}

// TestWorkflowEngine_GenerateBootstrap_RefusesRulesOfAnotherType: the engine
// takes the rules through stack.Workflow as a stack.LayoutRulesProvider and
// needs a layout.LayoutRules value to name the directory. Anything else, nil
// included, is refused as GenerateFromCluster and CreateLayoutWithResources
// refuse it, not replaced by default rules: with those the bootstrap would
// point at the root node's name whatever the tree was written with. It is
// refused whatever the config is.
func TestWorkflowEngine_GenerateBootstrap_RefusesRulesOfAnotherType(t *testing.T) {
	we := fluxstack.NewWorkflowEngine()
	for rulesName, rules := range map[string]stack.LayoutRulesProvider{"nil": nil, "another type": notLayoutRules{}} {
		for configName, config := range bootstrapConfigs() {
			t.Run(rulesName+"/"+configName, func(t *testing.T) {
				objs, err := we.GenerateBootstrap(config, &stack.Node{Name: "prod"}, rules)
				if err == nil || !strings.Contains(err.Error(), "rules must be of type layout.LayoutRules") {
					t.Fatalf("err = %v, want the rules refused by type", err)
				}
				if objs != nil {
					t.Errorf("got %d objects next to the error", len(objs))
				}
			})
		}
	}
}

// TestBootstrap_RootNodeNameIsNoSegmentUnderClusterName: the root node's name
// is checked as a directory name where it becomes a path segment, and under a
// ClusterName it becomes none: the path is the cluster directory. A name that
// is refused without a ClusterName generates with one, and the path does not
// hold it. In gotk mode the name still names the bootstrap source, so that
// check keeps refusing it.
func TestBootstrap_RootNodeNameIsNoSegmentUnderClusterName(t *testing.T) {
	root := &stack.Node{Name: "../outside"}
	rules := layout.LayoutRules{ClusterName: "clusters/eu"}
	bg := fluxstack.NewBootstrapGenerator()
	config := bootstrapConfigs()

	if _, err := bg.GenerateFluxInstance(config["flux-operator"], root, layout.LayoutRules{}); err == nil || !strings.Contains(err.Error(), "path separator") {
		t.Fatalf("without a ClusterName: err = %v, want the name refused as a directory name", err)
	}
	fi, err := bg.GenerateFluxInstance(config["flux-operator"], root, rules)
	if err != nil {
		t.Fatalf("GenerateFluxInstance under a ClusterName: %v", err)
	}
	if fi.Spec.Sync == nil || fi.Spec.Sync.Path != "./clusters/eu" {
		t.Errorf("FluxInstance sync = %+v, want the path ./clusters/eu", fi.Spec.Sync)
	}
	objs, err := bg.GenerateBootstrap(config["flux-operator"], root, rules)
	if err != nil || len(objs) == 0 {
		t.Errorf("GenerateBootstrap, flux-operator mode, under a ClusterName: %d objects, %v; want the bootstrap", len(objs), err)
	}

	objs, err = bg.GenerateBootstrap(config["gotk"], root, rules)
	if err == nil || !strings.Contains(err.Error(), "not a valid name for the bootstrap source") {
		t.Fatalf("gotk mode under a ClusterName: err = %v, want the name refused as a source name", err)
	}
	if objs != nil {
		t.Errorf("got %d objects next to the error", len(objs))
	}
}

// TestBootstrapPathIsTheTopOfTheWrittenTree renders the shapes of the Source
// invariant's matrix (sourceHostShapes) under every placement, grouping and
// ClusterName and holds the bootstrap to the walk: layout.TopDirectory is the
// directory of the tree the walk returns, with and without the integration,
// and in every writer's output the directory each bootstrap mode names is the
// one that holds the tree's first kustomization.yaml. Which combinations the
// integration refuses is pinned (sourceHostRefusal): any other error fails,
// and of a refused combination the walked tree is written instead.
func TestBootstrapPathIsTheTopOfTheWrittenTree(t *testing.T) {
	clusterNames := []string{"", ".", "prod", "env/prod", "platform", "/rooted"}
	shapes := make([]string, 0, len(sourceHostShapes))
	for s := range sourceHostShapes {
		shapes = append(shapes, s)
	}
	slices.Sort(shapes)
	integratedTrees := map[string]int{}
	for _, placement := range placements {
		for _, grouping := range []string{"nodeOnly", "GroupByName", "nodeFlat"} {
			for _, clusterName := range clusterNames {
				for _, shape := range shapes {
					t.Run(fmt.Sprintf("%s/%s/ClusterName=%q/%s", placement, grouping, clusterName, shape), func(t *testing.T) {
						rules := propertyGroupings[grouping]
						rules.FluxPlacement = placement
						rules.ClusterName = clusterName
						c := sourceHostShapes[shape]()

						top, err := layout.TopDirectory(c.Node, rules)
						if err != nil {
							t.Fatalf("TopDirectory: %v", err)
						}
						walked, err := layout.WalkCluster(c, rules)
						if err != nil {
							t.Fatalf("WalkCluster: %v", err)
						}
						if got := walked.FullRepoPath(); got != top {
							t.Fatalf("the walk's top is %q, TopDirectory says %q", got, top)
						}

						gotk, sync := bootstrapPaths(t, c.Node, rules)
						if want := strings.TrimLeft(top, "/"); path.Clean(gotk) != path.Clean(want) || path.Clean(sync) != path.Clean(want) {
							t.Fatalf("gotk spec.path %q and FluxInstance sync.path %q, want the directory %q", gotk, sync, want)
						}

						refusal := sourceHostRefusal(placement, grouping, clusterName, shape)
						ml, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(sourceHostShapes[shape](), rules)
						switch {
						case refusal == "":
							if err != nil {
								t.Fatalf("refused, want the tree integrated: %v", err)
							}
							if got := ml.FullRepoPath(); got != top {
								t.Fatalf("the integrated tree's top is %q, TopDirectory says %q", got, top)
							}
							integratedTrees[fmt.Sprintf("%s under ClusterName %q", placement, clusterName)]++
						case err == nil || !strings.Contains(err.Error(), refusal):
							t.Fatalf("got %v, want a refusal that says %q", err, refusal)
						default:
							ml = walked
						}
						for writer, tree := range writeAll(t, ml) {
							for mode, p := range map[string]string{"gotk spec.path": gotk, "FluxInstance sync.path": sync} {
								file := filepath.Join(tree.root, filepath.FromSlash(p), "kustomization.yaml")
								if _, err := os.Stat(file); err != nil {
									t.Errorf("%s: %s %q names no directory with a kustomization.yaml: %v", writer, mode, p, err)
								}
							}
						}
					})
				}
			}
		}
	}
	for _, placement := range placements {
		for _, clusterName := range clusterNames {
			if key := fmt.Sprintf("%s under ClusterName %q", placement, clusterName); integratedTrees[key] == 0 {
				t.Errorf("%s: no shape was integrated, so the bootstrap path was checked on no integrated tree", key)
			}
		}
	}
}
