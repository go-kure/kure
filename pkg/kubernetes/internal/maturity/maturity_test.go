package maturity

import (
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/go-kure/kure/pkg/kubernetes/internal/kinds"
	"github.com/go-kure/kure/pkg/kubernetes/internal/upstream"
)

func TestStabilityOf(t *testing.T) {
	for _, tc := range []struct {
		name string
		doc  string
		want Stability
	}{
		{"nothing", "Replicas is the number of replicas.", StabilityStable},
		{"alpha", "This is an alpha field and requires enabling the gate.", StabilityAlpha},
		{"beta", "This is a beta field.", StabilityBeta},
		{"alpha wins over beta", "Was beta, now an alpha field again.", StabilityAlpha},
		{"deprecated prefix", "Deprecated: use spec.other instead.", StabilityDeprecated},
		{"deprecated on a later line", "Name is the name.\nDeprecated: use other.", StabilityDeprecated},
		{"deprecated beats alpha", "This alpha field is going away.\nDeprecated: gone.", StabilityDeprecated},
		{"alphabetical is not alpha", "Names sorted alphabetically.", StabilityStable},
		{"betamax is not beta", "The betamax format.", StabilityStable},
		{"deprecated mid-sentence is not the convention", "This is deprecated in practice.", StabilityStable},
		{"comment slashes", "// This is an alpha field.", StabilityAlpha},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := stabilityOf(tc.doc); got != tc.want {
				t.Errorf("stabilityOf(%q) = %q, want %q", tc.doc, got, tc.want)
			}
		})
	}
}

func TestContainsWord(t *testing.T) {
	for _, tc := range []struct {
		doc, word string
		want      bool
	}{
		{"an alpha field", "alpha", true},
		{"ALPHA", "alpha", true},
		{"alphabetical", "alpha", false},
		{"prealpha", "alpha", false},
		{"alpha.", "alpha", true},
		{"(alpha)", "alpha", true},
		{"alpha_field", "alpha", false},
		{"", "alpha", false},
		{"alpha", "alpha", true},
	} {
		if got := containsWord(tc.doc, tc.word); got != tc.want {
			t.Errorf("containsWord(%q, %q) = %v, want %v", tc.doc, tc.word, got, tc.want)
		}
	}
}

func TestBaseType(t *testing.T) {
	for _, tc := range []struct{ expr, want string }{
		{"Container", "Container"},
		{"*Container", "Container"},
		{"[]Container", "Container"},
		{"[]*Container", "Container"},
		{"map[string]Container", "Container"},
		{"map[string][]*Container", "Container"},
		{"*[]map[string]*Container", "Container"},
		{"map[string", ""},
		{"string", "string"},
	} {
		if got := baseType(tc.expr); got != tc.want {
			t.Errorf("baseType(%q) = %q, want %q", tc.expr, got, tc.want)
		}
	}
}

func TestIsStatusType(t *testing.T) {
	for _, tc := range []struct {
		key  string
		want bool
	}{
		{"k8s.io/api/core/v1.PodStatus", true},
		{"k8s.io/api/core/v1.PodSpec", false},
		{"PodStatus", true},
		{"k8s.io/api/core/v1.StatusCause", false},
	} {
		if got := isStatusType(tc.key); got != tc.want {
			t.Errorf("isStatusType(%q) = %v, want %v", tc.key, got, tc.want)
		}
	}
}

