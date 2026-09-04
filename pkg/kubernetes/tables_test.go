package kubernetes

import (
	"slices"
	"sort"
	"strings"
	"testing"
)

// KindFor matches the version, not only the group and kind: everything else a
// KindInfo carries — GoType, ImportPath, ModuleVersion — describes one
// version's Go type, so answering about a different version would name a type
// that does not implement the version asked about.
func TestKindForMatchesTheVersion(t *testing.T) {
	got, ok := KindFor("autoscaling/v2", "HorizontalPodAutoscaler")
	if !ok {
		t.Fatal("autoscaling/v2 HorizontalPodAutoscaler is registered")
	}
	if got.ImportPath != "k8s.io/api/autoscaling/v2" || got.GoType != "HorizontalPodAutoscaler" {
		t.Errorf("KindFor returned %s.%s", got.ImportPath, got.GoType)
	}

	// The same group/kind at a version the scheme does not register. The row
	// above is not an answer to this question, and returning it would hand the
	// caller the v2 Go type for a v1 object.
	if got, ok := KindFor("autoscaling/v1", "HorizontalPodAutoscaler"); ok {
		t.Errorf("autoscaling/v1 HorizontalPodAutoscaler is not registered, got %s.%s", got.ImportPath, got.GoType)
	}

	// The core group is written without one, so the whole apiVersion is the
	// version and the group is empty.
	pod, ok := KindFor("v1", "Pod")
	if !ok || pod.Group != "" || pod.Version != "v1" {
		t.Errorf("KindFor(\"v1\", \"Pod\") = %+v, %v", pod, ok)
	}

	if _, ok := KindFor("example.com/v1", "Widget"); ok {
		t.Error("an unregistered kind must not resolve")
	}
}

// IsNamespaced ignores the version on purpose: scope is a property of the
// resource, the same across the versions of one group/kind, so a manifest
// written against a version kure does not register must still be answered.
func TestIsNamespacedIgnoresTheVersion(t *testing.T) {
	cases := []struct {
		apiVersion, kind       string
		wantNamespaced, wantOK bool
	}{
		{"autoscaling/v2", "HorizontalPodAutoscaler", true, true},
		{"autoscaling/v1", "HorizontalPodAutoscaler", true, true}, // not registered at v1
		{"v1", "Pod", true, true},
		{"v1", "Namespace", false, true},
		{"rbac.authorization.k8s.io/v1", "ClusterRole", false, true},
		{"example.com/v1", "Widget", false, false},
	}
	for _, c := range cases {
		namespaced, ok := IsNamespaced(c.apiVersion, c.kind)
		if namespaced != c.wantNamespaced || ok != c.wantOK {
			t.Errorf("IsNamespaced(%q, %q) = %v, %v; want %v, %v",
				c.apiVersion, c.kind, namespaced, ok, c.wantNamespaced, c.wantOK)
		}
	}
}

func TestKindByGroupKindAndKeys(t *testing.T) {
	dep, ok := KindByGroupKind("apps/Deployment")
	if !ok {
		t.Fatal("apps/Deployment is registered")
	}
	if dep.GroupKind() != "apps/Deployment" || dep.APIVersion() != "apps/v1" {
		t.Errorf("Deployment keys = %q / %q", dep.GroupKind(), dep.APIVersion())
	}
	pod, ok := KindByGroupKind("/Pod")
	if !ok {
		t.Fatal("/Pod is registered")
	}
	if pod.APIVersion() != "v1" {
		t.Errorf("core APIVersion = %q, want %q", pod.APIVersion(), "v1")
	}
	if _, ok := KindByGroupKind("example.com/Widget"); ok {
		t.Error("an unregistered key must not resolve")
	}
}

// The committed maturity table is what the generator's drift check compares, so
// its row order must be defined and total: sorted by import path, type and
// field, with no two rows sharing all three. An undefined order would show up
// as a spurious -check failure in CI rather than as a wrong answer here.
func TestFieldMaturitiesAreSortedAndDistinct(t *testing.T) {
	all := FieldMaturities()
	if len(all) == 0 {
		t.Fatal("the maturity table is empty")
	}
	key := func(m FieldMaturity) [3]string { return [3]string{m.ImportPath, m.TypeName, m.Field} }
	if !sort.SliceIsSorted(all, func(i, j int) bool {
		return less(key(all[i]), key(all[j]))
	}) {
		t.Error("FieldMaturities is not sorted by import path, type, field")
	}
	seen := map[[3]string]bool{}
	for _, m := range all {
		if seen[key(m)] {
			t.Errorf("%s.%s.%s appears twice, so the sort above cannot be total", m.ImportPath, m.TypeName, m.Field)
		}
		seen[key(m)] = true
	}
}

