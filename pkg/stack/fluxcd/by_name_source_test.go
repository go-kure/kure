package fluxcd_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// Tests for the source of a node's layout Kustomization when no layout at or
// above it renders a bundle (go-kure/kure#979, item 12): with a directory per
// bundle (BundleGrouping GroupByName) that is every node's.

// byNameGroupings are the rules that give every bundle a directory of its own.
var byNameGroupings = map[string]layout.LayoutRules{
	"applications by name": {BundleGrouping: layout.GroupByName, ApplicationGrouping: layout.GroupByName},
	"applications flat":    {BundleGrouping: layout.GroupByName, ApplicationGrouping: layout.GroupFlat},
}

// groupTree: platform (the bundle root builds, or none) -> apps (no bundle) ->
// shop (bundle shop, a SourceRef with a URL that generates shopSource).
func groupTree(root func() *stack.Bundle, shopSource string) func() *stack.Cluster {
	return func() *stack.Cluster {
		var rootBundle *stack.Bundle
		if root != nil {
			rootBundle = root()
		}
		apps := &stack.Node{Name: "apps", Children: []*stack.Node{{Name: "shop", Bundle: urlBundle("shop", shopSource)}}}
		return &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Bundle: rootBundle, Children: []*stack.Node{apps}}}
	}
}

// grandchildTree: platform (bundle core, Source shared) -> web (bundle web,
// Source web-src) -> empty (no bundle). Every SourceRef has a URL.
func grandchildTree() *stack.Cluster {
	web := &stack.Node{Name: "web", Bundle: urlBundle("web", "web-src"), Children: []*stack.Node{{Name: "empty"}}}
	return &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Bundle: urlBundle("core", "shared"), Children: []*stack.Node{web}}}
}

// sourcesByPath maps the spec.path of every Flux Kustomization in ml to the
// name of the Source it takes.
func sourcesByPath(ml *layout.ManifestLayout) map[string]string {
	out := map[string]string{}
	for _, k := range kustomizations(ml) {
		out[k.Spec.Path] = k.Spec.SourceRef.Name
	}
	return out
}

// checkWrittenSources holds every writer's output of ml to the Source
// invariant, and WriteToDisk's to WriteToTar's.
func checkWrittenSources(t *testing.T, ml *layout.ManifestLayout) {
	t.Helper()
	trees := writeAll(t, ml)
	for writer, w := range trees {
		checkSourceHosts(t, writer, w)
	}
	disk, tar := treeFiles(t, trees["WriteToDisk"].root), treeFiles(t, trees["WriteToTar"].root)
	if len(disk) != len(tar) {
		t.Errorf("WriteToDisk wrote %d files, WriteToTar %d", len(disk), len(tar))
	}
	for p, content := range disk {
		if other, ok := tar[p]; !ok || !bytes.Equal(content, other) {
			t.Errorf("%s differs between WriteToDisk and WriteToTar", p)
		}
	}
}

// TestPerLayout_ByName_NodeLayoutTakesItsNodesBundleSource: with a directory
// per bundle a node's layout renders no bundle and has a Kustomization of its
// own under FluxIntegratedPerLayout. Where no bundle encloses it and the
// bundles below it share no SourceRef without a URL, it takes the SourceRef of
// its own node's bundle, the one rendered one directory lower. A SourceRef
// without a URL below it is still taken first: no tree that was integrated
// before changes its source.
func TestPerLayout_ByName_NodeLayoutTakesItsNodesBundleSource(t *testing.T) {
	for _, tc := range []struct {
		name        string
		build       func() *stack.Cluster
		clusterName string
		// want maps the directory of a node's layout to its source.
		want map[string]string
	}{
		{
			name:  "a Source each",
			build: sourceHostShapes["root bundle and child node, a Source each"],
			want:  map[string]string{"platform/web": "web-src", "platform/web/web": "web-src", "platform/core": "core-src"},
		},
		{
			name:  "the node below takes its own, the node above the SourceRef without a URL",
			build: deepTree,
			want:  map[string]string{"platform/a": "flux-system", "platform/a/b": "shared", "platform/a/b/b": "shared"},
		},
		{
			name:        "below a ClusterName directory",
			build:       deepTree,
			clusterName: "prod",
			want:        map[string]string{"prod/platform": "flux-system", "prod/platform/a": "flux-system", "prod/platform/a/b": "shared"},
		},
		{
			// Integrated before this fallback: the one SourceRef without a
			// URL below node web is its source, not its own bundle's.
			name: "a SourceRef without a URL below is taken first",
			build: func() *stack.Cluster {
				ui := &stack.Node{Name: "ui", Bundle: srBundle("ui", cmApp("ui-app"))}
				web := &stack.Node{Name: "web", Bundle: urlBundle("web", "web-src"), Children: []*stack.Node{ui}}
				return &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Children: []*stack.Node{web}}}
			},
			want: map[string]string{"platform/web": "flux-system", "platform/web/web": "web-src", "platform/web/ui": "flux-system"},
		},
	} {
		for grouping, rules := range byNameGroupings {
			t.Run(tc.name+"/"+grouping, func(t *testing.T) {
				rules.FluxPlacement = layout.FluxIntegratedPerLayout
				rules.ClusterName = tc.clusterName
				ml := integrated(t, tc.build(), rules)
				got := sourcesByPath(ml)
				for dir, source := range tc.want {
					if got[dir] != source {
						t.Errorf("the Kustomization with spec.path %q takes its source from %q, want %q (all: %v)", dir, got[dir], source, got)
					}
				}
				checkWrittenSources(t, ml)
				checkIdempotent(t, tc.build(), rules)
			})
		}
	}
}

