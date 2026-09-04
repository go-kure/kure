package kubernetes

import (
	"sort"
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
	if len(FieldMaturities) == 0 {
		t.Fatal("the maturity table is empty")
	}
	key := func(m FieldMaturity) [3]string { return [3]string{m.ImportPath, m.TypeName, m.Field} }
	if !sort.SliceIsSorted(FieldMaturities, func(i, j int) bool {
		return less(key(FieldMaturities[i]), key(FieldMaturities[j]))
	}) {
		t.Error("FieldMaturities is not sorted by import path, type, field")
	}
	seen := map[[3]string]bool{}
	for _, m := range FieldMaturities {
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
	if len(got) == len(FieldMaturities) {
		t.Error("GatedFields returned every row; the gate filter is not filtering")
	}
	for _, m := range got {
		if len(m.Gates) == 0 {
			t.Errorf("%s.%s.%s has no gates", m.ImportPath, m.TypeName, m.Field)
		}
	}
}
