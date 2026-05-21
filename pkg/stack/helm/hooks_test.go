package helm_test

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack/helm"
)

func hookObj(name, hookAnno, weightAnno string) client.Object {
	u := &unstructured.Unstructured{}
	u.SetName(name)
	ann := map[string]string{}
	if hookAnno != "" {
		ann["helm.sh/hook"] = hookAnno
	}
	if weightAnno != "" {
		ann["helm.sh/hook-weight"] = weightAnno
	}
	u.SetAnnotations(ann)
	return u
}

func TestSplitByHookWeight_NoHooks(t *testing.T) {
	groups := helm.SplitByHookWeight([]client.Object{hookObj("a", "", ""), hookObj("b", "", "")})
	if len(groups) != 1 {
		t.Fatalf("want 1, got %d", len(groups))
	}
	if groups[0].Phase != "" {
		t.Errorf("want main phase, got %q", groups[0].Phase)
	}
	if len(groups[0].Resources) != 2 {
		t.Errorf("want 2 resources, got %d", len(groups[0].Resources))
	}
}

func TestSplitByHookWeight_PreMainPost(t *testing.T) {
	objs := []client.Object{
		hookObj("pre", "pre-install", "0"),
		hookObj("main", "", ""),
		hookObj("post", "post-install", "0"),
	}
	groups := helm.SplitByHookWeight(objs)
	if len(groups) != 3 {
		t.Fatalf("want 3, got %d", len(groups))
	}
	if groups[0].Phase != "pre-install" {
		t.Errorf("[0]: %q", groups[0].Phase)
	}
	if groups[1].Phase != "" {
		t.Errorf("[1]: %q", groups[1].Phase)
	}
	if groups[2].Phase != "post-install" {
		t.Errorf("[2]: %q", groups[2].Phase)
	}
}

func TestSplitByHookWeight_UpgradePhases_OrderedAroundMain(t *testing.T) {
	// pre-upgrade and post-upgrade are included; FluxCD reconciliations are equivalent to upgrades.
	objs := []client.Object{
		hookObj("pi", "pre-install", "0"),
		hookObj("pu", "pre-upgrade", "0"),
		hookObj("main", "", ""),
		hookObj("po", "post-install", "0"),
		hookObj("pou", "post-upgrade", "0"),
	}
	groups := helm.SplitByHookWeight(objs)
	if len(groups) != 5 {
		t.Fatalf("want 5, got %d", len(groups))
	}
	want := []string{"pre-install", "pre-upgrade", "", "post-install", "post-upgrade"}
	for i, w := range want {
		if groups[i].Phase != w {
			t.Errorf("[%d]: want %q got %q", i, w, groups[i].Phase)
		}
	}
}

func TestSplitByHookWeight_MultiWeight_AscendingOrder(t *testing.T) {
	objs := []client.Object{
		hookObj("pre-w2", "pre-install", "2"),
		hookObj("pre-w1", "pre-install", "1"),
		hookObj("main", "", ""),
	}
	groups := helm.SplitByHookWeight(objs)
	if len(groups) != 3 {
		t.Fatalf("want 3, got %d", len(groups))
	}
	if groups[0].Weight != 1 {
		t.Errorf("[0] weight want 1, got %d", groups[0].Weight)
	}
	if groups[1].Weight != 2 {
		t.Errorf("[1] weight want 2, got %d", groups[1].Weight)
	}
}

func TestSplitByHookWeight_ExcludedPhases_Dropped(t *testing.T) {
	// pre-delete, post-delete, pre-rollback, post-rollback, test have no FluxCD equivalent.
	objs := []client.Object{
		hookObj("main", "", ""),
		hookObj("predelete", "pre-delete", "0"),
		hookObj("postdel", "post-delete", "0"),
		hookObj("preroll", "pre-rollback", "0"),
		hookObj("postroll", "post-rollback", "0"),
		hookObj("test", "test", "0"),
	}
	groups := helm.SplitByHookWeight(objs)
	if len(groups) != 1 {
		t.Fatalf("want 1 (main only), got %d", len(groups))
	}
	if groups[0].Phase != "" {
		t.Errorf("want main phase, got %q", groups[0].Phase)
	}
}

func TestSplitByHookWeight_UnknownPhase_LastAlphabetical(t *testing.T) {
	// Unknown phases (not in the include/exclude lists) go after post-upgrade, sorted alphabetically.
	objs := []client.Object{
		hookObj("pre", "pre-install", "0"),
		hookObj("custom", "custom-hook", "0"), // unknown, not excluded
		hookObj("main", "", ""),
	}
	groups := helm.SplitByHookWeight(objs)
	if len(groups) != 3 {
		t.Fatalf("want 3, got %d", len(groups))
	}
	if groups[0].Phase != "pre-install" {
		t.Errorf("[0]: %q", groups[0].Phase)
	}
	if groups[1].Phase != "" {
		t.Errorf("[1]: %q", groups[1].Phase)
	}
	if groups[2].Phase != "custom-hook" {
		t.Errorf("[2]: %q (want unknown last)", groups[2].Phase)
	}
}

func TestSplitByHookWeight_NilInput(t *testing.T) {
	if groups := helm.SplitByHookWeight(nil); len(groups) != 0 {
		t.Fatalf("want 0, got %d", len(groups))
	}
}

func TestSplitByHookWeight_CommaAnnotation_TreatedAsUnknown(t *testing.T) {
	// Comma-separated annotations must remain opaque (no splitting), placed last as unknown.
	// This locks in the review decision to avoid later regression into duplication.
	objs := []client.Object{
		hookObj("pre", "pre-install", "0"),
		hookObj("main", "", ""),
		hookObj("multi", "pre-install,post-install", "0"),
	}
	groups := helm.SplitByHookWeight(objs)
	if len(groups) != 3 {
		t.Fatalf("want 3, got %d", len(groups))
	}
	if groups[0].Phase != "pre-install" {
		t.Errorf("[0]: %q", groups[0].Phase)
	}
	if groups[1].Phase != "" {
		t.Errorf("[1]: %q", groups[1].Phase)
	}
	if groups[2].Phase != "pre-install,post-install" {
		t.Errorf("[2]: %q (want comma annotation as unknown, last)", groups[2].Phase)
	}
}

func TestSplitByHookWeight_AllExcluded(t *testing.T) {
	objs := []client.Object{
		hookObj("a", "pre-delete", "0"),
		hookObj("b", "test", "0"),
		hookObj("c", "post-delete", "0"),
	}
	groups := helm.SplitByHookWeight(objs)
	// Non-empty input that filters to nothing should return a non-nil empty slice
	// (distinct from the nil returned for empty/nil input — see hooks.go).
	if groups == nil {
		t.Fatal("want non-nil empty slice for all-excluded input, got nil")
	}
	if len(groups) != 0 {
		t.Fatalf("want 0 groups when all objects have excluded phases, got %d", len(groups))
	}
}
