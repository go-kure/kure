package fluxcd_test

import (
	"errors"
	"fmt"
	"maps"
	"path"
	"slices"
	"strings"
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	kustv1beta2 "github.com/fluxcd/kustomize-controller/api/v1beta2"
	"github.com/fluxcd/pkg/apis/kustomize"
	metaapi "github.com/fluxcd/pkg/apis/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// The reconcile-order checks read a Flux Kustomization's spec fields in their
// context (go-kure/kure#1062): spec.path is a directory of the tree only on
// the tree's own source, a health check on another cluster names none of the
// tree's Kustomizations, and one a healthCheckExprs predicate decides waits
// for no readiness.

// remoteCluster is a spec.kubeConfig: Flux applies to, and polls, that
// cluster.
func remoteCluster() *metaapi.KubeConfigReference {
	return &metaapi.KubeConfigReference{SecretRef: &metaapi.SecretKeyReference{Name: "remote-kubeconfig"}}
}

// currentExpr is a healthCheckExprs entry for apiVersion and kind that
// evaluates every object it matches as current.
func currentExpr(apiVersion, kind string) kustomize.CustomHealthCheck {
	return kustomize.CustomHealthCheck{APIVersion: apiVersion, Kind: kind, HealthCheckExpressions: kustomize.HealthCheckExpressions{Current: "true"}}
}

// readyExpr is a healthCheckExprs entry for Flux Kustomizations that is a
// readiness wait written as an expression.
func readyExpr() kustomize.CustomHealthCheck {
	return kustomize.CustomHealthCheck{APIVersion: kustv1.GroupVersion.String(), Kind: "Kustomization", HealthCheckExpressions: kustomize.HealthCheckExpressions{Current: "status.conditions.filter(e, e.type == 'Ready').all(e, e.status == 'True')"}}
}

// webDirectory is the directory that holds web's Kustomization once
// noWaitCluster is integrated in placement.
func webDirectory(t *testing.T, placement layout.FluxPlacement) string {
	t.Helper()
	off, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(noWaitCluster(srBundle("platform", cmApp("platform-app")), true), withPlacement(nodeOnly, placement))
	if err != nil {
		t.Fatalf("without the setting: %v", err)
	}
	return path.Dir(path.Clean(findKept(off, "web").Spec.Path))
}

// integrateWithHandWritten integrates noWaitCluster, web depending on
// platform, whose application emits hand-written as edit leaves it: platform
// applies it, so web's dependsOn is written as one on hand-written.
func integrateWithHandWritten(placement layout.FluxPlacement, edit func(k *kustv1.Kustomization)) (*layout.ManifestLayout, error) {
	return integrateNoWait(handWrittenCluster(edit), nodeOnly, placement)
}

// handWrittenCluster is the tree integrateWithHandWritten integrates.
func handWrittenCluster(edit func(k *kustv1.Kustomization)) *stack.Cluster {
	k := handWritten("hand-written")
	edit(k)
	var o client.Object = k
	app := stack.NewApplication("platform-app", "default", &fakeAppConfig{objs: []*client.Object{&o}})
	return noWaitCluster(srBundle("platform", app), true)
}

// integrateNoWaitWithComponent is integrateNoWait with a kustomize Component
// written into the layout at dir as extra files, under the directory name: a
// Flux Kustomization building dir names it in spec.components as name. The
// layout's kustomization.yaml lists neither file.
func integrateNoWaitWithComponent(t *testing.T, c *stack.Cluster, placement layout.FluxPlacement, dir, name, component string, files map[string]string) (*layout.ManifestLayout, error) {
	t.Helper()
	rules := withPlacement(nodeOnly, placement)
	ml := mustWalk(t, c, rules)
	l := layoutAtPath(t, ml, dir)
	l.ExtraFiles = append(l.ExtraFiles, layout.ExtraFile{Name: name + "/kustomization.yaml", Content: []byte(component)})
	for _, f := range slices.Sorted(maps.Keys(files)) {
		l.ExtraFiles = append(l.ExtraFiles, layout.ExtraFile{Name: name + "/" + f, Content: []byte(files[f])})
	}
	return ml, fluxstack.NewLayoutIntegrator(noWaitGenerator()).IntegrateWithLayout(ml, c, rules)
}

// An application's Kustomization with wait on the tree's own source builds
// the directory its spec.path names: there, web's Kustomization, which
// depends on it once rewritten. Neither becomes Ready, and the rewrite is
// refused, whether or not its sourceRef spells the namespace out.
func TestReconcileContext_SameSourceWaitIsACycle(t *testing.T) {
	for _, placement := range integratedPlacements {
		for _, namespace := range []string{"", "flux-system"} {
			t.Run(string(placement)+"/sourceRef namespace "+namespace, func(t *testing.T) {
				dir := webDirectory(t, placement)
				_, err := integrateWithHandWritten(placement, func(k *kustv1.Kustomization) {
					k.Spec.Wait, k.Spec.Path = true, dir
					k.Spec.SourceRef.Namespace = namespace
				})
				if !errors.Is(err, fluxstack.ErrUmbrellaDependency) || !strings.Contains(err.Error(), "ready flux-system/hand-written -> ready flux-system/web -> apply flux-system/web") {
					t.Fatalf("got %v, want ErrUmbrellaDependency naming the chain through web", err)
				}
			})
		}
	}
}

// The same Kustomization reading another Source, or applying to another
// cluster, builds no directory of this tree: its wait waits for nothing
// here, and the rewrite is accepted. Another Source of the same repository
// (another ref) is another Source.
func TestReconcileContext_ForeignWaitIsNoCycle(t *testing.T) {
	cases := map[string]func(k *kustv1.Kustomization){
		"another Source name": func(k *kustv1.Kustomization) { k.Spec.SourceRef.Name = "flux-system-next" },
		"another Source kind": func(k *kustv1.Kustomization) { k.Spec.SourceRef.Kind = "OCIRepository" },
		"another Source namespace": func(k *kustv1.Kustomization) {
			k.Spec.SourceRef.Namespace = "elsewhere"
		},
		"another cluster": func(k *kustv1.Kustomization) { k.Spec.KubeConfig = remoteCluster() },
	}
	for name, edit := range cases {
		for _, placement := range integratedPlacements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				dir := webDirectory(t, placement)
				ml, err := integrateWithHandWritten(placement, func(k *kustv1.Kustomization) {
					k.Spec.Wait, k.Spec.Path = true, dir
					edit(k)
				})
				if err != nil {
					t.Fatalf("CreateLayoutWithResources: %v", err)
				}
				assertDependsOn(t, ml, "web", "hand-written")
			})
		}
	}
}

