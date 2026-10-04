package fluxcd_test

import (
	"slices"
	"strings"
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// typedItems is a typed object with an Items field, as a hand-written list
// type has one.
type typedItems struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Items             []unstructured.Unstructured `json:"items"`
}

func (l *typedItems) DeepCopyObject() runtime.Object {
	c := *l
	c.Items = slices.Clone(l.Items)
	return &c
}

// typedListOf returns a typed object of the given kind holding items.
func typedListOf(kind string, items ...*unstructured.Unstructured) *typedItems {
	l := &typedItems{TypeMeta: metav1.TypeMeta{APIVersion: "example.com/v1", Kind: kind}}
	l.Name, l.Namespace = "holder", "default"
	for _, it := range items {
		l.Items = append(l.Items, *it)
	}
	return l
}

// umbrellaCluster: prod -> apps (node) -> platform (umbrella bundle) with the
// child bundles infra and services, infra holding the child bundle network.
func umbrellaCluster() *stack.Cluster {
	network := srBundle("network", cmApp("network-app"))
	infra := srBundle("infra", cmApp("infra-app"))
	infra.Children = []*stack.Bundle{network}
	platform := srBundle("platform", cmApp("platform-app"))
	platform.Children = []*stack.Bundle{infra, srBundle("services", cmApp("services-app"))}
	apps := &stack.Node{Name: "apps", Bundle: platform}
	root := &stack.Node{Name: "demo", Children: []*stack.Node{apps}}
	apps.SetParent(root)
	return &stack.Cluster{Name: "demo", Node: root}
}

// umbrellaChildren returns every umbrella child layout of ml's tree.
func umbrellaChildren(ml *layout.ManifestLayout) []*layout.ManifestLayout {
	var out []*layout.ManifestLayout
	var walk func(l *layout.ManifestLayout)
	walk = func(l *layout.ManifestLayout) {
		if l.UmbrellaChild {
			out = append(out, l)
		}
		for _, c := range l.Children {
			walk(c)
		}
	}
	walk(ml)
	return out
}

// TestIntegratedTree_KeptKustomizationAppliesItsDirectory: a Kustomization
// the caller placed where the integration would place its own, with the same
// name and spec.path, is kept in place of a generated one. Its directory is
// applied by it, so the tree writes whatever form the kept object has.
func TestIntegratedTree_KeptKustomizationAppliesItsDirectory(t *testing.T) {
	for _, placement := range placements {
		for _, form := range []string{"typed", "unstructured", "in a List"} {
			t.Run(string(placement)+"/"+form, func(t *testing.T) {
				rules := recursiveRules("nodeOnly", placement)
				if placement == layout.FluxSeparate {
					t.Skip("the flux-system layout is the integration's own; a caller's copy there is refused")
				}

				// Where the integration places the CR of the umbrella child infra.
				first := integrated(t, umbrellaCluster(), rules)
				var hostPath string
				var generated *kustv1.Kustomization
				var find func(l *layout.ManifestLayout)
				find = func(l *layout.ManifestLayout) {
					for _, r := range l.Resources {
						if k, ok := r.(*kustv1.Kustomization); ok && k.Name == "infra" {
							hostPath, generated = l.FullRepoPath(), k
						}
					}
					for _, c := range l.Children {
						find(c)
					}
				}
				find(first)
				if generated == nil {
					t.Fatal("no generated Kustomization named infra")
				}

				cluster := umbrellaCluster()
				ml, err := layout.WalkCluster(cluster, rules)
				if err != nil {
					t.Fatalf("WalkCluster: %v", err)
				}
				content, err := runtime.DefaultUnstructuredConverter.ToUnstructured(generated.DeepCopy())
				if err != nil {
					t.Fatalf("ToUnstructured: %v", err)
				}
				u := &unstructured.Unstructured{Object: content}
				u.SetAPIVersion(kustv1.GroupVersion.String())
				u.SetKind("Kustomization")
				var own client.Object
				switch form {
				case "typed":
					own = generated.DeepCopy()
				case "unstructured":
					own = u
				case "in a List":
					own = wrapInList(u)
				}
				host := layoutAtPath(t, ml, hostPath)
				host.Resources = append(host.Resources, own)

				if err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, cluster, rules); err != nil {
					t.Fatalf("IntegrateWithLayout: %v", err)
				}
				for writer, err := range writeErrs(t, ml) {
					if err != nil {
						t.Errorf("%s: %v", writer, err)
					}
				}
			})
		}
	}
}

// TestIntegratedTree_UnlistedDirectoriesAreApplied: the pre-write check
// refuses a directory its parent does not list and no Flux Kustomization
// builds (go-kure/kure#977). The integrator marks every directory one of its
// Kustomizations names, so each umbrella child of an integrated tree is
// marked and every writer accepts the tree, in every placement. The check is
// live on such a tree: with one umbrella child's mark taken away, every
// writer refuses it and names the directory.
func TestIntegratedTree_UnlistedDirectoriesAreApplied(t *testing.T) {
	for _, placement := range placements {
		t.Run(string(placement), func(t *testing.T) {
			rules := recursiveRules("nodeOnly", placement)

			ml := integrated(t, umbrellaCluster(), rules)
			children := umbrellaChildren(ml)
			if len(children) != 3 {
				t.Fatalf("got %d umbrella child layouts, want 3", len(children))
			}
			for _, c := range children {
				if !c.FluxBuild() {
					t.Errorf("umbrella child %q is not marked as built by a Flux Kustomization", c.FullRepoPath())
				}
			}
			writeAll(t, ml)

			for i := range children {
				ml := integrated(t, umbrellaCluster(), rules)
				child := umbrellaChildren(ml)[i]
				child.SetFluxBuild(false)
				want := `layout "` + child.FullRepoPath() + `" is not listed by its parent layout`
				for writer, err := range writeErrs(t, ml) {
					if err == nil || !strings.Contains(err.Error(), want) {
						t.Errorf("%s: got %v, want it to contain %q", writer, err, want)
					}
				}
			}
		})
	}
}
