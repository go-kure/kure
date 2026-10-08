package fluxcd_test

import (
	"bytes"
	"fmt"
	"maps"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// Tests for the settings of a per-layout Kustomization: wait, timeout, retry
// interval, labels and annotations (go-kure/kure#1015), and interval, prune,
// force, suspend and the postBuild substitution (go-kure/kure#1021), inherited
// from the bundle that holds the layout's application and, but for postBuild,
// set on the layout itself. The bundle's patches are in
// layout_patches_test.go.

// settingsAugmenter is an application config that renders one ConfigMap and
// adds three layouts below its own: 00-pre, 01-main (which depends on 00-pre)
// and 02-deep below 01-main. set, if not nil, edits them and the
// application's own layout before the walk goes on.
type settingsAugmenter struct {
	set func(app, pre, main, deep *layout.ManifestLayout)
}

func (settingsAugmenter) Generate(app *stack.Application) ([]*client.Object, error) {
	return []*client.Object{cmObj(app.Name)}, nil
}

func (a settingsAugmenter) AugmentLayout(ml *layout.ManifestLayout) error {
	pre := &layout.ManifestLayout{
		Name:      "00-pre",
		Namespace: ml.FullRepoPath(),
		Resources: []client.Object{*cmObj("pre")},
	}
	main := &layout.ManifestLayout{
		Name:      "01-main",
		Namespace: ml.FullRepoPath(),
		Resources: []client.Object{*cmObj("main")},
		DependsOn: []string{"00-pre"},
	}
	deep := &layout.ManifestLayout{
		Name:      "02-deep",
		Namespace: main.FullRepoPath(),
		Resources: []client.Object{*cmObj("deep")},
	}
	main.Children = append(main.Children, deep)
	ml.Children = append(ml.Children, pre, main)
	if a.set != nil {
		a.set(ml, pre, main, deep)
	}
	return nil
}

// shopSettings is the bundle "shop" with the five settings a per-layout
// Kustomization inherits, and the application "db" with the layouts of
// settingsAugmenter.
func shopSettings(set func(app, pre, main, deep *layout.ManifestLayout)) *stack.Bundle {
	yes := true
	b := srBundle("shop", stack.NewApplication("db", "default", settingsAugmenter{set: set}))
	b.Wait = &yes
	b.Timeout = "5m"
	b.RetryInterval = "2m"
	b.Labels = map[string]string{"team": "shop"}
	b.Annotations = map[string]string{"owner": "shop"}
	return b
}

// unordered is set, after the DependsOn settingsAugmenter gives 01-main is
// cleared: only FluxIntegratedPerLayout carries a layout's DependsOn, and the
// other placements refuse it (go-kure/kure#1032).
func unordered(set func(app, pre, main, deep *layout.ManifestLayout)) func(app, pre, main, deep *layout.ManifestLayout) {
	return func(app, pre, main, deep *layout.ManifestLayout) {
		main.DependsOn = nil
		if set != nil {
			set(app, pre, main, deep)
		}
	}
}

func oneBundleCluster(b *stack.Bundle) *stack.Cluster {
	return stack.NewCluster("prod", &stack.Node{Name: "prod", Bundle: b})
}

func perLayoutRules() layout.LayoutRules {
	rules := layout.DefaultLayoutRules()
	rules.FluxPlacement = layout.FluxIntegratedPerLayout
	return rules
}

// shopLayoutNames are the per-layout Kustomizations of shopSettings: the
// application's own layout and the three its augmenter added.
var shopLayoutNames = []string{"shop-db", "shop-00-pre", "shop-01-main", "shop-02-deep"}

func mustKustomization(t *testing.T, ml *layout.ManifestLayout, name string) *kustv1.Kustomization {
	t.Helper()
	byName := kustomizationsByName(ml)
	k := byName[name]
	if k == nil {
		names := make([]string, 0, len(byName))
		for n := range byName {
			names = append(names, n)
		}
		slices.Sort(names)
		t.Fatalf("no Kustomization named %q among %q", name, names)
	}
	return k
}

// TestLayoutSettings_InheritedFromTheBundle: the Kustomization of an
// application's layout, and of every layout its augmenter added at any depth,
// carries the wait, timeout, retry interval, labels and annotations of the
// bundle that holds the application. The disk and tar writers write the same
// files, and the file of one of them is compared as a whole.
func TestLayoutSettings_InheritedFromTheBundle(t *testing.T) {
	ml := integrated(t, oneBundleCluster(shopSettings(nil)), perLayoutRules())
	bundle := mustKustomization(t, ml, "shop")
	for _, name := range shopLayoutNames {
		k := mustKustomization(t, ml, name)
		if !k.Spec.Wait {
			t.Errorf("%s: wait is off, the bundle's is on", name)
		}
		if !reflect.DeepEqual(k.Spec.Timeout, bundle.Spec.Timeout) || k.Spec.Timeout == nil || k.Spec.Timeout.Duration != 5*time.Minute {
			t.Errorf("%s: timeout = %v, want the bundle's 5m", name, k.Spec.Timeout)
		}
		if !reflect.DeepEqual(k.Spec.RetryInterval, bundle.Spec.RetryInterval) || k.Spec.RetryInterval == nil || k.Spec.RetryInterval.Duration != 2*time.Minute {
			t.Errorf("%s: retryInterval = %v, want the bundle's 2m", name, k.Spec.RetryInterval)
		}
		if want := map[string]string{"team": "shop"}; !reflect.DeepEqual(k.Labels, want) {
			t.Errorf("%s: labels = %v, want %v", name, k.Labels, want)
		}
		if want := map[string]string{"owner": "shop"}; !reflect.DeepEqual(k.Annotations, want) {
			t.Errorf("%s: annotations = %v, want %v", name, k.Annotations, want)
		}
		if len(k.Spec.HealthChecks) != 0 {
			t.Errorf("%s: health checks = %v, want none", name, k.Spec.HealthChecks)
		}
	}

	trees := writeAll(t, ml)
	disk, tar := treeFiles(t, trees["WriteToDisk"].root), treeFiles(t, trees["WriteToTar"].root)
	if len(disk) != len(tar) {
		t.Errorf("WriteToDisk wrote %d files, WriteToTar %d", len(disk), len(tar))
	}
	for p, content := range disk {
		if other, ok := tar[p]; !ok || !bytes.Equal(content, other) {
			t.Errorf("%s differs between WriteToDisk and WriteToTar", p)
		}
	}
	const want = `apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  annotations:
    owner: shop
  labels:
    team: shop
  name: shop-00-pre
  namespace: flux-system
spec:
  interval: 1h0m0s
  path: prod/shop/db/00-pre
  prune: false
  retryInterval: 2m0s
  sourceRef:
    kind: GitRepository
    name: flux-system
    namespace: flux-system
  timeout: 5m0s
  wait: true
`
	file := filepath.Join("prod", "shop", "db", "flux-system-kustomization-shop-00-pre.yaml")
	if got := string(disk[file]); got != want {
		t.Errorf("%s:\n%s\nwant:\n%s", file, got, want)
	}
}

// TestLayoutSettings_NoneSetRendersAsBefore is the control: a tree whose
// bundle and layouts set none of the settings gets per-layout Kustomizations
// with a name, a namespace, the generator's interval and prune, a path, a
// source and dependencies, and nothing else. The file compared is the one
// such a tree had before the settings existed.
func TestLayoutSettings_NoneSetRendersAsBefore(t *testing.T) {
	b := srBundle("shop", stack.NewApplication("db", "default", settingsAugmenter{}))
	ml := integrated(t, oneBundleCluster(b), perLayoutRules())

	want := &kustv1.Kustomization{
		TypeMeta:   metav1.TypeMeta{APIVersion: kustv1.GroupVersion.String(), Kind: "Kustomization"},
		ObjectMeta: metav1.ObjectMeta{Name: "shop-01-main", Namespace: fluxstack.DefaultNamespace},
		Spec: kustv1.KustomizationSpec{
			Interval:  metav1.Duration{Duration: fluxstack.DefaultInterval},
			Path:      "prod/shop/db/01-main",
			SourceRef: kustv1.CrossNamespaceSourceReference{Kind: "GitRepository", Name: "flux-system", Namespace: "flux-system"},
			DependsOn: []kustv1.DependencyReference{{Name: "shop-00-pre"}},
		},
	}
	if got := mustKustomization(t, ml, "shop-01-main"); !reflect.DeepEqual(got, want) {
		t.Errorf("shop-01-main:\n got %#v\nwant %#v", got, want)
	}

	// Every file of the tree, as the code before the settings wrote it.
	cr := func(name, path, dependsOn string) string {
		if dependsOn != "" {
			dependsOn = "  dependsOn:\n  - name: " + dependsOn + "\n"
		}
		return "apiVersion: kustomize.toolkit.fluxcd.io/v1\nkind: Kustomization\nmetadata:\n  name: " + name +
			"\n  namespace: flux-system\nspec:\n" + dependsOn + "  interval: 1h0m0s\n  path: " + path +
			"\n  prune: false\n  sourceRef:\n    kind: GitRepository\n    name: flux-system\n    namespace: flux-system\n"
	}
	cm := func(name string) string {
		return "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: " + name + "\n  namespace: default\n"
	}
	index := func(resources ...string) string {
		out := "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\nresources:\n"
		for _, r := range resources {
			out += "  - " + r + "\n"
		}
		return out
	}
	files := map[string]string{
		"prod/kustomization.yaml":                                          index("flux-system-kustomization-shop.yaml"),
		"prod/flux-system-kustomization-shop.yaml":                         cr("shop", "prod/shop", ""),
		"prod/shop/kustomization.yaml":                                     index("flux-system-kustomization-shop-db.yaml"),
		"prod/shop/flux-system-kustomization-shop-db.yaml":                 cr("shop-db", "prod/shop/db", ""),
		"prod/shop/db/kustomization.yaml":                                  index("default-configmap-db.yaml", "flux-system-kustomization-shop-00-pre.yaml", "flux-system-kustomization-shop-01-main.yaml"),
		"prod/shop/db/default-configmap-db.yaml":                           cm("db"),
		"prod/shop/db/flux-system-kustomization-shop-00-pre.yaml":          cr("shop-00-pre", "prod/shop/db/00-pre", ""),
		"prod/shop/db/flux-system-kustomization-shop-01-main.yaml":         cr("shop-01-main", "prod/shop/db/01-main", "shop-00-pre"),
		"prod/shop/db/00-pre/kustomization.yaml":                           index("default-configmap-pre.yaml"),
		"prod/shop/db/00-pre/default-configmap-pre.yaml":                   cm("pre"),
		"prod/shop/db/01-main/kustomization.yaml":                          index("default-configmap-main.yaml", "flux-system-kustomization-shop-02-deep.yaml"),
		"prod/shop/db/01-main/default-configmap-main.yaml":                 cm("main"),
		"prod/shop/db/01-main/flux-system-kustomization-shop-02-deep.yaml": cr("shop-02-deep", "prod/shop/db/01-main/02-deep", ""),
		"prod/shop/db/01-main/02-deep/kustomization.yaml":                  index("default-configmap-deep.yaml"),
		"prod/shop/db/01-main/02-deep/default-configmap-deep.yaml":         cm("deep"),
	}
	for writer, tree := range writeAll(t, ml) {
		written := treeFiles(t, tree.root)
		if len(written) != len(files) {
			t.Errorf("%s wrote %d files, want %d: %q", writer, len(written), len(files), slices.Sorted(maps.Keys(written)))
		}
		for name, want := range files {
			if got := string(written[filepath.FromSlash(name)]); got != want {
				t.Errorf("%s, %s:\n%s\nwant:\n%s", writer, name, got, want)
			}
		}
	}
}

// TestLayoutSettings_TheLayoutsOwnOverTheBundles: a layout that sets one of
// the settings differs from its sibling, which sets none, in that setting
// alone. A scalar replaces the bundle's, a false one turning the bundle's
// true off; a label or annotation is merged with the bundle's per key, the
// layout's value winning.
func TestLayoutSettings_TheLayoutsOwnOverTheBundles(t *testing.T) {
	yes, no := true, false
	for name, tc := range map[string]struct {
		bundle func(b *stack.Bundle)
		set    func(main *layout.ManifestLayout)
		want   func(k *kustv1.Kustomization)
	}{
		"another interval": {
			bundle: func(b *stack.Bundle) { b.Interval = "10m" },
			set:    func(main *layout.ManifestLayout) { main.Interval = "30m" },
			want:   func(k *kustv1.Kustomization) { k.Spec.Interval = metav1.Duration{Duration: 30 * time.Minute} },
		},
		"an interval the bundle does not set": {
			set:  func(main *layout.ManifestLayout) { main.Interval = "30m" },
			want: func(k *kustv1.Kustomization) { k.Spec.Interval = metav1.Duration{Duration: 30 * time.Minute} },
		},
		"prune turned on": {
			set:  func(main *layout.ManifestLayout) { main.Prune = &yes },
			want: func(k *kustv1.Kustomization) { k.Spec.Prune = true },
		},
		"prune turned off": {
			bundle: func(b *stack.Bundle) { b.Prune = &yes },
			set:    func(main *layout.ManifestLayout) { main.Prune = &no },
			want:   func(k *kustv1.Kustomization) { k.Spec.Prune = false },
		},
		"force turned on": {
			set:  func(main *layout.ManifestLayout) { main.Force = &yes },
			want: func(k *kustv1.Kustomization) { k.Spec.Force = true },
		},
		"force turned off": {
			bundle: func(b *stack.Bundle) { b.Force = &yes },
			set:    func(main *layout.ManifestLayout) { main.Force = &no },
			want:   func(k *kustv1.Kustomization) { k.Spec.Force = false },
		},
		"suspend turned on": {
			set:  func(main *layout.ManifestLayout) { main.Suspend = &yes },
			want: func(k *kustv1.Kustomization) { k.Spec.Suspend = true },
		},
		"suspend turned off": {
			bundle: func(b *stack.Bundle) { b.Suspend = &yes },
			set:    func(main *layout.ManifestLayout) { main.Suspend = &no },
			want:   func(k *kustv1.Kustomization) { k.Spec.Suspend = false },
		},
		"wait turned off": {
			set:  func(main *layout.ManifestLayout) { main.Wait = &no },
			want: func(k *kustv1.Kustomization) { k.Spec.Wait = false },
		},
		"another timeout": {
			set:  func(main *layout.ManifestLayout) { main.Timeout = "90s" },
			want: func(k *kustv1.Kustomization) { k.Spec.Timeout = &metav1.Duration{Duration: 90 * time.Second} },
		},
		"another retry interval": {
			set:  func(main *layout.ManifestLayout) { main.RetryInterval = "30s" },
			want: func(k *kustv1.Kustomization) { k.Spec.RetryInterval = &metav1.Duration{Duration: 30 * time.Second} },
		},
		"a label the bundle does not set": {
			set:  func(main *layout.ManifestLayout) { main.Labels = map[string]string{"tier": "data"} },
			want: func(k *kustv1.Kustomization) { k.Labels = map[string]string{"team": "shop", "tier": "data"} },
		},
		"another value for the bundle's label": {
			set:  func(main *layout.ManifestLayout) { main.Labels = map[string]string{"team": "db"} },
			want: func(k *kustv1.Kustomization) { k.Labels = map[string]string{"team": "db"} },
		},
		"an annotation the bundle does not set": {
			set:  func(main *layout.ManifestLayout) { main.Annotations = map[string]string{"note": "x"} },
			want: func(k *kustv1.Kustomization) { k.Annotations = map[string]string{"owner": "shop", "note": "x"} },
		},
		"another value for the bundle's annotation": {
			set:  func(main *layout.ManifestLayout) { main.Annotations = map[string]string{"owner": "db"} },
			want: func(k *kustv1.Kustomization) { k.Annotations = map[string]string{"owner": "db"} },
		},
	} {
		t.Run(name, func(t *testing.T) {
			b := shopSettings(func(_, _, main, _ *layout.ManifestLayout) { tc.set(main) })
			if tc.bundle != nil {
				tc.bundle(b)
			}
			ml := integrated(t, oneBundleCluster(b), perLayoutRules())
			sibling, got := mustKustomization(t, ml, "shop-00-pre"), mustKustomization(t, ml, "shop-01-main")

			want := sibling.DeepCopy()
			want.Name, want.Spec.Path, want.Spec.DependsOn = got.Name, got.Spec.Path, got.Spec.DependsOn
			tc.want(want)
			if !reflect.DeepEqual(got, want) {
				t.Errorf("shop-01-main:\n got %#v\nwant %#v", got, want)
			}
			// What the layout sets is its own: the layout below it has the
			// bundle's again.
			deep := sibling.DeepCopy()
			below := mustKustomization(t, ml, "shop-02-deep")
			deep.Name, deep.Spec.Path = below.Name, below.Spec.Path
			if !reflect.DeepEqual(below, deep) {
				t.Errorf("shop-02-deep:\n got %#v\nwant %#v", below, deep)
			}
		})
	}
}

// TestLayoutSettings_OtherPlacementsRefused: under FluxIntegratedPerBundle
// and FluxSeparate no layout gets a Kustomization of its own, so what the
// layouts set is refused (go-kure/kure#1032): the first such layout in
// pre-order, with every field it sets, before any value is checked as a
// Kustomization would check it. Without the settings the tree holds the
// bundle's Kustomization alone.
func TestLayoutSettings_OtherPlacementsRefused(t *testing.T) {
	yes := true
	set := func(app, pre, main, deep *layout.ManifestLayout) {
		app.Wait = &yes
		app.Labels = map[string]string{"tier": "data"}
		app.Suspend = &yes
		pre.Timeout = "not a duration"
		pre.Prune = &yes
		main.RetryInterval = "30s"
		main.Annotations = map[string]string{"bad key": "x"}
		main.Force = &yes
		deep.Interval = "not a duration"
	}
	for _, placement := range []layout.FluxPlacement{layout.FluxIntegratedPerBundle, layout.FluxSeparate} {
		t.Run(string(placement), func(t *testing.T) {
			rules := layout.DefaultLayoutRules()
			rules.FluxPlacement = placement
			_, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(oneBundleCluster(shopSettings(unordered(set))), rules)
			mustContainAll(t, err, `layout "prod/shop/db" sets Wait, Labels, Suspend, but has no Flux Kustomization of its own`, `FluxPlacement "integrated" (FluxIntegratedPerLayout)`)
			if err != nil && strings.Contains(err.Error(), "not a duration") {
				t.Errorf("a value was checked as a Kustomization's, though none carries it:\n%v", err)
			}

			without := integrated(t, oneBundleCluster(shopSettings(unordered(nil))), rules)
			if names := slices.Sorted(maps.Keys(kustomizationsByName(without))); !slices.Equal(names, []string{"shop"}) {
				t.Fatalf("Kustomizations = %q, want the bundle's alone", names)
			}
		})
	}
}

// TestLayoutSettings_Refusals: an interval, timeout or retry interval that is
// no duration, and a label or annotation the Kubernetes API does not accept, are
// refused where the Kustomization would be created, before anything is
// written. The refusal names the layout by its directory and, for a value the
// layout inherits, the bundle.
func TestLayoutSettings_Refusals(t *testing.T) {
	big := strings.Repeat("x", 200_000)
	for name, tc := range map[string]struct {
		bundle func(b *stack.Bundle)
		set    func(app, pre, main, deep *layout.ManifestLayout)
		wants  []string
		not    string
	}{
		"a layout's interval": {
			set:   func(_, pre, _, _ *layout.ManifestLayout) { pre.Interval = "hourly" },
			wants: []string{"ManifestLayout", "prod/shop/db/00-pre", `interval "hourly" is not a valid duration`},
			not:   "inherited",
		},
		"a layout's timeout": {
			set:   func(_, _, main, _ *layout.ManifestLayout) { main.Timeout = "soon" },
			wants: []string{"ManifestLayout", "prod/shop/db/01-main", `timeout "soon" is not a valid duration`},
			not:   "inherited",
		},
		"a layout's retry interval": {
			set:   func(_, _, _, deep *layout.ManifestLayout) { deep.RetryInterval = "5 minutes" },
			wants: []string{"prod/shop/db/01-main/02-deep", `retryInterval "5 minutes" is not a valid duration`},
			not:   "inherited",
		},
		"a layout's label key": {
			set:   func(_, pre, _, _ *layout.ManifestLayout) { pre.Labels = map[string]string{"bad key": "x"} },
			wants: []string{"prod/shop/db/00-pre", `labels entry "bad key" is not one the Kubernetes API accepts`, "metadata.labels"},
			not:   "inherited",
		},
		"a layout's label value": {
			set:   func(app, _, _, _ *layout.ManifestLayout) { app.Labels = map[string]string{"tier": "not a value"} },
			wants: []string{`'prod/shop/db'`, `labels entry "tier" is not one the Kubernetes API accepts`},
			not:   "inherited",
		},
		"a layout's annotation key": {
			set:   func(_, _, main, _ *layout.ManifestLayout) { main.Annotations = map[string]string{"bad key": "x"} },
			wants: []string{"prod/shop/db/01-main", `annotations entry "bad key" is not one the Kubernetes API accepts`, "metadata.annotations"},
			not:   "inherited",
		},
		"the bundle's label, inherited": {
			bundle: func(b *stack.Bundle) { b.Labels = map[string]string{"bad key": "x"} },
			wants:  []string{`'prod/shop/db'`, `labels entry "bad key" (inherited from bundle "shop") is not one`},
		},
		"the bundle's annotation, inherited": {
			bundle: func(b *stack.Bundle) { b.Annotations = map[string]string{"bad key": "x"} },
			wants:  []string{`'prod/shop/db'`, `annotations entry "bad key" (inherited from bundle "shop") is not one`},
		},
		"annotations too large together": {
			bundle: func(b *stack.Bundle) { b.Annotations = map[string]string{"a": big} },
			set:    func(_, pre, _, _ *layout.ManifestLayout) { pre.Annotations = map[string]string{"b": big} },
			wants:  []string{"prod/shop/db/00-pre", "the annotations of the layout's Flux Kustomization are not what the Kubernetes API accepts"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			b := shopSettings(tc.set)
			if tc.bundle != nil {
				tc.bundle(b)
			}
			_, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(oneBundleCluster(b), perLayoutRules())
			mustContainAll(t, err, tc.wants...)
			if tc.not != "" && strings.Contains(err.Error(), tc.not) {
				t.Errorf("the refusal says %q of a value the layout sets itself: %v", tc.not, err)
			}
		})
	}
}

// TestLayoutSettings_MergedBundles: where a grouping axis merges two bundles
// into one directory, an application's layout inherits from the bundle that
// lists the application, not from the first bundle of the directory. The
// bundles agree on the scalars (one Kustomization holds one value) and each
// brings its own labels.
func TestLayoutSettings_MergedBundles(t *testing.T) {
	yes := true
	bundle := func(name string, labels map[string]string) *stack.Bundle {
		b := srBundle(name, cmApp(name+"-app"))
		b.Wait = &yes
		b.Timeout = "5m"
		b.Labels = labels
		return b
	}
	child := &stack.Node{Name: "apps", Bundle: bundle("apps", map[string]string{"tier": "apps"})}
	root := &stack.Node{Name: "platform", Bundle: bundle("platform", map[string]string{"team": "platform"}), Children: []*stack.Node{child}}
	child.SetParent(root)
	rules := layout.LayoutRules{
		NodeGrouping:        layout.GroupFlat,
		BundleGrouping:      layout.GroupFlat,
		ApplicationGrouping: layout.GroupByName,
		FluxPlacement:       layout.FluxIntegratedPerLayout,
	}
	ml := integrated(t, &stack.Cluster{Name: "demo", Node: root}, rules)

	unit := mustKustomization(t, ml, "platform")
	if want := map[string]string{"team": "platform", "tier": "apps"}; !reflect.DeepEqual(unit.Labels, want) {
		t.Fatalf("the merged bundles' Kustomization has labels %v, want %v", unit.Labels, want)
	}
	for name, want := range map[string]map[string]string{
		"platform-platform-app": {"team": "platform"},
		"platform-apps-app":     {"tier": "apps"},
	} {
		k := mustKustomization(t, ml, name)
		if !reflect.DeepEqual(k.Labels, want) {
			t.Errorf("%s: labels = %v, want %v, those of the bundle that lists the application", name, k.Labels, want)
		}
		if !k.Spec.Wait || k.Spec.Timeout == nil || k.Spec.Timeout.Duration != 5*time.Minute {
			t.Errorf("%s: wait = %v, timeout = %v, want the bundles' wait and 5m", name, k.Spec.Wait, k.Spec.Timeout)
		}
	}
}

// TestLayoutSettings_UmbrellaChild: an application of an umbrella child
// inherits from the child bundle, which holds it, not from the umbrella
// above.
func TestLayoutSettings_UmbrellaChild(t *testing.T) {
	yes := true
	infra := srBundle("infra", cmApp("infra-app"))
	infra.Timeout = "3m"
	infra.Labels = map[string]string{"part": "infra"}
	shop := srBundle("shop", cmApp("shop-app"))
	shop.Wait = &yes
	shop.Timeout = "5m"
	shop.Labels = map[string]string{"team": "shop"}
	shop.Children = []*stack.Bundle{infra}
	rules := perLayoutRules()
	rules.ApplicationGrouping = layout.GroupByName
	ml := integrated(t, oneBundleCluster(shop), rules)

	k := mustKustomization(t, ml, "infra-infra-app")
	if k.Spec.Wait || k.Spec.Timeout == nil || k.Spec.Timeout.Duration != 3*time.Minute ||
		!reflect.DeepEqual(k.Labels, map[string]string{"part": "infra"}) {
		t.Errorf("infra-infra-app: wait = %v, timeout = %v, labels = %v, want those of the child bundle (no wait, 3m, part=infra)",
			k.Spec.Wait, k.Spec.Timeout, k.Labels)
	}
	k = mustKustomization(t, ml, "shop-shop-app")
	if !k.Spec.Wait || k.Spec.Timeout == nil || k.Spec.Timeout.Duration != 5*time.Minute ||
		!reflect.DeepEqual(k.Labels, map[string]string{"team": "shop"}) {
		t.Errorf("shop-shop-app: wait = %v, timeout = %v, labels = %v, want those of the umbrella (wait, 5m, team=shop)",
			k.Spec.Wait, k.Spec.Timeout, k.Labels)
	}
}

// TestLayoutSettings_NodeLayout: a node's own Kustomization inherits nothing,
// whatever the bundles above and below it set, and carries what its layout
// sets.
func TestLayoutSettings_NodeLayout(t *testing.T) {
	yes := true
	build := func() *stack.Cluster {
		leaf := &stack.Node{Name: "web", Bundle: shopSettings(nil)}
		sixSettings(leaf.Bundle)
		group := &stack.Node{Name: "apps", Children: []*stack.Node{leaf}}
		root := &stack.Node{Name: "platform", Bundle: srBundle("platform", cmApp("platform-app")), Children: []*stack.Node{group}}
		sixSettings(root.Bundle)
		root.Bundle.Prune = &yes
		root.Bundle.Wait = &yes
		root.Bundle.Labels = map[string]string{"team": "platform"}
		leaf.SetParent(group)
		group.SetParent(root)
		return &stack.Cluster{Name: "demo", Node: root}
	}
	rules := layout.LayoutRules{FluxPlacement: layout.FluxIntegratedPerLayout}
	const name = "platform-apps-node"

	t.Run("inherits nothing", func(t *testing.T) {
		k := mustKustomization(t, integrated(t, build(), rules), name)
		if k.Spec.Wait || k.Spec.Timeout != nil || k.Spec.RetryInterval != nil || len(k.Labels) != 0 || len(k.Annotations) != 0 {
			t.Errorf("the node's Kustomization carries settings no one set on its layout: %#v", k)
		}
		if k.Spec.Interval.Duration != fluxstack.DefaultInterval || k.Spec.Prune || k.Spec.Force || k.Spec.Suspend ||
			k.Spec.PostBuild != nil || len(k.Spec.Patches) != 0 {
			t.Errorf("the node's Kustomization carries an interval, prune, force, suspend, postBuild or patch of a bundle: %#v", k.Spec)
		}
	})
	t.Run("carries what its layout sets", func(t *testing.T) {
		c := build()
		ml := mustWalk(t, c, rules)
		l := layoutAtPath(t, ml, "platform/apps")
		l.Wait = &yes
		l.Timeout = "10m"
		l.RetryInterval = "1m"
		l.Labels = map[string]string{"tier": "apps"}
		l.Annotations = map[string]string{"owner": "apps"}
		l.Interval = "15m"
		l.Prune = &yes
		l.Force = &yes
		l.Suspend = &yes
		if err := integrate(ml, c, rules); err != nil {
			t.Fatalf("IntegrateWithLayout: %v", err)
		}
		k := mustKustomization(t, ml, name)
		if k.Spec.Interval.Duration != 15*time.Minute || !k.Spec.Prune || !k.Spec.Force || !k.Spec.Suspend {
			t.Errorf("the node's Kustomization does not carry the interval, prune, force and suspend its layout sets: %#v", k.Spec)
		}
		if !k.Spec.Wait || k.Spec.Timeout == nil || k.Spec.Timeout.Duration != 10*time.Minute ||
			k.Spec.RetryInterval == nil || k.Spec.RetryInterval.Duration != time.Minute ||
			!reflect.DeepEqual(k.Labels, map[string]string{"tier": "apps"}) ||
			!reflect.DeepEqual(k.Annotations, map[string]string{"owner": "apps"}) {
			t.Errorf("the node's Kustomization does not carry what its layout sets: %#v", k)
		}
	})
	t.Run("a value on its layout is refused with the layout", func(t *testing.T) {
		c := build()
		ml := mustWalk(t, c, rules)
		layoutAtPath(t, ml, "platform/apps").Timeout = "soon"
		mustContainAll(t, integrate(ml, c, rules), "'platform/apps'", `timeout "soon" is not a valid duration`)
	})
}

// TestLayoutSettings_LayoutOutsideAnApplication: a layout a caller adds to a
// bundle's directory belongs to no application and inherits nothing; one added
// below an application's layout is the application's, like those an augmenter
// adds.
func TestLayoutSettings_LayoutOutsideAnApplication(t *testing.T) {
	c := oneBundleCluster(shopSettings(nil))
	rules := perLayoutRules()
	ml := mustWalk(t, c, rules)
	dir := layoutAtPath(t, ml, "prod/shop")
	dir.Children = append(dir.Children, &layout.ManifestLayout{
		Name:      "extra",
		Namespace: dir.FullRepoPath(),
		Resources: []client.Object{*cmObj("extra")},
	})
	app := layoutAtPath(t, ml, "prod/shop/db")
	app.Children = append(app.Children, &layout.ManifestLayout{
		Name:      "03-late",
		Namespace: app.FullRepoPath(),
		Resources: []client.Object{*cmObj("late")},
	})
	if err := integrate(ml, c, rules); err != nil {
		t.Fatalf("IntegrateWithLayout: %v", err)
	}
	if k := mustKustomization(t, ml, "shop-extra"); k.Spec.Wait || k.Spec.Timeout != nil || len(k.Labels) != 0 {
		t.Errorf("a layout outside every application inherits the bundle's settings: %#v", k)
	}
	if k := mustKustomization(t, ml, "shop-03-late"); !k.Spec.Wait || k.Spec.Timeout == nil || !reflect.DeepEqual(k.Labels, map[string]string{"team": "shop"}) {
		t.Errorf("a layout below the application's does not inherit the bundle's settings: %#v", k)
	}
}

// sixSettings sets on b the six settings a per-layout Kustomization takes
// from its holding bundle besides the readiness ones: an interval, prune
// turned off, force, suspend, a postBuild substitution and one patch with a
// target.
func sixSettings(b *stack.Bundle) {
	yes, no := true, false
	b.Interval = "10m"
	b.Prune = &no
	b.Force = &yes
	b.Suspend = &yes
	b.PostBuild = &stack.PostBuild{
		Substitute:     map[string]string{"REGION": "eu"},
		SubstituteFrom: []stack.SubstituteRef{{Kind: "ConfigMap", Name: "cluster-vars", Optional: true}},
	}
	b.Patches = []stack.Patch{{
		Patch:  "- op: add\n  path: /metadata/labels/patched\n  value: \"yes\"\n",
		Target: &stack.PatchSelector{Kind: "ConfigMap"},
	}}
}

// TestLayoutSettings_SixMoreInheritedFromTheBundle: a bundle with suspend,
// force, prune turned off, an interval, one patch with a target and a
// postBuild substitution, holding an application with layouts below its own.
// The Kustomization of the application's layout and those of the layouts
// below it carry all six. The generator's prune is on, so the prune they carry
// is the bundle's. The disk and tar writers write the same files, and three of
// them are compared as a whole.
func TestLayoutSettings_SixMoreInheritedFromTheBundle(t *testing.T) {
	yes := true
	b := shopSettings(nil)
	sixSettings(b)
	g := fluxstack.NewResourceGenerator()
	g.Prune = &yes
	ml, err := fluxstack.NewLayoutIntegrator(g).CreateLayoutWithResources(oneBundleCluster(b), perLayoutRules())
	if err != nil {
		t.Fatalf("CreateLayoutWithResources: %v", err)
	}

	own := mustKustomization(t, ml, "shop")
	for _, name := range shopLayoutNames {
		k := mustKustomization(t, ml, name)
		if k.Spec.Interval != own.Spec.Interval || k.Spec.Interval.Duration != 10*time.Minute {
			t.Errorf("%s: interval = %v, want the bundle's 10m", name, k.Spec.Interval.Duration)
		}
		if k.Spec.Prune || !k.Spec.Force || !k.Spec.Suspend {
			t.Errorf("%s: prune = %v, force = %v, suspend = %v, want the bundle's (off, on, on)", name, k.Spec.Prune, k.Spec.Force, k.Spec.Suspend)
		}
		if !reflect.DeepEqual(k.Spec.PostBuild, own.Spec.PostBuild) || k.Spec.PostBuild == nil {
			t.Errorf("%s: postBuild = %#v, want the bundle's %#v", name, k.Spec.PostBuild, own.Spec.PostBuild)
		} else if k.Spec.PostBuild == own.Spec.PostBuild {
			t.Errorf("%s shares its postBuild with the bundle's own Kustomization", name)
		}
		if !reflect.DeepEqual(k.Spec.Patches, own.Spec.Patches) || len(k.Spec.Patches) != 1 {
			t.Errorf("%s: patches = %#v, want the bundle's one", name, k.Spec.Patches)
		}
	}
	// One Kustomization's substitution is its own: changing it changes no
	// other's, and not the bundle's. That holds for the map and for the list
	// of references alike.
	db, pre := mustKustomization(t, ml, "shop-db").Spec.PostBuild, mustKustomization(t, ml, "shop-00-pre").Spec.PostBuild
	db.Substitute["REGION"] = "us"
	if got := pre.Substitute["REGION"]; got != "eu" || b.PostBuild.Substitute["REGION"] != "eu" || own.Spec.PostBuild.Substitute["REGION"] != "eu" {
		t.Errorf("a change to shop-db's substitution reached shop-00-pre (%q), the bundle (%q) or its own Kustomization (%q)",
			got, b.PostBuild.Substitute["REGION"], own.Spec.PostBuild.Substitute["REGION"])
	}
	db.Substitute["REGION"] = "eu"
	db.SubstituteFrom[0].Name = "other-vars"
	if got := pre.SubstituteFrom[0].Name; got != "cluster-vars" || b.PostBuild.SubstituteFrom[0].Name != "cluster-vars" || own.Spec.PostBuild.SubstituteFrom[0].Name != "cluster-vars" {
		t.Errorf("a change to shop-db's substituteFrom reached shop-00-pre (%q), the bundle (%q) or its own Kustomization (%q)",
			got, b.PostBuild.SubstituteFrom[0].Name, own.Spec.PostBuild.SubstituteFrom[0].Name)
	}
	db.SubstituteFrom[0].Name = "cluster-vars"

	trees := writeAll(t, ml)
	disk, tar := treeFiles(t, trees["WriteToDisk"].root), treeFiles(t, trees["WriteToTar"].root)
	if len(disk) != len(tar) {
		t.Errorf("WriteToDisk wrote %d files, WriteToTar %d", len(disk), len(tar))
	}
	for p, content := range disk {
		if other, ok := tar[p]; !ok || !bytes.Equal(content, other) {
			t.Errorf("%s differs between WriteToDisk and WriteToTar", p)
		}
	}
	// What the three files share, from the retry interval down but for the
	// path, which each golden sets.
	const settings = `  force: true
  interval: 10m0s
  patches:
  - patch: |
      - op: add
        path: /metadata/labels/patched
        value: "yes"
    target:
      kind: ConfigMap
  path: %s
  postBuild:
    substitute:
      REGION: eu
    substituteFrom:
    - kind: ConfigMap
      name: cluster-vars
      optional: true
  prune: false
  retryInterval: 2m0s
  sourceRef:
    kind: GitRepository
    name: flux-system
    namespace: flux-system
  suspend: true
  timeout: 5m0s
  wait: true
`
	const head = `apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  annotations:
    owner: shop
  labels:
    team: shop
  name: %s
  namespace: flux-system
spec:
`
	for file, want := range map[string]string{
		filepath.Join("prod", "shop", "flux-system-kustomization-shop-db.yaml"): fmt.Sprintf(head, "shop-db") +
			fmt.Sprintf(settings, "prod/shop/db"),
		filepath.Join("prod", "shop", "db", "flux-system-kustomization-shop-00-pre.yaml"): fmt.Sprintf(head, "shop-00-pre") +
			fmt.Sprintf(settings, "prod/shop/db/00-pre"),
		filepath.Join("prod", "shop", "db", "flux-system-kustomization-shop-01-main.yaml"): fmt.Sprintf(head, "shop-01-main") +
			"  dependsOn:\n  - name: shop-00-pre\n" + fmt.Sprintf(settings, "prod/shop/db/01-main"),
	} {
		if got := string(disk[file]); got != want {
			t.Errorf("%s:\n%s\nwant:\n%s", file, got, want)
		}
	}
}

// TestLayoutSettings_PruneAndInterval: prune and interval on a per-layout
// Kustomization are the layout's, else the holding bundle's, else the
// generator's. Before the bundle's were read they were the generator's
// whatever the bundle set, so a bundle that sets one changes what its
// applications' Kustomizations carry: a bundle's prune turns garbage
// collection on for them where the generator's is unset.
func TestLayoutSettings_PruneAndInterval(t *testing.T) {
	yes, no := true, false
	for name, tc := range map[string]struct {
		generator, bundle, layout *bool
		want                      bool
	}{
		"none set: off":                              {nil, nil, nil, false},
		"the generator's alone":                      {&yes, nil, nil, true},
		"the bundle's on, the generator's unset":     {nil, &yes, nil, true},
		"the bundle's off over the generator's on":   {&yes, &no, nil, false},
		"the layout's on over the bundle's off":      {&yes, &no, &yes, true},
		"the layout's off over the bundle's on":      {nil, &yes, &no, false},
		"the layout's off over the generator's on":   {&yes, nil, &no, false},
		"the layout's on, bundle and generator both": {&no, &no, &yes, true},
	} {
		t.Run("prune/"+name, func(t *testing.T) {
			b := shopSettings(func(_, _, main, _ *layout.ManifestLayout) { main.Prune = tc.layout })
			b.Prune = tc.bundle
			g := fluxstack.NewResourceGenerator()
			g.Prune = tc.generator
			ml, err := fluxstack.NewLayoutIntegrator(g).CreateLayoutWithResources(oneBundleCluster(b), perLayoutRules())
			if err != nil {
				t.Fatalf("CreateLayoutWithResources: %v", err)
			}
			if got := mustKustomization(t, ml, "shop-01-main").Spec.Prune; got != tc.want {
				t.Errorf("shop-01-main: prune = %v, want %v", got, tc.want)
			}
			// The bundle's own Kustomization reads the bundle alone.
			if got, want := mustKustomization(t, ml, "shop").Spec.Prune, tc.bundle != nil && *tc.bundle; got != want {
				t.Errorf("shop: prune = %v, want the bundle's %v", got, want)
			}
		})
	}
	for name, tc := range map[string]struct {
		bundle, layout string
		want           time.Duration
	}{
		"none set: the generator's":        {"", "", 7 * time.Minute},
		"the bundle's":                     {"10m", "", 10 * time.Minute},
		"the layout's over the bundle's":   {"10m", "30m", 30 * time.Minute},
		"the layout's, the bundle's unset": {"", "30m", 30 * time.Minute},
	} {
		t.Run("interval/"+name, func(t *testing.T) {
			b := shopSettings(func(_, _, main, _ *layout.ManifestLayout) { main.Interval = tc.layout })
			b.Interval = tc.bundle
			g := fluxstack.NewResourceGenerator()
			g.DefaultInterval = 7 * time.Minute
			ml, err := fluxstack.NewLayoutIntegrator(g).CreateLayoutWithResources(oneBundleCluster(b), perLayoutRules())
			if err != nil {
				t.Fatalf("CreateLayoutWithResources: %v", err)
			}
			if got := mustKustomization(t, ml, "shop-01-main").Spec.Interval.Duration; got != tc.want {
				t.Errorf("shop-01-main: interval = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestLayoutSettings_WaitAndADependencyOnTheParent: with wait a Kustomization
// is Ready only once every Kustomization it applied is, so a layout that
// depends on the layout whose directory holds its own Kustomization can never
// be applied when that one waits. The wait an application's layout inherits
// closes that cycle, which the reconcile-order check refuses; the layout
// turning its wait off opens it again.
func TestLayoutSettings_WaitAndADependencyOnTheParent(t *testing.T) {
	no := false
	dependOnParent := func(_, pre, _, _ *layout.ManifestLayout) { pre.DependsOn = []string{"db"} }

	_, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).
		CreateLayoutWithResources(oneBundleCluster(shopSettings(dependOnParent)), perLayoutRules())
	mustContainAll(t, err, "can never all become Ready", "shop-db", "shop-00-pre", "a Kustomization with wait becomes Ready only once every Kustomization it applied is")

	ml := integrated(t, oneBundleCluster(shopSettings(func(app, pre, main, deep *layout.ManifestLayout) {
		dependOnParent(app, pre, main, deep)
		app.Wait = &no
	})), perLayoutRules())
	if k := mustKustomization(t, ml, "shop-db"); k.Spec.Wait {
		t.Error("shop-db waits although its layout turns wait off")
	}
	if k := mustKustomization(t, ml, "shop-00-pre"); !k.Spec.Wait || !slices.Equal(k.Spec.DependsOn, []kustv1.DependencyReference{{Name: "shop-db"}}) {
		t.Errorf("shop-00-pre: wait = %v, dependsOn = %v, want the bundle's wait and shop-db", k.Spec.Wait, k.Spec.DependsOn)
	}
}
