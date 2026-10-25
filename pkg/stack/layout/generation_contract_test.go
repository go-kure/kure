package layout_test

import (
	"fmt"
	"slices"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// The generation contract (stack.ApplicationConfig, layout.LayoutAugmenter):
// who owns what Generate returns, how often kure calls it, and what an
// augmenter sees. Design: docs/history/20261025-DESIGN-generation-ownership.md
// (go-kure/kure#1067).

// recordingConfig builds a fresh ConfigMap on every Generate call, named
// after the call, and records each call's objects, every Validate call and
// what each AugmentLayout call found in its layout. own answers
// WantsOwnLayout; fail makes Generate fail.
type recordingConfig struct {
	name        string
	own         bool
	fail        bool
	validations int
	generated   [][]client.Object
	augmented   [][]client.Object
}

func (c *recordingConfig) Validate() error {
	c.validations++
	return nil
}

func (c *recordingConfig) Generate(*stack.Application) ([]*client.Object, error) {
	if c.fail {
		return nil, errors.New("generation failed")
	}
	o := namedConfigMap(fmt.Sprintf("%s-%d", c.name, len(c.generated)))
	c.generated = append(c.generated, []client.Object{o})
	return []*client.Object{&o}, nil
}

func (c *recordingConfig) AugmentLayout(ml *layout.ManifestLayout) error {
	c.augmented = append(c.augmented, slices.Clone(ml.Resources))
	return nil
}

func (c *recordingConfig) WantsOwnLayout() bool { return c.own }

func contractCluster(apps ...*stack.Application) *stack.Cluster {
	return &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Bundle: &stack.Bundle{Name: "platform", Applications: apps}}}
}

// TestGenerationContract_OncePerWalk: a walk calls Generate once per
// application, and AugmentLayout once, on the objects that call returned. A
// second walk calls both again; kure keeps nothing from the first.
func TestGenerationContract_OncePerWalk(t *testing.T) {
	walkers := map[string]func(*stack.Cluster, layout.LayoutRules) error{
		"WalkCluster": func(c *stack.Cluster, r layout.LayoutRules) error {
			_, err := layout.WalkCluster(c, r)
			return err
		},
		"WalkClusterByPackage": func(c *stack.Cluster, r layout.LayoutRules) error {
			_, err := layout.WalkClusterByPackage(c, r)
			return err
		},
	}
	for walker, walkFn := range walkers {
		for rulesName, rules := range map[string]layout.LayoutRules{"groupByName": groupByName, "nodeOnly": nodeOnly} {
			t.Run(walker+"/"+rulesName, func(t *testing.T) {
				cfg := &recordingConfig{name: "chart", own: true}
				c := contractCluster(stack.NewApplication("chart", "default", cfg))
				for walk := 1; walk <= 2; walk++ {
					if err := walkFn(c, rules); err != nil {
						t.Fatalf("walk %d: %v", walk, err)
					}
					if len(cfg.generated) != walk || len(cfg.augmented) != walk || cfg.validations != walk {
						t.Fatalf("after walk %d: %d Generate, %d AugmentLayout, %d Validate calls, want %d of each",
							walk, len(cfg.generated), len(cfg.augmented), cfg.validations, walk)
					}
					got, want := cfg.augmented[walk-1], cfg.generated[walk-1]
					if len(got) != 1 || got[0] != want[0] {
						t.Errorf("walk %d: the augmenter saw %v, want the very object that walk's Generate call returned", walk, objectNames(got))
					}
				}
				if cfg.augmented[1][0] == cfg.generated[0][0] {
					t.Error("the second walk's augmenter saw the first walk's object")
				}
			})
		}
	}
}

