package fluxcd_test

import (
	"path"
	"strings"
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
)

// Without UmbrellasDoNotWait the reconcile-order check follows a Flux
// Kustomization the integration did not place too, read from its writer's
// substituted result, as it does under the setting (go-kure/kure#1094).

// platform waits for what its build applies, hand-written among it, which
// waits for web's directory, and web depends on platform. Refused in every
// integrated placement.
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

// tieredWebCluster is noWaitCluster with web one node deeper, under tier, so
// web's CR sits in tier's directory, which holds no other CR of the tree.
func tieredWebCluster(u *stack.Bundle, dependsOnU bool) *stack.Cluster {
	web := srBundle("web", cmApp("web-app"))
	if dependsOnU {
		web.DependsOn = []*stack.Bundle{u}
	}
	apps := &stack.Node{Name: "apps", Bundle: u}
	webN := &stack.Node{Name: "web", Bundle: web}
	tier := &stack.Node{Name: "tier", Children: []*stack.Node{webN}}
	root := &stack.Node{Name: "demo", Children: []*stack.Node{apps, tier}}
	apps.SetParent(root)
	tier.SetParent(root)
	webN.SetParent(tier)
	return &stack.Cluster{Name: "demo", Node: root}
}

// The same cycle with web's the only one: hand-written waits for tier's
// directory, which holds web's CR and not platform's, so it waits for web,
// which depends on platform, which waits for hand-written. Without web's
// dependsOn the tree is admitted: no other cycle closes.
func TestMissedCycles_NonMemberWaitClosesTheWebCycleAlone(t *testing.T) {
	for _, placement := range integratedPlacements {
		t.Run(string(placement), func(t *testing.T) {
			off, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(tieredWebCluster(srBundle("platform", cmApp("platform-app")), true), withPlacement(nodeOnly, placement))
			if err != nil {
				t.Fatalf("without hand-written: %v", err)
			}
			dir := path.Dir(path.Clean(findKept(off, "web").Spec.Path))
			cluster := func(dependsOnU bool) *stack.Cluster {
				k := handWritten("hand-written")
				k.Spec.Wait, k.Spec.Path = true, dir
				platform := srBundle("platform", objApp("platform-app", k))
				platform.Wait = boolPtr(true)
				return tieredWebCluster(platform, dependsOnU)
			}
			integrator := func() *fluxstack.LayoutIntegrator {
				return fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator())
			}
			if _, err := integrator().CreateLayoutWithResources(cluster(false), withPlacement(nodeOnly, placement)); err != nil {
				t.Fatalf("without web's dependsOn: %v, want admitted", err)
			}
			_, err = integrator().CreateLayoutWithResources(cluster(true), withPlacement(nodeOnly, placement))
			if err == nil || !strings.Contains(err.Error(), "can never all become Ready") || !strings.Contains(err.Error(), "ready flux-system/hand-written -> ready flux-system/web") {
				t.Fatalf("got %v, want the cycle through hand-written's wait on web refused", err)
			}
		})
	}
}

// h's spec.interval is ${INTERVAL}, and h waits for web's directory. With
// platform's postBuild setting INTERVAL, h is read from platform's substituted
// result and the cycle through h's wait is refused. Without it h cannot be read
// even there: platform's apply fails, a stall the checks do not judge, and the
// tree is admitted.
func TestMissedCycles_SubstitutedNonMemberIsRead(t *testing.T) {
	for _, substituted := range []bool{true, false} {
		for _, placement := range integratedPlacements {
			name := "INTERVAL set inline"
			if !substituted {
				name = "INTERVAL set nowhere"
			}
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				u := unstructuredObj(kustv1.GroupVersion.String(), "Kustomization", "flux-system", "h")
				for _, f := range []struct {
					value any
					path  []string
				}{
					{"${INTERVAL}", []string{"spec", "interval"}},
					{webDirectory(t, placement), []string{"spec", "path"}},
					{true, []string{"spec", "prune"}},
					{true, []string{"spec", "wait"}},
					{map[string]any{"kind": "GitRepository", "name": "flux-system"}, []string{"spec", "sourceRef"}},
				} {
					if err := unstructured.SetNestedField(u.Object, f.value, f.path...); err != nil {
						t.Fatalf("SetNestedField: %v", err)
					}
				}
				platform := srBundle("platform", objApp("platform-app", u))
				platform.Wait = boolPtr(true)
				if substituted {
					platform.PostBuild = &stack.PostBuild{Substitute: map[string]string{"INTERVAL": "1m"}}
				}
				_, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(noWaitCluster(platform, true), withPlacement(nodeOnly, placement))
				if !substituted {
					if err != nil {
						t.Fatalf("got %v, want the tree admitted", err)
					}
					return
				}
				if err == nil || !strings.Contains(err.Error(), "can never all become Ready") || !strings.Contains(err.Error(), "ready flux-system/h -> ready flux-system/") {
					t.Fatalf("got %v, want the cycle through h's wait refused", err)
				}
			})
		}
	}
}

// h depends on platform with a readyExpr, and platform waits for h, which its
// application emits. Under kustomize-controller's default feature gates Flux
// evaluates only the expression, so with "true" this tree converges; but kure
// does not evaluate CEL and cannot see the gate. Without
// AdditiveCELDependencyCheckOff the dependsOn waits for platform's Ready and
// the cycle is refused. With it, "true" waits only for platform to exist,
// which the Flux bootstrap creates, and the tree is admitted; any other
// expression may itself require Ready, and is still refused
// (go-kure/kure#1161). This tree integrated before go-kure/kure#1094.
func TestMissedCycles_NonMemberReadyExpr(t *testing.T) {
	const onReady = "dep.status.conditions.filter(c, c.type == 'Ready').all(c, c.status == 'True')"
	for name, c := range map[string]struct {
		expr     string
		off      bool
		admitted bool
	}{
		"true":                             {expr: "true"},
		"an expression on Ready":           {expr: onReady},
		"true, gate off":                   {expr: "true", off: true, admitted: true},
		"an expression on Ready, gate off": {expr: onReady, off: true},
	} {
		for _, placement := range integratedPlacements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				h := handWritten("h")
				h.Spec.DependsOn = []kustv1.DependencyReference{{Name: "platform", ReadyExpr: c.expr}}
				platform := srBundle("platform", objApp("platform-app", client.Object(h)))
				platform.Wait = boolPtr(true)
				_, err := fluxstack.NewLayoutIntegrator(celGateOffGenerator(c.off)).CreateLayoutWithResources(noWaitCluster(platform, true), withPlacement(nodeOnly, placement))
				if c.admitted {
					if err != nil {
						t.Fatalf("got %v, want the tree admitted", err)
					}
					return
				}
				if err == nil || !strings.Contains(err.Error(), "can never all become Ready") || !strings.Contains(err.Error(), "flux-system/h") {
					t.Fatalf("got %v, want the cycle through h's dependsOn refused", err)
				}
			})
		}
	}
}
