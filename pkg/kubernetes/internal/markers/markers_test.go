package markers

import (
	"strings"
	"testing"
)

// The two scope-marker spellings below are copied verbatim from the pinned
// module sources, because the difference between them is the whole reason this
// package exists: a parser written against only the first silently mis-scopes
// every kind that uses the second.
const (
	// cloudnative-pg@v1.30.0/api/v1/clusterimagecatalog_types.go
	cnpgClusterMarker = "// +kubebuilder:resource:scope=Cluster"
	// cilium@v1.19.5/pkg/k8s/apis/cilium.io/v2/cidrgroups_types.go
	ciliumClusterMarker = `// +kubebuilder:resource:categories={cilium},singular="ciliumcidrgroup",path="ciliumcidrgroups",scope="Cluster",shortName={ccg}`
	// cilium@v1.19.5/pkg/k8s/apis/cilium.io/v2/cnp_types.go
	ciliumNamespacedMarker = `// +kubebuilder:resource:categories={cilium,ciliumpolicy},singular="ciliumnetworkpolicy",path="ciliumnetworkpolicies",scope="Namespaced",shortName={cnp,ciliumnp}`
	// cilium@v1.19.5 — a resource marker that declares no scope at all
	ciliumNoScopeMarker = "// +kubebuilder:resource:categories={cilium}"
)

func TestResourceScopeAcceptsBothObservedSpellings(t *testing.T) {
	for _, tc := range []struct {
		name string
		doc  string
		want Scope
	}{
		{"bare unquoted, as cnpg writes it", cnpgClusterMarker, ScopeCluster},
		{"quoted inside a braced value list, as cilium writes it", ciliumClusterMarker, ScopeCluster},
		{"quoted Namespaced", ciliumNamespacedMarker, ScopeNamespaced},
		{"resource marker declaring no scope", ciliumNoScopeMarker, ScopeNamespaced},
		{"no marker at all", "// Cluster is a PostgreSQL cluster.", ScopeNamespaced},
		{"empty doc", "", ScopeNamespaced},
		{"bare marker with no value list", "// +kubebuilder:resource", ScopeNamespaced},
		{"unquoted Namespaced", "// +kubebuilder:resource:scope=Namespaced", ScopeNamespaced},
		{"scope not first in the list", `// +kubebuilder:resource:path="x",scope=Cluster`, ScopeCluster},
		{"other markers present", "// +genclient\n// +kubebuilder:object:root=true\n// +kubebuilder:resource:scope=Cluster\n// +kubebuilder:subresource:status", ScopeCluster},
		{"marker without the comment slashes", "+kubebuilder:resource:scope=Cluster", ScopeCluster},
		{"prose mentioning scope=Cluster is not a marker", "// The scope=Cluster form is used upstream.", ScopeNamespaced},
		{"scope declared twice, consistently", "// +kubebuilder:resource:scope=Cluster\n// +kubebuilder:resource:scope=Cluster", ScopeCluster},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResourceScope(tc.doc)
			if err != nil {
				t.Fatalf("ResourceScope: unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("scope = %s, want %s", got, tc.want)
			}
		})
	}
}

// A marker this parser cannot read must be an error, not the default. Absent
// marker legitimately means Namespaced, so a failed match that fell through to
// the default would be indistinguishable from upstream declaring nothing.
func TestResourceScopeRejectsWhatItCannotRead(t *testing.T) {
	for _, tc := range []struct {
		name string
		doc  string
		want string
	}{
		{"unrecognised value", "// +kubebuilder:resource:scope=Global", "unrecognised scope"},
		{"lowercase value", "// +kubebuilder:resource:scope=cluster", "unrecognised scope"},
		{"empty value", "// +kubebuilder:resource:scope=", "unrecognised scope"},
		{"unbalanced open brace", "// +kubebuilder:resource:categories={a,scope=Cluster", "unbalanced"},
		{"unbalanced close brace", "// +kubebuilder:resource:categories=a},scope=Cluster", "unbalanced"},
		{"unterminated quote", `// +kubebuilder:resource:path="x,scope=Cluster`, "unterminated quote"},
		{"scope declared twice, inconsistently", "// +kubebuilder:resource:scope=Cluster\n// +kubebuilder:resource:scope=Namespaced", "declared twice"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResourceScope(tc.doc)
			if err == nil {
				t.Fatalf("ResourceScope(%q) = %s, want an error", tc.doc, got)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want it to mention %q", err, tc.want)
			}
			if got != ScopeNamespaced {
				t.Errorf("scope on error = %s, want the zero value", got)
			}
		})
	}
}