// TestPerLayout_ByName_BundleLessNodeTakesTheNearestBundleAbove: a node
// without a bundle takes the SourceRef of the bundle of the nearest node above
// it, as it does when the bundles are rendered in their nodes' directories
// (BundleGrouping GroupFlat), where that bundle encloses it.
func TestPerLayout_ByName_BundleLessNodeTakesTheNearestBundleAbove(t *testing.T) {
	for _, tc := range []struct {
		name  string
		build func() *stack.Cluster
		// want maps the directory of a node's layout to its source;
		// bundleLess are the ones among them of a node without a bundle.
		want       map[string]string
		bundleLess []string
	}{
		{
			name:       "beside the root bundle",
			build:      sourceHostShapes["root bundle beside a bundle-less child node"],
			want:       map[string]string{"platform/empty": "shared", "platform/web": "shared"},
			bundleLess: []string{"platform/empty"},
		},
		{
			name:       "the node above, not the root",
			build:      grandchildTree,
			want:       map[string]string{"platform/web": "web-src", "platform/web/empty": "web-src"},
			bundleLess: []string{"platform/web/empty"},
		},
		{
			name:       "a group node above a bundle with a URL, root bundle without one",
			build:      groupTree(func() *stack.Bundle { return srBundle("core", cmApp("core-app")) }, "shared"),
			want:       map[string]string{"platform/apps": "flux-system", "platform/apps/shop": "shared"},
			bundleLess: []string{"platform/apps"},
		},
		{
			name:       "a group node above a bundle with a URL, root bundle with one",
			build:      groupTree(func() *stack.Bundle { return urlBundle("core", "shared") }, "shop-src"),
			want:       map[string]string{"platform/apps": "shared", "platform/apps/shop": "shop-src"},
			bundleLess: []string{"platform/apps"},
		},
	} {
		for grouping, rules := range byNameGroupings {
			t.Run(tc.name+"/"+grouping, func(t *testing.T) {
				rules.FluxPlacement = layout.FluxIntegratedPerLayout
				ml := integrated(t, tc.build(), rules)
				got := sourcesByPath(ml)
				for dir, source := range tc.want {
					if got[dir] != source {
						t.Errorf("the Kustomization with spec.path %q takes its source from %q, want %q (all: %v)", dir, got[dir], source, got)
					}
				}
				flatRules := propertyGroupings["nodeOnly"]
				flatRules.FluxPlacement = layout.FluxIntegratedPerLayout
				flat := sourcesByPath(integrated(t, tc.build(), flatRules))
				for _, dir := range tc.bundleLess {
					if flat[dir] == "" || flat[dir] != got[dir] {
						t.Errorf("node layout %q takes its source from %q, and from %q with BundleGrouping GroupFlat: want the same", dir, got[dir], flat[dir])
					}
				}
				checkWrittenSources(t, ml)
				checkIdempotent(t, tc.build(), rules)
			})
		}
	}
}

// TestPerLayout_NodeLayoutWithoutASource_RefusalSaysWhatHelps: a node without
// a bundle, under a root without one, above bundles whose SourceRefs all have
// a URL has no source under either bundle grouping. The refusal names the
// node, its Kustomization and the three ways out; it does not blame a grouping
// and does not say a SourceRef is missing.
func TestPerLayout_NodeLayoutWithoutASource_RefusalSaysWhatHelps(t *testing.T) {
	groupings := map[string]layout.LayoutRules{"BundleGrouping GroupFlat": propertyGroupings["nodeOnly"]}
	for name, rules := range byNameGroupings {
		groupings["BundleGrouping GroupByName, "+name] = rules
	}
	for grouping, rules := range groupings {
		for _, name := range []string{"", "apps-group"} {
			t.Run(fmt.Sprintf("%s/KustomizationName=%q", grouping, name), func(t *testing.T) {
				rules.FluxPlacement = layout.FluxIntegratedPerLayout
				c := groupTree(nil, "shared")()
				c.Node.Children[0].KustomizationName = name
				want := name
				if want == "" {
					want = "platform-apps-node"
				}
				_, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(c, rules)
				if err == nil {
					t.Fatal("got no error, want node apps refused: it has no source")
				}
				for _, say := range []string{
					fmt.Sprintf("Flux Kustomization %q of node %q (spec.path %q) has no source", want, "platform/apps", "platform/apps"),
					"every SourceRef on the bundles below it has a URL",
					"give a bundle below it a SourceRef without a URL",
					"give a node at or above it a bundle with a SourceRef",
					"use FluxIntegratedPerBundle",
				} {
					if !strings.Contains(err.Error(), say) {
						t.Errorf("refusal %q does not say %q", err, say)
					}
				}
				for _, not := range []string{"GroupByName", "GroupFlat", "unsupported", "requires a SourceRef"} {
					if strings.Contains(err.Error(), not) {
						t.Errorf("refusal %q says %q", err, not)
					}
				}
			})
		}
	}

	// No bundle at all at, below or above the node: there a SourceRef is
	// what is missing, and the refusal says so.
	t.Run("no bundle at, below or above", func(t *testing.T) {
		rules := byNameGroupings["applications by name"]
		rules.FluxPlacement = layout.FluxIntegratedPerLayout
		c := &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Children: []*stack.Node{{Name: "empty"}}}}
		_, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(c, rules)
		if err == nil {
			t.Fatal("got no error, want node empty refused: it has no source")
		}
		for _, say := range []string{
			"FluxIntegratedPerLayout mode requires a SourceRef with Kind and Name",
			fmt.Sprintf("Flux Kustomization %q of node %q (spec.path %q) has no source", "platform-empty-node", "platform/empty", "platform/empty"),
			"no bundle at, below or above it has one",
		} {
			if !strings.Contains(err.Error(), say) {
				t.Errorf("refusal %q does not say %q", err, say)
			}
		}
	})
}