// A synthetic package graph, so resolution and the status skip are proven
// without depending on what upstream happens to ship today.
func syntheticTypes() map[string]upstream.Type {
	const api = "example.com/api/v1"
	const other = "example.com/other/v1"
	return map[string]upstream.Type{
		upstream.Key(api, "Thing"): {
			ImportPath: api, Name: "Thing", Module: "example.com", Version: "v1",
			Imports: map[string]string{"other": other},
			Fields: []upstream.Field{
				{Name: "Spec", JSONName: "spec", TypeExpr: "ThingSpec"},
				{Name: "Status", JSONName: "status", TypeExpr: "ThingStatus"},
			},
		},
		upstream.Key(api, "ThingSpec"): {
			ImportPath: api, Name: "ThingSpec", Module: "example.com", Version: "v1",
			Imports: map[string]string{"other": other},
			Fields: []upstream.Field{
				{Name: "Gated", JSONName: "gated", TypeExpr: "string", Doc: "// +featureGate=ExampleGate"},
				{Name: "Alpha", JSONName: "alpha", TypeExpr: "string", Doc: "This is an alpha field."},
				{Name: "Plain", JSONName: "plain", TypeExpr: "string", Doc: "Nothing special."},
				{Name: "Nested", JSONName: "nested", TypeExpr: "[]*Nested"},
				{Name: "Far", JSONName: "far", TypeExpr: "other.Far"},
				{Name: "Loop", JSONName: "loop", TypeExpr: "*ThingSpec"},
				// The same status type reached a second time, from a second
				// place. SkippedStatusTypes must still report it once: a
				// per-reference count measures how densely the API
				// cross-references its status types, not how many the walk
				// refused to enter.
				{Name: "LastStatus", JSONName: "lastStatus", TypeExpr: "*ThingStatus"},
				{Name: "Unloaded", JSONName: "unloaded", TypeExpr: "missing.Type"},
			},
		},
		upstream.Key(api, "ThingStatus"): {
			ImportPath: api, Name: "ThingStatus", Module: "example.com", Version: "v1",
			Fields: []upstream.Field{
				{Name: "GatedStatus", JSONName: "gatedStatus", TypeExpr: "string", Doc: "// +featureGate=StatusGate"},
			},
		},
		upstream.Key(api, "Nested"): {
			ImportPath: api, Name: "Nested", Module: "example.com", Version: "v1",
			Fields: []upstream.Field{
				{Name: "Deep", JSONName: "deep", TypeExpr: "string", Doc: "// +featureGate=DeepGate"},
			},
		},
		upstream.Key(other, "Far"): {
			ImportPath: other, Name: "Far", Module: "example.com", Version: "v1",
			Fields: []upstream.Field{
				{Name: "Across", JSONName: "across", TypeExpr: "string", Doc: "// +featureGate=AcrossGate"},
			},
		},
	}
}

func TestWalkFindsGatesAcrossPackagesAndSkipsStatus(t *testing.T) {
	entries, err := Walk([]Root{{ImportPath: "example.com/api/v1", TypeName: "Thing"}}, syntheticTypes())
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]Entry{}
	for _, e := range entries {
		got[e.TypeName+"."+e.Field] = e
	}
	for _, want := range []string{"ThingSpec.gated", "ThingSpec.alpha", "Nested.deep", "Far.across"} {
		if _, ok := got[want]; !ok {
			t.Errorf("missing entry %s; got %v", want, sortedKeys(got))
		}
	}
	if _, ok := got["ThingStatus.gatedStatus"]; ok {
		t.Error("a status field was reported; status types must be skipped")
	}
	if _, ok := got["ThingSpec.plain"]; ok {
		t.Error("a field with no maturity signal was reported")
	}
	if e := got["ThingSpec.gated"]; len(e.Gates) != 1 || e.Gates[0] != "ExampleGate" {
		t.Errorf("gates = %v, want [ExampleGate]", e.Gates)
	}
	if e := got["ThingSpec.alpha"]; e.Stability != StabilityAlpha {
		t.Errorf("stability = %q, want %q", e.Stability, StabilityAlpha)
	}
}

// A self-referential field must not hang the walk. JSONSchemaProps in
// apiextensions names itself, so this is the real shape, not a hypothetical.
//
// The deadline is the whole point: waiting on the channel with no timeout is
// the same as calling Walk here directly, and a walk that stopped making
// progress would hang the test binary until the package-wide timeout killed it
// — a failure attributed to every test in this package at once, minutes later,
// rather than to this one. The goroutine reports over a buffered channel and
// never touches t, so it cannot log after this test has returned.
//
// It catches a stalled walk, not every way the cycle cut can regress: [visit]
// recurses, so simply losing the visited check overflows the stack in a couple
// of seconds, and a fatal runtime error takes the process down before any
// deadline here can fire. Both mutations were tried; this one reports the
// failure by name, the other cannot be caught at test level by any means.
func TestWalkTerminatesOnSelfReference(t *testing.T) {
	const deadline = 30 * time.Second // ~5 orders of magnitude over the real walk

	done := make(chan error, 1)
	go func() {
		_, err := Walk([]Root{{ImportPath: "example.com/api/v1", TypeName: "Thing"}}, syntheticTypes())
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(deadline):
		t.Fatalf("Walk did not return within %s on a self-referential type: the visited set is not cutting the cycle", deadline)
	}
}

