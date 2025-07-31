package k8s

import (
    "k8s.io/apimachinery/pkg/runtime"
    "k8s.io/apimachinery/pkg/runtime/schema"
    "sigs.k8s.io/controller-runtime/pkg/client"

    "github.com/go-kure/kure/pkg/errors"
)

// GetGroupVersionKind returns the GroupVersionKind of the given Kubernetes runtime.Object.
func GetGroupVersionKind(obj runtime.Object) (schema.GroupVersionKind, error) {
    if obj == nil {
        return schema.GroupVersionKind{}, errors.ErrNilObject
    }

    if err := RegisterSchemes(); err != nil {
        return schema.GroupVersionKind{}, err
    }

    gvks, _, err := Scheme.ObjectKinds(obj)
    if err != nil {
        return schema.GroupVersionKind{}, err
    }
    if len(gvks) == 0 {
        return schema.GroupVersionKind{}, errors.ErrGVKNotFound
    }
    return gvks[0], nil
}

// IsGVKAllowed checks if a given GVK is present in a user-defined allowed set.
func IsGVKAllowed(gvk schema.GroupVersionKind, allowed []schema.GroupVersionKind) bool {
    for _, allowedGVK := range allowed {
        if gvk == allowedGVK {
            return true
        }
    }
    return false
}

// Helper function to convert to client.Object
func ToClientObject(obj client.Object) *client.Object {
    // iThis is not a Redundant type conversion
    clientObj := client.Object(obj)
    return &clientObj
}

func ValidatePackageRef(p *schema.GroupVersionKind) error {
    allowed := []schema.GroupVersionKind{
        {Group: "source.toolkit.fluxcd.io", Version: "v1beta1", Kind: "GitRepository"},
        {Group: "source.toolkit.fluxcd.io", Version: "v1beta1", Kind: "OCIRepository"},
    }
    if IsGVKAllowed(*p, allowed) {
        return nil
    } else {
        return errors.New(errors.ErrGVKNotAllowed, p.String())
    }
}