// A health check of hand-written on web closes the same cycle as a wait,
// unless it names another Kustomization: one on another cluster. A
// healthCheckExprs entry matching Flux Kustomizations is arbitrary CEL, which
// may itself wait for readiness, so it is read as readiness whatever it says,
// with or without wait: each is refused, the always-current one a false
// refusal kept on purpose (go-kure/kure#1094).
func TestReconcileContext_HealthCheckContext(t *testing.T) {
	checksWeb := func(k *kustv1.Kustomization) {
		k.Spec.HealthChecks = []metaapi.NamespacedObjectKindReference{{APIVersion: kustv1.GroupVersion.String(), Kind: "Kustomization", Name: "web"}}
	}
	type verdict struct {
		edit    func(t *testing.T, placement layout.FluxPlacement, k *kustv1.Kustomization)
		refused bool
	}
	withExpr := func(e kustomize.CustomHealthCheck) func(*testing.T, layout.FluxPlacement, *kustv1.Kustomization) {
		return func(_ *testing.T, _ layout.FluxPlacement, k *kustv1.Kustomization) {
			checksWeb(k)
			k.Spec.HealthCheckExprs = []kustomize.CustomHealthCheck{e}
		}
	}
	cases := map[string]verdict{
		"a health check on web": {edit: func(_ *testing.T, _ layout.FluxPlacement, k *kustv1.Kustomization) { checksWeb(k) }, refused: true},
		"on another cluster": {edit: func(_ *testing.T, _ layout.FluxPlacement, k *kustv1.Kustomization) {
			checksWeb(k)
			k.Spec.KubeConfig = remoteCluster()
		}},
		"an expression for Kustomization":          {edit: withExpr(currentExpr(kustv1.GroupVersion.String(), "Kustomization")), refused: true},
		"an expression waiting for readiness":      {edit: withExpr(readyExpr()), refused: true},
		"an expression at another version":         {edit: withExpr(currentExpr(kustv1beta2.GroupVersion.String(), "Kustomization")), refused: true},
		"an expression for the whole group":        {edit: withExpr(currentExpr(kustv1.GroupVersion.String(), "")), refused: true},
		"an expression for another kind":           {edit: withExpr(currentExpr("apps/v1", "Deployment")), refused: true},
		"an expression for the group's other kind": {edit: withExpr(currentExpr(kustv1.GroupVersion.String(), "Other")), refused: true},
		"wait with an expression for Kustomization": {edit: func(t *testing.T, placement layout.FluxPlacement, k *kustv1.Kustomization) {
			k.Spec.Wait, k.Spec.Path = true, webDirectory(t, placement)
			k.Spec.HealthCheckExprs = []kustomize.CustomHealthCheck{currentExpr(kustv1.GroupVersion.String(), "Kustomization")}
		}, refused: true},
	}
	for name, c := range cases {
		for _, placement := range integratedPlacements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				ml, err := integrateWithHandWritten(placement, func(k *kustv1.Kustomization) { c.edit(t, placement, k) })
				if c.refused {
					if !errors.Is(err, fluxstack.ErrUmbrellaDependency) || !strings.Contains(err.Error(), "ready flux-system/hand-written -> ready flux-system/web") {
						t.Fatalf("got %v, want ErrUmbrellaDependency naming the chain through web", err)
					}
					return
				}
				if err != nil {
					t.Fatalf("CreateLayoutWithResources: %v", err)
				}
				assertDependsOn(t, ml, "web", "hand-written")
			})
		}
	}
}

// A Kustomization a healthCheckExprs predicate decides is waited Ready for,
// as without the predicate: hand-written, checking inner that web's apply
// writes, becomes Ready only once inner is, so once web applied, and web,
// depending on it once rewritten, applies only once it is Ready. Refused; on
// another cluster, inner is not this one, and it is accepted.
func TestReconcileContext_ExistenceIsAwaited(t *testing.T) {
	for _, remote := range []bool{false, true} {
		for _, placement := range integratedPlacements {
			t.Run(string(placement)+map[bool]string{false: "/local", true: "/remote"}[remote], func(t *testing.T) {
				k := handWritten("hand-written")
				k.Spec.HealthChecks = []metaapi.NamespacedObjectKindReference{{APIVersion: kustv1.GroupVersion.String(), Kind: "Kustomization", Name: "inner"}}
				k.Spec.HealthCheckExprs = []kustomize.CustomHealthCheck{currentExpr(kustv1.GroupVersion.String(), "Kustomization")}
				if remote {
					k.Spec.KubeConfig = remoteCluster()
				}
				var o client.Object = k
				platform := srBundle("platform", stack.NewApplication("platform-app", "default", &fakeAppConfig{objs: []*client.Object{&o}}))
				c := noWaitCluster(platform, true)
				web := c.Node.Children[1].Bundle
				web.Applications = append(web.Applications, fluxKustomizationApp("inner-app", "inner"))
				ml, err := integrateNoWait(c, nodeOnly, placement)
				if remote {
					if err != nil {
						t.Fatalf("CreateLayoutWithResources: %v", err)
					}
					assertDependsOn(t, ml, "web", "hand-written")
					return
				}
				if !errors.Is(err, fluxstack.ErrUmbrellaDependency) || !strings.Contains(err.Error(), "ready flux-system/hand-written -> ready flux-system/inner -> apply flux-system/inner -> apply flux-system/web") {
					t.Fatalf("got %v, want ErrUmbrellaDependency naming hand-written's wait for inner's readiness", err)
				}
			})
		}
	}
}

// hand-written building web's own directory writes inner as web does, so
// inner could exist once either applies. The check does not model such an
// alternate creator (whether hand-written applies depends on controller
// settings the tree does not carry), so hand-written's check on inner, by a
// health check, a predicate or its wait, waits for web's apply, and web
// depending on hand-written is refused: a false refusal, kept on purpose
// (go-kure/kure#1094).
func TestReconcileContext_TargetItsOwnBuildWrites(t *testing.T) {
	checksInner := func(k *kustv1.Kustomization) {
		k.Spec.HealthChecks = []metaapi.NamespacedObjectKindReference{{APIVersion: kustv1.GroupVersion.String(), Kind: "Kustomization", Name: "inner"}}
	}
	exprs := func(k *kustv1.Kustomization) {
		k.Spec.HealthCheckExprs = []kustomize.CustomHealthCheck{currentExpr(kustv1.GroupVersion.String(), "Kustomization")}
	}
	cases := map[string]func(k *kustv1.Kustomization){
		"existence":       func(k *kustv1.Kustomization) { checksInner(k); exprs(k) },
		"health check":    checksInner,
		"wait":            func(k *kustv1.Kustomization) { k.Spec.Wait = true },
		"wait, existence": func(k *kustv1.Kustomization) { k.Spec.Wait = true; exprs(k) },
	}
	for name, edit := range cases {
		for _, placement := range integratedPlacements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				k := handWritten("hand-written")
				k.Spec.Path = webBuildDirectory(t, placement)
				edit(k)
				var o client.Object = k
				platform := srBundle("platform", stack.NewApplication("platform-app", "default", &fakeAppConfig{objs: []*client.Object{&o}}))
				c := noWaitCluster(platform, true)
				web := c.Node.Children[1].Bundle
				web.Applications = append(web.Applications, fluxKustomizationApp("inner-app", "inner"))
				_, err := integrateNoWait(c, nodeOnly, placement)
				if !errors.Is(err, fluxstack.ErrUmbrellaDependency) || !strings.Contains(err.Error(), "ready flux-system/hand-written -> ready flux-system/inner -> apply flux-system/inner -> apply flux-system/web") {
					t.Fatalf("got %v, want ErrUmbrellaDependency naming hand-written's wait for web's apply", err)
				}
			})
		}
	}
}

