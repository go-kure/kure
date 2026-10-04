package argocd

import (
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// oneConfigMap is a minimal application config: one ConfigMap named after the
// application.
type oneConfigMap struct{}

func (oneConfigMap) Generate(app *stack.Application) ([]*client.Object, error) {
	u := &unstructured.Unstructured{}
	u.SetAPIVersion("v1")
	u.SetKind("ConfigMap")
	u.SetName(app.Name)
	u.SetNamespace("default")
	var o client.Object = u
	return []*client.Object{&o}, nil
}

// intentCluster is platform (bundle platform-bundle: core) -> web (umbrella
// web-bundle with child web-db: db). Application db carries intent.
func intentCluster(intent stack.DeliveryIntent) *stack.Cluster {
	db := stack.NewApplication("db", "default", oneConfigMap{})
	db.Delivery = intent
	umbrella := &stack.Bundle{Name: "web-bundle", Children: []*stack.Bundle{{Name: "web-db", Applications: []*stack.Application{db}}}}
	web := &stack.Node{Name: "web", Bundle: umbrella}
	root := &stack.Bundle{Name: "platform-bundle", Applications: []*stack.Application{stack.NewApplication("core", "default", oneConfigMap{})}}
	return &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Bundle: root, Children: []*stack.Node{web}}}
}

// TestDeliveryIntent_Refused: the ArgoCD workflow has no mapping for a
// delivery intent, so CreateLayoutWithResources and IntegrateWithLayout refuse
// a set one under every grouping, naming the application, rather than
// rendering its objects as if nothing was asked.
func TestDeliveryIntent_Refused(t *testing.T) {
	intents := map[string]stack.DeliveryIntent{
		"prune": {PruneProtection: true},
		"force": {ForceReplace: true},
	}
	for name, intent := range intents {
		t.Run(name, func(t *testing.T) {
			check := func(entry string, err error) {
				t.Helper()
				if err == nil {
					t.Errorf("%s accepted a delivery intent", entry)
					return
				}
				for _, want := range []string{`application "db"`, "delivery intent", "ArgoCD"} {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("%s: error %q does not contain %q", entry, err, want)
					}
				}
			}
			for gname, rules := range map[string]layout.LayoutRules{
				"default": layout.DefaultLayoutRules(),
				"flat":    {BundleGrouping: layout.GroupFlat, ApplicationGrouping: layout.GroupFlat},
				"byName":  {BundleGrouping: layout.GroupByName, ApplicationGrouping: layout.GroupByName},
			} {
				_, err := Engine().CreateLayoutWithResources(intentCluster(intent), rules)
				check("CreateLayoutWithResources/"+gname, err)

				c := intentCluster(intent)
				ml, err := layout.WalkCluster(c, rules)
				if err != nil {
					t.Fatal(err)
				}
				check("IntegrateWithLayout/"+gname, Engine().IntegrateWithLayout(ml, c, rules))
			}
		})
	}
}

// TestDeliveryIntent_RefusedForAHandBuiltLayout: a layout the caller built
// carries no application records, so IntegrateWithLayout reads the intent from
// the cluster it is given too, umbrella children included.
func TestDeliveryIntent_RefusedForAHandBuiltLayout(t *testing.T) {
	handBuilt := &layout.ManifestLayout{Name: "manifests"}
	err := Engine().IntegrateWithLayout(handBuilt, intentCluster(stack.DeliveryIntent{PruneProtection: true}), layout.DefaultLayoutRules())
	if err == nil {
		t.Fatal("IntegrateWithLayout accepted a cluster whose application sets a delivery intent")
	}
	for _, want := range []string{`application "db"`, "delivery intent", "ArgoCD"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err, want)
		}
	}
	if err := Engine().IntegrateWithLayout(handBuilt, intentCluster(stack.DeliveryIntent{}), layout.DefaultLayoutRules()); err != nil {
		t.Errorf("without an intent: %v", err)
	}
	if err := Engine().IntegrateWithLayout(handBuilt, nil, layout.DefaultLayoutRules()); err != nil {
		t.Errorf("without a cluster: %v", err)
	}

	// A node tree that loops back is read once, not forever.
	c := intentCluster(stack.DeliveryIntent{})
	c.Node.Children[0].Children = []*stack.Node{c.Node}
	if err := Engine().IntegrateWithLayout(handBuilt, c, layout.DefaultLayoutRules()); err != nil {
		t.Errorf("looping node tree without an intent: %v", err)
	}
}

// TestDeliveryIntent_UnsetAccepted: without an intent the same cluster renders.
func TestDeliveryIntent_UnsetAccepted(t *testing.T) {
	c := intentCluster(stack.DeliveryIntent{})
	if _, err := Engine().CreateLayoutWithResources(intentCluster(stack.DeliveryIntent{}), layout.DefaultLayoutRules()); err != nil {
		t.Errorf("CreateLayoutWithResources: %v", err)
	}
	ml, err := layout.WalkCluster(c, layout.DefaultLayoutRules())
	if err != nil {
		t.Fatal(err)
	}
	if err := Engine().IntegrateWithLayout(ml, c, layout.DefaultLayoutRules()); err != nil {
		t.Errorf("IntegrateWithLayout: %v", err)
	}
}
