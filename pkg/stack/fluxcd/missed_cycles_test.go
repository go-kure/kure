package fluxcd_test

import (
	"errors"
	"slices"
	"strconv"
	"strings"
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	"github.com/fluxcd/pkg/apis/kustomize"
	metaapi "github.com/fluxcd/pkg/apis/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// The reconcile-order checks miss no cycle the tree shows (go-kure/kure#1133):
// a health check on any kind waits for the object to exist, so for the apply
// of the Kustomization whose build writes it (F1), and on integration every
// edge of the tree is judged, not only those the integration rewrote (F2).

// objApp is an application whose objects are objs.
func objApp(name string, objs ...client.Object) *stack.Application {
	var out []*client.Object
	for _, o := range objs {
		out = append(out, &o)
	}
	return stack.NewApplication(name, "default", &fakeAppConfig{objs: out})
}

// unstructuredObj is an object of apiVersion and kind named namespace/name.
func unstructuredObj(apiVersion, kind, namespace, name string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetAPIVersion(apiVersion)
	u.SetKind(kind)
	u.SetName(name)
	u.SetNamespace(namespace)
	return u
}

// listOf is a v1 List holding items.
func listOf(items ...*unstructured.Unstructured) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetAPIVersion("v1")
	u.SetKind("List")
	var raw []any
	for _, i := range items {
		raw = append(raw, i.Object)
	}
	u.Object["items"] = raw
	return u
}

// layoutHolding returns the layout of ml whose resources hold an object named
// name, or the one named name when none does.
func layoutHolding(t *testing.T, ml *layout.ManifestLayout, name string) *layout.ManifestLayout {
	t.Helper()
	var byObject, byName *layout.ManifestLayout
	var walk func(l *layout.ManifestLayout)
	walk = func(l *layout.ManifestLayout) {
		for _, r := range l.Resources {
			if r != nil && r.GetName() == name && byObject == nil {
				byObject = l
			}
		}
		if l.Name == name && byName == nil {
			byName = l
		}
		for _, c := range l.Children {
			if c != nil {
				walk(c)
			}
		}
	}
	walk(ml)
	if byObject != nil {
		return byObject
	}
	if byName == nil {
		t.Fatalf("no layout holds or is named %q", name)
	}
	return byName
}

// checkerCase is the go-kure/kure#1133 F1 repro: platform's application emits
// hand-written, web depends on platform, which the setting writes as one on
// hand-written, and hand-written, edited by check, health-checks what web's
// application writes. webApps replaces web's applications, rules edits the
// rules and walked the walked layout before it is integrated.
type checkerCase struct {
	check   func(k *kustv1.Kustomization)
	webApps []*stack.Application
	rules   func(r *layout.LayoutRules)
	walked  func(t *testing.T, ml *layout.ManifestLayout)
}

func (c checkerCase) integrate(t *testing.T, placement layout.FluxPlacement) (*layout.ManifestLayout, error) {
	t.Helper()
	k := handWritten("hand-written")
	c.check(k)
	cluster := noWaitCluster(srBundle("platform", objApp("platform-app", k)), true)
	if c.webApps != nil {
		cluster.Node.Children[1].Bundle.Applications = c.webApps
	}
	rules := withPlacement(nodeOnly, placement)
	if c.rules != nil {
		c.rules(&rules)
	}
	ml, err := layout.WalkCluster(cluster, rules)
	if err != nil {
		t.Fatalf("WalkCluster: %v", err)
	}
	if c.walked != nil {
		c.walked(t, ml)
	}
	return ml, fluxstack.NewLayoutIntegrator(noWaitGenerator()).IntegrateWithLayout(ml, cluster, rules)
}

// checks is a health check on apiVersion/kind namespace/name.
func checks(apiVersion, kind, namespace, name string) func(k *kustv1.Kustomization) {
	return func(k *kustv1.Kustomization) {
		k.Spec.HealthChecks = append(k.Spec.HealthChecks, metaapi.NamespacedObjectKindReference{APIVersion: apiVersion, Kind: kind, Namespace: namespace, Name: name})
	}
}

