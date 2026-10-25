package argocd

import (
	"maps"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// metadataCluster is intentCluster without an intent, with labels and
// annotations on each of its three bundles, which it returns as well.
func metadataCluster() (*stack.Cluster, []*stack.Bundle) {
	c := intentCluster(stack.DeliveryIntent{})
	root, umbrella := c.Node.Bundle, c.Node.Children[0].Bundle
	child := umbrella.Children[0]
	root.Labels = map[string]string{"team": "platform"}
	root.Annotations = map[string]string{"owner": "platform"}
	umbrella.Labels = map[string]string{"tier": "web"}
	umbrella.Annotations = map[string]string{"note": "web"}
	child.Labels = map[string]string{"tier": "web", "part": "db"}
	child.Annotations = map[string]string{}
	return c, []*stack.Bundle{root, umbrella, child}
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

func allLayoutObjects(ml *layout.ManifestLayout) []client.Object {
	out := append([]client.Object(nil), ml.Resources...)
	for _, c := range ml.Children {
		out = append(out, allLayoutObjects(c)...)
	}
	return out
}

// TestOutputSharesNoBundleMetadata: the Applications GenerateFromCluster
// returns, and every object of the layout CreateLayoutWithResources returns,
// are the caller's to edit: an in-place edit of their label and annotation
// maps leaves every input bundle as it was (go-kure/kure#1082). The flat rules
// merge the three bundles into one Application.
func TestOutputSharesNoBundleMetadata(t *testing.T) {
	flat := layout.LayoutRules{NodeGrouping: layout.GroupFlat, BundleGrouping: layout.GroupFlat, ApplicationGrouping: layout.GroupFlat}
	entryPoints := map[string]func(*stack.Cluster, layout.LayoutRules) ([]client.Object, error){
		"GenerateFromCluster": func(c *stack.Cluster, rules layout.LayoutRules) ([]client.Object, error) {
			return Engine().GenerateFromCluster(c, rules)
		},
		"CreateLayoutWithResources": func(c *stack.Cluster, rules layout.LayoutRules) ([]client.Object, error) {
			res, err := Engine().CreateLayoutWithResources(c, rules)
			if err != nil {
				return nil, err
			}
			return allLayoutObjects(res.(*layout.ManifestLayout)), nil
		},
	}
	for ename, generate := range entryPoints {
		for rname, rules := range map[string]layout.LayoutRules{"default": layout.DefaultLayoutRules(), "flat": flat} {
			t.Run(ename+"/"+rname, func(t *testing.T) {
				c, bundles := metadataCluster()
				before := make([][2]map[string]string, len(bundles))
				for i, b := range bundles {
					before[i] = [2]map[string]string{maps.Clone(b.Labels), maps.Clone(b.Annotations)}
				}
				objs, err := generate(c, rules)
				if err != nil {
					t.Fatalf("%s: %v", ename, err)
				}
				labelled := 0
				for _, o := range objs {
					if o.GetObjectKind().GroupVersionKind().Kind == "Application" && len(o.GetLabels()) > 0 {
						labelled++
					}
				}
				if labelled == 0 {
					t.Fatal("no Application carries labels: the test edits nothing that could be shared")
				}
				editMetadata(objs)
				for i, b := range bundles {
					if !maps.Equal(b.Labels, before[i][0]) {
						t.Errorf("bundle %s: labels = %v after editing the output, want %v", b.Name, b.Labels, before[i][0])
					}
					if !maps.Equal(b.Annotations, before[i][1]) {
						t.Errorf("bundle %s: annotations = %v after editing the output, want %v", b.Name, b.Annotations, before[i][1])
					}
				}
			})
		}
	}
}
