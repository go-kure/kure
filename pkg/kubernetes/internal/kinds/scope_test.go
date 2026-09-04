package kinds

import (
	"testing"

	"github.com/go-kure/kure/pkg/kubernetes/internal/markers"
	"github.com/go-kure/kure/pkg/kubernetes/internal/upstream"
)

// loadRegistered returns the registered kinds and the pinned upstream sources
// they are declared in. Loading is the expensive step, so the functions under
// test take the loaded map rather than each loading it again.
func loadRegistered(t *testing.T) ([]Kind, map[string]upstream.Type) {
	t.Helper()
	all, err := Registered()
	if err != nil {
		t.Fatal(err)
	}
	types, err := LoadTypes(all)
	if err != nil {
		t.Fatal(err)
	}
	return all, types
}

// TestResolvedScopesMatchTheHandSeededTables is the crossover proof: the
// marker-derived scope, plus the built-in table, agrees with the hand-seeded
// sets on all 128 registered kinds. It runs while both sources exist, so the
// derivation is proven against what it replaces rather than trusted after.
func TestResolvedScopesMatchTheHandSeededTables(t *testing.T) {
	all, types := loadRegistered(t)
	resolved, err := ResolveScopes(all, types)
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
	all, types := loadRegistered(t)
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
	all, types := loadRegistered(t)
	resolved, err := ResolveScopes(all, types)
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

// A kind whose module is neither built-in nor marker-bearing resolves to
// namespaced by default, which is indistinguishable from the derivation having
// read nothing at all. Every module in that state must be a recorded decision:
// this fails both when a new one appears and when a recorded one starts
// carrying markers and should leave the list.
func TestUnmarkedModulesMatchTheRecordedExceptions(t *testing.T) {
	_, types := loadRegistered(t)
	got := UnmarkedModules(types)
	for _, m := range got {
		if !unmarkedModules[m] {
			t.Errorf("module %s carries no +kubebuilder:resource marker on any type and is not a recorded exception; check the CRD it ships and record the decision in unmarkedModules", m)
		}
	}
	found := map[string]bool{}
	for _, m := range got {
		found[m] = true
	}
	for m := range unmarkedModules {
		if !found[m] {
			t.Errorf("module %s is recorded as unmarked but now carries markers; drop its entry and let the derivation answer", m)
		}
	}
	modules := map[string]bool{}
	for _, tp := range types {
		if tp.Module != "" && !builtinModules[tp.Module] {
			modules[tp.Module] = true
		}
	}
	if len(modules) == 0 {
		t.Fatal("no non-built-in modules seen")
	}
	t.Logf("%d non-built-in modules checked, %d recorded as unmarked", len(modules), len(got))
}

// Every kind of a recorded unmarked module must be namespaced in the
// hand-seeded table: that is the claim the recorded exception rests on, and it
// is checkable while the table still exists.
func TestUnmarkedModuleKindsAreNamespaced(t *testing.T) {
	all, types := loadRegistered(t)
	resolved, err := ResolveScopes(all, types)
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, d := range resolved {
		if !unmarkedModules[d.Module] {
			continue
		}
		checked++
		if clusterKinds[d.Key] {
			t.Errorf("%s comes from unmarked module %s but the hand-seeded table says cluster-scoped; the namespaced default is wrong for it", d.Key, d.Module)
		}
	}
	if checked == 0 {
		t.Fatal("no kinds from a recorded unmarked module; the exception list no longer matches any registered kind")
	}
}

// The mutation probe for the test above: a module that genuinely carries no
// resource marker must be reported. Without this, TestNoRegisteredModuleIsUnmarked
// passing says only that the loop found nothing — the same thing it would say
// if the check could never fire.
func TestUnmarkedModulesReportsAMarkerLessModule(t *testing.T) {
	types := map[string]upstream.Type{
		"example.com/marked/v1.Marked": {
			ImportPath: "example.com/marked/v1",
			Name:       "Marked",
			Module:     "example.com/marked",
			Doc:        "// +kubebuilder:resource:scope=Cluster\n// Marked is marked.\n",
		},
		"example.com/unmarked/v1.Bare": {
			ImportPath: "example.com/unmarked/v1",
			Name:       "Bare",
			Module:     "example.com/unmarked",
			Doc:        "// Bare declares no resource marker.\n",
		},
		// A built-in module carrying no marker is the expected state, not a
		// finding: builtinClusterScoped answers for it.
		"k8s.io/api/core/v1.Pod": {
			ImportPath: "k8s.io/api/core/v1",
			Name:       "Pod",
			Module:     "k8s.io/api",
			Doc:        "// Pod is a pod.\n",
		},
	}
	got := UnmarkedModules(types)
	if len(got) != 1 || got[0] != "example.com/unmarked" {
		t.Errorf("UnmarkedModules = %v, want [example.com/unmarked]", got)
	}
}

// A module whose only marker declares scope=Namespaced is marked: the
// derivation read a real declaration, and the namespaced answer is upstream's,
// not a default standing in for a failed parse. HasResource, not ResourceScope,
// is what makes that distinction — this pins it.
func TestUnmarkedModulesAcceptsANamespacedOnlyModule(t *testing.T) {
	types := map[string]upstream.Type{
		"example.com/ns/v1.Thing": {
			ImportPath: "example.com/ns/v1",
			Name:       "Thing",
			Module:     "example.com/ns",
			Doc:        "// +kubebuilder:resource:scope=Namespaced\n// Thing is namespaced.\n",
		},
	}
	if got := UnmarkedModules(types); len(got) != 0 {
		t.Errorf("UnmarkedModules = %v, want none", got)
	}
}