// The same path on another Source is another directory: hand-written then
// writes no inner, and its health check on inner waits for web's apply.
// Refused: web depending on hand-written closes the cycle.
func TestReconcileContext_TargetAForeignBuildDoesNotWrite(t *testing.T) {
	for _, placement := range integratedPlacements {
		t.Run(string(placement), func(t *testing.T) {
			k := handWritten("hand-written")
			k.Spec.Path = webBuildDirectory(t, placement)
			k.Spec.SourceRef.Name = "flux-system-next"
			k.Spec.HealthChecks = []metaapi.NamespacedObjectKindReference{{APIVersion: kustv1.GroupVersion.String(), Kind: "Kustomization", Name: "inner"}}
			var o client.Object = k
			platform := srBundle("platform", stack.NewApplication("platform-app", "default", &fakeAppConfig{objs: []*client.Object{&o}}))
			c := noWaitCluster(platform, true)
			web := c.Node.Children[1].Bundle
			web.Applications = append(web.Applications, fluxKustomizationApp("inner-app", "inner"))
			_, err := integrateNoWait(c, nodeOnly, placement)
			if !errors.Is(err, fluxstack.ErrUmbrellaDependency) || !strings.Contains(err.Error(), "ready flux-system/hand-written -> ready flux-system/inner -> apply flux-system/inner -> apply flux-system/web") {
				t.Fatalf("got %v, want ErrUmbrellaDependency naming hand-written's check on inner, which web's apply writes", err)
			}
		})
	}
}

// webBuildDirectory is the directory web's Kustomization builds once
// noWaitCluster, inner added to web, is integrated in placement.
func webBuildDirectory(t *testing.T, placement layout.FluxPlacement) string {
	t.Helper()
	c := noWaitCluster(srBundle("platform", cmApp("platform-app")), true)
	web := c.Node.Children[1].Bundle
	web.Applications = append(web.Applications, fluxKustomizationApp("inner-app", "inner"))
	off, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(c, withPlacement(nodeOnly, placement))
	if err != nil {
		t.Fatalf("without the setting: %v", err)
	}
	return path.Clean(findKept(off, "web").Spec.Path)
}

// hand-written waiting on web's directory, as in
// TestReconcileContext_SameSourceWaitIsACycle, but transforming what its
// build writes. The check reads that build as kustomize-controller builds
// it. A patch deleting web, written on hand-written or in a component it
// uses, takes web out of what hand-written's wait waits for, and the rewrite
// is accepted. A patch that keeps web, a component that only adds an object,
// or a name prefix or suffix leave web, renamed or not, in hand-written's build,
// still depending on hand-written, since dependsOn is no reference kustomize
// rewrites: a deadlock, refused through web under the name it is built with.
// A target namespace moves web's copy into another namespace, where its
// dependsOn, written without one, names a hand-written that does not exist:
// the copy never becomes Ready, a stall the checks do not judge, and no
// cycle, so the rewrite is accepted.
func TestReconcileContext_TransformedBuildIsRead(t *testing.T) {
	const throughWeb = "ready flux-system/hand-written -> ready flux-system/web -> apply flux-system/web"
	const deleteWeb = "$patch: delete\napiVersion: kustomize.toolkit.fluxcd.io/v1\nkind: Kustomization\nmetadata:\n  name: web\n  namespace: flux-system\n"
	// A component adding an object beside web, and one whose patch deletes
	// web: the second is admitted only because the component is read.
	addsObject := map[string]string{
		"kustomization.yaml": "apiVersion: kustomize.config.k8s.io/v1alpha1\nkind: Component\nresources:\n- added.yaml\n",
		"added.yaml":         "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: added\n  namespace: default\n",
	}
	deletesWeb := map[string]string{
		"kustomization.yaml": "apiVersion: kustomize.config.k8s.io/v1alpha1\nkind: Component\npatches:\n- patch: |\n    " +
			strings.ReplaceAll(strings.TrimSuffix(deleteWeb, "\n"), "\n", "\n    ") + "\n",
	}
	cases := map[string]struct {
		edit      func(k *kustv1.Kustomization)
		component map[string]string
		chain     string
	}{
		"patch deleting web": {edit: func(k *kustv1.Kustomization) {
			k.Spec.Patches = []kustomize.Patch{{Patch: deleteWeb}}
		}},
		"patch keeping web": {edit: func(k *kustv1.Kustomization) {
			k.Spec.Patches = []kustomize.Patch{{Patch: "apiVersion: kustomize.toolkit.fluxcd.io/v1\nkind: Kustomization\nmetadata:\n  name: web\n  namespace: flux-system\n  labels:\n    patched: \"yes\"\n"}}
		}, chain: throughWeb},
		"component keeping web":  {edit: func(k *kustv1.Kustomization) { k.Spec.Components = []string{"extra"} }, component: addsObject, chain: throughWeb},
		"component deleting web": {edit: func(k *kustv1.Kustomization) { k.Spec.Components = []string{"extra"} }, component: deletesWeb},
		"name prefix": {edit: func(k *kustv1.Kustomization) { k.Spec.NamePrefix = "x-" },
			chain: "ready flux-system/hand-written -> ready flux-system/x-web -> apply flux-system/x-web"},
		"name suffix": {edit: func(k *kustv1.Kustomization) { k.Spec.NameSuffix = "-x" },
			chain: "ready flux-system/hand-written -> ready flux-system/web-x -> apply flux-system/web-x"},
		"target namespace": {edit: func(k *kustv1.Kustomization) { k.Spec.TargetNamespace = "elsewhere" }},
	}
	for name, tc := range cases {
		for _, placement := range integratedPlacements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				dir := webDirectory(t, placement)
				c := handWrittenCluster(func(k *kustv1.Kustomization) {
					k.Spec.Wait, k.Spec.Path = true, dir
					tc.edit(k)
				})
				var ml *layout.ManifestLayout
				var err error
				if tc.component != nil {
					files := maps.Clone(tc.component)
					delete(files, "kustomization.yaml")
					ml, err = integrateNoWaitWithComponent(t, c, placement, dir, "extra", tc.component["kustomization.yaml"], files)
				} else {
					ml, err = integrateNoWait(c, nodeOnly, placement)
				}
				if tc.chain == "" {
					if err != nil {
						t.Fatalf("got %v, want the rewrite accepted", err)
					}
					assertDependsOn(t, ml, "web", "hand-written")
					return
				}
				if !errors.Is(err, fluxstack.ErrUmbrellaDependency) || !strings.Contains(err.Error(), tc.chain) {
					t.Fatalf("got %v, want ErrUmbrellaDependency naming the chain %s", err, tc.chain)
				}
			})
		}
	}
}

