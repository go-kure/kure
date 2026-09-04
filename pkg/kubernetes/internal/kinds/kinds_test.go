package kinds

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/go-kure/kure/pkg/kubernetes/internal/markers"
)

func TestRegistered_IsTotalSortedAndRouted(t *testing.T) {
	all, err := Registered()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) == 0 {
		t.Fatal("no kinds registered")
	}
	seen := map[string]bool{}
	for _, k := range all {
		if strings.HasSuffix(k.GVK.Kind, "List") {
			t.Errorf("%s: list kinds must be excluded", k.GVK)
		}
		if strings.HasPrefix(k.ImportPath, "k8s.io/apimachinery/") {
			t.Errorf("%s: apimachinery types must be excluded", k.GVK)
		}
		if k.Type == nil || k.TypeName == "" || k.ImportPath == "" {
			t.Errorf("%s: incomplete kind %+v", k.GVK, k)
		}
		if _, ok := routePackage(k.ImportPath); !ok {
			t.Errorf("%s: unrouted import path %s", k.GVK, k.ImportPath)
		}
		id := k.GVK.String()
		if seen[id] {
			t.Errorf("%s listed twice", id)
		}
		seen[id] = true
	}
	if !sort.SliceIsSorted(all, func(i, j int) bool {
		a, b := all[i], all[j]
		if a.Package != b.Package {
			return a.Package < b.Package
		}
		if a.GVK.Kind != b.GVK.Kind {
			return a.GVK.Kind < b.GVK.Kind
		}
		if a.GVK.Group != b.GVK.Group {
			return a.GVK.Group < b.GVK.Group
		}
		return a.GVK.Version < b.GVK.Version
	}) {
		t.Error("kinds are not sorted by package, kind, group, version")
	}
}

func TestRegistered_KnownKindsAndScopes(t *testing.T) {
	all, err := Registered()
	if err != nil {
		t.Fatal(err)
	}
	byKey := map[string]Kind{}
	for _, k := range all {
		byKey[k.Key()] = k
	}
	cases := []struct {
		key        string
		pkg        string
		namespaced bool
	}{
		{"apps/Deployment", "", true},
		{"/Namespace", "", false},
		{"gateway.networking.k8s.io/GatewayClass", "", false},
		{"cert-manager.io/ClusterIssuer", "certmanager", false},
		{"cilium.io/CiliumNetworkPolicy", "cilium", true},
		{"postgresql.cnpg.io/Cluster", "cnpg", true},
		{"barmancloud.cnpg.io/ObjectStore", "cnpg", true},
		{"external-secrets.io/ClusterSecretStore", "externalsecrets", false},
		{"kustomize.toolkit.fluxcd.io/Kustomization", "fluxcd", true},
		{"fluxcd.controlplane.io/FluxInstance", "fluxcd", true},
		{"metallb.io/IPAddressPool", "metallb", true},
		{"monitoring.coreos.com/ServiceMonitor", "prometheus", true},
		{"volsync.backube/ReplicationSource", "volsync", true},
		{"autoscaling/HorizontalPodAutoscaler", "", true},
		{"policy/PodDisruptionBudget", "", true},
	}
	for _, c := range cases {
		k, ok := byKey[c.key]
		if !ok {
			t.Errorf("%s: not registered", c.key)
			continue
		}
		if k.Package != c.pkg || k.Namespaced != c.namespaced {
			t.Errorf("%s: package=%q namespaced=%v, want %q/%v", c.key, k.Package, k.Namespaced, c.pkg, c.namespaced)
		}
	}
}

// The built-in cluster-scoped set is a subset of the frozen fixture: it names
// kinds the API server scopes, and every one of them is cluster-scoped. An entry
// in one and not the other means the two disagree about the same kind.
func TestBuiltinClusterScopedIsInTheFrozenFixture(t *testing.T) {
	for key := range builtinClusterScoped {
		if !frozenClusterScoped[key] {
			t.Errorf("%s is in builtinClusterScoped but not in the frozen fixture", key)
		}
	}
}

// local is an object type from a package no route knows.
type local struct {
	metav1.TypeMeta
	metav1.ObjectMeta
}

func (l *local) DeepCopyObject() runtime.Object { c := *l; return &c }

func TestClassify_ErrorsOnUnroutedAndSkipsNonObjects(t *testing.T) {
	unrouted := map[schema.GroupVersionKind]reflect.Type{
		{Group: "x", Version: "v1", Kind: "Local"}: reflect.TypeOf(local{}),
	}
	if _, err := classify(unrouted); err == nil || !strings.Contains(err.Error(), "no kure package routes") {
		t.Errorf("unrouted import path: err = %v", err)
	}

	skipped := map[schema.GroupVersionKind]reflect.Type{
		{Group: "", Version: "v1", Kind: "PodList"}: reflect.TypeOf(corev1.PodList{}),
		{Group: "", Version: "v1", Kind: "Status"}:  reflect.TypeOf(metav1.Status{}),
		{Group: "", Version: "v1", Kind: "PodSpec"}: reflect.TypeOf(corev1.PodSpec{}),
		{Group: "", Version: "v1", Kind: "Pod"}:     reflect.TypeOf(corev1.Pod{}),
	}
	got, err := classify(skipped)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].GVK.Kind != "Pod" {
		t.Errorf("classify should keep only Pod: %+v", got)
	}
	// classify no longer states a scope; the resolution does. A non-zero value
	// here would mean a second, undocumented scope source had crept back in.
	if got[0].Namespaced {
		t.Error("classify must leave the scope unset for the resolution to fill in")
	}
}

func TestApplyScopes(t *testing.T) {
	all := []Kind{
		{GVK: schema.GroupVersionKind{Version: "v1", Kind: "Pod"}},
		{GVK: schema.GroupVersionKind{Version: "v1", Kind: "Namespace"}},
	}
	resolved := []DerivedScope{
		{Key: "/Pod", Scope: markers.ScopeNamespaced},
		{Key: "/Namespace", Scope: markers.ScopeCluster},
	}
	if err := applyScopes(all, resolved); err != nil {
		t.Fatal(err)
	}
	if !all[0].Namespaced {
		t.Error("Pod must come out namespaced")
	}
	if all[1].Namespaced {
		t.Error("Namespace must come out cluster-scoped")
	}

	// A kind the resolution says nothing about must be an error, not the zero
	// value: Namespaced is the right answer for most kinds, so a silent gap
	// here would read as a correct table.
	err := applyScopes(all, resolved[:1])
	if err == nil {
		t.Fatal("a kind with no resolved scope must be an error")
	}
	if !strings.Contains(err.Error(), "/Namespace") {
		t.Errorf("the error must name the kind: %v", err)
	}
}

func TestRoutePackage_Unknown(t *testing.T) {
	if _, ok := routePackage("example.com/unknown/api/v1"); ok {
		t.Error("unknown import path must not route")
	}
}
