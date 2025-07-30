package cluster

import (
    "k8s.io/apimachinery/pkg/runtime/schema"

    "github.com/go-kure/kure/pkg/errors"
    "github.com/go-kure/kure/pkg/k8s"
)

func ValidatePackageRef(p *schema.GroupVersionKind) error {
    allowed := []schema.GroupVersionKind{
        {Group: "source.toolkit.fluxcd.io", Version: "v1beta1", Kind: "GitRepository"},
        {Group: "source.toolkit.fluxcd.io", Version: "v1beta1", Kind: "OCIRepository"},
    }
    if k8s.IsGVKAllowed(*p, allowed) {
        return nil
    } else {
        return errors.New(errors.ErrGVKNotAllowed, p.String())
    }
}