// F1: hand-written becomes Ready only once the object it health-checks exists,
// so once web applied, and web applies only once hand-written is Ready. Flux
// fetches the object before it evaluates an expression, so one that is always
// current changes nothing. Each is refused through the rewrite.
func TestMissedCycles_HealthCheckOnObjectIsACycle(t *testing.T) {
	cm := checks("v1", "ConfigMap", "default", "web-app-cm")
	cases := map[string]checkerCase{
		"a ConfigMap": {check: cm},
		"with an expression that is always current": {check: func(k *kustv1.Kustomization) {
			cm(k)
			k.Spec.HealthCheckExprs = []kustomize.CustomHealthCheck{currentExpr("v1", "ConfigMap")}
		}},
		"a Deployment with no apiVersion": {
			check:   checks("", "Deployment", "default", "web-deploy"),
			webApps: []*stack.Application{objApp("web-app", unstructuredObj("apps/v1", "Deployment", "default", "web-deploy"))},
		},
		"a List item": {
			check:   checks("v1", "ConfigMap", "default", "web-list-cm"),
			webApps: []*stack.Application{objApp("web-app", listOf(unstructuredObj("v1", "ConfigMap", "default", "web-list-cm")))},
		},
		"an application written as one file": {
			check: cm,
			rules: func(r *layout.LayoutRules) { r.ApplicationGrouping = layout.GroupByName },
			walked: func(t *testing.T, ml *layout.ManifestLayout) {
				layoutHolding(t, ml, "web-app").ApplicationFileMode = layout.AppFileSingle
			},
		},
	}
	for name, c := range cases {
		for _, placement := range integratedPlacements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				_, err := c.integrate(t, placement)
				if !errors.Is(err, fluxstack.ErrUmbrellaDependency) || !strings.Contains(err.Error(), "ready flux-system/hand-written -> apply flux-system/web") {
					t.Fatalf("got %v, want ErrUmbrellaDependency naming the chain through web's apply", err)
				}
			})
		}
	}
}

// The controls: a health check on an object no Kustomization of the tree
// writes, one the Flux bootstrap writes, one on another cluster, or one Flux
// ignores under wait adds no edge, and the rewrite is accepted.
func TestMissedCycles_HealthCheckWithoutAWriter(t *testing.T) {
	cm := checks("v1", "ConfigMap", "default", "web-app-cm")
	cases := map[string]checkerCase{
		"an object no build writes":       {check: checks("v1", "ConfigMap", "default", "nowhere-cm")},
		"the object in another namespace": {check: checks("v1", "ConfigMap", "elsewhere", "web-app-cm")},
		"another kind of the same name":   {check: checks("v1", "Secret", "default", "web-app-cm")},
		"an object the bootstrap writes": {
			check: checks("v1", "ConfigMap", "default", "root-cm"),
			walked: func(t *testing.T, ml *layout.ManifestLayout) {
				ml.Resources = append(ml.Resources, unstructuredObj("v1", "ConfigMap", "default", "root-cm"))
			},
		},
		"a generated ConfigMap, whose hashed name is not known": {
			check: checks("v1", "ConfigMap", "default", "web-values-5h7k8m2c9t"),
			walked: func(t *testing.T, ml *layout.ManifestLayout) {
				l := layoutHolding(t, ml, "web-app-cm")
				l.ConfigMapGenerators = append(l.ConfigMapGenerators, layout.ConfigMapGeneratorSpec{Name: "web-values", Files: []string{"values.yaml"}})
				l.ExtraFiles = append(l.ExtraFiles, layout.ExtraFile{Name: "values.yaml", Content: []byte("k: v\n")})
			},
		},
		"an object written with no namespace": {
			check:   checks("v1", "ConfigMap", "default", "web-bare-cm"),
			webApps: []*stack.Application{objApp("web-app", unstructuredObj("v1", "ConfigMap", "", "web-bare-cm"))},
		},
		"on another cluster": {check: func(k *kustv1.Kustomization) {
			cm(k)
			k.Spec.KubeConfig = remoteCluster()
		}},
		"with wait, which ignores health checks": {check: func(k *kustv1.Kustomization) {
			cm(k)
			k.Spec.Wait = true
		}},
	}
	for name, c := range cases {
		for _, placement := range integratedPlacements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				ml, err := c.integrate(t, placement)
				if err != nil {
					t.Fatalf("IntegrateWithLayout: %v", err)
				}
				assertDependsOn(t, ml, "web", "hand-written")
			})
		}
	}
}

// checkCluster is demo -> {a, b}: bundle a health-checks check, and b, whose
// application writes the Deployment default/b-deploy, depends on a.
func checkCluster(check stack.HealthCheck) *stack.Cluster {
	a := srBundle("a", objApp("a-app", unstructuredObj("apps/v1", "Deployment", "default", "a-deploy")))
	a.HealthChecks = []stack.HealthCheck{check}
	b := srBundle("b", objApp("b-app", unstructuredObj("apps/v1", "Deployment", "default", "b-deploy")))
	b.DependsOn = []*stack.Bundle{a}
	an := &stack.Node{Name: "a", Bundle: a}
	bn := &stack.Node{Name: "b", Bundle: b}
	root := &stack.Node{Name: "demo", Children: []*stack.Node{an, bn}}
	an.SetParent(root)
	bn.SetParent(root)
	return &stack.Cluster{Name: "demo", Node: root}
}

