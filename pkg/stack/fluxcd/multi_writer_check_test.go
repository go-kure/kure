package fluxcd_test

import (
	"slices"
	"strings"
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	"github.com/fluxcd/pkg/apis/kustomize"
	metaapi "github.com/fluxcd/pkg/apis/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// A health check on an object several Kustomizations apply waits for
// whichever of them creates it first (go-kure/kure#1172). The tree can never
// become Ready when every one of those writers waits for the checking
// Kustomization, and is refused; when one writer need not wait, the check is
// met and the tree is admitted. In each tree the checked object is the
// ConfigMap default/ready.

// mwMember is a member of a multi-writer tree: a bundle whose application
// holds name-cm, which marks its layout, and objs, depending on deps by
// name and health-checking checks.
type mwMember struct {
	name   string
	objs   []client.Object
	deps   []string
	checks []stack.HealthCheck
}

func mwCluster(members ...mwMember) *stack.Cluster {
	root := &stack.Node{Name: "demo"}
	for _, m := range members {
		b := srBundle(m.name, objApp(m.name+"-app", append([]client.Object{unstructuredObj("v1", "ConfigMap", "default", m.name+"-cm")}, m.objs...)...))
		b.NamedDependsOn = m.deps
		b.HealthChecks = m.checks
		n := &stack.Node{Name: m.name, Bundle: b}
		n.SetParent(root)
		root.Children = append(root.Children, n)
	}
	return &stack.Cluster{Name: "demo", Node: root}
}

// readyCheck health-checks default/ready.
var readyCheck = []stack.HealthCheck{{APIVersion: "v1", Kind: "ConfigMap", Namespace: "default", Name: "ready"}}

// ready is default/ready, with annotations.
func ready(annotations ...string) client.Object {
	o := unstructuredObj("v1", "ConfigMap", "default", "ready")
	if len(annotations) > 0 {
		m := map[string]string{}
		for i := 0; i < len(annotations); i += 2 {
			m[annotations[i]] = annotations[i+1]
		}
		o.SetAnnotations(m)
	}
	return o
}

// memberLayout is the layout of the member named name.
func memberLayout(t *testing.T, ml *layout.ManifestLayout, name string) *layout.ManifestLayout {
	t.Helper()
	l := findLayout(ml, name+"-cm")
	if l == nil {
		t.Fatalf("no layout holds %s-cm", name)
	}
	return l
}

// checksReady makes k health-check default/ready.
func checksReady(k *kustv1.Kustomization) *kustv1.Kustomization {
	k.Spec.HealthChecks = []metaapi.NamespacedObjectKindReference{{APIVersion: "v1", Kind: "ConfigMap", Namespace: "default", Name: "ready"}}
	return k
}

// building is k building the directory of the member named member.
func building(t *testing.T, ml *layout.ManifestLayout, k *kustv1.Kustomization, member string) *kustv1.Kustomization {
	t.Helper()
	k.Spec.Path = "./" + memberLayout(t, ml, member).FullRepoPath()
	return k
}

// authorsKnot is a tree whose knot is made only of Kustomizations the
// integration does not place: p's application emits checker x and writers u
// and v, which build d's directory with a commonMetadata that applies d's
// ready, which d's own apply excludes, and depend on x; s depends on x, so
// the knot is reached. uDeps are u's dependencies.
func authorsKnot(extra ...mwMember) (func() *stack.Cluster, func(uDeps ...string) func(t *testing.T, ml *layout.ManifestLayout)) {
	cluster := func() *stack.Cluster {
		return mwCluster(append([]mwMember{
			{name: "p"},
			{name: "d", objs: []client.Object{ready(ssaKey, "Ignore")}},
			{name: "s", deps: []string{"x"}},
		}, extra...)...)
	}
	walked := func(uDeps ...string) func(t *testing.T, ml *layout.ManifestLayout) {
		return func(t *testing.T, ml *layout.ManifestLayout) {
			merge := func(k *kustv1.Kustomization) *kustv1.Kustomization {
				k.Spec.CommonMetadata = &kustv1.CommonMetadata{Annotations: map[string]string{ssaKey: "Merge"}}
				return k
			}
			p := memberLayout(t, ml, "p")
			p.Resources = append(p.Resources, checksReady(handWritten("x")),
				merge(building(t, ml, handWritten("u", uDeps...), "d")),
				merge(building(t, ml, handWritten("v", "x"), "d")))
		}
	}
	return cluster, walked
}

type multiWriterCase struct {
	cluster func() *stack.Cluster
	walked  func(t *testing.T, ml *layout.ManifestLayout)
	// writers are the writers the refusal names; or marks a refusal of the
	// health check's alternatives rather than of one cycle.
	writers []string
	or      bool
	// absent are writers the refusal must not name.
	absent []string
}

func (c multiWriterCase) integrate(t *testing.T, placement layout.FluxPlacement) error {
	t.Helper()
	var walked func(ml *layout.ManifestLayout)
	if c.walked != nil {
		walked = func(ml *layout.ManifestLayout) { c.walked(t, ml) }
	}
	return integrateWalked(t, c.cluster(), placement, walked)
}

func TestMultiWriterCheck_EveryWriterWaitsIsRefused(t *testing.T) {
	cases := map[string]multiWriterCase{
		// The issue's first tree.
		"T1.1 two members write it and depend on the checker": {
			cluster: func() *stack.Cluster {
				return mwCluster(mwMember{name: "a", checks: readyCheck},
					mwMember{name: "b", objs: []client.Object{ready()}, deps: []string{"a"}},
					mwMember{name: "c", objs: []client.Object{ready()}, deps: []string{"a"}})
			},
			writers: []string{"b", "c"}, or: true,
		},
		// The second case below is the same tree as T1.1 with c waiting
		// for b, which waits for a: c's chain runs through b's apply, which
		// b's own chain starts from.
		"T1.4 one writer reaches the checker through the other": {
			cluster: func() *stack.Cluster {
				return mwCluster(mwMember{name: "a", checks: readyCheck},
					mwMember{name: "b", objs: []client.Object{ready()}, deps: []string{"a"}},
					mwMember{name: "c", objs: []client.Object{ready()}, deps: []string{"b"}})
			},
			writers: []string{"b", "c"}, or: true,
		},
		// c's own build holds an object whose identity may change at apply:
		// c may write ready through it too, so c is still only a writer.
		"T1.5 a writer's own build holds an unresolved object": {
			cluster: func() *stack.Cluster {
				return mwCluster(mwMember{name: "a", checks: readyCheck},
					mwMember{name: "b", objs: []client.Object{ready()}, deps: []string{"a"}},
					mwMember{name: "c", objs: []client.Object{ready(), unstructuredObj("v1", "${KIND}", "blue", "other")}, deps: []string{"a"}})
			},
			writers: []string{"b", "c"}, or: true,
		},
		// c's copy is excluded (go-kure/kure#1163), so it is no writer and
		// b is certain: the cycle through b's apply is refused, although c
		// waits for nothing.
		"T1.6 an excluded copy is no alternative": {
			cluster: func() *stack.Cluster {
				return mwCluster(mwMember{name: "a", checks: readyCheck},
					mwMember{name: "b", objs: []client.Object{ready()}, deps: []string{"a"}},
					mwMember{name: "c", objs: []client.Object{ready(ssaKey, "Ignore")}})
			},
			writers: []string{"b"}, absent: []string{"c"},
		},
		// a's application emits h, which builds b's directory and depends on
		// a: the tree go-kure/kure#1172's admitted control becomes when h
		// waits.
		"T1.3 a Kustomization the checker's apply emits waits for it": {
			cluster: func() *stack.Cluster {
				return mwCluster(mwMember{name: "a", checks: readyCheck},
					mwMember{name: "b", objs: []client.Object{ready()}, deps: []string{"a"}})
			},
			walked: func(t *testing.T, ml *layout.ManifestLayout) {
				a := memberLayout(t, ml, "a")
				a.Resources = append(a.Resources, building(t, ml, handWritten("h", "a"), "b"))
			},
			writers: []string{"b", "h"}, or: true,
		},
		// Checker x is no member, so it is not judged; members b and c, the
		// writers, are.
		"T1.7 a checker the integration does not place": {
			cluster: func() *stack.Cluster {
				return mwCluster(mwMember{name: "p"},
					mwMember{name: "b", objs: []client.Object{ready()}, deps: []string{"x"}},
					mwMember{name: "c", objs: []client.Object{ready()}, deps: []string{"x"}})
			},
			walked: func(t *testing.T, ml *layout.ManifestLayout) {
				p := memberLayout(t, ml, "p")
				p.Resources = append(p.Resources, checksReady(handWritten("x")))
			},
			writers: []string{"b", "c"}, or: true,
		},
	}
	// The issue's second tree: p's application emits h, whose own apply is
	// excluded, and q, which builds p's directory with a patch that applies h
	// and points it at b's directory. h depends on a. q comes before or after
	// h in p's build.
	for _, order := range []string{"before", "after"} {
		cases["T1.2 a copy another build applies, its builder "+order] = multiWriterCase{
			cluster: func() *stack.Cluster {
				return mwCluster(mwMember{name: "a", checks: readyCheck},
					mwMember{name: "b", objs: []client.Object{ready()}, deps: []string{"a"}},
					mwMember{name: "p"})
			},
			walked: func(t *testing.T, ml *layout.ManifestLayout) {
				h := handWritten("h", "a")
				h.SetAnnotations(map[string]string{ssaKey: "Ignore"})
				q := building(t, ml, handWritten("q"), "p")
				q.Spec.Patches = []kustomize.Patch{{
					Patch: "- op: remove\n  path: /metadata/annotations/kustomize.toolkit.fluxcd.io~1ssa\n" +
						"- op: replace\n  path: /spec/path\n  value: ./" + memberLayout(t, ml, "b").FullRepoPath() + "\n",
					Target: &kustomize.Selector{Kind: "Kustomization", Name: "h"},
				}}
				p := memberLayout(t, ml, "p")
				if order == "before" {
					p.Resources = append(p.Resources, q, h)
				} else {
					p.Resources = append(p.Resources, h, q)
				}
			},
			writers: []string{"b", "h"}, or: true,
		}
	}
	// The authors' knot of the control below, with u also waiting for
	// member m, which waits for x: m is on one of u's chains whichever of
	// u's dependencies comes first.
	knot, knotWalked := authorsKnot(mwMember{name: "m", deps: []string{"x"}})
	for name, deps := range map[string][]string{"x first": {"x", "m"}, "m first": {"m", "x"}} {
		cases["T1.8 a judged member off the first chain, "+name] = multiWriterCase{
			cluster: knot, walked: knotWalked(deps...), writers: []string{"u", "v"}, or: true,
		}
	}
	for name, c := range cases {
		for _, placement := range append(slices.Clone(integratedPlacements), layout.FluxSeparate) {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				err := c.integrate(t, placement)
				if err == nil || !strings.Contains(err.Error(), "can never all become Ready") {
					t.Fatalf("got %v, want the tree refused", err)
				}
				msg := err.Error()
				if c.or && !strings.Contains(msg, "health-checks ConfigMap default/ready") {
					t.Errorf("got %v, want the health check's writers named", err)
				}
				for _, w := range c.writers {
					if !strings.Contains(msg, "apply flux-system/"+w+" ->") {
						t.Errorf("got %v, want a chain from %s's apply", err, w)
					}
				}
				for _, w := range c.absent {
					if strings.Contains(msg, "flux-system/"+w) {
						t.Errorf("got %v, want %s not named", err, w)
					}
				}
			})
		}
	}
}

