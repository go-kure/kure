package cilium

import (
	ciliumv2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateCiliumLocalRedirectPolicy returns a new CiliumLocalRedirectPolicy with
// TypeMeta and ObjectMeta set. The resource is namespace-scoped.
func CreateCiliumLocalRedirectPolicy(name, namespace string) *ciliumv2.CiliumLocalRedirectPolicy {
	return &ciliumv2.CiliumLocalRedirectPolicy{
		TypeMeta: metav1.TypeMeta{
			Kind:       ciliumv2.CLRPKindDefinition,
			APIVersion: ciliumv2.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

// SetCiliumLocalRedirectPolicySpec sets the full spec on the policy.
func SetCiliumLocalRedirectPolicySpec(obj *ciliumv2.CiliumLocalRedirectPolicy, spec ciliumv2.CiliumLocalRedirectPolicySpec) {
	obj.Spec = spec
}

// SetCiliumLocalRedirectPolicyFrontend sets the redirect frontend.
func SetCiliumLocalRedirectPolicyFrontend(obj *ciliumv2.CiliumLocalRedirectPolicy, frontend ciliumv2.RedirectFrontend) {
	obj.Spec.RedirectFrontend = frontend
}

// SetCiliumLocalRedirectPolicyBackend sets the redirect backend.
func SetCiliumLocalRedirectPolicyBackend(obj *ciliumv2.CiliumLocalRedirectPolicy, backend ciliumv2.RedirectBackend) {
	obj.Spec.RedirectBackend = backend
}

// SetCiliumLocalRedirectPolicyDescription sets the description on the policy.
func SetCiliumLocalRedirectPolicyDescription(obj *ciliumv2.CiliumLocalRedirectPolicy, desc string) {
	obj.Spec.Description = desc
}

// SetCiliumLocalRedirectPolicySkipRedirectFromBackend sets whether to skip
// redirect for traffic originating from the backend itself.
func SetCiliumLocalRedirectPolicySkipRedirectFromBackend(obj *ciliumv2.CiliumLocalRedirectPolicy, skip bool) {
	obj.Spec.SkipRedirectFromBackend = skip
}