// F1 on the Kustomizations the integration places, without the setting: a
// health-checks b's Deployment, and b depends on a. Refused in every
// placement; a check on a's own Deployment is no cycle. GenerateFromLayout,
// which knows no tree, is unchanged.
func TestMissedCycles_MemberHealthCheckOnObject(t *testing.T) {
	cycle := stack.HealthCheck{APIVersion: "apps/v1", Kind: "Deployment", Namespace: "default", Name: "b-deploy"}
	own := stack.HealthCheck{APIVersion: "apps/v1", Kind: "Deployment", Namespace: "default", Name: "a-deploy"}
	for _, placement := range append(slices.Clone(integratedPlacements), layout.FluxSeparate) {
		t.Run(string(placement), func(t *testing.T) {
			_, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(checkCluster(cycle), withPlacement(nodeOnly, placement))
			if err == nil || !strings.Contains(err.Error(), "can never all become Ready") || !strings.Contains(err.Error(), "ready flux-system/a -> apply flux-system/b") {
				t.Fatalf("got %v, want the cycle through b's apply refused", err)
			}
			if _, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(checkCluster(own), withPlacement(nodeOnly, placement)); err != nil {
				t.Fatalf("a health check on a's own Deployment: %v", err)
			}
		})
	}
	t.Run("GenerateFromLayout", func(t *testing.T) {
		c := checkCluster(cycle)
		ml, err := layout.WalkCluster(c, nodeOnly)
		if err != nil {
			t.Fatalf("WalkCluster: %v", err)
		}
		if _, err := fluxstack.NewResourceGenerator().GenerateFromLayout(ml, c); err != nil {
			t.Fatalf("GenerateFromLayout: %v", err)
		}
	})
}

// handWrittenIn returns the Flux Kustomization hand-written as ml holds it.
func handWrittenIn(t *testing.T, ml *layout.ManifestLayout) *kustv1.Kustomization {
	t.Helper()
	for _, k := range kustomizations(ml) {
		if k.Name == "hand-written" {
			return k
		}
	}
	t.Fatalf("no hand-written in %s", describeKustomizations(ml))
	return nil
}

// records lists every layout's Flux Kustomization record, for a comparison.
func records(ml *layout.ManifestLayout) [][]layout.FluxKustomizationRecord {
	var out [][]layout.FluxKustomizationRecord
	var walk func(l *layout.ManifestLayout)
	walk = func(l *layout.ManifestLayout) {
		out = append(out, l.FluxKustomizations())
		for _, c := range l.Children {
			if c != nil {
				walk(c)
			}
		}
	}
	walk(ml)
	return out
}

// F2: the first integration writes web's dependsOn on platform as one on
// hand-written. hand-written then waits for web's directory, which web's
// Kustomization sits in, so it becomes Ready only once web is, and web applies
// only once it is. The second integration rewrites nothing, and judges the
// tree: refused, and the layout is left as it was, its records included.
func TestMissedCycles_ReintegrationRechecksEveryEdge(t *testing.T) {
	for _, placement := range integratedPlacements {
		t.Run(string(placement), func(t *testing.T) {
			dir := webDirectory(t, placement)
			k := handWritten("hand-written")
			c := noWaitCluster(srBundle("platform", objApp("platform-app", k)), true)
			rules := withPlacement(nodeOnly, placement)
			ml, err := fluxstack.NewLayoutIntegrator(noWaitGenerator()).CreateLayoutWithResources(c, rules)
			if err != nil {
				t.Fatalf("CreateLayoutWithResources: %v", err)
			}
			assertDependsOn(t, ml, "web", "hand-written")
			held := handWrittenIn(t, ml)
			held.Spec.Wait, held.Spec.Path = true, dir
			before, beforeRecords := describeKustomizations(ml), records(ml)
			err = fluxstack.NewLayoutIntegrator(noWaitGenerator()).IntegrateWithLayout(ml, c, rules)
			if err == nil || !strings.Contains(err.Error(), "can never all become Ready") || !strings.Contains(err.Error(), "ready flux-system/hand-written -> ready flux-system/web") {
				t.Fatalf("got %v, want the cycle through hand-written's wait refused", err)
			}
			if after := describeKustomizations(ml); after != before {
				t.Errorf("after a refused integration %s, want %s", after, before)
			}
			if !slices.EqualFunc(records(ml), beforeRecords, slices.Equal) {
				t.Errorf("a refused integration changed the records: %v, want %v", records(ml), beforeRecords)
			}
		})
	}
}

// A cycle among Kustomizations the integration did not place is their
// author's: platform's application emits x and y, which depend on each
// other. Without the setting platform waits for both and web depends on
// platform; under it, which follows those Kustomizations, web health-checks
// x. No Kustomization the integration placed is on the cycle, so the tree is
// accepted.
func TestMissedCycles_NonMemberCycleIsTheAuthors(t *testing.T) {
	for _, placement := range integratedPlacements {
		t.Run(string(placement), func(t *testing.T) {
			platform := srBundle("platform", objApp("platform-app", handWritten("x", "y"), handWritten("y", "x")))
			platform.Wait = boolPtr(true)
			if _, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(noWaitCluster(platform, true), withPlacement(nodeOnly, placement)); err != nil {
				t.Fatalf("CreateLayoutWithResources: %v", err)
			}
		})
		t.Run("UmbrellasDoNotWait/"+string(placement), func(t *testing.T) {
			c := noWaitCluster(srBundle("platform", objApp("platform-app", handWritten("x", "y"), handWritten("y", "x"), handWritten("z"))), false)
			web := c.Node.Children[1].Bundle
			web.Wait = boolPtr(false)
			web.HealthChecks = []stack.HealthCheck{{APIVersion: kustv1.GroupVersion.String(), Kind: "Kustomization", Namespace: "flux-system", Name: "x"}}
			if _, err := fluxstack.NewLayoutIntegrator(noWaitGenerator()).CreateLayoutWithResources(c, withPlacement(nodeOnly, placement)); err != nil {
				t.Fatalf("CreateLayoutWithResources: %v", err)
			}
		})
	}
}

