package kinds

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"

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

// frozenClusterScoped is the cluster-scoped half of the hand-seeded scope table
// this package used to carry, copied verbatim before that table was deleted and
// frozen at the pins current on 2026-09-04 (k8s.io/api v0.37.0, cilium v1.19.5,
// cert-manager v1.21.1, cnpg v1.30.0, external-secrets, gateway-api v1.6.1).
//
// It is a fixture, not a source: nothing reads it outside this test. Its job is
// to keep the crossover proof alive now that its oracle is gone. The derivation
// fails silently by construction — an absent, unread or detached
// +kubebuilder:resource marker resolves to Namespaced, which is also the correct
// answer for 95 of the 128 kinds — so a regression that reverted internal/upstream
// to reading GenDecl.Doc alone would turn 31 of these 33 namespaced with nothing
// else going red. Against this literal it goes red.
//
// A legitimate upstream change to any of these — a kind re-scoped, added or
// removed by a pin bump — is a deliberate edit here, made with the upstream
// change named in the commit message. Never edit it to make the test pass.
var frozenClusterScoped = set(
	"/ComponentStatus",
	"/Namespace",
	"/Node",
	"/PersistentVolume",
	"/RangeAllocation",
	"apiextensions.k8s.io/CustomResourceDefinition",
	"cert-manager.io/ClusterIssuer",
	"cilium.io/CiliumBGPAdvertisement",
	"cilium.io/CiliumBGPClusterConfig",
	"cilium.io/CiliumBGPNodeConfig",
	"cilium.io/CiliumBGPNodeConfigOverride",
	"cilium.io/CiliumBGPPeerConfig",
	"cilium.io/CiliumCIDRGroup",
	"cilium.io/CiliumClusterwideEnvoyConfig",
	"cilium.io/CiliumClusterwideNetworkPolicy",
	"cilium.io/CiliumEgressGatewayPolicy",
	"cilium.io/CiliumIdentity",
	"cilium.io/CiliumLoadBalancerIPPool",
	"cilium.io/CiliumNode",
	"external-secrets.io/ClusterExternalSecret",
	"external-secrets.io/ClusterSecretStore",
	"gateway.networking.k8s.io/GatewayClass",
	"networking.k8s.io/IPAddress",
	"networking.k8s.io/IngressClass",
	"networking.k8s.io/ServiceCIDR",
	"postgresql.cnpg.io/ClusterImageCatalog",
	"rbac.authorization.k8s.io/ClusterRole",
	"rbac.authorization.k8s.io/ClusterRoleBinding",
	"storage.k8s.io/CSIDriver",
	"storage.k8s.io/CSINode",
	"storage.k8s.io/StorageClass",
	"storage.k8s.io/VolumeAttachment",
	"storage.k8s.io/VolumeAttributesClass",
)

