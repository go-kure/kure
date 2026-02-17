package patch

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestDetectSMPConflicts_Conflicting(t *testing.T) {
	lookup, err := DefaultKindLookup()
	if err != nil {
		t.Fatalf("DefaultKindLookup: %v", err)
	}

	gvk := schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}

	patches := []map[string]interface{}{
		{
			"spec": map[string]interface{}{
				"replicas": int64(3),
			},
		},
		{
			"spec": map[string]interface{}{
				"replicas": int64(5),
			},
		},
	}

	report, err := DetectSMPConflicts(patches, lookup, gvk)
	if err != nil {
		t.Fatalf("DetectSMPConflicts: %v", err)
	}
	if !report.HasConflicts() {
		t.Fatal("expected conflicts to be detected")
	}
	if len(report.Conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(report.Conflicts))
	}
}

func TestDetectSMPConflicts_NonConflicting(t *testing.T) {
	lookup, err := DefaultKindLookup()
	if err != nil {
		t.Fatalf("DefaultKindLookup: %v", err)
	}

	gvk := schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}

	patches := []map[string]interface{}{
		{
			"spec": map[string]interface{}{
				"replicas": int64(3),
			},
		},
		{
			"spec": map[string]interface{}{
				"template": map[string]interface{}{
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"name":  "main",
								"image": "nginx:latest",
							},
						},
					},
				},
			},
		},
	}

	report, err := DetectSMPConflicts(patches, lookup, gvk)
	if err != nil {
		t.Fatalf("DetectSMPConflicts: %v", err)
	}
	if report.HasConflicts() {
		t.Fatal("expected no conflicts")
	}
}

func TestDetectSMPConflicts_UnknownKindFallback(t *testing.T) {
	lookup, err := DefaultKindLookup()
	if err != nil {
		t.Fatalf("DefaultKindLookup: %v", err)
	}

	gvk := schema.GroupVersionKind{Group: "example.com", Version: "v1", Kind: "MyCRD"}

	// Two patches that set the same top-level key to different values
	patches := []map[string]interface{}{
		{
			"spec": map[string]interface{}{
				"foo": "bar",
			},
		},
		{
			"spec": map[string]interface{}{
				"foo": "baz",
			},
		},
	}

	report, err := DetectSMPConflicts(patches, lookup, gvk)
	if err != nil {
		t.Fatalf("DetectSMPConflicts: %v", err)
	}
	if !report.HasConflicts() {
		t.Fatal("expected conflict for unknown kind")
	}
}

func TestDetectSMPConflicts_SinglePatch(t *testing.T) {
	patches := []map[string]interface{}{
		{"spec": map[string]interface{}{"replicas": int64(3)}},
	}

	report, err := DetectSMPConflicts(patches, nil, schema.GroupVersionKind{})
	if err != nil {
		t.Fatalf("DetectSMPConflicts: %v", err)
	}
	if report.HasConflicts() {
		t.Fatal("single patch should never have conflicts")
	}
}

// TestDetectSMPConflicts_TypeMismatchDetected verifies that the fallback
// conflict detector treats values with different types (e.g. int vs string)
// as conflicting, even when their printed forms are identical.
func TestDetectSMPConflicts_TypeMismatchDetected(t *testing.T) {
	gvk := schema.GroupVersionKind{Group: "example.com", Version: "v1", Kind: "MyCRD"}

	patches := []map[string]interface{}{
		{
			"spec": map[string]interface{}{
				"replicas": int64(1),
			},
		},
		{
			"spec": map[string]interface{}{
				"replicas": "1", // string "1" vs int64 1
			},
		},
	}

	report, err := DetectSMPConflicts(patches, nil, gvk)
	if err != nil {
		t.Fatalf("DetectSMPConflicts: %v", err)
	}
	if !report.HasConflicts() {
		t.Fatal("expected conflict between int64(1) and string(\"1\") for unknown kind")
	}
}
