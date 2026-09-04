package kinds

import (
	"sort"
	"strings"
	"testing"

	"github.com/go-kure/kure/pkg/kubernetes/internal/markers"
)

// The shape of the registered surface, pinned as numbers so a change to it is
// deliberate rather than incidental. They are asserted against the derivation
// and against the frozen fixture, which is what the hand-seeded scope table
// used to answer for.
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
	cluster := 0
	for _, k := range all {
		if !k.Namespaced {
			cluster++
		}
	}
	if cluster != wantCluster {
		t.Errorf("cluster-scoped kinds = %d, want %d", cluster, wantCluster)
	}
	if len(all)-cluster != wantNamespaced {
		t.Errorf("namespaced kinds = %d, want %d", len(all)-cluster, wantNamespaced)
	}
	if len(frozenClusterScoped) != wantCluster {
		t.Errorf("the frozen fixture holds %d kinds, want %d", len(frozenClusterScoped), wantCluster)
	}
}

// Registered fills each Kind's scope from the resolution. A dropped or mis-keyed
// fill-in step would leave the zero value, Namespaced=false, for every kind —
// and the zero value is the right answer for 33 of the 128, so it neither
// crashes nor looks obviously wrong.
func TestRegisteredScopesComeFromTheResolution(t *testing.T) {
	all, types := loadRegistered(t)
	resolved, err := ResolveScopes(all, types)
	if err != nil {
		t.Fatal(err)
	}
	scopes := map[string]markers.Scope{}
	for _, d := range resolved {
		scopes[d.Key] = d.Scope
	}
	namespaced, cluster := 0, 0
	for _, k := range all {
		scope, ok := scopes[k.Key()]
		if !ok {
			t.Errorf("%s: no resolved scope", k.Key())
			continue
		}
		if want := scope != markers.ScopeCluster; k.Namespaced != want {
			t.Errorf("%s: Registered says namespaced=%v, the resolution says %v", k.Key(), k.Namespaced, want)
		}
		if k.Namespaced {
			namespaced++
		} else {
			cluster++
		}
	}
	// Both counts non-zero: agreement between two all-false answers would be
	// agreement about nothing.
	if namespaced == 0 || cluster == 0 {
		t.Errorf("%d namespaced / %d cluster-scoped; both must be non-zero for the agreement above to mean anything", namespaced, cluster)
	}
}

// DeriveScopes reads the markers and nothing else. For every kind a marker does
// answer for, that answer must match the frozen fixture: this is the check that
// fails if the marker parsing or the go/ast comment reattachment regresses.
// Kinds answered by the built-in table or by a shipped CRD are separated out
// rather than folded in, so the second cannot be hidden behind the first.
func TestDerivedScopesAgreeWithTheFrozenFixture(t *testing.T) {
	all, types := loadRegistered(t)
	derived, err := DeriveScopes(all, types)
	if err != nil {
		t.Fatal(err)
	}
	if len(derived) != len(all) {
		t.Fatalf("derived %d scopes for %d kinds", len(derived), len(all))
	}
	resolved, err := ResolveScopes(all, types)
	if err != nil {
		t.Fatal(err)
	}
	source := map[string]ScopeSource{}
	for _, d := range resolved {
		source[d.Key] = d.Source
	}
	byKey := map[string]Kind{}
	for _, k := range all {
		byKey[k.Key()] = k
	}

	fromMarker, clusterFromMarker := 0, 0
	var elsewhere []string
	for _, d := range derived {
		k := byKey[d.Key]
		wantCluster := frozenClusterScoped[d.Key]
		gotCluster := d.Scope == markers.ScopeCluster
		if source[d.Key] != SourceMarker {
			// The built-in modules carry no markers at all, and an unmarked CRD
			// type is answered from the manifest its module ships. Neither is a
			// parser result, so neither is evidence about the parser.
			if gotCluster {
				t.Errorf("%s (%s.%s): a marker derived Cluster, but the resolution used %s", d.Key, k.ImportPath, k.TypeName, source[d.Key])
			}
			elsewhere = append(elsewhere, d.Key+" ["+string(source[d.Key])+"]")
			continue
		}
		fromMarker++
		if gotCluster {
			clusterFromMarker++
		}
		if gotCluster != wantCluster {
			t.Errorf("%s (%s.%s): the marker derives %s, the frozen fixture says cluster=%v", d.Key, k.ImportPath, k.TypeName, d.Scope, wantCluster)
		}
	}
	if fromMarker == 0 || clusterFromMarker == 0 {
		t.Fatalf("%d kinds answered by a marker, %d of them cluster-scoped; both must be non-zero or this test asserts nothing", fromMarker, clusterFromMarker)
	}
	sort.Strings(elsewhere)
	t.Logf("%d kinds answered by their own marker (%d cluster-scoped); %d answered elsewhere:\n  %s",
		fromMarker, clusterFromMarker, len(elsewhere), strings.Join(elsewhere, "\n  "))
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
