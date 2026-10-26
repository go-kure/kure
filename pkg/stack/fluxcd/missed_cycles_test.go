package fluxcd_test

import (
	"errors"
	"slices"
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
		"a generated ConfigMap by its hashed name": {
			check: checks("v1", "ConfigMap", "default", "web-values-5h7k8m2c9t"),
			walked: func(t *testing.T, ml *layout.ManifestLayout) {
				l := layoutHolding(t, ml, "web-app-cm")
				l.ConfigMapGenerators = append(l.ConfigMapGenerators, layout.ConfigMapGeneratorSpec{Name: "web-values", Files: []string{"values.yaml"}})
				l.ExtraFiles = append(l.ExtraFiles, layout.ExtraFile{Name: "values.yaml", Content: []byte("k: v\n")})
			},
		},
		"a Deployment with no apiVersion": {
			check:   checks("", "Deployment", "default", "web-deploy"),
			webApps: []*stack.Application{objApp("web-app", unstructuredObj("apps/v1", "Deployment", "default", "web-deploy"))},
		},
		"a List item": {
			check:   checks("v1", "ConfigMap", "default", "web-list-cm"),
			webApps: []*stack.Application{objApp("web-app", listOf(unstructuredObj("v1", "ConfigMap", "default", "web-list-cm")))},
		},
		"an object with no namespace": {
			check:   checks("v1", "ConfigMap", "default", "web-bare-cm"),
			webApps: []*stack.Application{objApp("web-app", unstructuredObj("v1", "ConfigMap", "", "web-bare-cm"))},
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

// F2 without the setting: platform waits for what its build applies,
// hand-written among it, which waits for web's directory, and web depends on
// platform. Refused in every integrated placement.
func TestMissedCycles_NonMemberWaitIsACycleWithoutTheSetting(t *testing.T) {
	for _, placement := range integratedPlacements {
		t.Run(string(placement), func(t *testing.T) {
			dir := webDirectory(t, placement)
			k := handWritten("hand-written")
			k.Spec.Wait, k.Spec.Path = true, dir
			platform := srBundle("platform", objApp("platform-app", k))
			platform.Wait = boolPtr(true)
			_, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(noWaitCluster(platform, true), withPlacement(nodeOnly, placement))
			if err == nil || !strings.Contains(err.Error(), "can never all become Ready") || !strings.Contains(err.Error(), "ready flux-system/hand-written -> ready flux-system/") {
				t.Fatalf("got %v, want the cycle through hand-written's wait refused", err)
			}
		})
	}
}

// A cycle among Kustomizations the integration did not place is their
// author's: platform waits for x and y, which depend on each other, and web
// depends on platform. No Kustomization the integration placed is on the
// cycle, so the tree is accepted.
func TestMissedCycles_NonMemberCycleIsTheAuthors(t *testing.T) {
	for _, placement := range integratedPlacements {
		t.Run(string(placement), func(t *testing.T) {
			platform := srBundle("platform", objApp("platform-app", handWritten("x", "y"), handWritten("y", "x")))
			platform.Wait = boolPtr(true)
			if _, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(noWaitCluster(platform, true), withPlacement(nodeOnly, placement)); err != nil {
				t.Fatalf("CreateLayoutWithResources: %v", err)
			}
		})
	}
}
