package patch

import (
	"fmt"
	"reflect"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

// ConflictReport describes conflicts detected among strategic merge patches
// targeting the same resource.
type ConflictReport struct {
	ResourceName string
	ResourceKind string
	Conflicts    []PatchConflict
}

// PatchConflict describes a single conflict between two patches.
type PatchConflict struct {
	PatchIndexA int
	PatchIndexB int
	Description string
}

// HasConflicts returns true if any conflicts were detected.
func (r *ConflictReport) HasConflicts() bool {
	return len(r.Conflicts) > 0
}

// DetectSMPConflicts checks pairwise conflicts among strategic merge patches
// targeting the same resource. For known kinds, it uses
// MergingMapsHaveConflicts with PatchMetaFromStruct. For unknown kinds,
// it performs a simple key-overlap check.
func DetectSMPConflicts(
	patches []map[string]interface{},
	lookup KindLookup,
	gvk schema.GroupVersionKind,
) (*ConflictReport, error) {
	if len(patches) < 2 {
		return &ConflictReport{}, nil
	}

	report := &ConflictReport{
		ResourceKind: gvk.Kind,
	}

	// Determine schema for conflict detection
	var schema strategicpatch.LookupPatchMeta
	if lookup != nil {
		if typedObj, ok := lookup.LookupKind(gvk); ok {
			meta, err := strategicpatch.NewPatchMetaFromStruct(typedObj)
			if err == nil {
				schema = meta
			}
		}
	}

	for i := 0; i < len(patches); i++ {
		for j := i + 1; j < len(patches); j++ {
			hasConflict, err := detectPairConflict(patches[i], patches[j], schema)
			if err != nil {
				return nil, fmt.Errorf("conflict detection failed between patch %d and %d: %w", i, j, err)
			}
			if hasConflict {
				report.Conflicts = append(report.Conflicts, PatchConflict{
					PatchIndexA: i,
					PatchIndexB: j,
					Description: fmt.Sprintf("patches %d and %d have conflicting values", i, j),
				})
			}
		}
	}

	return report, nil
}

// detectPairConflict checks if two patches conflict. Uses strategic merge
// conflict detection when a schema is available, otherwise falls back to
// simple key overlap checking.
func detectPairConflict(a, b map[string]interface{}, schema strategicpatch.LookupPatchMeta) (bool, error) {
	if schema != nil {
		return strategicpatch.MergingMapsHaveConflicts(a, b, schema)
	}
	return simpleKeyOverlapConflict(a, b), nil
}

// simpleKeyOverlapConflict checks if two maps set the same top-level keys
// to different values. This is a conservative fallback for unknown kinds.
func simpleKeyOverlapConflict(a, b map[string]interface{}) bool {
	for key, va := range a {
		if vb, exists := b[key]; exists {
			if !deepEqual(va, vb) {
				return true
			}
		}
	}
	return false
}

// deepEqual performs a recursive comparison of two values.
func deepEqual(a, b interface{}) bool {
	switch va := a.(type) {
	case map[string]interface{}:
		vb, ok := b.(map[string]interface{})
		if !ok || len(va) != len(vb) {
			return false
		}
		for k, valA := range va {
			valB, exists := vb[k]
			if !exists || !deepEqual(valA, valB) {
				return false
			}
		}
		return true
	case []interface{}:
		vb, ok := b.([]interface{})
		if !ok || len(va) != len(vb) {
			return false
		}
		for i := range va {
			if !deepEqual(va[i], vb[i]) {
				return false
			}
		}
		return true
	default:
		return reflect.DeepEqual(a, b)
	}
}
