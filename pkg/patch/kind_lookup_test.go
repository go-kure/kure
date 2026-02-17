package patch

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestSchemeKindLookup_KnownKinds(t *testing.T) {
	lookup, err := DefaultKindLookup()
	if err != nil {
		t.Fatalf("DefaultKindLookup: %v", err)
	}

	tests := []struct {
		name string
		gvk  schema.GroupVersionKind
	}{
		{
			name: "Deployment",
			gvk:  schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
		},
		{
			name: "Service",
			gvk:  schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Service"},
		},
		{
			name: "ConfigMap",
			gvk:  schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"},
		},
		{
			name: "StatefulSet",
			gvk:  schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "StatefulSet"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj, ok := lookup.LookupKind(tt.gvk)
			if !ok {
				t.Fatalf("expected GVK %v to be found", tt.gvk)
			}
			if obj == nil {
				t.Fatal("expected non-nil object")
			}
		})
	}
}

func TestSchemeKindLookup_UnknownKind(t *testing.T) {
	lookup, err := DefaultKindLookup()
	if err != nil {
		t.Fatalf("DefaultKindLookup: %v", err)
	}

	gvk := schema.GroupVersionKind{
		Group:   "example.com",
		Version: "v1",
		Kind:    "MyCRD",
	}
	_, ok := lookup.LookupKind(gvk)
	if ok {
		t.Fatal("expected unknown GVK to return false")
	}
}