// A build that patches what it holds may write a Kustomization the tree
// holds as local config: kustomize transforms before it drops local-config
// objects, so a patch, or a component's, setting the annotation to "false"
// keeps it. hand-written waits for the seed directory, which holds inner,
// local config, depending on web; web depends on hand-written once
// rewritten. With a patch or a component the build keeps inner and the
// check refuses the cycle; without one, or with a transform that cannot
// change an annotation, inner is dropped and the rewrite is accepted, even
// where hand-written depends on it by name. A kept inner that health-checks
// web, instead of depending on it, closes the same cycle through its own
// readiness. Where a sibling bundle's directory holds a live inner with no
// dependsOn, the kept copy and the live one are one object with two specs,
// each overwriting the other: refused, naming both writers. Renamed by a
// prefix, the kept copy is another object, x-inner, and closes the cycle
// under that name.
func TestReconcileContext_PatchedBuildKeepsLocalConfig(t *testing.T) {
	patch := "apiVersion: kustomize.toolkit.fluxcd.io/v1\nkind: Kustomization\nmetadata:\n  name: inner\n  namespace: flux-system\n  annotations:\n    " + localConfigAnnotation + `: "false"` + "\n"
	keepInner := "apiVersion: kustomize.config.k8s.io/v1alpha1\nkind: Component\npatches:\n- patch: |\n    " +
		strings.ReplaceAll(strings.TrimSuffix(patch, "\n"), "\n", "\n    ") + "\n"
	cases := map[string]struct {
		edit    func(k *kustv1.Kustomization)
		check   bool
		refused bool
		live    bool
	}{
		"patch and prefix, live namesake": {func(k *kustv1.Kustomization) {
			k.Spec.Patches = []kustomize.Patch{{Patch: patch}}
			k.Spec.NamePrefix = "x-"
		}, false, true, true},
		"patch, live namesake, health check": {func(k *kustv1.Kustomization) { k.Spec.Patches = []kustomize.Patch{{Patch: patch}} }, true, true, true},
		"patch":                              {func(k *kustv1.Kustomization) { k.Spec.Patches = []kustomize.Patch{{Patch: patch}} }, false, true, false},
		"component":                          {func(k *kustv1.Kustomization) { k.Spec.Components = []string{"keep-inner"} }, false, true, false},
		"patch, health check":                {func(k *kustv1.Kustomization) { k.Spec.Patches = []kustomize.Patch{{Patch: patch}} }, true, true, false},
		"no patch":                           {func(*kustv1.Kustomization) {}, false, false, false},
		"no patch, health check":             {func(*kustv1.Kustomization) {}, true, false, false},
		"no patch, dependsOn inner": {func(k *kustv1.Kustomization) {
			k.Spec.DependsOn = []kustv1.DependencyReference{{Name: "inner"}}
		}, false, false, false},
		"name prefix": {func(k *kustv1.Kustomization) { k.Spec.NamePrefix = "x-" }, false, false, false},
	}
	build := func(edit func(k *kustv1.Kustomization), dir string, check, live bool) *stack.Cluster {
		k := handWritten("hand-written")
		k.Spec.Wait = true
		if dir != "" {
			k.Spec.Path = dir
		}
		edit(k)
		var hw client.Object = k
		dropped := "true"
		in := handWritten("inner", "web")
		if check {
			in = handWritten("inner")
			in.Spec.HealthChecks = []metaapi.NamespacedObjectKindReference{{APIVersion: kustv1.GroupVersion.String(), Kind: "Kustomization", Name: "web"}}
		}
		var inner client.Object = annotateLocalConfig(in, &dropped)
		c := noWaitCluster(srBundle("platform",
			stack.NewApplication("platform-app", "default", &fakeAppConfig{objs: []*client.Object{&hw}}),
			stack.NewApplication("seed-app", "default", &fakeAppConfig{objs: []*client.Object{&inner}}),
		), true)
		if live {
			// In a sibling bundle: in platform's directory the tree would
			// hold inner twice, which no writer writes.
			var namesake client.Object = handWritten("inner")
			other := &stack.Node{Name: "other", Bundle: srBundle("other", stack.NewApplication("live-app", "default", &fakeAppConfig{objs: []*client.Object{&namesake}}))}
			other.SetParent(c.Node)
			c.Node.Children = append(c.Node.Children, other)
		}
		return c
	}
	for _, placement := range integratedPlacements {
		first, err := integrateNoWait(build(func(*kustv1.Kustomization) {}, "", false, false), nodeOnly, placement)
		if err != nil {
			t.Fatalf("%s: first integration: %v", placement, err)
		}
		seed := hostDirectory(t, first, "inner")
		for name, c := range cases {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				tree := build(c.edit, seed, c.check, c.live)
				var ml *layout.ManifestLayout
				var err error
				if name == "component" {
					ml, err = integrateNoWaitWithComponent(t, tree, placement, seed, "keep-inner", keepInner, nil)
				} else {
					ml, err = integrateNoWait(tree, nodeOnly, placement)
				}
				if !c.refused {
					if err != nil {
						t.Fatalf("got %v, want the rewrite accepted", err)
					}
					assertDependsOn(t, ml, "web", "hand-written")
					return
				}
				if c.live && c.check {
					if err == nil || !strings.Contains(err.Error(), `Flux Kustomization "flux-system/inner" is written by`) ||
						!strings.Contains(err.Error(), `"flux-system/hand-written"`) || !strings.Contains(err.Error(), `"flux-system/other"`) {
						t.Fatalf("got %v, want the two writers of inner refused, naming hand-written and other", err)
					}
					return
				}
				chain := "ready flux-system/hand-written -> ready flux-system/inner -> apply flux-system/inner -> ready flux-system/web -> apply flux-system/web"
				if c.check {
					chain = "ready flux-system/hand-written -> ready flux-system/inner -> ready flux-system/web -> apply flux-system/web"
				}
				if c.live {
					chain = "ready flux-system/x-inner -> apply flux-system/x-inner -> ready flux-system/web -> apply flux-system/web"
				}
				if !errors.Is(err, fluxstack.ErrUmbrellaDependency) || !strings.Contains(err.Error(), chain) {
					t.Fatalf("got %v, want ErrUmbrellaDependency naming the chain %s", err, chain)
				}
			})
		}
	}
}