func TestMultiWriterCheck_AWriterThatNeedNotWaitIsAdmitted(t *testing.T) {
	knot, knotWalked := authorsKnot()
	cases := map[string]multiWriterCase{
		// c applies ready whatever a's state, and a is Ready once it has.
		"T2.2 one writer does not depend on the checker": {
			cluster: func() *stack.Cluster {
				return mwCluster(mwMember{name: "a", checks: readyCheck},
					mwMember{name: "b", objs: []client.Object{ready()}, deps: []string{"a"}},
					mwMember{name: "c", objs: []client.Object{ready()}})
			},
		},
		// c reaches a only through d's health check on other, which e and f
		// write, both depending on a: a deadlock through two multi-writer
		// checks at once, which the check does not follow and admits.
		"T2.3 a chain through another multi-writer check (residual)": {
			cluster: func() *stack.Cluster {
				other := unstructuredObj("v1", "ConfigMap", "default", "other")
				return mwCluster(mwMember{name: "a", checks: readyCheck},
					mwMember{name: "b", objs: []client.Object{ready()}, deps: []string{"a"}},
					mwMember{name: "c", objs: []client.Object{ready()}, deps: []string{"d"}},
					mwMember{name: "d", checks: []stack.HealthCheck{{APIVersion: "v1", Kind: "ConfigMap", Namespace: "default", Name: "other"}}},
					mwMember{name: "e", objs: []client.Object{other}, deps: []string{"a"}},
					mwMember{name: "f", objs: []client.Object{unstructuredObj("v1", "ConfigMap", "default", "other")}, deps: []string{"a"}})
			},
		},
		// Applied when absent, so c may create it on a fresh install: the
		// writer stays uncertain, and no alternative is read.
		"T2.4 a copy applied only if absent": {
			cluster: func() *stack.Cluster {
				return mwCluster(mwMember{name: "a", checks: readyCheck},
					mwMember{name: "b", objs: []client.Object{ready()}, deps: []string{"a"}},
					mwMember{name: "c", objs: []client.Object{ready(ssaKey, "IfNotPresent")}, deps: []string{"a"}})
			},
		},
		// x, u and v deadlock, but none is placed by the integration: their
		// author's, as a cycle of them would be.
		"T2.5 a knot of Kustomizations the integration does not place": {
			cluster: knot, walked: knotWalked("x"),
		},
	}
	for name, c := range cases {
		for _, placement := range append(slices.Clone(integratedPlacements), layout.FluxSeparate) {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				if err := c.integrate(t, placement); err != nil {
					t.Fatalf("got %v, want the tree admitted", err)
				}
			})
		}
	}
}