// TestGenerationContract_AugmentationSkipped: AugmentLayout runs only where a
// walk gives the application a layout of its own. WantsOwnLayout() false
// keeps it from running under flat application grouping only, and
// Application.Generate and Bundle.Generate never run it.
func TestGenerationContract_AugmentationSkipped(t *testing.T) {
	cases := []struct {
		name      string
		rules     layout.LayoutRules
		own       bool
		augmented int
	}{
		{"groupByName, wants its own layout", groupByName, true, 1},
		{"groupByName, does not want its own layout", groupByName, false, 1},
		{"flat, wants its own layout", nodeOnly, true, 1},
		{"flat, does not want its own layout", nodeOnly, false, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &recordingConfig{name: "chart", own: tc.own}
			walk(t, contractCluster(stack.NewApplication("chart", "default", cfg)), tc.rules)
			if len(cfg.generated) != 1 || len(cfg.augmented) != tc.augmented {
				t.Errorf("%d Generate and %d AugmentLayout calls, want 1 and %d", len(cfg.generated), len(cfg.augmented), tc.augmented)
			}
		})
	}
	t.Run("Application.Generate", func(t *testing.T) {
		cfg := &recordingConfig{name: "chart", own: true}
		if _, err := stack.NewApplication("chart", "default", cfg).Generate(); err != nil {
			t.Fatal(err)
		}
		if len(cfg.generated) != 1 || len(cfg.augmented) != 0 {
			t.Errorf("%d Generate and %d AugmentLayout calls, want 1 and 0", len(cfg.generated), len(cfg.augmented))
		}
	})
	t.Run("Bundle.Generate", func(t *testing.T) {
		cfg := &recordingConfig{name: "chart", own: true}
		b := &stack.Bundle{Name: "platform", Applications: []*stack.Application{stack.NewApplication("chart", "default", cfg)}}
		if _, err := b.Generate(); err != nil {
			t.Fatal(err)
		}
		if len(cfg.generated) != 1 || len(cfg.augmented) != 0 {
			t.Errorf("%d Generate and %d AugmentLayout calls, want 1 and 0", len(cfg.generated), len(cfg.augmented))
		}
	})
}

// TestGenerationContract_FailureLeavesNoResult: when one application's
// Generate fails, the walk returns no layout, and a later walk depends on
// nothing the failed one did.
func TestGenerationContract_FailureLeavesNoResult(t *testing.T) {
	ok := &recordingConfig{name: "core", own: true}
	failing := &recordingConfig{name: "chart", own: true, fail: true}
	c := contractCluster(stack.NewApplication("core", "default", ok), stack.NewApplication("chart", "default", failing))
	ml, err := layout.WalkCluster(c, groupByName)
	if err == nil || ml != nil {
		t.Fatalf("WalkCluster = %v, %v; want no layout and the generation error", ml, err)
	}
	failing.fail = false
	ml = walk(t, c, groupByName)
	recs := applicationObjects(t, ml)
	if got, want := objectNames(recs["core"].Objects), []string{"core-1"}; !slices.Equal(got, want) {
		t.Errorf("core objects after the retry = %v, want %v, the retry's own", got, want)
	}
	if got, want := objectNames(recs["chart"].Objects), []string{"chart-0"}; !slices.Equal(got, want) {
		t.Errorf("chart objects after the retry = %v, want %v", got, want)
	}
}

// cachingConfig renders once and returns the cached result on every call:
// as it is (shared) or as a deep copy.
type cachingConfig struct {
	shared bool
	cached *unstructured.Unstructured
}

func (c *cachingConfig) Generate(*stack.Application) ([]*client.Object, error) {
	if c.cached == nil {
		c.cached = namedConfigMap("cached").(*unstructured.Unstructured)
	}
	var o client.Object = c.cached
	if !c.shared {
		o = c.cached.DeepCopy()
	}
	return []*client.Object{&o}, nil
}

// TestGenerationContract_ResultsBelongToTheCaller: the objects a walk holds
// are the caller's to change, so an edit to one walk's objects must not reach
// another walk's. A config that builds fresh objects, or deep-copies a cached
// result, keeps that; one that returns its cache as it is does not, and the
// edit shows up in the next walk.
func TestGenerationContract_ResultsBelongToTheCaller(t *testing.T) {
	for _, tc := range []struct {
		name        string
		cfg         stack.ApplicationConfig
		independent bool
	}{
		{"fresh objects", &recordingConfig{name: "web", own: true}, true},
		{"a deep copy of a cached result", &cachingConfig{}, true},
		{"a cached result returned as it is", &cachingConfig{shared: true}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := contractCluster(stack.NewApplication("web", "default", tc.cfg))
			first := applicationObjects(t, walk(t, c, groupByName))["web"].Objects
			first[0].SetLabels(map[string]string{"edited": "by-the-caller"})
			second := applicationObjects(t, walk(t, c, groupByName))["web"].Objects
			_, leaked := second[0].GetLabels()["edited"]
			if leaked == tc.independent {
				t.Errorf("the caller's edit to the first walk's object reached the second walk: %t, want %t", leaked, !tc.independent)
			}
		})
	}
}

