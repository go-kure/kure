package kinds

import (
	"testing"

	"github.com/go-kure/kure/pkg/kubernetes/internal/markers"
	"github.com/go-kure/kure/pkg/kubernetes/internal/upstream"
)

// TestResolvedScopesMatchTheHandSeededTables is the crossover proof: the
// marker-derived scope, plus the built-in table, agrees with the hand-seeded
// sets on all 128 registered kinds. It runs while both sources exist, so the
// derivation is proven against what it replaces rather than trusted after.
func TestResolvedScopesMatchTheHandSeededTables(t *testing.T) {
	all, err := Registered()
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := ResolveScopes(all)
	if err != nil {
		t.Fatal(err)
	}
	if len(resolved) != len(all) {
		t.Fatalf("resolved %d scopes for %d kinds", len(resolved), len(all))
	}
	byKey := map[string]Kind{}
	for _, k := range all {
		byKey[k.Key()] = k
	}
	mismatches := 0
	for _, d := range resolved {
		k := byKey[d.Key]
		wantCluster := clusterKinds[d.Key]
		if !wantCluster && !namespacedKinds[d.Key] {
			t.Errorf("%s: in neither hand-seeded set", d.Key)
			continue
		}
		if gotCluster := d.Scope == markers.ScopeCluster; gotCluster != wantCluster {
			mismatches++
			t.Errorf("%s (%s.%s, module %s): resolved %s, hand-seeded table says cluster=%v",
				d.Key, k.ImportPath, k.TypeName, d.Module, d.Scope, wantCluster)
		}
	}
	if mismatches == 0 {
		t.Logf("all %d kinds agree; %d built-in kinds came from the table, the rest from markers",
			len(resolved), countBuiltins(resolved))
	}
}

func countBuiltins(resolved []DerivedScope) int {
	n := 0
	for _, d := range resolved {
		if builtinModules[d.Module] {
			n++
		}
	}
	return n
}

// The built-in table is only correct while those modules really carry no
// scope markers. If upstream starts emitting them, this fails and the modules
// should move to the derived path rather than keeping a table that has silently
// become a second, competing source of truth.
func TestBuiltinModulesCarryNoScopeMarkers(t *testing.T) {
	all, err := Registered()
	if err != nil {
		t.Fatal(err)
	}
	paths := map[string]bool{}
	for _, k := range all {
		paths[k.ImportPath] = true
	}
	types, err := upstream.Load(sortedKeys(paths))
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, k := range all {
		tp, ok := types[upstream.Key(k.ImportPath, k.TypeName)]
		if !ok || !builtinModules[tp.Module] {
			continue
		}
		checked++
		scope, err := markers.ResourceScope(tp.Doc)
		if err != nil {
			t.Errorf("%s: %v", k.Key(), err)
			continue
		}
		if scope == markers.ScopeCluster {
			t.Errorf("%s (%s): a built-in module now declares scope=Cluster; move %s to the derived path and drop its builtinClusterScoped entry",
				k.Key(), tp.Module, tp.Module)
		}
	}
	if checked == 0 {
		t.Fatal("no built-in kinds were checked; builtinModules no longer matches any registered kind")
	}
}

// Every entry in the built-in table must name a kind that is actually
// registered and actually built-in, so a kind leaving the scheme cannot leave a
// stale row behind.
func TestBuiltinClusterScopedHasNoStaleEntries(t *testing.T) {
	all, err := Registered()
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := ResolveScopes(all)
	if err != nil {
		t.Fatal(err)
	}
	module := map[string]string{}
	for _, d := range resolved {
		module[d.Key] = d.Module
	}
	for key := range builtinClusterScoped {
		mod, ok := module[key]
		if !ok {
			t.Errorf("builtinClusterScoped lists %q, which is not registered", key)
			continue
		}
		if !builtinModules[mod] {
			t.Errorf("builtinClusterScoped lists %q, but its module %q is marker-derived", key, mod)
		}
	}
}

// A kind whose module is neither built-in nor marker-bearing would resolve to
// namespaced by default. Prove no such kind exists: every non-built-in kind
// comes from a module that marks at least one of its types.
func TestEveryNonBuiltinModuleCarriesMarkers(t *testing.T) {
	all, err := Registered()
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := ResolveScopes(all)
	if err != nil {
		t.Fatal(err)
	}
	paths := map[string]bool{}
	for _, k := range all {
		paths[k.ImportPath] = true
	}
	types, err := upstream.Load(sortedKeys(paths))
	if err != nil {
		t.Fatal(err)
	}
	marked := map[string]bool{}
	for _, tp := range types {
		if tp.Module == "" {
			continue
		}
		if scope, err := markers.ResourceScope(tp.Doc); err == nil && scope == markers.ScopeCluster {
			marked[tp.Module] = true
		}
	}
	seen := map[string]bool{}
	for _, d := range resolved {
		if builtinModules[d.Module] || seen[d.Module] {
			continue
		}
		seen[d.Module] = true
		if !marked[d.Module] && d.Scope == markers.ScopeCluster {
			t.Errorf("module %s resolved a cluster scope but declares no cluster marker anywhere", d.Module)
		}
	}
	if len(seen) == 0 {
		t.Fatal("no non-built-in modules seen")
	}
}
