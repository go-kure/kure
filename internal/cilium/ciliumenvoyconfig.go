package cilium

import (
	ciliumv2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	slimv1 "github.com/cilium/cilium/pkg/k8s/slim/k8s/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateCiliumEnvoyConfig returns a new CiliumEnvoyConfig with TypeMeta and
// ObjectMeta set. The resource is namespace-scoped.
func CreateCiliumEnvoyConfig(name, namespace string) *ciliumv2.CiliumEnvoyConfig {
	return &ciliumv2.CiliumEnvoyConfig{
		TypeMeta: metav1.TypeMeta{
			Kind:       ciliumv2.CECKindDefinition,
			APIVersion: ciliumv2.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

// SetCiliumEnvoyConfigSpec sets the full spec on the config.
func SetCiliumEnvoyConfigSpec(obj *ciliumv2.CiliumEnvoyConfig, spec ciliumv2.CiliumEnvoyConfigSpec) {
	obj.Spec = spec
}

// AddCiliumEnvoyConfigService appends a service listener.
func AddCiliumEnvoyConfigService(obj *ciliumv2.CiliumEnvoyConfig, svc *ciliumv2.ServiceListener) {
	obj.Spec.Services = append(obj.Spec.Services, svc)
}

// AddCiliumEnvoyConfigBackendService appends a backend service.
func AddCiliumEnvoyConfigBackendService(obj *ciliumv2.CiliumEnvoyConfig, svc *ciliumv2.Service) {
	obj.Spec.BackendServices = append(obj.Spec.BackendServices, svc)
}

// AddCiliumEnvoyConfigResource appends an Envoy xDS resource.
func AddCiliumEnvoyConfigResource(obj *ciliumv2.CiliumEnvoyConfig, res ciliumv2.XDSResource) {
	obj.Spec.Resources = append(obj.Spec.Resources, res)
}

// SetCiliumEnvoyConfigNodeSelector sets the node selector on the config.
// Uses Cilium's slim LabelSelector from github.com/cilium/cilium/pkg/k8s/slim/k8s/apis/meta/v1.
func SetCiliumEnvoyConfigNodeSelector(obj *ciliumv2.CiliumEnvoyConfig, sel *slimv1.LabelSelector) {
	obj.Spec.NodeSelector = sel
}

// CreateCiliumClusterwideEnvoyConfig returns a new CiliumClusterwideEnvoyConfig
// with TypeMeta and ObjectMeta set. The resource is cluster-scoped.
func CreateCiliumClusterwideEnvoyConfig(name string) *ciliumv2.CiliumClusterwideEnvoyConfig {
	return &ciliumv2.CiliumClusterwideEnvoyConfig{
		TypeMeta: metav1.TypeMeta{
			Kind:       ciliumv2.CCECKindDefinition,
			APIVersion: ciliumv2.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

// SetCiliumClusterwideEnvoyConfigSpec sets the full spec on the config.
func SetCiliumClusterwideEnvoyConfigSpec(obj *ciliumv2.CiliumClusterwideEnvoyConfig, spec ciliumv2.CiliumEnvoyConfigSpec) {
	obj.Spec = spec
}

// AddCiliumClusterwideEnvoyConfigService appends a service listener.
func AddCiliumClusterwideEnvoyConfigService(obj *ciliumv2.CiliumClusterwideEnvoyConfig, svc *ciliumv2.ServiceListener) {
	obj.Spec.Services = append(obj.Spec.Services, svc)
}

// AddCiliumClusterwideEnvoyConfigBackendService appends a backend service.
func AddCiliumClusterwideEnvoyConfigBackendService(obj *ciliumv2.CiliumClusterwideEnvoyConfig, svc *ciliumv2.Service) {
	obj.Spec.BackendServices = append(obj.Spec.BackendServices, svc)
}

// AddCiliumClusterwideEnvoyConfigResource appends an Envoy xDS resource.
func AddCiliumClusterwideEnvoyConfigResource(obj *ciliumv2.CiliumClusterwideEnvoyConfig, res ciliumv2.XDSResource) {
	obj.Spec.Resources = append(obj.Spec.Resources, res)
}

// SetCiliumClusterwideEnvoyConfigNodeSelector sets the node selector on the config.
// Uses Cilium's slim LabelSelector from github.com/cilium/cilium/pkg/k8s/slim/k8s/apis/meta/v1.
func SetCiliumClusterwideEnvoyConfigNodeSelector(obj *ciliumv2.CiliumClusterwideEnvoyConfig, sel *slimv1.LabelSelector) {
	obj.Spec.NodeSelector = sel
}
