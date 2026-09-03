package kubernetes

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/errors"
)

// Create allocates a registered Kubernetes object of type T and sets its
// identity — nothing else.
//
// It resolves TypeMeta (apiVersion, kind) from the package scheme, the same
// mechanism [GetGroupVersionKind] uses, and sets metadata.name and
// metadata.namespace. Every other field is left at its Go zero value: no
// labels, no annotations, no selector, no defaults. The upstream struct is the
// construction API for everything else:
//
//	d := kubernetes.Create[appsv1.Deployment]("web", "default")
//	d.Spec.Replicas = ptr.To(int32(3))
//
// PT is inferred from T (core-type inference), so the one-argument call form
// above is the normal spelling. Cluster-scoped kinds pass "" for namespace.
// The generated per-kind wrappers (CreateDeployment, CreateNamespace, …) are
// one-line calls to Create with the scope baked into the signature.
//
// Create panics when T is not registered in the scheme: an unregistered type is
// a programming error, the same rule the sugar helpers apply to nil receivers.
func Create[T any, PT interface {
	*T
	client.Object
}](name, namespace string) PT {
	obj := PT(new(T))
	obj.GetObjectKind().SetGroupVersionKind(mustGVK(obj))
	obj.SetName(name)
	obj.SetNamespace(namespace)
	return obj
}

// mustGVK resolves the single GVK of obj from the package scheme or panics.
func mustGVK(obj runtime.Object) schema.GroupVersionKind {
	if err := RegisterSchemes(); err != nil {
		panic(errors.Wrap(err, "kubernetes.Create: registering schemes"))
	}
	gvks, _, err := Scheme.ObjectKinds(obj)
	if err != nil {
		panic(errors.Wrapf(err, "kubernetes.Create: %T is not registered in the scheme", obj))
	}
	if len(gvks) == 0 {
		panic(errors.Wrapf(errors.ErrGVKNotFound, "kubernetes.Create: %T", obj))
	}
	return gvks[0]
}