// A patched build that keeps a local-config inner, renamed by a prefix, while
// a sibling bundle's directory holds a live inner: the build writes the kept
// copy as x-inner, another object than the live inner, which hand-written's
// wait does not reach. Both wait for web's directory; the kept copy's own
// wait, read from the build, waits for web, which depends on hand-written
// once rewritten, and closes the cycle (go-kure/kure#1133).
func TestReconcileContext_KeptCopyBesideLiveNamesakeWaits(t *testing.T) {
	patch := "apiVersion: kustomize.toolkit.fluxcd.io/v1\nkind: Kustomization\nmetadata:\n  name: inner\n  namespace: flux-system\n  annotations:\n    " + localConfigAnnotation + `: "false"` + "\n"
	build := func(seed, web string) *stack.Cluster {
		k := handWritten("hand-written")
		k.Spec.Wait = true
		if seed != "" {
			k.Spec.Path = seed
			k.Spec.Patches = []kustomize.Patch{{Patch: patch}}
			k.Spec.NamePrefix = "x-"
		}
		var hw client.Object = k
		dropped := "true"
		kept, live := handWritten("inner"), handWritten("inner")
		if web != "" {
			kept.Spec.Wait, kept.Spec.Path = true, web
			live.Spec.Wait, live.Spec.Path = true, web
		}
		var inner client.Object = annotateLocalConfig(kept, &dropped)
		var namesake client.Object = live
		c := noWaitCluster(srBundle("platform",
			stack.NewApplication("platform-app", "default", &fakeAppConfig{objs: []*client.Object{&hw}}),
			stack.NewApplication("seed-app", "default", &fakeAppConfig{objs: []*client.Object{&inner}}),
		), true)
		// The live inner sits in a sibling bundle, outside platform: only
		// hand-written's wait reaches it.
		other := &stack.Node{Name: "other", Bundle: srBundle("other", stack.NewApplication("live-app", "default", &fakeAppConfig{objs: []*client.Object{&namesake}}))}
		other.SetParent(c.Node)
		c.Node.Children = append(c.Node.Children, other)
		return c
	}
	for _, placement := range integratedPlacements {
		t.Run(string(placement), func(t *testing.T) {
			first, err := integrateNoWait(build("", ""), nodeOnly, placement)
			if err != nil {
				t.Fatalf("first integration: %v", err)
			}
			var seed string
			var walk func(l *layout.ManifestLayout)
			walk = func(l *layout.ManifestLayout) {
				for _, r := range l.Resources {
					if r.GetName() == "inner" && droppedByBuild(r) {
						seed = l.FullRepoPath()
					}
				}
				for _, c := range l.Children {
					walk(c)
				}
			}
			walk(first)
			if seed == "" {
				t.Fatal("no layout holds the local-config inner")
			}
			_, err = integrateNoWait(build(seed, hostDirectory(t, first, "web")), nodeOnly, placement)
			if !errors.Is(err, fluxstack.ErrUmbrellaDependency) || !strings.Contains(err.Error(), "ready flux-system/x-inner -> ready flux-system/web -> apply flux-system/web") {
				t.Fatalf("got %v, want ErrUmbrellaDependency naming the chain through the kept copy's own wait", err)
			}
		})
	}
}

// A local-config Kustomization a patched build may keep, that cannot be read
// in typed form (here spec.dependsOn is a string), refuses the tree as an
// unreadable other Kustomization does, instead of being read as depending on
// nothing. Where no build patches what it holds it is not there, and the tree
// is not refused. Nor is it where the patched build's spec.path names a
// single-file (AppFileSingle) layout: Flux finds no directory there, so that
// build holds nothing (go-kure/kure#1197). A single file beside the patched
// build's directory, of the same path, changes nothing: the tree is refused.
func TestReconcileContext_KeptCopyUnreadableIsRefused(t *testing.T) {
	patch := "apiVersion: kustomize.toolkit.fluxcd.io/v1\nkind: Kustomization\nmetadata:\n  name: inner\n  namespace: flux-system\n  annotations:\n    " + localConfigAnnotation + `: "false"` + "\n"
	build := func(seed string, patched bool) *stack.Cluster {
		k := handWritten("hand-written")
		if seed != "" {
			k.Spec.Path = seed
		}
		if patched {
			k.Spec.Patches = []kustomize.Patch{{Patch: patch}}
		}
		var hw client.Object = k
		dropped := "true"
		var inner client.Object = annotateLocalConfig(handWritten("inner"), &dropped)
		return noWaitCluster(srBundle("platform",
			stack.NewApplication("platform-app", "default", &fakeAppConfig{objs: []*client.Object{&hw}}),
			stack.NewApplication("seed-app", "default", &fakeAppConfig{objs: []*client.Object{&inner}}),
		), true)
	}
	for _, placement := range integratedPlacements {
		first, err := integrateNoWait(build("", false), nodeOnly, placement)
		if err != nil {
			t.Fatalf("%s: first integration: %v", placement, err)
		}
		seed := hostDirectory(t, first, "inner")
		for name, c := range map[string]struct{ patched, onFile, beside bool }{
			"patch":                          {patched: true},
			"no patch":                       {},
			"patch, path names a file":       {patched: true, onFile: true},
			"patch, a file beside the build": {patched: true, beside: true},
		} {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				specPath := seed
				if c.onFile {
					specPath = path.Join(seed, "helper")
				}
				ml, err := integrateNoWait(build(specPath, c.patched), nodeOnly, placement)
				if err != nil {
					t.Fatalf("got %v, want the integration accepted", err)
				}
				var inner client.Object
				var host, parent *layout.ManifestLayout
				var walk func(l, up *layout.ManifestLayout)
				walk = func(l, up *layout.ManifestLayout) {
					for _, r := range l.Resources {
						if r.GetName() == "inner" && droppedByBuild(r) {
							inner, host, parent = r, l, up
						}
					}
					for _, child := range l.Children {
						walk(child, l)
					}
				}
				walk(ml, nil)
				if inner == nil {
					t.Fatal("no layout holds the local-config inner")
				}
				if c.onFile {
					// inner moves into helper, a single-file child of its
					// directory, which hand-written's spec.path names.
					host.Resources = slices.DeleteFunc(host.Resources, func(o client.Object) bool { return o == inner })
					host.Children = append(host.Children, &layout.ManifestLayout{
						Name: "helper", Namespace: host.FullRepoPath(), ApplicationFileMode: layout.AppFileSingle,
						Resources: []client.Object{inner},
					})
				}
				if c.beside {
					// A single file of the same path as inner's directory,
					// after it in the walk: the writers accept both, and the
					// file must not hide the directory hand-written builds.
					if parent == nil {
						t.Fatal("inner's directory is the top of the tree, want one below it")
					}
					parent.Children = append(parent.Children, &layout.ManifestLayout{
						Name: host.Name, Namespace: parent.FullRepoPath(), ApplicationFileMode: layout.AppFileSingle,
						Resources: []client.Object{unstructuredObj("v1", "ConfigMap", "default", "beside")},
					})
				}
				replaceUnstructured(t, ml, inner, func(u *unstructured.Unstructured) {
					if err := unstructured.SetNestedField(u.Object, "web", "spec", "dependsOn"); err != nil {
						t.Fatal(err)
					}
				})
				_, err = fluxstack.UmbrellaTerminals(ml)
				if !c.patched || c.onFile {
					if err != nil {
						t.Fatalf("got %v, want the tree accepted: no build may keep inner", err)
					}
					return
				}
				if !errors.Is(err, fluxstack.ErrUmbrellaTerminals) || !strings.Contains(err.Error(), `read Flux Kustomization "flux-system/inner" in typed form`) {
					t.Fatalf("got %v, want ErrUmbrellaTerminals naming the unreadable inner", err)
				}
			})
		}
	}
}

