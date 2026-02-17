package patch

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/go-kure/kure/pkg/kubernetes"
)

// KindLookup resolves a GroupVersionKind to a typed Go struct, enabling
// strategic merge patch to read struct tags for merge strategy metadata.
type KindLookup interface {
	LookupKind(gvk schema.GroupVersionKind) (runtime.Object, bool)
}

// SchemeKindLookup implements KindLookup using a runtime.Scheme.
type SchemeKindLookup struct {
	Scheme *runtime.Scheme
}

// NewSchemeKindLookup returns a KindLookup backed by the given scheme.
func NewSchemeKindLookup(scheme *runtime.Scheme) *SchemeKindLookup {
	return &SchemeKindLookup{Scheme: scheme}
}

// LookupKind returns a zero-value typed object for the given GVK, or false
// if the GVK is not registered in the scheme.
func (s *SchemeKindLookup) LookupKind(gvk schema.GroupVersionKind) (runtime.Object, bool) {
	obj, err := s.Scheme.New(gvk)
	if err != nil {
		return nil, false
	}
	return obj, true
}

// DefaultKindLookup returns a KindLookup backed by pkg/kubernetes.Scheme,
// which has all core Kubernetes and registered CRD types.
func DefaultKindLookup() (KindLookup, error) {
	if err := kubernetes.RegisterSchemes(); err != nil {
		return nil, err
	}
	return NewSchemeKindLookup(kubernetes.Scheme), nil
}
