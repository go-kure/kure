package patch

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// PatchableAppSet represents a collection of resources together with the
// patches that should be applied to them.
type PatchableAppSet struct {
	Resources []*unstructured.Unstructured
	Patches   []struct {
		Target string
		Patch  PatchOp
	}
}

// Resolve groups patches by their target resource and returns them as
// ResourceWithPatches objects.
func (s *PatchableAppSet) Resolve() ([]*ResourceWithPatches, error) {
	out := make(map[string]*ResourceWithPatches)
	for _, r := range s.Resources {
		name := r.GetName()
		out[name] = &ResourceWithPatches{
			Name: name,
			Base: r.DeepCopy(),
		}
	}
	for _, p := range s.Patches {
		if rw, ok := out[p.Target]; ok {
			rw.Patches = append(rw.Patches, p.Patch)
		} else {
			return nil, fmt.Errorf("target not found: %s", p.Target)
		}
	}
	var result []*ResourceWithPatches
	for _, r := range out {
		result = append(result, r)
	}
	return result, nil
}