// A recorded member whose spec.path names a single-file layout writes nothing
// of it (go-kure/kure#1197): Flux finds no directory there. demo applies only
// its own directory (FluxIntegratedPerLayout); m1 builds apps, which lists
// helper, an AppFileSingle file holding o; o builds apps on m1's Source and its
// patch keeps inner, a local-config Kustomization that cannot be read in
// typed form, so the tree is refused. m2, on another Source, names helper's
// path: it is not o's writer, and the tree is still refused.
func TestReconcileContext_RecordedMemberOnAFileWritesNothing(t *testing.T) {
	patch := "apiVersion: kustomize.toolkit.fluxcd.io/v1\nkind: Kustomization\nmetadata:\n  name: inner\n  namespace: flux-system\n  annotations:\n    " + localConfigAnnotation + `: "false"` + "\n"
	for name, withM2 := range map[string]bool{"m1 only": false, "m2 names the file": true} {
		t.Run(name, func(t *testing.T) {
			root := &layout.ManifestLayout{Name: "demo", Namespace: "clusters", FluxPlacement: layout.FluxIntegratedPerLayout}
			apps := &layout.ManifestLayout{Name: "apps", Namespace: root.FullRepoPath()}
			helper := &layout.ManifestLayout{Name: "helper", Namespace: apps.FullRepoPath(), ApplicationFileMode: layout.AppFileSingle}
			root.Children = []*layout.ManifestLayout{apps}
			apps.Children = []*layout.ManifestLayout{helper}

			m1 := handWritten("m1")
			m1.Spec.Path = apps.FullRepoPath()
			root.Resources = []client.Object{m1}
			recs := []layout.FluxKustomizationRecord{{
				Namespace: "flux-system", Name: "m1", Path: apps.FullRepoPath(),
				SourceKind: "GitRepository", SourceNamespace: "flux-system", SourceName: "flux-system",
			}}
			if withM2 {
				m2 := handWritten("m2")
				m2.Spec.Path = helper.FullRepoPath()
				m2.Spec.SourceRef.Name = "other"
				root.Resources = append(root.Resources, m2)
				recs = append(recs, layout.FluxKustomizationRecord{
					Namespace: "flux-system", Name: "m2", Path: helper.FullRepoPath(),
					SourceKind: "GitRepository", SourceNamespace: "flux-system", SourceName: "other",
				})
			}
			root.SetFluxKustomizations(recs)

			o := handWritten("o")
			o.Spec.Path = apps.FullRepoPath()
			o.Spec.Patches = []kustomize.Patch{{Patch: patch}}
			helper.Resources = []client.Object{o}
			dropped := "true"
			inner := annotateLocalConfig(handWritten("inner"), &dropped)
			apps.Resources = []client.Object{inner}
			replaceUnstructured(t, root, inner, func(u *unstructured.Unstructured) {
				if err := unstructured.SetNestedField(u.Object, "web", "spec", "dependsOn"); err != nil {
					t.Fatal(err)
				}
			})

			_, err := fluxstack.UmbrellaTerminals(root)
			if !errors.Is(err, fluxstack.ErrUmbrellaTerminals) || !strings.Contains(err.Error(), `read Flux Kustomization "flux-system/inner" in typed form`) {
				t.Fatalf("got %v, want ErrUmbrellaTerminals naming the unreadable inner", err)
			}
		})
	}
}

// A Kustomization the tree holds as local config is read only where a build
// that patches what it holds may keep it. Without one it is not there: a live
// namesake keeps its own dependsOn, none here, so platform's terminal leaves
// stay a and b, and a dependsOn naming it from outside the tree reads nothing
// of it, so inner's dependency on web closes no cycle.
func TestReconcileContext_LocalConfigUnpatchedIsNotRead(t *testing.T) {
	dropped := "true"
	cases := map[string]struct {
		objs func() []client.Object
		want []string
	}{
		"live namesake": {func() []client.Object {
			return []client.Object{handWritten("a"), handWritten("b"), annotateLocalConfig(handWritten("a", "b"), &dropped)}
		}, []string{"a", "b"}},
		"dependsOn from outside the tree": {func() []client.Object {
			return []client.Object{handWritten("hand-written", "inner"), annotateLocalConfig(handWritten("inner", "web"), &dropped)}
		}, []string{"hand-written"}},
	}
	for name, c := range cases {
		for _, placement := range integratedPlacements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				var objs []*client.Object
				for _, o := range c.objs() {
					objs = append(objs, &o)
				}
				app := stack.NewApplication("platform-app", "default", &fakeAppConfig{objs: objs})
				ml, err := integrateNoWait(noWaitCluster(srBundle("platform", app), true), nodeOnly, placement)
				if err != nil {
					t.Fatalf("got %v, want the rewrite accepted", err)
				}
				assertDependsOn(t, ml, "web", c.want...)
			})
		}
	}
}

