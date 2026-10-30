package fluxcd_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// A Kustomization of the tree that is no member and that no result holds is
// created by no apply when every build that loads a copy of it is a writer's
// (go-kure/kure#1183). Each case starts from excluded_copy_test.go's refused
// tree (a health-checks default/b-cm, b-longer depends on a, only b-longer's
// apply creates b-cm) and adds h, on another Source, whose build is
// b-longer's directory: an apply of h would write b-cm too.

// withDeletedH is baseCluster with a's bundle patching h out of its build.
func withDeletedH() *stack.Cluster {
	c := baseCluster()
	c.Node.Children[0].Bundle.Patches = []stack.Patch{{Patch: deleteKust("h"), Target: &stack.PatchSelector{Kind: "Kustomization", Name: "h"}}}
	return c
}

// TestDeletedNonMember_IsNoWriter: h is in a's directory, which only writers'
// builds load, and a's own patch deletes it there, so no apply creates h:
// b-longer is b-cm's only writer, and the cycle a -> b-longer -> a is refused.
func TestDeletedNonMember_IsNoWriter(t *testing.T) {
	c := excludedCopyCase{cluster: withDeletedH,
		walked: func(_ *testing.T, _, l, a *layout.ManifestLayout) {
			emitH(l, a, otherSource)
		}}
	for _, placement := range append(slices.Clone(integratedPlacements), layout.FluxSeparate) {
		t.Run(string(placement), func(t *testing.T) {
			err := c.integrate(t, placement)
			if err == nil || !strings.Contains(err.Error(), "can never all become Ready") || !strings.Contains(err.Error(), "apply flux-system/b-longer") {
				t.Fatalf("got %v, want the cycle through b-longer's apply refused", err)
			}
		})
	}
}

// TestDeletedNonMember_ControlsStayUncertain: where an apply that is no
// writer's may still create h, or a build holds it under another name, b-cm
// has no certain writer and the tree is admitted, as before.
func TestDeletedNonMember_ControlsStayUncertain(t *testing.T) {
	cases := map[string]excludedCopyCase{
		// n, on another Source and held by b-longer's build, applies a's
		// directory as it holds it, h included.
		"another Kustomization builds h's directory": {cluster: withDeletedH,
			walked: func(_ *testing.T, _, l, a *layout.ManifestLayout) {
				emitH(l, a, otherSource)
				n := handWritten("n")
				n.Spec.Path = "./" + a.FullRepoPath()
				otherSource(n)
				l.Resources = append(l.Resources, n)
			}},
		// h is in a directory no build of the tree loads: what applies it
		// is not the tree's to say.
		"a directory no writer loads": {cluster: func() *stack.Cluster { return baseCluster() },
			walked: func(_ *testing.T, ml, l, _ *layout.ManifestLayout) {
				stray := &layout.ManifestLayout{Name: "stray", Namespace: ml.FullRepoPath(), UmbrellaChild: true}
				emitH(l, stray, otherSource)
				ml.Children = append(ml.Children, stray)
			}},
		// a's patch renames h: its build holds h2, which builds b-longer's
		// directory as h would.
		"h renamed": {cluster: func() *stack.Cluster {
			c := baseCluster()
			c.Node.Children[0].Bundle.Patches = []stack.Patch{{
				Patch:  "- op: replace\n  path: /metadata/name\n  value: h2\n",
				Target: &stack.PatchSelector{Kind: "Kustomization", Name: "h"},
			}}
			return c
		}, walked: func(_ *testing.T, _, l, a *layout.ManifestLayout) {
			emitH(l, a, otherSource)
		}},
	}
	for name, c := range cases {
		for _, placement := range append(slices.Clone(integratedPlacements), layout.FluxSeparate) {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				if err := c.integrate(t, placement); err != nil {
					t.Fatalf("got %v, want the tree admitted: another apply may create b-cm", err)
				}
			})
		}
	}
}