// pairCluster is demo -> {a, b-longer}: a's application writes aObjs and a
// health-checks check; b-longer, whose application writes bObjs, depends on
// a. Its name is longer than a's, so its spec.path is too.
func pairCluster(check stack.HealthCheck, aObjs, bObjs []client.Object) *stack.Cluster {
	a := srBundle("a", objApp("a-app", aObjs...))
	a.HealthChecks = []stack.HealthCheck{check}
	b := srBundle("b-longer", objApp("b-app", bObjs...))
	b.DependsOn = []*stack.Bundle{a}
	an := &stack.Node{Name: "a", Bundle: a}
	bn := &stack.Node{Name: "b-longer", Bundle: b}
	root := &stack.Node{Name: "demo", Children: []*stack.Node{an, bn}}
	an.SetParent(root)
	bn.SetParent(root)
	return &stack.Cluster{Name: "demo", Node: root}
}

// integrateWalked walks c, edits the walked layout with walked, and
// integrates it without the setting.
func integrateWalked(t *testing.T, c *stack.Cluster, placement layout.FluxPlacement, walked func(ml *layout.ManifestLayout)) error {
	t.Helper()
	rules := withPlacement(nodeOnly, placement)
	ml, err := layout.WalkCluster(c, rules)
	if err != nil {
		t.Fatalf("WalkCluster: %v", err)
	}
	if walked != nil {
		walked(ml)
	}
	return fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, c, rules)
}

// findLayout returns the layout of ml whose resources hold an object named
// name, or nil.
func findLayout(ml *layout.ManifestLayout, name string) *layout.ManifestLayout {
	for _, r := range ml.Resources {
		if r != nil && r.GetName() == name {
			return ml
		}
	}
	for _, c := range ml.Children {
		if c != nil {
			if l := findLayout(c, name); l != nil {
				return l
			}
		}
	}
	return nil
}