// TestResolvedScopesMatchTheFrozenFixture is the crossover proof, kept after the
// hand-seeded table it originally ran against was deleted. Both directions are
// checked: a kind in the fixture must resolve to Cluster, and a kind resolving
// to Cluster must be in the fixture. One direction alone would miss half the
// failures — a derivation that answered Cluster for everything would pass a
// fixture-only check.
func TestResolvedScopesMatchTheFrozenFixture(t *testing.T) {
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
	gotCluster := map[string]bool{}
	for _, d := range resolved {
		k := byKey[d.Key]
		isCluster := d.Scope == markers.ScopeCluster
		if isCluster {
			gotCluster[d.Key] = true
		}
		if want := frozenClusterScoped[d.Key]; isCluster != want {
			t.Errorf("%s (%s.%s, module %s, source %s): resolved %s, the frozen fixture says cluster=%v",
				d.Key, k.ImportPath, k.TypeName, d.Module, d.Source, d.Scope, want)
		}
	}
	for key := range frozenClusterScoped {
		if _, ok := byKey[key]; !ok {
			t.Errorf("the frozen fixture lists %q, which is no longer registered", key)
			continue
		}
		if !gotCluster[key] {
			t.Errorf("the frozen fixture lists %q as cluster-scoped, but nothing resolved it that way", key)
		}
	}
	if !t.Failed() {
		by := map[ScopeSource]int{}
		for _, d := range resolved {
			by[d.Source]++
		}
		t.Logf("all %d kinds agree with the frozen fixture (%d cluster-scoped); %d from their own marker, %d from the built-in table, %d from the CRD their module ships",
			len(resolved), len(gotCluster), by[SourceMarker], by[SourceBuiltinTable], by[SourceShippedCRD])
	}
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

// Not every registered kind carries a marker of its own, and the ones that do
// not are answered from the CRD their module ships. This records how many of
// each there are and asserts every unmarked kind was in fact answered that way
// — the count is the thing a module reorganising its comments would move.
func TestUnmarkedKindsAreAnsweredByTheShippedCRDs(t *testing.T) {
	all, types := loadRegistered(t)
	unmarked := map[string]bool{}
	for _, key := range UnmarkedKinds(all, types) {
		unmarked[key] = true
	}
	resolved, err := ResolveScopes(all, types)
	if err != nil {
		t.Fatal(err)
	}
	bySource := map[ScopeSource]int{}
	for _, d := range resolved {
		bySource[d.Source]++
		if unmarked[d.Key] && d.Source != SourceShippedCRD {
			t.Errorf("%s carries no marker but was resolved from %q", d.Key, d.Source)
		}
		if !unmarked[d.Key] && d.Source == SourceShippedCRD {
			t.Errorf("%s carries a marker but was resolved from the shipped CRD", d.Key)
		}
		if d.Source == "" {
			t.Errorf("%s: no scope source recorded", d.Key)
		}
	}
	if bySource[SourceShippedCRD] == 0 {
		t.Error("no kind was resolved from a shipped CRD; the fallback is no longer exercised by the real pins")
	}
	t.Logf("scope sources: %d marker, %d builtin, %d shipped CRD",
		bySource[SourceMarker], bySource[SourceBuiltinTable], bySource[SourceShippedCRD])
}

// markerFixture is a two-root module: one root type marked, the other not. It
// is the case a per-module check cannot see — the module marks something, so
// "this module has markers" holds while Unmarked's own kind falls to the
// default. It is also the shape internal/upstream produces for a grouped
// `type (...)` declaration, whose preceding marker block it drops because it
// cannot attribute it to one spec.
func markerFixture() ([]Kind, map[string]upstream.Type) {
	const path = "example.com/api/v1"
	kind := func(name string) Kind {
		return Kind{
			GVK:        schema.GroupVersionKind{Group: "example.com", Version: "v1", Kind: name},
			ImportPath: path,
			TypeName:   name,
		}
	}
	return []Kind{kind("Marked"), kind("Unmarked")}, map[string]upstream.Type{
		upstream.Key(path, "Marked"): {
			ImportPath: path, Name: "Marked", Module: "example.com", Version: "v1",
			Doc: "// +kubebuilder:resource:scope=Cluster\n// Marked is marked.\n",
		},
		upstream.Key(path, "Unmarked"): {
			ImportPath: path, Name: "Unmarked", Module: "example.com", Version: "v1",
			Doc: "// Unmarked declares no resource marker.\n",
		},
	}
}

func TestUnmarkedKindsSeesThroughAPartlyMarkedModule(t *testing.T) {
	all, types := markerFixture()
	got := UnmarkedKinds(all, types)
	if len(got) != 1 || got[0] != "example.com/Unmarked" {
		t.Errorf("UnmarkedKinds = %v, want [example.com/Unmarked]", got)
	}
}

// The guard, probed: an unmarked kind whose module ships nothing to answer for
// it must make resolution fail, not fall to the namespaced default.
func TestResolveScopesRejectsAnUnanswerableUnmarkedKind(t *testing.T) {
	all, types := markerFixture()
	_, err := ResolveScopes(all, types)
	if err == nil {
		t.Fatal("ResolveScopes accepted a kind whose type declares no resource marker")
	}
	if !strings.Contains(err.Error(), "example.com/Unmarked") {
		t.Errorf("error = %v, want it to name the unmarked kind", err)
	}
	// The marked sibling on its own resolves, so the rejection is about the
	// unmarked kind and not about the fixture as a whole.
	resolved, err := ResolveScopes(all[:1], types)
	if err != nil {
		t.Fatalf("ResolveScopes rejected the marked kind alone: %v", err)
	}
	if len(resolved) != 1 || resolved[0].Scope != markers.ScopeCluster || resolved[0].Source != SourceMarker {
		t.Errorf("resolved = %v, want one cluster-scoped marker-sourced entry", resolved)
	}
}

// A module that ships a CRD for its unmarked kind answers for it — and the
// answer is read, not assumed: the fixture's CRD declares Cluster, which is
// the opposite of the default the missing marker would otherwise imply.
func TestResolveScopesReadsTheShippedCRDForAnUnmarkedKind(t *testing.T) {
	dir := t.TempDir()
	manifest := `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: unmarkeds.example.com
spec:
  group: example.com
  names:
    kind: Unmarked
  scope: Cluster
`
	if err := os.WriteFile(filepath.Join(dir, "crd.yaml"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	all, types := markerFixture()
	for key, tp := range types {
		tp.ModuleDir = dir
		types[key] = tp
	}
	resolved, err := ResolveScopes(all, types)
	if err != nil {
		t.Fatalf("ResolveScopes: %v", err)
	}
	for _, d := range resolved {
		if d.Key != "example.com/Unmarked" {
			continue
		}
		if d.Scope != markers.ScopeCluster {
			t.Errorf("%s resolved %s, want Cluster from the shipped CRD", d.Key, d.Scope)
		}
		if d.Source != SourceShippedCRD {
			t.Errorf("%s source = %q, want %q", d.Key, d.Source, SourceShippedCRD)
		}
		return
	}
	t.Fatal("the unmarked kind was not resolved")
}

// A module that ships CRDs but none for this kind is still unanswerable. The
// fallback is not "the module ships CRDs, so trust the default".
func TestResolveScopesRejectsAKindMissingFromTheShippedCRDs(t *testing.T) {
	dir := t.TempDir()
	manifest := `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: others.example.com
spec:
  group: example.com
  names:
    kind: Other
  scope: Namespaced
`
	if err := os.WriteFile(filepath.Join(dir, "crd.yaml"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	all, types := markerFixture()
	for key, tp := range types {
		tp.ModuleDir = dir
		types[key] = tp
	}
	_, err := ResolveScopes(all, types)
	if err == nil {
		t.Fatal("ResolveScopes accepted an unmarked kind its module ships no CRD for")
	}
	if !strings.Contains(err.Error(), "ships no CustomResourceDefinition") {
		t.Errorf("error = %v, want it to say the module ships no CRD for the kind", err)
	}
}

// A kind whose only marker declares scope=Namespaced is marked: the derivation
// read a real declaration, and the namespaced answer is upstream's, not a
// default standing in for a failed parse. HasResource, not ResourceScope, is
// what makes that distinction — this pins it.
func TestANamespacedOnlyMarkerCountsAsMarked(t *testing.T) {
	const path = "example.com/ns/v1"
	all := []Kind{{
		GVK:        schema.GroupVersionKind{Group: "example.com", Version: "v1", Kind: "Thing"},
		ImportPath: path,
		TypeName:   "Thing",
	}}
	types := map[string]upstream.Type{
		upstream.Key(path, "Thing"): {
			ImportPath: path, Name: "Thing", Module: "example.com", Version: "v1",
			Doc: "// +kubebuilder:resource:scope=Namespaced\n// Thing is namespaced.\n",
		},
	}
	if got := UnmarkedKinds(all, types); len(got) != 0 {
		t.Errorf("UnmarkedKinds = %v, want none", got)
	}
	if _, err := ResolveScopes(all, types); err != nil {
		t.Errorf("ResolveScopes: %v", err)
	}
}