func less(a, b [3]string) bool {
	for i := range a {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return false
}

func TestMaturityForType(t *testing.T) {
	got := MaturityForType("k8s.io/api/autoscaling/v2", "HPAScalingRules")
	if len(got) == 0 {
		t.Fatal("HPAScalingRules declares at least one gated field")
	}
	for _, m := range got {
		if m.ImportPath != "k8s.io/api/autoscaling/v2" || m.TypeName != "HPAScalingRules" {
			t.Errorf("MaturityForType returned an entry for %s.%s", m.ImportPath, m.TypeName)
		}
	}
	if got := MaturityForType("example.com/api/v1", "Widget"); got != nil {
		t.Errorf("MaturityForType for an unknown type = %v, want nil", got)
	}
}

func TestGatedFields(t *testing.T) {
	got := GatedFields()
	if len(got) == 0 {
		t.Fatal("the pinned k8s.io/api carries feature-gated construction-side fields")
	}
	if len(got) == len(FieldMaturities()) {
		t.Error("GatedFields returned every row; the gate filter is not filtering")
	}
	for _, m := range got {
		if len(m.Gates) == 0 {
			t.Errorf("%s.%s.%s has no gates", m.ImportPath, m.TypeName, m.Field)
		}
	}
}

// The tables are process-global and every lookup reads them, so what a caller
// gets back must not be the storage itself. A consumer relabelling a row for its
// own output would otherwise change the scope every later caller is told —
// including the one inside pkg/manifest — with nothing to trace it to.
func TestTheTablesCannotBeMutatedThroughWhatIsReturned(t *testing.T) {
	const (
		apiVersion = "apps/v1"
		kind       = "Deployment"
	)

	before, ok := KindFor(apiVersion, kind)
	if !ok {
		t.Fatalf("%s %s is registered", apiVersion, kind)
	}

	rows := Kinds()
	for i := range rows {
		rows[i].Namespaced = !rows[i].Namespaced
		rows[i].Kind = "Mutated"
	}
	after, ok := KindFor(apiVersion, kind)
	if !ok {
		t.Fatal("the kind table lost a row to a caller's edit")
	}
	if after != before {
		t.Errorf("KindFor now answers %+v, was %+v: Kinds returned the table itself", after, before)
	}

	// Gate names, not gate counts. Overwriting the strings in a Gates slice
	// changes neither the row count nor any len(Gates), so a count comparison
	// here could not fail whatever the copying did — and the sharing this is
	// looking for would corrupt a snapshot of the rows themselves just as it
	// corrupts the table. The snapshot is therefore of string values, which are
	// immutable: writing into a []string slot replaces the slot, and cannot
	// reach a string already copied out of it.
	want := gateNames(GatedFields())
	if len(want) == 0 {
		t.Fatal("the pinned sources carry gated construction-side fields")
	}

	for _, rows := range [][]FieldMaturity{
		FieldMaturities(),
		MaturityForType("k8s.io/api/autoscaling/v2", "HPAScalingRules"),
		GatedFields(),
	} {
		for i := range rows {
			for j := range rows[i].Gates {
				rows[i].Gates[j] = ""
			}
		}
	}

	got := gateNames(GatedFields())
	if !slices.Equal(got, want) {
		for i := range got {
			if i < len(want) && got[i] != want[i] {
				t.Fatalf("after a caller overwrote the gates it was handed, GatedFields reports %q where it reported %q: the returned rows share their Gates slice with the table",
					got[i], want[i])
			}
		}
		t.Fatalf("GatedFields now reports %d gated rows, was %d", len(got), len(want))
	}

	// The copy has to hold for one row read twice, not only in aggregate: take
	// a row, overwrite its gates, and read that same row again.
	first := GatedFields()[0]
	for i := range first.Gates {
		first.Gates[i] = "overwritten"
	}
	again := GatedFields()[0]
	if slices.Contains(again.Gates, "overwritten") {
		t.Errorf("%s.%s.%s reads back with a caller's edit: %v",
			again.ImportPath, again.TypeName, again.Field, again.Gates)
	}
}

// gateNames renders the gate lists as one string per row, so a comparison is
// over immutable values that no later write into the tables can reach.
func gateNames(rows []FieldMaturity) []string {
	out := make([]string, 0, len(rows))
	for _, m := range rows {
		out = append(out, m.ImportPath+"."+m.TypeName+"."+m.Field+"="+strings.Join(m.Gates, ","))
	}
	return out
}