// A health check adds an edge only onto the apply of the one member certain
// to write the checked object; a match that is not certain adds none, and a
// tree that converges is not refused for it. In each case a health-checks
// an object, b-longer depends on a, and b-longer's build holds a match:
// with the edge, a would wait for b-longer's apply, and b-longer for a's
// readiness.
func TestMissedCycles_UncertainWriterAddsNoEdge(t *testing.T) {
	cm := func(ns, name string) client.Object { return unstructuredObj("v1", "ConfigMap", ns, name) }
	check := func(ns, name string) stack.HealthCheck {
		return stack.HealthCheck{APIVersion: "v1", Kind: "ConfigMap", Namespace: ns, Name: name}
	}
	cases := map[string]struct {
		cluster func() *stack.Cluster
		walked  func(t *testing.T, ml *layout.ManifestLayout)
	}{
		// a checks its own red/ready; b-longer writes a ConfigMap ready with
		// no namespace, which its build may apply into another namespace.
		"b's match is written with no namespace": {cluster: func() *stack.Cluster {
			return pairCluster(check("red", "ready"), []client.Object{cm("red", "ready")}, []client.Object{cm("", "ready")})
		}},
		// a checks its own settings-extra; b-longer generates settings,
		// applied as settings plus a content hash, not settings-extra.
		"b generates a ConfigMap whose name the check's starts with": {
			cluster: func() *stack.Cluster {
				return pairCluster(check("default", "settings-extra"), []client.Object{cm("default", "settings-extra")}, []client.Object{cm("default", "b-cm")})
			},
			walked: func(t *testing.T, ml *layout.ManifestLayout) {
				l := findLayout(ml, "b-cm")
				if l == nil {
					t.Fatal("no layout holds b-cm")
				}
				l.ConfigMapGenerators = append(l.ConfigMapGenerators, layout.ConfigMapGeneratorSpec{Name: "settings", Files: []string{"values.yaml"}})
				l.ExtraFiles = append(l.ExtraFiles, layout.ExtraFile{Name: "values.yaml", Content: []byte("k: v\n")})
			},
		},
		// The Flux bootstrap applies shared from the root directory before
		// any of the tree's Kustomizations.
		"the bootstrap writes a copy": {
			cluster: func() *stack.Cluster {
				return pairCluster(check("default", "shared"), nil, []client.Object{cm("default", "shared")})
			},
			walked: func(_ *testing.T, ml *layout.ManifestLayout) {
				ml.Resources = append(ml.Resources, cm("default", "shared"))
			},
		},
		// a's application emits h, whose build is b-longer's directory: h
		// may create the object before b-longer applies.
		"another Kustomization's build holds it too": {
			cluster: func() *stack.Cluster {
				return pairCluster(check("default", "b-cm"), []client.Object{cm("default", "a-cm")}, []client.Object{cm("default", "b-cm")})
			},
			walked: func(t *testing.T, ml *layout.ManifestLayout) {
				l, a := findLayout(ml, "b-cm"), findLayout(ml, "a-cm")
				if l == nil || a == nil {
					t.Fatal("no layout holds b-cm or a-cm")
				}
				h := handWritten("h")
				h.Spec.Path = "./" + l.FullRepoPath()
				a.Resources = append(a.Resources, h)
			},
		},
		// As above, but h cannot be read in typed form, so where its build
		// is, and what it creates, is not known.
		"a Kustomization that cannot be read may hold it": {
			cluster: func() *stack.Cluster {
				return pairCluster(check("default", "b-cm"), []client.Object{cm("default", "a-cm")}, []client.Object{cm("default", "b-cm")})
			},
			walked: func(t *testing.T, ml *layout.ManifestLayout) {
				l, a := findLayout(ml, "b-cm"), findLayout(ml, "a-cm")
				if l == nil || a == nil {
					t.Fatal("no layout holds b-cm or a-cm")
				}
				h := unstructuredObj(kustv1.GroupVersion.String(), "Kustomization", "flux-system", "h")
				for _, f := range []struct {
					value any
					path  []string
				}{
					{"${INTERVAL}", []string{"spec", "interval"}},
					{"./" + l.FullRepoPath(), []string{"spec", "path"}},
					{true, []string{"spec", "prune"}},
					{map[string]any{"kind": "GitRepository", "name": "flux-system"}, []string{"spec", "sourceRef"}},
				} {
					if err := unstructured.SetNestedField(h.Object, f.value, f.path...); err != nil {
						t.Fatalf("SetNestedField: %v", err)
					}
				}
				a.Resources = append(a.Resources, h)
			},
		},
		// b-longer writes red/ready, and a's own build may write it too: a
		// ConfigMap ready with no namespace, which a's apply may put in red.
		"a's match is written with no namespace": {cluster: func() *stack.Cluster {
			return pairCluster(check("red", "ready"), []client.Object{cm("", "ready")}, []client.Object{cm("red", "ready")})
		}},
		// b-longer writes settings-abc, and a's own build may write it too:
		// its generator settings, applied as settings plus a content hash.
		"a generates a ConfigMap whose name the check's starts with": {
			cluster: func() *stack.Cluster {
				return pairCluster(check("default", "settings-abc"), []client.Object{cm("default", "a-cm")}, []client.Object{cm("default", "settings-abc")})
			},
			walked: func(t *testing.T, ml *layout.ManifestLayout) {
				l := findLayout(ml, "a-cm")
				if l == nil {
					t.Fatal("no layout holds a-cm")
				}
				l.ConfigMapGenerators = append(l.ConfigMapGenerators, layout.ConfigMapGeneratorSpec{Name: "settings", Files: []string{"values.yaml"}})
				l.ExtraFiles = append(l.ExtraFiles, layout.ExtraFile{Name: "values.yaml", Content: []byte("k: v\n")})
			},
		},
		// a checks the default Namespace, which b-longer writes: a
		// cluster-scoped object, written and checked with no namespace, may
		// exist before any apply (Kubernetes creates default itself).
		"a cluster-scoped object": {cluster: func() *stack.Cluster {
			return pairCluster(stack.HealthCheck{APIVersion: "v1", Kind: "Namespace", Name: "default"},
				[]client.Object{cm("default", "a-cm")}, []client.Object{unstructuredObj("v1", "Namespace", "", "default")})
		}},
		// b-longer's patch renames b-cm.
		"b patches its build": {cluster: func() *stack.Cluster {
			c := pairCluster(check("default", "b-cm"), []client.Object{cm("default", "a-cm")}, []client.Object{cm("default", "b-cm")})
			c.Node.Children[1].Bundle.Patches = []stack.Patch{{Patch: "- op: replace\n  path: /metadata/name\n  value: b-renamed\n", Target: &stack.PatchSelector{Kind: "ConfigMap", Name: "b-cm"}}}
			return c
		}},
		// b-longer writes b-cm, and a's patch renames a-cm to b-cm, so a's
		// own apply writes it too.
		"a patches its build": {cluster: func() *stack.Cluster {
			c := pairCluster(check("default", "b-cm"), []client.Object{cm("default", "a-cm")}, []client.Object{cm("default", "b-cm")})
			c.Node.Children[0].Bundle.Patches = []stack.Patch{{Patch: "- op: replace\n  path: /metadata/name\n  value: b-cm\n", Target: &stack.PatchSelector{Kind: "ConfigMap", Name: "a-cm"}}}
			return c
		}},
		// b-longer writes red/ready; a's application emits h, whose build is
		// a's directory with targetNamespace red, so h applies blue/ready as
		// red/ready.
		"another Kustomization's build moves it into the checked namespace": {
			cluster: func() *stack.Cluster {
				return pairCluster(check("red", "ready"), []client.Object{cm("default", "a-cm"), cm("blue", "ready")}, []client.Object{cm("red", "ready")})
			},
			walked: func(t *testing.T, ml *layout.ManifestLayout) {
				a := findLayout(ml, "a-cm")
				if a == nil {
					t.Fatal("no layout holds a-cm")
				}
				h := handWritten("h")
				h.Spec.Path = "./" + a.FullRepoPath()
				h.Spec.TargetNamespace = "red"
				a.Resources = append(a.Resources, h)
			},
		},
		// b-longer writes the ConfigMap default/ready; a holds only a Secret
		// of that name, and a patch changes its kind.
		"a patch may change a kind": {cluster: func() *stack.Cluster {
			c := pairCluster(check("default", "ready"), []client.Object{unstructuredObj("v1", "Secret", "default", "ready")}, []client.Object{cm("default", "ready")})
			c.Node.Children[0].Bundle.Patches = []stack.Patch{{Patch: "- op: replace\n  path: /kind\n  value: ConfigMap\n", Target: &stack.PatchSelector{Kind: "Secret", Name: "ready"}}}
			return c
		}},
		// b-longer writes default/ready; a holds its own copy as local config,
		// which a patch keeps by removing the annotation, and otherwise no
		// ConfigMap.
		"a patch may keep a local-config object": {cluster: func() *stack.Cluster {
			local := cm("default", "ready")
			local.SetAnnotations(map[string]string{"config.kubernetes.io/local-config": "true"})
			c := pairCluster(check("default", "ready"), []client.Object{unstructuredObj("v1", "Secret", "default", "a-secret"), local}, []client.Object{cm("default", "ready")})
			c.Node.Children[0].Bundle.Patches = []stack.Patch{{Patch: "- op: remove\n  path: /metadata/annotations/config.kubernetes.io~1local-config\n", Target: &stack.PatchSelector{Kind: "ConfigMap", Name: "ready"}}}
			return c
		}},
		// b-longer writes red/ready; a's application emits h, which builds a's
		// directory, holding no ConfigMap, with a component, which adds
		// red/ready.
		"another Kustomization's component may add it": {
			cluster: func() *stack.Cluster {
				return pairCluster(check("red", "ready"), []client.Object{unstructuredObj("v1", "Secret", "default", "a-secret")}, []client.Object{cm("red", "ready")})
			},
			walked: func(t *testing.T, ml *layout.ManifestLayout) {
				a := findLayout(ml, "a-secret")
				if a == nil {
					t.Fatal("no layout holds a-secret")
				}
				h := handWritten("h")
				h.Spec.Path = "./" + a.FullRepoPath()
				h.Spec.Components = []string{"component"}
				a.Resources = append(a.Resources, h)
				// a's kustomization.yaml lists neither file: only h's build,
				// through its component, holds red/ready.
				a.ExtraFiles = append(a.ExtraFiles,
					layout.ExtraFile{Name: "component/kustomization.yaml", Content: []byte("apiVersion: kustomize.config.k8s.io/v1alpha1\nkind: Component\nresources:\n- ready.yaml\n")},
					layout.ExtraFile{Name: "component/ready.yaml", Content: []byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: ready\n  namespace: red\n")})
			},
		},
		// b-longer writes red/ready; a writes ready in a namespace postBuild
		// substitutes, which may be red.
		"a's match has a substituted namespace": {cluster: func() *stack.Cluster {
			return pairCluster(check("red", "ready"), []client.Object{cm("${NS}", "ready")}, []client.Object{cm("red", "ready")})
		}},
		// As above, with the name SOPS encrypts.
		"a's match has an encrypted name": {cluster: func() *stack.Cluster {
			return pairCluster(check("red", "ready"), []client.Object{cm("red", "ENC[AES256_GCM,data:cmVhZHk=,type:str]")}, []client.Object{cm("red", "ready")})
		}},
		// b-longer writes the Deployment red/web; a writes it too, under an
		// apiVersion postBuild substitutes whole, which may be apps/v1.
		"a's match has a substituted apiVersion": {cluster: func() *stack.Cluster {
			return pairCluster(stack.HealthCheck{APIVersion: "apps/v1", Kind: "Deployment", Namespace: "red", Name: "web"},
				[]client.Object{unstructuredObj("${API_VERSION}", "Deployment", "red", "web")},
				[]client.Object{unstructuredObj("apps/v1", "Deployment", "red", "web")})
		}},
		// b-longer writes red/web-ready; a's application emits h, which builds
		// a's directory with the name suffix -ready, and a's web sits in a
		// group postBuild substitutes, so h may apply it as apps red/web-ready.
		"another Kustomization's build renames a substituted object into it": {
			cluster: func() *stack.Cluster {
				return pairCluster(stack.HealthCheck{APIVersion: "apps/v1", Kind: "Deployment", Namespace: "red", Name: "web-ready"},
					[]client.Object{cm("default", "a-cm"), unstructuredObj("${GROUP}/v1", "Deployment", "red", "web")},
					[]client.Object{unstructuredObj("apps/v1", "Deployment", "red", "web-ready")})
			},
			walked: func(t *testing.T, ml *layout.ManifestLayout) {
				a := findLayout(ml, "a-cm")
				if a == nil {
					t.Fatal("no layout holds a-cm")
				}
				h := handWritten("h")
				h.Spec.Path = "./" + a.FullRepoPath()
				h.Spec.NameSuffix = "-ready"
				a.Resources = append(a.Resources, h)
			},
		},
		// b-longer writes red/ready; a writes an object whose kind postBuild
		// substitutes and whose name and namespace differ from the check's.
		// Kustomize may still give it the checked identity (a replacement, a
		// generator option), so any unresolved object outside the writer's
		// build leaves the writer uncertain, whatever it names.
		"a writes an unresolved object under another name": {cluster: func() *stack.Cluster {
			return pairCluster(check("red", "ready"), []client.Object{unstructuredObj("v1", "${KIND}", "blue", "other")}, []client.Object{cm("red", "ready")})
		}},
		// b-longer writes b-cm, and a generates a ConfigMap whose name
		// postBuild substitutes, which may be b-cm.
		"a generates a ConfigMap with a substituted name": {
			cluster: func() *stack.Cluster {
				return pairCluster(check("default", "b-cm"), []client.Object{cm("default", "a-cm")}, []client.Object{cm("default", "b-cm")})
			},
			walked: func(t *testing.T, ml *layout.ManifestLayout) {
				l := findLayout(ml, "a-cm")
				if l == nil {
					t.Fatal("no layout holds a-cm")
				}
				l.ConfigMapGenerators = append(l.ConfigMapGenerators, layout.ConfigMapGeneratorSpec{Name: "${CM_NAME}", Files: []string{"values.yaml"}})
				l.ExtraFiles = append(l.ExtraFiles, layout.ExtraFile{Name: "values.yaml", Content: []byte("k: v\n")})
			},
		},
		// b-longer writes red/ready only if it does not exist yet.
		"b's copy is applied only if absent": {cluster: func() *stack.Cluster {
			copyIfAbsent := cm("red", "ready")
			copyIfAbsent.SetAnnotations(map[string]string{"kustomize.toolkit.fluxcd.io/ssa": "IfNotPresent"})
			return pairCluster(check("red", "ready"), []client.Object{cm("default", "a-cm")}, []client.Object{copyIfAbsent})
		}},
		// b-longer writes red/ready, labelled so its apply skips it.
		"b's copy is not reconciled": {cluster: func() *stack.Cluster {
			disabled := cm("red", "ready")
			disabled.SetLabels(map[string]string{"kustomize.toolkit.fluxcd.io/reconcile": "Disabled"})
			return pairCluster(check("red", "ready"), []client.Object{cm("default", "a-cm")}, []client.Object{disabled})
		}},
		// a health-checks a Kustomization of another group under its own
		// namespace and name, which b-longer writes: Flux drops a check on any
		// Kustomization named as the checker itself, so a waits for nothing.
		"a checks a Kustomization under its own name": {cluster: func() *stack.Cluster {
			return pairCluster(stack.HealthCheck{APIVersion: "example.io/v1", Kind: "Kustomization", Namespace: "flux-system", Name: "a"},
				[]client.Object{cm("default", "a-cm")}, []client.Object{unstructuredObj("example.io/v1", "Kustomization", "flux-system", "a")})
		}},
		// b-longer adds to red/default, which Kubernetes creates in every
		// namespace.
		"Kubernetes creates it": {cluster: func() *stack.Cluster {
			return pairCluster(stack.HealthCheck{APIVersion: "v1", Kind: "ServiceAccount", Namespace: "red", Name: "default"},
				[]client.Object{cm("default", "a-cm")}, []client.Object{unstructuredObj("v1", "ServiceAccount", "red", "default")})
		}},
	}
	for name, c := range cases {
		for _, placement := range append(slices.Clone(integratedPlacements), layout.FluxSeparate) {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				var walked func(ml *layout.ManifestLayout)
				if c.walked != nil {
					walked = func(ml *layout.ManifestLayout) { c.walked(t, ml) }
				}
				if err := integrateWalked(t, c.cluster(), placement, walked); err != nil {
					t.Fatalf("got %v, want the tree admitted", err)
				}
			})
		}
	}
}

