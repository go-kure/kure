package argocd

import (
	"strings"
	"testing"

	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// TestDeliveryIntent_GenerateFromClusterRefused: GenerateFromCluster walks the
// cluster with the caller's rules and refuses a set delivery intent as the
// other entry points do, naming the application.
func TestDeliveryIntent_GenerateFromClusterRefused(t *testing.T) {
	intents := map[string]stack.DeliveryIntent{
		"prune": {PruneProtection: true},
		"force": {ForceReplace: true},
	}
	groupings := map[string]layout.LayoutRules{
		"default": layout.DefaultLayoutRules(),
		"flat":    {BundleGrouping: layout.GroupFlat, ApplicationGrouping: layout.GroupFlat},
		"byName":  {BundleGrouping: layout.GroupByName, ApplicationGrouping: layout.GroupByName},
	}
	for name, intent := range intents {
		for gname, rules := range groupings {
			t.Run(name+"/"+gname, func(t *testing.T) {
				objs, err := Engine().GenerateFromCluster(intentCluster(intent), rules)
				if err == nil {
					t.Fatalf("GenerateFromCluster accepted a delivery intent and returned %d objects", len(objs))
				}
				for _, want := range []string{`application "db"`, "delivery intent", "ArgoCD"} {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("error %q does not contain %q", err, want)
					}
				}
			})
		}
	}
}

// TestDeliveryIntent_GenerateFromClusterUnsetAccepted: without an intent the
// same cluster renders its three Applications.
func TestDeliveryIntent_GenerateFromClusterUnsetAccepted(t *testing.T) {
	objs, err := Engine().GenerateFromCluster(intentCluster(stack.DeliveryIntent{}), layout.DefaultLayoutRules())
	if err != nil || len(objs) != 3 {
		t.Fatalf("GenerateFromCluster: %d objects, %v; want the three Applications", len(objs), err)
	}
}
