package fluxcd_test

import (
	"slices"
	"strings"
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// A copy of the checked object that its apply excludes
// (kustomize.toolkit.fluxcd.io/ssa: Ignore or
// kustomize.toolkit.fluxcd.io/reconcile: disabled, on the object or through
// that apply's spec.commonMetadata) is never created, so it competes with no
// writer (go-kure/kure#1163). Each case starts from a tree whose cycle is
// refused: a health-checks default/b-cm, b-longer depends on a, and only
// b-longer's apply creates b-cm. An excluded copy elsewhere keeps that
// refusal; a copy the apply may still create, or one whose marks
// substitution or decryption may change, leaves the writer uncertain and the
// tree admitted, as before.

const (
	ssaKey       = "kustomize.toolkit.fluxcd.io/ssa"
	reconcileKey = "kustomize.toolkit.fluxcd.io/reconcile"
)

// excludedCopyCase edits the walked layout: l holds b-cm, a holds a-cm.
type excludedCopyCase struct {
	cluster func() *stack.Cluster
	walked  func(t *testing.T, ml, l, a *layout.ManifestLayout)
}

// baseCluster is the refused tree, with extra objects in a's build.
func baseCluster(aObjs ...client.Object) *stack.Cluster {
	return pairCluster(stack.HealthCheck{APIVersion: "v1", Kind: "ConfigMap", Namespace: "default", Name: "b-cm"},
		append([]client.Object{unstructuredObj("v1", "ConfigMap", "default", "a-cm")}, aObjs...),
		[]client.Object{unstructuredObj("v1", "ConfigMap", "default", "b-cm")})
}

// withPostBuild is baseCluster with a's bundle substituting: OTHER inline,
// so the offline substitution runs, and a substituteFrom the cluster reads.
func withPostBuild(aObjs ...client.Object) *stack.Cluster {
	c := baseCluster(aObjs...)
	c.Node.Children[0].Bundle.PostBuild = &stack.PostBuild{
		Substitute:     map[string]string{"OTHER": "x"},
		SubstituteFrom: []stack.SubstituteRef{{Kind: "ConfigMap", Name: "cluster-vars"}},
	}
	return c
}

// marked is a v1 ConfigMap namespace/name with labels and annotations.
func marked(namespace, name string, labels, annotations map[string]string) *unstructured.Unstructured {
	u := unstructuredObj("v1", "ConfigMap", namespace, name)
	if labels != nil {
		u.SetLabels(labels)
	}
	if annotations != nil {
		u.SetAnnotations(annotations)
	}
	return u
}

// emitH makes a's application emit h, whose build is dir; edit shapes it.
func emitH(dir, a *layout.ManifestLayout, edit func(h *kustv1.Kustomization)) {
	h := handWritten("h")
	h.Spec.Path = "./" + dir.FullRepoPath()
	edit(h)
	a.Resources = append(a.Resources, h)
}

func otherSource(h *kustv1.Kustomization) {
	h.Spec.SourceRef = kustv1.CrossNamespaceSourceReference{Kind: "GitRepository", Name: "other-repo"}
}

func commonAnnotations(kv ...string) func(h *kustv1.Kustomization) {
	return func(h *kustv1.Kustomization) {
		h.Spec.CommonMetadata = &kustv1.CommonMetadata{Annotations: map[string]string{}}
		for i := 0; i < len(kv); i += 2 {
			h.Spec.CommonMetadata.Annotations[kv[i]] = kv[i+1]
		}
	}
}

func (c excludedCopyCase) integrate(t *testing.T, placement layout.FluxPlacement) error {
	t.Helper()
	var walked func(ml *layout.ManifestLayout)
	if c.walked != nil {
		walked = func(ml *layout.ManifestLayout) {
			l, a := findLayout(ml, "b-cm"), findLayout(ml, "a-cm")
			if l == nil || a == nil {
				t.Fatal("no layout holds b-cm or a-cm")
			}
			c.walked(t, ml, l, a)
		}
	}
	return integrateWalked(t, c.cluster(), placement, walked)
}

func TestExcludedCopy_IsNoCompetingWriter(t *testing.T) {
	cases := map[string]excludedCopyCase{
		// The issue's case: h builds b-longer's directory and its
		// commonMetadata excludes every object of it.
		"commonMetadata ssa Ignore": {cluster: func() *stack.Cluster { return baseCluster() },
			walked: func(_ *testing.T, _, l, a *layout.ManifestLayout) {
				emitH(l, a, commonAnnotations(ssaKey, "Ignore"))
			}},
		"commonMetadata label reconcile disabled": {cluster: func() *stack.Cluster { return baseCluster() },
			walked: func(_ *testing.T, _, l, a *layout.ManifestLayout) {
				emitH(l, a, func(h *kustv1.Kustomization) {
					h.Spec.CommonMetadata = &kustv1.CommonMetadata{Labels: map[string]string{reconcileKey: "disabled"}}
				})
			}},
		"commonMetadata ssa ignore in any case": {cluster: func() *stack.Cluster { return baseCluster() },
			walked: func(_ *testing.T, _, l, a *layout.ManifestLayout) {
				emitH(l, a, commonAnnotations(ssaKey, "ignore"))
			}},
		"a's own copy marked on the object": {cluster: func() *stack.Cluster {
			return baseCluster(marked("default", "b-cm", nil, map[string]string{ssaKey: "Ignore"}))
		}},
		"the bootstrap's copy": {cluster: func() *stack.Cluster { return baseCluster() },
			walked: func(_ *testing.T, ml, _, _ *layout.ManifestLayout) {
				ml.Resources = append(ml.Resources, marked("default", "b-cm", nil, map[string]string{ssaKey: "Ignore"}))
			}},
		"a's own copy with no namespace": {cluster: func() *stack.Cluster {
			return baseCluster(marked("", "b-cm", nil, map[string]string{ssaKey: "Ignore"}))
		}},
		// h's build is not the tree's: its copy is read as the directory
		// holds it, and commonMetadata transforms nothing.
		"another Source's copy": {cluster: func() *stack.Cluster { return baseCluster() },
			walked: func(_ *testing.T, _, l, a *layout.ManifestLayout) {
				emitH(l, a, func(h *kustv1.Kustomization) {
					otherSource(h)
					commonAnnotations(ssaKey, "Ignore")(h)
				})
			}},
	}
	for name, c := range cases {
		for _, placement := range append(slices.Clone(integratedPlacements), layout.FluxSeparate) {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				err := c.integrate(t, placement)
				if err == nil || !strings.Contains(err.Error(), "can never all become Ready") || !strings.Contains(err.Error(), "apply flux-system/b-longer") {
					t.Fatalf("got %v, want the cycle through b-longer's apply refused", err)
				}
			})
		}
	}
}