// A kept writer whose spec.commonMetadata lets its apply skip what it writes
// may not create the checked object: a's kept Kustomization health-checks
// red/ready, which only b-longer's build holds, and b-longer depends on a.
// Without that metadata the edge is certain and the cycle is refused.
func TestMissedCycles_KeptWriterThatSkipsItsObjectsAddsNoEdge(t *testing.T) {
	for _, skip := range []bool{false, true} {
		for _, placement := range integratedPlacements {
			t.Run("skip="+strconv.FormatBool(skip)+"/"+string(placement), func(t *testing.T) {
				c := pairCluster(stack.HealthCheck{}, []client.Object{unstructuredObj("v1", "ConfigMap", "default", "a-cm")}, []client.Object{unstructuredObj("v1", "ConfigMap", "red", "ready")})
				c.Node.Children[0].Bundle.HealthChecks = nil
				rules := withPlacement(nodeOnly, placement)
				ml, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(c, rules)
				if err != nil {
					t.Fatalf("CreateLayoutWithResources: %v", err)
				}
				byName := kustomizationsByName(ml)
				a, b := byName["a"], byName["b-longer"]
				if a == nil || b == nil {
					t.Fatalf("no a or b-longer in %s", describeKustomizations(ml))
				}
				checks("v1", "ConfigMap", "red", "ready")(a)
				if skip {
					b.Spec.CommonMetadata = &kustv1.CommonMetadata{Annotations: map[string]string{"kustomize.toolkit.fluxcd.io/ssa": "Ignore"}}
				}
				err = fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, c, rules)
				if skip && err != nil {
					t.Fatalf("got %v, want the tree admitted", err)
				}
				if !skip && (err == nil || !strings.Contains(err.Error(), "can never all become Ready")) {
					t.Fatalf("got %v, want the cycle through a's health check refused", err)
				}
			})
		}
	}
}

