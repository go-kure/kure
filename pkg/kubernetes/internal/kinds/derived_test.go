package kinds

import (
	"sort"
	"strings"
	"testing"

	"github.com/go-kure/kure/pkg/kubernetes/internal/markers"
)

// The baseline this work item starts from. Pinned as numbers so the first
// output of the derived table is asserted against the hand-seeded one before
// the hand-kept sets are removed: a derivation that agrees with the tables on
// every kind, and a count that moves only when the scheme does.
const (
	wantKinds      = 128
	wantCluster    = 33
	wantNamespaced = 95
)

func TestBaselineCounts(t *testing.T) {
	all, err := Registered()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != wantKinds {
		t.Errorf("registered kinds = %d, want %d (a scheme change must move this number deliberately)", len(all), wantKinds)
	}
	if len(clusterKinds) != wantCluster {
		t.Errorf("clusterKinds = %d, want %d", len(clusterKinds), wantCluster)
	}
	if len(namespacedKinds) != wantNamespaced {
		t.Errorf("namespacedKinds = %d, want %d", len(namespacedKinds), wantNamespaced)
	}
	if len(clusterKinds)+len(namespacedKinds) != len(all) {
		t.Errorf("tables cover %d kinds, scheme has %d", len(clusterKinds)+len(namespacedKinds), len(all))
	}
}

// TestDerivedScopesAgreeWithTheHandSeededTables is the crossover check. It runs
// while both sources exist, so the marker derivation is proven against the
// table it replaces rather than trusted after the table is gone.
func TestDerivedScopesAgreeWithTheHandSeededTables(t *testing.T) {
	all, types := loadRegistered(t)
	derived, err := DeriveScopes(all, types)
	if err != nil {
		t.Fatal(err)
	}
	if len(derived) != len(all) {
		t.Fatalf("derived %d scopes for %d kinds", len(derived), len(all))
	}

	byKey := map[string]Kind{}
	for _, k := range all {
		byKey[k.Key()] = k
	}

	var unmarked []string
	for _, d := range derived {
		k := byKey[d.Key]
		wantCluster := clusterKinds[d.Key]
		gotCluster := d.Scope == markers.ScopeCluster
		if gotCluster == wantCluster {
			continue
		}
		// A disagreement is either a module that carries no scope markers at
		// all (built-ins: the API server defines their scope), or a real
		// defect in the parser. Separate the two so the second is not hidden
		// by the first.
		if !gotCluster && wantCluster {
			unmarked = append(unmarked, d.Key+" ["+k.ImportPath+"]")
			continue
		}
		t.Errorf("%s (%s.%s): derived %s, table says cluster=%v", d.Key, k.ImportPath, k.TypeName, d.Scope, wantCluster)
	}
	sort.Strings(unmarked)
	if len(unmarked) > 0 {
		t.Logf("%d cluster-scoped kinds derived as namespaced (expected for unmarked modules):\n  %s",
			len(unmarked), strings.Join(unmarked, "\n  "))
	}
}

// Every kind's module and version must be resolved, or the kinds table would
// carry blank provenance columns.
func TestDerivedScopesCarryModuleProvenance(t *testing.T) {
	all, types := loadRegistered(t)
	derived, err := DeriveScopes(all, types)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range derived {
		if d.Module == "" || d.Version == "" {
			t.Errorf("%s: module/version = %q/%q, want both set", d.Key, d.Module, d.Version)
		}
	}
}