// Marker lines are not prose. k8s.io/api v0.37.0 spells its declarative
// validation markers "+k8s:alpha(since: "1.37")=+k8s:required" — a statement
// about the required rule, on a field that has been GA since 1.9. Reading them
// as prose reported 45 long-stable built-in fields as alpha or beta.
func TestStabilityIgnoresMarkerLines(t *testing.T) {
	for _, tc := range []struct {
		name string
		doc  string
		want Stability
	}{
		{
			"validation rule alpha, field GA",
			"selector is a label query over pods.\n+required\n+k8s:alpha(since: \"1.37\")=+k8s:required\n",
			StabilityStable,
		},
		{
			"validation rule beta, field GA",
			"rules holds all the PolicyRules for this ClusterRole\n+optional\n+k8s:beta(since: \"1.37\")=+k8s:optional\n",
			StabilityStable,
		},
		{
			"the field's own prose still counts",
			"resources is the total amount of CPU and Memory.\nThis is an alpha field.\n+featureGate=PodLevelResources\n",
			StabilityAlpha,
		},
		{
			"a gate alone is no stability claim",
			"hostnameOverride specifies an explicit hostname.\n+featureGate=HostnameOverride\n",
			StabilityStable,
		},
		{
			"deprecation is prose too",
			"Deprecated: use serviceAccountName instead.\n+k8s:alpha(since: \"1.37\")=+k8s:optional\n",
			StabilityDeprecated,
		},
		{
			"comment-prefixed lines are handled the same",
			"// selector is a label query.\n// +k8s:alpha(since: \"1.37\")=+k8s:immutable\n",
			StabilityStable,
		},
		{
			"a gate's maturity is not the field's",
			"minReplicas is the lower limit for the number of replicas.\nminReplicas is allowed to be 0 if the alpha feature gate\nHPAScaleToZero is enabled.\n+optional\n",
			StabilityStable,
		},
		{
			"a field that is itself alpha and names an alpha gate",
			"This is an alpha field.\nIt requires the alpha feature gate InPlacePodVerticalScaling.\n",
			StabilityAlpha,
		},
		{
			"the marker's own spelling varies",
			"roleRef can reference a Role in the current namespace.\nThis field is immutable.\n+required\n+k8s:alpha(since:\"1.37\")=+k8s:immutable\n",
			StabilityStable,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := stabilityOf(tc.doc); got != tc.want {
				t.Errorf("stabilityOf = %q, want %q", got, tc.want)
			}
		})
	}
}

// frozenGAFields have been GA for many releases, and every one of them was
// reported alpha or beta before the prose scan was confined to the words that
// describe the field itself — sixteen of them by a validation marker's own
// subject, minReplicas by the gate its prose names.
// They are asserted against the loaded types rather than against the walk's
// output: a stable ungated field is not recorded at all, so scanning the entries
// for them would pass no matter what the derivation said — the same vacuous
// shape this table exists to avoid.
var frozenGAFields = []struct{ importPath, typeName, field string }{
	{"k8s.io/api/apps/v1", "StatefulSetSpec", "Selector"},
	{"k8s.io/api/apps/v1", "StatefulSetSpec", "ServiceName"},
	{"k8s.io/api/apps/v1", "StatefulSetSpec", "PodManagementPolicy"},
	{"k8s.io/api/apps/v1", "StatefulSetSpec", "VolumeClaimTemplates"},
	{"k8s.io/api/core/v1", "Secret", "Type"},
	{"k8s.io/api/core/v1", "PodSpec", "Tolerations"},
	{"k8s.io/api/core/v1", "Toleration", "Key"},
	{"k8s.io/api/core/v1", "NodeSpec", "ProviderID"},
	{"k8s.io/api/rbac/v1", "ClusterRole", "Rules"},
	{"k8s.io/api/rbac/v1", "RoleBinding", "RoleRef"},
	{"k8s.io/api/rbac/v1", "ClusterRoleBinding", "RoleRef"},
	{"k8s.io/api/rbac/v1", "PolicyRule", "Verbs"},
	{"k8s.io/api/storage/v1", "StorageClass", "Provisioner"},
	{"k8s.io/api/storage/v1", "StorageClass", "ReclaimPolicy"},
	{"k8s.io/api/batch/v1", "CronJobSpec", "Schedule"},
	{"k8s.io/api/autoscaling/v2", "HorizontalPodAutoscalerSpec", "MaxReplicas"},
	{"k8s.io/api/autoscaling/v2", "HorizontalPodAutoscalerSpec", "MinReplicas"},
}

// assertFrozenGAFieldsAreStable checks each frozen field twice over: that the
// pinned source still declares it, so a rename cannot turn this check into a
// no-op, and that its documentation makes no stability claim.
func assertFrozenGAFieldsAreStable(t *testing.T, types map[string]upstream.Type) {
	t.Helper()
	for _, want := range frozenGAFields {
		tp, ok := types[upstream.Key(want.importPath, want.typeName)]
		if !ok {
			t.Errorf("%s.%s is not among the loaded types; the frozen list names a type the pins no longer declare", want.importPath, want.typeName)
			continue
		}
		found := false
		for _, f := range tp.Fields {
			if f.Name != want.field {
				continue
			}
			found = true
			if got := stabilityOf(f.Doc); got != StabilityStable {
				t.Errorf("%s.%s.%s is derived %q; it has been GA for years, so words about something else — a validation marker's own subject, or a feature gate the prose names — are being read as the field's claim",
					want.importPath, want.typeName, want.field, got)
			}
		}
		if !found {
			t.Errorf("%s.%s has no field %s; the frozen list is out of date with the pins", want.importPath, want.typeName, want.field)
		}
	}
}