// embeddingWrapper embeds the ApplicationConfig interface: its method set is
// Generate alone, whatever the wrapped config implements.
type embeddingWrapper struct{ stack.ApplicationConfig }

// concreteWrapper embeds the concrete config, whose methods it promotes.
type concreteWrapper struct{ *recordingConfig }

// forwardingWrapper wraps through the interface and forwards every optional
// interface it wants kure to see.
type forwardingWrapper struct{ inner *recordingConfig }

func (w forwardingWrapper) Generate(app *stack.Application) ([]*client.Object, error) {
	return w.inner.Generate(app)
}
func (w forwardingWrapper) Validate() error { return w.inner.Validate() }
func (w forwardingWrapper) AugmentLayout(ml *layout.ManifestLayout) error {
	return w.inner.AugmentLayout(ml)
}
func (w forwardingWrapper) WantsOwnLayout() bool { return w.inner.WantsOwnLayout() }

// augmentOnlyWrapper forwards AugmentLayout but not WantsOwnLayout, so kure
// sees a LayoutAugmenter without the intent: presence-only placement,
// whatever the wrapped config wants.
type augmentOnlyWrapper struct{ inner *recordingConfig }

func (w augmentOnlyWrapper) Generate(app *stack.Application) ([]*client.Object, error) {
	return w.inner.Generate(app)
}
func (w augmentOnlyWrapper) AugmentLayout(ml *layout.ManifestLayout) error {
	return w.inner.AugmentLayout(ml)
}

// TestGenerationContract_WrappersForwardOptionalInterfaces: kure finds
// Validator and the augmenter interfaces by type assertion on the config
// itself. A wrapper that embeds the ApplicationConfig interface hides them,
// silently: no validation, no augmentation and, under flat application
// grouping, no layout of its own even where the wrapped config wants one.
// One that embeds the concrete type, or forwards them, keeps them. Only flat
// grouping with WantsOwnLayout() false shows a lost WantsOwnLayout: the
// wrapper then gets its own layout and AugmentLayout anyway.
func TestGenerationContract_WrappersForwardOptionalInterfaces(t *testing.T) {
	groupings := []struct {
		name  string
		rules layout.LayoutRules
		own   bool // the wrapped config's WantsOwnLayout
	}{
		{"groupByName", groupByName, true},
		{"flat, wants its own layout", nodeOnly, true},
		{"flat, does not want its own layout", nodeOnly, false},
	}
	for _, tc := range []struct {
		name        string
		wrap        func(*recordingConfig) stack.ApplicationConfig
		validations int
		// per grouping, in the order above
		ownLayout [3]bool
		augmented [3]int
	}{
		{"embeds the interface", func(c *recordingConfig) stack.ApplicationConfig { return embeddingWrapper{c} }, 0,
			[3]bool{true, false, false}, [3]int{0, 0, 0}},
		{"embeds the concrete type", func(c *recordingConfig) stack.ApplicationConfig { return concreteWrapper{c} }, 1,
			[3]bool{true, true, false}, [3]int{1, 1, 0}},
		{"forwards", func(c *recordingConfig) stack.ApplicationConfig { return forwardingWrapper{c} }, 1,
			[3]bool{true, true, false}, [3]int{1, 1, 0}},
		{"forwards AugmentLayout only", func(c *recordingConfig) stack.ApplicationConfig { return augmentOnlyWrapper{c} }, 0,
			[3]bool{true, true, true}, [3]int{1, 1, 1}},
	} {
		for i, g := range groupings {
			t.Run(tc.name+"/"+g.name, func(t *testing.T) {
				inner := &recordingConfig{name: "chart", own: g.own}
				ml := walk(t, contractCluster(stack.NewApplication("chart", "default", tc.wrap(inner))), g.rules)
				if len(inner.generated) != 1 || inner.validations != tc.validations || len(inner.augmented) != tc.augmented[i] {
					t.Errorf("%d Generate, %d Validate and %d AugmentLayout calls, want 1, %d and %d",
						len(inner.generated), inner.validations, len(inner.augmented), tc.validations, tc.augmented[i])
				}
				if own := applicationObjects(t, ml)["chart"].Layout != nil; own != tc.ownLayout[i] {
					t.Errorf("the application has a layout of its own: %t, want %t", own, tc.ownLayout[i])
				}
			})
		}
	}
}
