package fluxcd_test

import (
	"maps"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// Tests that what the Flux workflow returns shares no label or annotation map
// with the bundles it was generated from (go-kure/kure#1082): an in-place
// edit of a returned object's maps leaves every input bundle as it was.

// metadataCluster builds a fresh cluster whose every bundle sets labels and
// annotations, and returns its bundles. An empty map that is not nil is one of
// them: a generator that hands it on as it is shares it as surely as a full one.
type metadataCluster func() (*stack.Cluster, []*stack.Bundle)

var metadataClusters = map[string]metadataCluster{
	"three bundles": func() (*stack.Cluster, []*stack.Bundle) {
		var bundles []*stack.Bundle
		c := mergedCluster(func(rb, b1, b2 *stack.Bundle) {
			rb.Labels = map[string]string{"team": "platform"}
			rb.Annotations = map[string]string{"owner": "platform"}
			b1.Labels = map[string]string{"tier": "web"}
			b1.Annotations = map[string]string{"note": "one"}
			b2.Labels = map[string]string{"zone": "a"}
			b2.Annotations = map[string]string{}
			bundles = []*stack.Bundle{rb, b1, b2}
		})
		return c, bundles
	},
	"layouts below the application": func() (*stack.Cluster, []*stack.Bundle) {
		b := shopSettings(unordered(nil))
		return oneBundleCluster(b), []*stack.Bundle{b}
	},
}

// snapshotMetadata copies the label and annotation maps of every bundle.
func snapshotMetadata(bundles []*stack.Bundle) [][2]map[string]string {
	out := make([][2]map[string]string, len(bundles))
	for i, b := range bundles {
		out[i] = [2]map[string]string{maps.Clone(b.Labels), maps.Clone(b.Annotations)}
	}
	return out
}

// editMetadata adds a key to the label and annotation maps of every object,
// in place: an unstructured object's own metadata maps, not the copies its
// getters return.
func editMetadata(objs []client.Object) {
	for _, o := range objs {
		if u, ok := o.(*unstructured.Unstructured); ok {
			for _, field := range []string{"labels", "annotations"} {
				if m, found, _ := unstructured.NestedFieldNoCopy(u.Object, "metadata", field); found {
					if m, ok := m.(map[string]any); ok {
						m["edited"] = "x"
					}
				}
			}
			continue
		}
		if l := o.GetLabels(); l != nil {
			l["edited"] = "x"
		}
		if a := o.GetAnnotations(); a != nil {
			a["edited"] = "x"
		}
	}
}

func assertMetadataUnchanged(t *testing.T, bundles []*stack.Bundle, before [][2]map[string]string) {
	t.Helper()
	for i, b := range bundles {
		if !maps.Equal(b.Labels, before[i][0]) {
			t.Errorf("bundle %s: labels = %v after editing the output, want %v", b.Name, b.Labels, before[i][0])
		}
		if !maps.Equal(b.Annotations, before[i][1]) {
			t.Errorf("bundle %s: annotations = %v after editing the output, want %v", b.Name, b.Annotations, before[i][1])
		}
	}
}

// allLayoutObjects returns the resources of ml and of every layout below it.
func allLayoutObjects(ml *layout.ManifestLayout) []client.Object {
	out := append([]client.Object(nil), ml.Resources...)
	for _, c := range ml.Children {
		out = append(out, allLayoutObjects(c)...)
	}
	return out
}

// metadataRules are the layout rules each entry point is run with: the
// default, and one that merges every bundle into one Kustomization.
func metadataRules(placement layout.FluxPlacement) map[string]layout.LayoutRules {
	def := layout.DefaultLayoutRules()
	def.FluxPlacement = placement
	flat := allFlat
	flat.FluxPlacement = placement
	return map[string]layout.LayoutRules{"default": def, "flat": flat}
}

// TestGenerateFromCluster_OutputSharesNoBundleMetadata: the Kustomizations
// GenerateFromCluster returns, one per bundle or one for several, are the
// caller's to edit.
func TestGenerateFromCluster_OutputSharesNoBundleMetadata(t *testing.T) {
	for cname, build := range metadataClusters {
		for rname, rules := range metadataRules(layout.FluxSeparate) {
			t.Run(cname+"/"+rname, func(t *testing.T) {
				c, bundles := build()
				before := snapshotMetadata(bundles)
				objs, err := fluxstack.NewResourceGenerator().GenerateFromCluster(c, rules)
				if err != nil {
					t.Fatalf("GenerateFromCluster: %v", err)
				}
				if len(kustomizationsIn(objs)) == 0 {
					t.Fatal("GenerateFromCluster returned no Kustomization")
				}
				editMetadata(objs)
				assertMetadataUnchanged(t, bundles, before)
			})
		}
	}
}

// TestCreateLayoutWithResources_OutputSharesNoBundleMetadata: an in-place
// edit of every object the layout CreateLayoutWithResources returns holds
// (the Kustomizations, per-layout ones included, and the applications'
// objects, which the walk gives no bundle label) reaches no bundle, in every
// placement.
func TestCreateLayoutWithResources_OutputSharesNoBundleMetadata(t *testing.T) {
	for cname, build := range metadataClusters {
		for _, placement := range allPlacements {
			for rname, rules := range metadataRules(placement) {
				t.Run(cname+"/"+string(placement)+"/"+rname, func(t *testing.T) {
					c, bundles := build()
					before := snapshotMetadata(bundles)
					ml := integrated(t, c, rules)
					objs := allLayoutObjects(ml)
					if len(kustomizationsIn(objs)) == 0 {
						t.Fatal("the layout holds no Kustomization")
					}
					editMetadata(objs)
					assertMetadataUnchanged(t, bundles, before)
				})
			}
		}
	}
}

// TestGenerateForBundle_OutputSharesNoBundleMetadata: the one-bundle entry
// point's Kustomization is the caller's to edit too.
func TestGenerateForBundle_OutputSharesNoBundleMetadata(t *testing.T) {
	_, bundles := metadataClusters["three bundles"]()
	b := bundles[0]
	before := snapshotMetadata(bundles[:1])
	objs, err := fluxstack.NewResourceGenerator().GenerateForBundle(b, "clusters/demo/rb")
	if err != nil {
		t.Fatalf("GenerateForBundle: %v", err)
	}
	if len(kustomizationsIn(objs)) != 1 {
		t.Fatalf("GenerateForBundle returned %d Kustomizations, want 1", len(kustomizationsIn(objs)))
	}
	editMetadata(objs)
	assertMetadataUnchanged(t, bundles[:1], before)
}

func kustomizationsIn(objs []client.Object) []client.Object {
	var out []client.Object
	for _, o := range objs {
		if o.GetObjectKind().GroupVersionKind().Kind == "Kustomization" {
			out = append(out, o)
		}
	}
	return out
}