func TestExcludedCopy_ControlsStayUncertain(t *testing.T) {
	cases := map[string]excludedCopyCase{
		// Applied when absent, so on a fresh install.
		"commonMetadata ssa IfNotPresent": {cluster: func() *stack.Cluster { return baseCluster() },
			walked: func(_ *testing.T, _, l, a *layout.ManifestLayout) {
				emitH(l, a, commonAnnotations(ssaKey, "IfNotPresent"))
			}},
		// a's copy is excluded, but h applies a's directory with
		// commonMetadata ssa: Merge, set over the object's mark.
		"commonMetadata overrides the object's exclusion": {cluster: func() *stack.Cluster {
			return baseCluster(marked("default", "b-cm", nil, map[string]string{ssaKey: "Ignore"}))
		}, walked: func(_ *testing.T, _, _, a *layout.ManifestLayout) {
			emitH(a, a, commonAnnotations(ssaKey, "Merge"))
		}},
		// An unresolved identity is always written with $ or ENC[, so its
		// marks may change at apply with it (go-kure/kure#1163, N1).
		"an unresolved object marked Ignore": {cluster: func() *stack.Cluster {
			u := unstructuredObj("v1", "${KIND}", "blue", "other")
			u.SetAnnotations(map[string]string{ssaKey: "Ignore"})
			return baseCluster(u)
		}},
		"an object whose name substituteFrom sets, marked Ignore": {cluster: func() *stack.Cluster {
			return withPostBuild(marked("default", "${NAME}", nil, map[string]string{ssaKey: "Ignore"}))
		}},
		// Offline the default reads disabled or Ignore; the cluster may set
		// the variable.
		"a label substituteFrom may set": {cluster: func() *stack.Cluster {
			return withPostBuild(marked("default", "b-cm", map[string]string{reconcileKey: "${R:=disabled}"}, nil))
		}},
		"an annotation substituteFrom may set": {cluster: func() *stack.Cluster {
			return withPostBuild(marked("default", "b-cm", nil, map[string]string{ssaKey: "${MODE:=Ignore}"}))
		}},
		// Flux substitutes keys too: the second key may become ssa, and the
		// later value wins. h reads a's directory as it holds it.
		"a key with a variable": {cluster: func() *stack.Cluster {
			return baseCluster(marked("default", "b-cm", nil, map[string]string{ssaKey: "Ignore", ssaKey + "${EMPTY}": "Merge"}))
		}, walked: func(_ *testing.T, _, _, a *layout.ManifestLayout) {
			emitH(a, a, func(h *kustv1.Kustomization) {
				otherSource(h)
				h.Spec.PostBuild = &kustv1.PostBuild{Substitute: map[string]string{"EMPTY": ""}}
			})
		}},
		// A value outside the marks may carry a line break into them.
		"a variable in the data": {cluster: func() *stack.Cluster {
			u := marked("default", "b-cm", nil, map[string]string{ssaKey: "Ignore"})
			if err := unstructured.SetNestedField(u.Object, "${VALUE:=ok}", "data", "v"); err != nil {
				panic(err)
			}
			return withPostBuild(u)
		}},
		"a variable in another annotation": {cluster: func() *stack.Cluster {
			return withPostBuild(marked("default", "b-cm", nil, map[string]string{ssaKey: "Ignore", "z": "${VALUE:=ok}"}))
		}},
		// kustomize-controller decrypts a whole SOPS document.
		"an encrypted value": {cluster: func() *stack.Cluster {
			return baseCluster(marked("default", "b-cm", nil, map[string]string{ssaKey: "Ignore", reconcileKey: "ENC[x]"}))
		}},
	}
	for name, c := range cases {
		for _, placement := range append(slices.Clone(integratedPlacements), layout.FluxSeparate) {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				if err := c.integrate(t, placement); err != nil {
					t.Fatalf("got %v, want the tree admitted: another copy may create b-cm", err)
				}
			})
		}
	}
}