// An unbalanced brace hides a scope entry from the split rather than merely
// looking untidy: this proves the malformed list would have been misread, which
// is why splitValues errors instead of doing its best.
func TestUnbalancedBraceWouldHaveHiddenTheScope(t *testing.T) {
	const malformed = `categories={a,scope=Cluster`
	if _, err := splitValues(malformed); err == nil {
		t.Fatal("splitValues accepted an unbalanced brace")
	}
	// With the brace balanced, the same text splits into two values and the
	// scope becomes visible — so the error above is not pedantry.
	values, err := splitValues(`categories={a},scope=Cluster`)
	if err != nil {
		t.Fatalf("splitValues: %v", err)
	}
	if len(values) != 2 {
		t.Fatalf("values = %q, want 2 entries", values)
	}
}

func TestSplitValuesKeepsBracedAndQuotedCommas(t *testing.T) {
	values, err := splitValues(`categories={cilium,ciliumbgp},singular="a,b",scope="Cluster",shortName={c,d}`)
	if err != nil {
		t.Fatalf("splitValues: %v", err)
	}
	want := []string{`categories={cilium,ciliumbgp}`, `singular="a,b"`, `scope="Cluster"`, `shortName={c,d}`}
	if len(values) != len(want) {
		t.Fatalf("values = %q, want %q", values, want)
	}
	for i := range want {
		if values[i] != want[i] {
			t.Errorf("values[%d] = %q, want %q", i, values[i], want[i])
		}
	}
}

func TestFeatureGates(t *testing.T) {
	for _, tc := range []struct {
		name string
		doc  string
		want []string
	}{
		{"none", "// Resources is the compute resources.", nil},
		{"one", "// +featureGate=InPlacePodVerticalScaling", []string{"InPlacePodVerticalScaling"}},
		{
			"two on one field, in order",
			"// +featureGate=DRAAdminAccess\n// +featureGate=DynamicResourceAllocation",
			[]string{"DRAAdminAccess", "DynamicResourceAllocation"},
		},
		{
			"mixed with other markers and prose",
			"// Resources is alpha.\n// +optional\n// +featureGate=PodLevelResources\n// +listType=atomic",
			[]string{"PodLevelResources"},
		},
		{"empty gate name is not a gate", "// +featureGate=", nil},
		{"prose mentioning a gate is not a marker", "// Gated behind +featureGate=Foo upstream.", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := FeatureGates(tc.doc)
			if len(got) != len(tc.want) {
				t.Fatalf("gates = %q, want %q", got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Errorf("gates[%d] = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// HasResource exists to separate the two cases ResourceScope collapses into
// ScopeNamespaced: a type declaring the default, and a type declaring nothing.
func TestHasResource(t *testing.T) {
	for _, tc := range []struct {
		name, doc string
		want      bool
	}{
		{"no markers", "// Thing is a thing.\n", false},
		{"other markers only", "// +genclient\n// +kubebuilder:object:root=true\n", false},
		{"bare resource marker", "// +kubebuilder:resource\n", true},
		{"scope cluster", "// +kubebuilder:resource:scope=Cluster\n", true},
		{"scope namespaced", "// +kubebuilder:resource:scope=Namespaced\n", true},
		{"quoted list", `// +kubebuilder:resource:path="things",scope="Cluster"`, true},
		{"not a marker line", "// see +kubebuilder:resource:scope=Cluster for details\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := HasResource(tc.doc); got != tc.want {
				t.Errorf("HasResource(%q) = %v, want %v", tc.doc, got, tc.want)
			}
		})
	}
}

func TestScopeString(t *testing.T) {
	if got := ScopeCluster.String(); got != "Cluster" {
		t.Errorf("ScopeCluster = %q, want %q", got, "Cluster")
	}
	if got := ScopeNamespaced.String(); got != "Namespaced" {
		t.Errorf("ScopeNamespaced = %q, want %q", got, "Namespaced")
	}
	// An out-of-range value reads as the default rather than a Go-ish
	// "Scope(7)", because the string lands in a generated table.
	if got := Scope(7).String(); got != "Namespaced" {
		t.Errorf("Scope(7) = %q, want %q", got, "Namespaced")
	}
}