// hostDirectory is the directory of the layout of ml that holds the object
// named name.
func hostDirectory(t *testing.T, ml *layout.ManifestLayout, name string) string {
	t.Helper()
	var dir string
	var walk func(l *layout.ManifestLayout)
	walk = func(l *layout.ManifestLayout) {
		for _, r := range l.Resources {
			if r.GetName() == name {
				dir = l.FullRepoPath()
			}
		}
		for _, c := range l.Children {
			walk(c)
		}
	}
	walk(ml)
	if dir == "" {
		t.Fatalf("no layout holds %q", name)
	}
	return dir
}

// A Kustomization kept from the tree in place of a generated one must read
// what the generated one would, on this cluster: its spec.path is a directory
// of this tree only then. One on another Source, or with spec.kubeConfig, is
// a namesake, and refused as one. Its sourceRef namespace may be left out.
//
// Every Kustomization of the first integration is tried: the bundles' (kept
// where the integration adds its objects) and, under FluxIntegratedPerLayout
// with grouping by name, the layouts' (kept where it places them).
func TestReconcileContext_KeptKustomizationSource(t *testing.T) {
	cases := map[string]struct {
		edit func(k *kustv1.Kustomization)
		want string
	}{
		"another Source":    {func(k *kustv1.Kustomization) { k.Spec.SourceRef.Name = "flux-system-next" }, "on GitRepository flux-system/flux-system-next"},
		"another cluster":   {func(k *kustv1.Kustomization) { k.Spec.KubeConfig = remoteCluster() }, "with spec.kubeConfig"},
		"namespace omitted": {func(k *kustv1.Kustomization) { k.Spec.SourceRef.Namespace = "" }, ""},
	}
	ruleSets := map[string]layout.LayoutRules{
		"per bundle": withPlacement(nodeOnly, layout.FluxIntegratedPerBundle),
		"per layout": withPlacement(nodeOnly, layout.FluxIntegratedPerLayout),
		"per layout, grouped by name": {
			BundleGrouping: layout.GroupByName, ApplicationGrouping: layout.GroupByName,
			FluxPlacement: layout.FluxIntegratedPerLayout,
		},
	}
	build := func() *stack.Cluster { return noWaitCluster(srBundle("platform", cmApp("platform-app")), false) }
	for name, c := range cases {
		for rulesName, rules := range ruleSets {
			first, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(build(), rules)
			if err != nil {
				t.Fatalf("first integration: %v", err)
			}
			for _, kept := range kustomizations(first) {
				t.Run(name+"/"+rulesName+"/"+kept.Name, func(t *testing.T) {
					c2 := build()
					ml, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(c2, rules)
					if err != nil {
						t.Fatalf("first integration: %v", err)
					}
					c.edit(findKept(ml, kept.Name))
					err = fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, c2, rules)
					if c.want == "" {
						if err != nil {
							t.Fatalf("got %v, want the Kustomization kept", err)
						}
						return
					}
					if err == nil || !strings.Contains(err.Error(), `already has Flux Kustomization "`+kept.Name+`"`) || !strings.Contains(err.Error(), c.want) {
						t.Fatalf("got %v, want %q refused as a namesake %s", err, kept.Name, c.want)
					}
				})
			}
		}
	}
}

// A kept Kustomization read from YAML is unstructured, and there a
// spec.kubeConfig of null is no kubeConfig, as it is once decoded into the
// typed object: the Kustomization is kept. One that names a kubeConfig is
// still a namesake.
func TestReconcileContext_KeptUnstructuredKubeConfig(t *testing.T) {
	cases := map[string]struct {
		kubeConfig any
		want       string
	}{
		"null":            {nil, ""},
		"another cluster": {map[string]any{"secretRef": map[string]any{"name": "remote-kubeconfig"}}, "with spec.kubeConfig"},
	}
	rules := withPlacement(nodeOnly, layout.FluxIntegratedPerBundle)
	build := func() *stack.Cluster { return noWaitCluster(srBundle("platform", cmApp("platform-app")), false) }
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			c2 := build()
			ml, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(c2, rules)
			if err != nil {
				t.Fatalf("first integration: %v", err)
			}
			kept := findKept(ml, "platform")
			replaceUnstructured(t, ml, kept, func(u *unstructured.Unstructured) {
				if err := unstructured.SetNestedField(u.Object, runtime.DeepCopyJSONValue(c.kubeConfig), "spec", "kubeConfig"); err != nil {
					t.Fatal(err)
				}
			})
			err = fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, c2, rules)
			if c.want == "" {
				if err != nil {
					t.Fatalf("got %v, want the Kustomization kept", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), `already has Flux Kustomization "platform"`) || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("got %v, want platform refused as a namesake %s", err, c.want)
			}
		})
	}
}

// A per-layout Kustomization kept where the Source the integration would
// give it cannot be resolved is kept unjudged on its Source, but not when it
// sets spec.kubeConfig: what it applies goes to another cluster, so it does
// not stand in for the slot, and the integration refuses the tree as it
// refuses that tree without it. On this cluster it is kept.
//
// The slot is the root node's layout of a tree whose every bundle names the
// generated Source shared: that Source is delivered inside what the layout
// applies, so the layout has no Source to resolve. The kept copy reads the
// bootstrap's Source.
func TestReconcileContext_KeptUnresolvedSourceOnAnotherCluster(t *testing.T) {
	const rootDir = "prod/platform"
	build := urlAndPlainTree(nil)
	rules := propertyGroupings["nodeOnly"]
	rules.FluxPlacement = layout.FluxIntegratedPerLayout
	rules.ClusterName = "prod"
	rootCR := rootLayoutCRName(t, build, rules, rootDir)
	bootstrap := kustv1.CrossNamespaceSourceReference{Kind: "GitRepository", Name: "flux-system", Namespace: "flux-system"}
	keptOnShared := func(t *testing.T, form string, change func(k *kustv1.Kustomization)) (*layout.ManifestLayout, *stack.Cluster) {
		ml, c := keptIn(t, build, rules, rootCR, form, change, nil)
		c.Node.Children[0].Bundle.SourceRef = &stack.SourceRef{Kind: "GitRepository", Name: "shared"}
		return ml, c
	}
	for _, form := range keptForms {
		t.Run(form+"/another cluster", func(t *testing.T) {
			ml, c := keptOnShared(t, form, func(k *kustv1.Kustomization) {
				k.Spec.SourceRef = bootstrap
				k.Spec.KubeConfig = remoteCluster()
			})
			err := integrate(ml, c, rules)
			want := fmt.Sprintf("Flux Kustomization %q (spec.path %q) %s GitRepository %q", rootCR, rootDir, ownSourceRefusal, "shared")
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("got %v, want the tree refused: %s", err, want)
			}
		})
		t.Run(form+"/this cluster", func(t *testing.T) {
			ml, c := keptOnShared(t, form, func(k *kustv1.Kustomization) {
				k.Spec.SourceRef = bootstrap
			})
			if err := integrate(ml, c, rules); err != nil {
				t.Fatalf("got %v, want the Kustomization kept", err)
			}
		})
	}
}

