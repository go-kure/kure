package fluxcd_test

import (
	"strings"
	"testing"
	"time"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// A non-empty duration that does not parse is an error on every generation
// path, never a silent fallback to the default (go-kure/kure#762). Empty keeps meaning
// "use the default" for Interval and "leave unset" for Timeout/RetryInterval.

var invalidDurations = []string{"5 minutes", "5min", "5"}

func bundleWith(field, value string) *stack.Bundle {
	b := &stack.Bundle{Name: "b"}
	switch field {
	case "interval":
		b.Interval = value
	case "timeout":
		b.Timeout = value
	case "retryInterval":
		b.RetryInterval = value
	}
	return b
}

func TestGenerateForBundle_RejectsInvalidDurations(t *testing.T) {
	gen := fluxstack.NewResourceGenerator()
	for _, field := range []string{"interval", "timeout", "retryInterval"} {
		for _, v := range invalidDurations {
			t.Run(field+"="+v, func(t *testing.T) {
				_, err := gen.GenerateForBundle(bundleWith(field, v), "b")
				if err == nil {
					t.Fatalf("expected an error for %s %q", field, v)
				}
				if !strings.Contains(err.Error(), field) || !strings.Contains(err.Error(), v) {
					t.Errorf("error %q does not name the field and the rejected value", err)
				}
			})
		}
	}
}

func TestGenerateForBundle_EmptyDurationsKeepDefaults(t *testing.T) {
	gen := fluxstack.NewResourceGenerator()
	objs, err := gen.GenerateForBundle(&stack.Bundle{Name: "b"}, "b")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	k := objs[0].(*kustv1.Kustomization)
	if k.Spec.Interval.Duration != gen.DefaultInterval {
		t.Errorf("Interval = %v, want DefaultInterval %v", k.Spec.Interval.Duration, gen.DefaultInterval)
	}
	if k.Spec.Timeout != nil || k.Spec.RetryInterval != nil {
		t.Errorf("empty Timeout/RetryInterval must stay unset, got %v / %v", k.Spec.Timeout, k.Spec.RetryInterval)
	}
}

func TestGenerateFromLayout_RejectsInvalidDurationOnNestedUmbrellaChild(t *testing.T) {
	// GenerateFromLayout does not run ValidateCluster; the generator itself
	// must refuse the value on a grandchild of the umbrella. Walk a valid
	// cluster, then corrupt the grandchild.
	grandchild := &stack.Bundle{Name: "gc"}
	child := &stack.Bundle{Name: "c", Children: []*stack.Bundle{grandchild}}
	umbrella := &stack.Bundle{Name: "u", Children: []*stack.Bundle{child}}
	c := &stack.Cluster{Name: "c", Node: &stack.Node{Name: "n", Bundle: umbrella}}
	ml, err := layout.WalkCluster(c, layout.DefaultLayoutRules())
	if err != nil {
		t.Fatalf("WalkCluster: %v", err)
	}
	grandchild.Timeout = "5min"
	_, err = fluxstack.NewResourceGenerator().GenerateFromLayout(ml, c)
	if err == nil || !strings.Contains(err.Error(), `"5min"`) {
		t.Fatalf("expected an error naming the rejected timeout, got %v", err)
	}
}

func TestGenerateFromCluster_RejectsInvalidInterval(t *testing.T) {
	c := &stack.Cluster{Name: "c", Node: &stack.Node{Name: "n", Bundle: &stack.Bundle{Name: "b", Interval: "5"}}}
	if _, err := fluxstack.NewResourceGenerator().GenerateFromCluster(c, layout.DefaultLayoutRules()); err == nil {
		t.Fatal("expected an error for an unparsable interval")
	}
}

// Every duration the Flux workflow writes into an object it generates is held
// to the pattern the Flux API holds it to, where that object is created and
// in the form it is written in (go-kure/kure#1015): a bundle's interval,
// timeout and retryInterval on the bundle's Kustomization, the timeout and
// retryInterval in effect on a per-layout one, and the generator's
// DefaultInterval wherever it is written, a generated source and the
// FluxInstance sync included.
// Bundle.Validate and stack.ValidateCluster take the same values as before
// (TestBundleValidate_Durations), and so does the Argo CD workflow.

// fluxDurationCases are the values of those tests. refused is the text of the
// refusal, empty for a value that is taken, which is then written as want.
var fluxDurationCases = []struct {
	value   string
	refused string
	want    time.Duration
}{
	{value: "-1s", refused: `"-1s" is written as "-1s", which the Flux API does not take`},
	{value: "1us", refused: `"1us" is written as "1µs", which the Flux API does not take`},
	// time.ParseDuration takes no day unit, so "1d" is refused as before, as
	// a value that is no duration, and never reaches Flux's pattern.
	{value: "1d", refused: `"1d" is not a valid duration`},
	{value: "1h30m", want: 90 * time.Minute},
	{value: "1ms", want: time.Millisecond},
}

// kustomizationDuration returns the duration field of k, nil when it is unset.
func kustomizationDuration(t *testing.T, k *kustv1.Kustomization, field string) *time.Duration {
	t.Helper()
	switch field {
	case "interval":
		return &k.Spec.Interval.Duration
	case "timeout":
		if k.Spec.Timeout != nil {
			return &k.Spec.Timeout.Duration
		}
	case "retryInterval":
		if k.Spec.RetryInterval != nil {
			return &k.Spec.RetryInterval.Duration
		}
	default:
		t.Fatalf("no duration field %q", field)
	}
	return nil
}

// TestFluxDurations_Bundle: a bundle's own interval, timeout and
// retryInterval, where its Kustomization is created.
func TestFluxDurations_Bundle(t *testing.T) {
	for _, field := range []string{"interval", "timeout", "retryInterval"} {
		for _, tc := range fluxDurationCases {
			t.Run(field+"="+tc.value, func(t *testing.T) {
				objs, err := fluxstack.NewResourceGenerator().GenerateForBundle(bundleWith(field, tc.value), "b")
				if tc.refused != "" {
					mustContainAll(t, err, "Bundle", "'b'", field+" "+tc.refused)
					return
				}
				if err != nil {
					t.Fatalf("refused: %v", err)
				}
				got := kustomizationDuration(t, objs[0].(*kustv1.Kustomization), field)
				if got == nil || *got != tc.want {
					t.Errorf("%s = %v, want %v", field, got, tc.want)
				}
			})
		}
	}
}

// TestFluxDurations_BundleByPath: the refusal names the bundle by its path,
// which for an umbrella child of a walked cluster holds its umbrella's, on
// every generation path. Bundle.Validate takes the value, so the generator's
// check is the only one.
func TestFluxDurations_BundleByPath(t *testing.T) {
	build := func() *stack.Cluster {
		child := srBundle("infra", cmApp("infra-app"))
		child.RetryInterval = "-2m"
		umbrella := srBundle("shop", cmApp("shop-app"))
		umbrella.Children = []*stack.Bundle{child}
		return oneBundleCluster(umbrella)
	}
	wants := []string{"Bundle", "'shop/infra'", `retryInterval "-2m" is written as "-2m0s", which the Flux API does not take`}

	if err := stack.ValidateCluster(build()); err != nil {
		t.Fatalf("ValidateCluster refuses a duration only Flux does not take: %v", err)
	}
	t.Run("GenerateFromCluster", func(t *testing.T) {
		_, err := fluxstack.NewResourceGenerator().GenerateFromCluster(build(), layout.DefaultLayoutRules())
		mustContainAll(t, err, wants...)
	})
	t.Run("GenerateFromLayout", func(t *testing.T) {
		c := build()
		_, err := fluxstack.NewResourceGenerator().GenerateFromLayout(mustWalk(t, c, layout.DefaultLayoutRules()), c)
		mustContainAll(t, err, wants...)
	})
	for _, placement := range []layout.FluxPlacement{layout.FluxIntegratedPerBundle, layout.FluxIntegratedPerLayout, layout.FluxSeparate} {
		t.Run("CreateLayoutWithResources/"+string(placement), func(t *testing.T) {
			rules := layout.DefaultLayoutRules()
			rules.FluxPlacement = placement
			_, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(build(), rules)
			mustContainAll(t, err, wants...)
		})
	}
}

// TestFluxDurations_Layout: the interval, timeout and retryInterval a layout
// sets itself, where its per-layout Kustomization is created. The refusal
// names the layout by its directory and says nothing of an inherited value.
func TestFluxDurations_Layout(t *testing.T) {
	for _, field := range []string{"interval", "timeout", "retryInterval"} {
		for _, tc := range fluxDurationCases {
			t.Run(field+"="+tc.value, func(t *testing.T) {
				b := shopSettings(func(_, _, main, _ *layout.ManifestLayout) {
					switch field {
					case "interval":
						main.Interval = tc.value
					case "timeout":
						main.Timeout = tc.value
					default:
						main.RetryInterval = tc.value
					}
				})
				ml, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(oneBundleCluster(b), perLayoutRules())
				if tc.refused != "" {
					mustContainAll(t, err, "ManifestLayout", "'prod/shop/db/01-main'", field+" "+tc.refused)
					if strings.Contains(err.Error(), "inherited") {
						t.Errorf("the refusal says \"inherited\" of a value the layout sets itself: %v", err)
					}
					return
				}
				if err != nil {
					t.Fatalf("refused: %v", err)
				}
				got := kustomizationDuration(t, mustKustomization(t, ml, "shop-01-main"), field)
				if got == nil || *got != tc.want {
					t.Errorf("%s = %v, want %v", field, got, tc.want)
				}
			})
		}
	}
}

// TestFluxDurations_InheritedByALayout: the interval, timeout and
// retryInterval a layout inherits from the bundle that holds its application.
// A value Flux takes reaches every per-layout Kustomization. One it does not
// take is refused on the bundle's own Kustomization, which is created before
// those of the layouts below it, so the refusal names the bundle; the wording
// of the refusal on the layout is pinned on its own
// (TestLayoutDurationRefusalNamesLayoutAndBundle).
func TestFluxDurations_InheritedByALayout(t *testing.T) {
	for _, field := range []string{"interval", "timeout", "retryInterval"} {
		for _, tc := range fluxDurationCases {
			t.Run(field+"="+tc.value, func(t *testing.T) {
				b := shopSettings(nil)
				switch field {
				case "interval":
					b.Interval = tc.value
				case "timeout":
					b.Timeout = tc.value
				default:
					b.RetryInterval = tc.value
				}
				ml, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(oneBundleCluster(b), perLayoutRules())
				if tc.refused != "" {
					mustContainAll(t, err, "Bundle", "'shop'", field+" "+tc.refused)
					return
				}
				if err != nil {
					t.Fatalf("refused: %v", err)
				}
				for _, name := range append([]string{"shop"}, shopLayoutNames...) {
					got := kustomizationDuration(t, mustKustomization(t, ml, name), field)
					if got == nil || *got != tc.want {
						t.Errorf("%s: %s = %v, want %v", name, field, got, tc.want)
					}
				}
			})
		}
	}
}

// TestFluxDurations_DefaultInterval: a generator's DefaultInterval is held to
// the same pattern wherever it is written into an object: on a bundle's
// Kustomization when the bundle sets no interval, on a per-layout
// Kustomization when neither its layout nor a holding bundle sets one, on the
// gotk bootstrap Kustomization, on a generated source and on the FluxInstance
// sync. Where no object takes it, nothing is refused.
// The five places that are no Kustomization are each called on their own in
// TestDefaultIntervalOnSourcesAndSync.
func TestFluxDurations_DefaultInterval(t *testing.T) {
	for name, tc := range map[string]struct {
		interval time.Duration
		written  string
	}{
		"negative":            {-time.Minute, "-1m0s"},
		"under a millisecond": {500 * time.Microsecond, "500µs"},
	} {
		refusal := func(generator string) []string {
			return []string{generator, "DefaultInterval",
				`interval "` + tc.written + `" (the generator's DefaultInterval) is written as "` + tc.written + `", which the Flux API does not take`}
		}
		gen := func() *fluxstack.ResourceGenerator {
			g := fluxstack.NewResourceGenerator()
			g.DefaultInterval = tc.interval
			return g
		}
		t.Run(name+"/a bundle without an interval", func(t *testing.T) {
			_, err := gen().GenerateForBundle(&stack.Bundle{Name: "b"}, "b")
			mustContainAll(t, err, refusal("ResourceGenerator")...)
		})
		t.Run(name+"/a bundle with its own interval", func(t *testing.T) {
			objs, err := gen().GenerateForBundle(bundleWith("interval", "10m"), "b")
			if err != nil {
				t.Fatalf("refused, though no object takes the DefaultInterval: %v", err)
			}
			if got := objs[0].(*kustv1.Kustomization).Spec.Interval.Duration; got != 10*time.Minute {
				t.Errorf("interval = %v, want the bundle's 10m", got)
			}
		})
		t.Run(name+"/a per-layout Kustomization", func(t *testing.T) {
			// The bundle's own interval keeps its Kustomization clear of the
			// DefaultInterval, and those of its applications' layouts, which
			// inherit it. The Kustomization of a node's own layout inherits
			// from no bundle and takes the DefaultInterval.
			build := func() *stack.Cluster {
				b := shopSettings(nil)
				b.Interval = "10m"
				leaf := &stack.Node{Name: "web", Bundle: b}
				group := &stack.Node{Name: "apps", Children: []*stack.Node{leaf}}
				root := &stack.Node{Name: "platform", Bundle: srBundle("platform", cmApp("platform-app")), Children: []*stack.Node{group}}
				root.Bundle.Interval = "10m"
				leaf.SetParent(group)
				group.SetParent(root)
				return &stack.Cluster{Name: "demo", Node: root}
			}
			rules := layout.LayoutRules{FluxPlacement: layout.FluxIntegratedPerLayout}
			_, err := fluxstack.NewLayoutIntegrator(gen()).CreateLayoutWithResources(build(), rules)
			mustContainAll(t, err, refusal("ResourceGenerator")...)

			// Without that node, every per-layout Kustomization has the
			// bundle's interval or its layout's own, and nothing is refused.
			own := func(_, _, main, _ *layout.ManifestLayout) { main.Interval = "30m" }
			b := shopSettings(own)
			b.Interval = "10m"
			ml, err := fluxstack.NewLayoutIntegrator(gen()).CreateLayoutWithResources(oneBundleCluster(b), perLayoutRules())
			if err != nil {
				t.Fatalf("refused, though no object takes the DefaultInterval: %v", err)
			}
			for name, want := range map[string]time.Duration{"shop-db": 10 * time.Minute, "shop-00-pre": 10 * time.Minute, "shop-01-main": 30 * time.Minute} {
				if got := mustKustomization(t, ml, name).Spec.Interval.Duration; got != want {
					t.Errorf("%s: interval = %v, want %v", name, got, want)
				}
			}

			rules.FluxPlacement = layout.FluxIntegratedPerBundle
			if _, err := fluxstack.NewLayoutIntegrator(gen()).CreateLayoutWithResources(build(), rules); err != nil {
				t.Fatalf("refused without a per-layout Kustomization, though no object takes the DefaultInterval: %v", err)
			}
		})
		t.Run(name+"/the gotk bootstrap Kustomization", func(t *testing.T) {
			bg := fluxstack.NewBootstrapGenerator()
			bg.DefaultInterval = tc.interval
			_, err := bg.GenerateBootstrap(&stack.BootstrapConfig{Enabled: true, FluxMode: fluxstack.ModeGotk},
				&stack.Node{Name: "prod"}, layout.LayoutRules{})
			mustContainAll(t, err, refusal("BootstrapGenerator")...)
		})
		t.Run(name+"/a source generated for a bundle", func(t *testing.T) {
			// The bundle's own interval keeps its Kustomization clear of the
			// DefaultInterval; the source it derives always takes it.
			b := bundleWith("interval", "10m")
			b.SourceRef = &stack.SourceRef{Kind: "GitRepository", Name: "src", Namespace: "flux-system", URL: "https://example.com/repo.git"}
			_, err := gen().GenerateForBundle(b, "b")
			mustContainAll(t, err, refusal("ResourceGenerator")...)
		})
		t.Run(name+"/the FluxInstance sync", func(t *testing.T) {
			config := &stack.BootstrapConfig{Enabled: true, FluxVersion: "v2.4.0", Registry: "ghcr.io/fluxcd",
				SourceURL: "oci://registry.example.com/flux-system"}
			bg := fluxstack.NewBootstrapGenerator()
			bg.DefaultInterval = tc.interval
			_, err := bg.GenerateBootstrap(config, &stack.Node{Name: "prod"}, layout.LayoutRules{})
			mustContainAll(t, err, refusal("BootstrapGenerator")...)
			_, err = bg.GenerateFluxInstance(config, &stack.Node{Name: "prod"}, layout.LayoutRules{})
			mustContainAll(t, err, refusal("BootstrapGenerator")...)

			// Without a SourceURL the FluxInstance carries no sync, so no
			// object takes the DefaultInterval.
			config.SourceURL = ""
			if _, err := bg.GenerateBootstrap(config, &stack.Node{Name: "prod"}, layout.LayoutRules{}); err != nil {
				t.Fatalf("refused without a sync, though no object takes the DefaultInterval: %v", err)
			}
		})
	}

	t.Run("taken", func(t *testing.T) {
		g := fluxstack.NewResourceGenerator()
		g.DefaultInterval = 90 * time.Minute
		ml, err := fluxstack.NewLayoutIntegrator(g).CreateLayoutWithResources(oneBundleCluster(shopSettings(nil)), perLayoutRules())
		if err != nil {
			t.Fatalf("refused: %v", err)
		}
		for _, name := range append([]string{"shop"}, shopLayoutNames...) {
			if got := mustKustomization(t, ml, name).Spec.Interval.Duration; got != 90*time.Minute {
				t.Errorf("%s: interval = %v, want the DefaultInterval 1h30m", name, got)
			}
		}
	})
}

func TestIntegrateWithLayout_RejectsInvalidDurationOnUmbrellaChild(t *testing.T) {
	// Walk a valid cluster, then corrupt an umbrella child's RetryInterval:
	// IntegrateWithLayout places child CRs without re-validating, so the
	// generator must refuse the value itself.
	child := &stack.Bundle{Name: "child", SourceRef: testSR()}
	umbrella := &stack.Bundle{Name: "platform", SourceRef: testSR(), Children: []*stack.Bundle{child}}
	node := &stack.Node{Name: "apps", Bundle: umbrella}
	root := &stack.Node{Name: "demo", Children: []*stack.Node{node}}
	node.SetParent(root)
	cluster := &stack.Cluster{Name: "demo", Node: root}
	rules := layout.LayoutRules{
		BundleGrouping:      layout.GroupFlat,
		ApplicationGrouping: layout.GroupFlat,
		FluxPlacement:       layout.FluxIntegratedPerLayout,
	}
	ml, err := layout.WalkCluster(cluster, rules)
	if err != nil {
		t.Fatalf("WalkCluster: %v", err)
	}
	child.RetryInterval = "2 minutes"
	err = fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, cluster, rules)
	if err == nil || !strings.Contains(err.Error(), "retryInterval") {
		t.Fatalf("expected an error naming retryInterval, got %v", err)
	}
}