func TestWalkRejectsBadInput(t *testing.T) {
	if _, err := Walk(nil, syntheticTypes()); err == nil {
		t.Error("Walk accepted no roots")
	}
	if _, err := Walk([]Root{{ImportPath: "nope", TypeName: "Nope"}}, syntheticTypes()); err == nil {
		t.Error("Walk accepted a root that is not loaded")
	}
}

// The synthetic graph names ThingStatus from two fields, so this also pins the
// deduplication: one entry, not one per reference.
func TestSkippedStatusTypesReportsTheSkip(t *testing.T) {
	skipped := SkippedStatusTypes([]Root{{ImportPath: "example.com/api/v1", TypeName: "Thing"}}, syntheticTypes())
	if len(skipped) != 1 || !strings.HasSuffix(skipped[0], "ThingStatus") {
		t.Errorf("skipped = %v, want exactly one ThingStatus entry", skipped)
	}
	// A root that is not loaded is simply not walked.
	if got := SkippedStatusTypes([]Root{{ImportPath: "nope", TypeName: "Nope"}}, syntheticTypes()); len(got) != 0 {
		t.Errorf("skipped = %v, want none", got)
	}
}

func TestEntriesAreSortedDeterministically(t *testing.T) {
	types := syntheticTypes()
	first, err := Walk([]Root{{ImportPath: "example.com/api/v1", TypeName: "Thing"}}, types)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		again, err := Walk([]Root{{ImportPath: "example.com/api/v1", TypeName: "Thing"}}, types)
		if err != nil {
			t.Fatal(err)
		}
		if len(again) != len(first) {
			t.Fatalf("run %d produced %d entries, first produced %d", i, len(again), len(first))
		}
		for j := range first {
			if !sameEntry(again[j], first[j]) {
				t.Fatalf("run %d differs at %d: %+v vs %+v", i, j, again[j], first[j])
			}
		}
	}
}

// The real walk over the registered kinds. This is the number the work item's
// own figure has to be recomputed against, and it also proves every reported
// field exists in the pinned type.
func TestWalkOverRegisteredKinds(t *testing.T) {
	all, err := kinds.Registered()
	if err != nil {
		t.Fatal(err)
	}
	paths := map[string]bool{}
	roots := make([]Root, 0, len(all))
	for _, k := range all {
		paths[k.ImportPath] = true
		roots = append(roots, Root{ImportPath: k.ImportPath, TypeName: k.TypeName})
	}
	types, err := upstream.Load(sortedBoolKeys(paths))
	if err != nil {
		t.Fatal(err)
	}
	entries, err := Walk(roots, types)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("no maturity entries found; the walk is not reaching the API types")
	}

	// Every entry must name a field that really exists in the parsed type.
	for _, e := range entries {
		tp, ok := types[upstream.Key(e.ImportPath, e.TypeName)]
		if !ok {
			t.Errorf("%s.%s: declaring type not loaded", e.TypeName, e.Field)
			continue
		}
		found := false
		for _, f := range tp.Fields {
			if f.Name == e.GoField {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s.%s: no such field in %s", e.TypeName, e.Field, e.ImportPath)
		}
	}

	assertFrozenGAFieldsAreStable(t, types)

	gated, alpha, beta, deprecated := 0, 0, 0, 0
	byModule := map[string]int{}
	for _, e := range entries {
		if len(e.Gates) > 0 {
			gated++
			byModule[e.Module]++
		}
		switch e.Stability {
		case StabilityAlpha:
			alpha++
		case StabilityBeta:
			beta++
		case StabilityDeprecated:
			deprecated++
		case StabilityStable:
		}
	}
	t.Logf("maturity entries: %d total, %d gated, %d alpha, %d beta, %d deprecated",
		len(entries), gated, alpha, beta, deprecated)
	for _, m := range sortedIntKeys(byModule) {
		t.Logf("  gated by module: %-60s %d", m, byModule[m])
	}
	t.Logf("status types skipped: %d", len(SkippedStatusTypes(roots, types)))
}

// sameEntry compares two entries field by field; Entry carries a slice and so
// is not comparable with ==.
func sameEntry(a, b Entry) bool {
	if a.ImportPath != b.ImportPath || a.TypeName != b.TypeName ||
		a.Field != b.Field || a.GoField != b.GoField ||
		a.Stability != b.Stability || a.Module != b.Module || a.Version != b.Version {
		return false
	}
	if len(a.Gates) != len(b.Gates) {
		return false
	}
	for i := range a.Gates {
		if a.Gates[i] != b.Gates[i] {
			return false
		}
	}
	return true
}

func sortedKeys(m map[string]Entry) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedBoolKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedIntKeys(m map[string]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