// replaceUnstructured replaces obj, wherever ml holds it, with its
// unstructured form after edit.
func replaceUnstructured(t *testing.T, ml *layout.ManifestLayout, obj client.Object, edit func(*unstructured.Unstructured)) {
	t.Helper()
	m, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		t.Fatal(err)
	}
	u := &unstructured.Unstructured{Object: m}
	u.SetGroupVersionKind(kustv1.GroupVersion.WithKind(kustv1.KustomizationKind))
	edit(u)
	var walk func(l *layout.ManifestLayout) bool
	walk = func(l *layout.ManifestLayout) bool {
		for i, r := range l.Resources {
			if r == obj {
				l.Resources[i] = u
				return true
			}
		}
		for _, c := range l.Children {
			if walk(c) {
				return true
			}
		}
		return false
	}
	if !walk(ml) {
		t.Fatalf("%s is not in the layout", obj.GetName())
	}
}

// The reconcile-order check of every integration reads a kept
// Kustomization's health checks the same way. platform, kept without wait,
// health-checks a Kustomization while web depends on platform: on web itself,
// platform's readiness waits for web's, which waits for platform's. A
// healthCheckExprs predicate deciding it is read as readiness too (it may be
// one), so each is refused, and on sub, which web's apply creates, the same
// way.
func TestReconcileContext_PlacedOrderReadsExpressions(t *testing.T) {
	build := func() *stack.Cluster {
		c := noWaitCluster(srBundle("platform", cmApp("platform-app")), true)
		web := c.Node.Children[1]
		sub := &stack.Node{Name: "sub", Bundle: srBundle("sub", cmApp("sub-app"))}
		web.Children = []*stack.Node{sub}
		sub.SetParent(web)
		return c
	}
	cases := map[string]struct {
		checked string
		expr    bool
		refused bool
	}{
		"web":                 {checked: "web", refused: true},
		"web, by a predicate": {checked: "web", expr: true, refused: true},
		"sub":                 {checked: "sub", refused: true},
		"sub, by a predicate": {checked: "sub", expr: true, refused: true},
	}
	for name, c := range cases {
		for _, placement := range integratedPlacements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				rules := withPlacement(nodeOnly, placement)
				cl := build()
				ml, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(cl, rules)
				if err != nil {
					t.Fatalf("first integration: %v", err)
				}
				k := findKept(ml, "platform")
				k.Spec.Wait = false
				k.Spec.HealthChecks = []metaapi.NamespacedObjectKindReference{{APIVersion: kustv1.GroupVersion.String(), Kind: "Kustomization", Name: findKept(ml, c.checked).Name}}
				if c.expr {
					k.Spec.HealthCheckExprs = []kustomize.CustomHealthCheck{readyExpr()}
				}
				err = fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, cl, rules)
				if c.refused {
					if err == nil || !strings.Contains(err.Error(), "can never all become Ready") {
						t.Fatalf("got %v, want a reconcile-order refusal (%s)", err, describeKustomizations(ml))
					}
					return
				}
				if err != nil {
					t.Fatalf("IntegrateWithLayout: %v", err)
				}
			})
		}
	}
}

// As above, sub decided by a predicate (read as readiness), but an
// application also emits hand-written, building web's directory on the
// tree's Source, so that it writes sub too. Whoever emits it, sub's creation
// still waits for web's apply: a Kustomization the tree's own build also
// writes is not modelled as an alternate creator (whether it applies depends
// on controller settings the tree does not carry: a watch label selector,
// cross-namespace Source references being blocked, a default service
// account), so the cycle is refused, a false refusal where hand-written
// applies freely (go-kure/kure#1094). Also when platform's application is
// written as one file into platform's directory.
func TestReconcileContext_PlacedOrderAnotherWriter(t *testing.T) {
	cases := map[string]struct {
		emitter string
		single  bool
	}{
		"emitted by platform":           {emitter: "platform"},
		"emitted by platform, one file": {emitter: "platform", single: true},
		"emitted by sub":                {emitter: "sub"},
	}
	build := func(emitter string, app *stack.Application) *stack.Cluster {
		platformApp, subApp := cmApp("platform-app"), cmApp("sub-app")
		if emitter == "platform" {
			platformApp = app
		} else if emitter == "sub" {
			subApp = app
		}
		c := noWaitCluster(srBundle("platform", platformApp), true)
		web := c.Node.Children[1]
		sub := &stack.Node{Name: "sub", Bundle: srBundle("sub", subApp)}
		web.Children = []*stack.Node{sub}
		sub.SetParent(web)
		return c
	}
	for name, c := range cases {
		for _, placement := range integratedPlacements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				rules := withPlacement(nodeOnly, placement)
				if c.single {
					rules.ApplicationGrouping = layout.GroupByName
				}
				integrate := func(cl *stack.Cluster) (*layout.ManifestLayout, error) {
					if !c.single {
						return fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(cl, rules)
					}
					ml := singleFile(t, cl, rules, "platform-app")
					return ml, fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, cl, rules)
				}
				off, err := integrate(build("", nil))
				if err != nil {
					t.Fatalf("without hand-written: %v", err)
				}
				k := handWritten("hand-written")
				k.Spec.Path = path.Clean(findKept(off, "web").Spec.Path)
				var o client.Object = k
				cl := build(c.emitter, stack.NewApplication(c.emitter+"-app", "default", &fakeAppConfig{objs: []*client.Object{&o}}))
				ml, err := integrate(cl)
				if err != nil {
					t.Fatalf("first integration: %v", err)
				}
				p := findKept(ml, "platform")
				p.Spec.Wait = false
				p.Spec.HealthChecks = []metaapi.NamespacedObjectKindReference{{APIVersion: kustv1.GroupVersion.String(), Kind: "Kustomization", Name: findKept(ml, "sub").Name}}
				p.Spec.HealthCheckExprs = []kustomize.CustomHealthCheck{currentExpr(kustv1.GroupVersion.String(), "Kustomization")}
				err = fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, cl, rules)
				if err == nil || !strings.Contains(err.Error(), "can never all become Ready") {
					t.Fatalf("got %v, want a reconcile-order refusal (%s)", err, describeKustomizations(ml))
				}
			})
		}
	}
}