// An unresolved object in the writer's own build is that writer's, whatever
// it becomes: a health-checks red/ready, only b-longer's build holds it and an
// object whose namespace postBuild substitutes, and b-longer depends on a. The
// edge stays certain and the cycle is refused.
func TestMissedCycles_WritersOwnUnresolvedObjectKeepsTheEdge(t *testing.T) {
	for _, placement := range append(slices.Clone(integratedPlacements), layout.FluxSeparate) {
		t.Run(string(placement), func(t *testing.T) {
			c := pairCluster(stack.HealthCheck{APIVersion: "v1", Kind: "ConfigMap", Namespace: "red", Name: "ready"},
				[]client.Object{unstructuredObj("v1", "ConfigMap", "default", "a-cm")},
				[]client.Object{unstructuredObj("v1", "ConfigMap", "red", "ready"), unstructuredObj("v1", "ConfigMap", "${NS}", "ready")})
			err := integrateWalked(t, c, placement, nil)
			if err == nil || !strings.Contains(err.Error(), "can never all become Ready") {
				t.Fatalf("got %v, want the cycle through a's health check refused", err)
			}
		})
	}
}

// Without UmbrellasDoNotWait a Kustomization the integration did not place is
// followed too, and adds only the edges it has: platform waits for h, which
// its application emits, and h closes no cycle. h depending on platform with
// a readyExpr is refused (TestMissedCycles_NonMemberReadyExprWaitsForReady).
func TestMissedCycles_NonMemberWithoutACycleIsAdmittedWithoutTheSetting(t *testing.T) {
	cases := map[string]func(t *testing.T, placement layout.FluxPlacement) client.Object{
		// h's spec.interval is ${INTERVAL} and no postBuild sets it, so h
		// cannot be read in typed form, even from platform's result:
		// platform's apply fails, a stall, and h adds no edge.
		"h cannot be read before substitution": func(t *testing.T, _ layout.FluxPlacement) client.Object {
			u := unstructuredObj(kustv1.GroupVersion.String(), "Kustomization", "flux-system", "h")
			for _, f := range []struct {
				value any
				path  []string
			}{
				{"${INTERVAL}", []string{"spec", "interval"}},
				{"./elsewhere", []string{"spec", "path"}},
				{true, []string{"spec", "prune"}},
				{map[string]any{"kind": "GitRepository", "name": "flux-system"}, []string{"spec", "sourceRef"}},
			} {
				if err := unstructured.SetNestedField(u.Object, f.value, f.path...); err != nil {
					t.Fatalf("SetNestedField: %v", err)
				}
			}
			return u
		},
		// h builds web's directory and health-checks web's ConfigMap: h's
		// own apply creates it.
		"h health-checks what its own build writes": func(t *testing.T, placement layout.FluxPlacement) client.Object {
			off, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(noWaitCluster(srBundle("platform", cmApp("platform-app")), true), withPlacement(nodeOnly, placement))
			if err != nil {
				t.Fatalf("without h: %v", err)
			}
			h := handWritten("h")
			h.Spec.Path = findKept(off, "web").Spec.Path
			checks("v1", "ConfigMap", "default", "web-app-cm")(h)
			return h
		},
	}
	for name, h := range cases {
		for _, placement := range integratedPlacements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				platform := srBundle("platform", objApp("platform-app", h(t, placement)))
				platform.Wait = boolPtr(true)
				if _, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(noWaitCluster(platform, true), withPlacement(nodeOnly, placement)); err != nil {
					t.Fatalf("got %v, want the tree admitted", err)
				}
			})
		}
	}
}
