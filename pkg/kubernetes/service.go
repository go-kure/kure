package kubernetes

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/go-kure/kure/pkg/errors"
)

// CreateService creates a new v1 Service with the given name and namespace.
// The returned object has TypeMeta, labels, annotations, and an empty selector
// and ports slice pre-populated so it can be serialized to YAML immediately.
func CreateService(name string, namespace string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"app": name,
			},
			Annotations: map[string]string{
				"app": name,
			},
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: corev1.SchemeGroupVersion.String(),
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{},
			Ports:    []corev1.ServicePort{},
		},
	}
}

// AddServicePort appends a port to the Service spec.
func AddServicePort(service *corev1.Service, port corev1.ServicePort) error {
	if service == nil {
		return errors.ErrNilService
	}
	service.Spec.Ports = append(service.Spec.Ports, port)
	return nil
}

// SetServiceSelector sets the selector map on the Service spec.
func SetServiceSelector(service *corev1.Service, selector map[string]string) error {
	if service == nil {
		return errors.ErrNilService
	}
	service.Spec.Selector = selector
	return nil
}

// SetServiceType sets the service type (ClusterIP, NodePort, LoadBalancer, etc.).
func SetServiceType(service *corev1.Service, type_ corev1.ServiceType) error {
	if service == nil {
		return errors.ErrNilService
	}
	service.Spec.Type = type_
	return nil
}

// SetServiceClusterIP sets the clusterIP on the Service spec.
func SetServiceClusterIP(service *corev1.Service, ip string) error {
	if service == nil {
		return errors.ErrNilService
	}
	service.Spec.ClusterIP = ip
	return nil
}

// AddServiceExternalIP appends an external IP address to the Service spec.
func AddServiceExternalIP(service *corev1.Service, ip string) error {
	if service == nil {
		return errors.ErrNilService
	}
	service.Spec.ExternalIPs = append(service.Spec.ExternalIPs, ip)
	return nil
}

// SetServiceLoadBalancerIP sets the load balancer IP on the Service spec.
//
// Deprecated: This field was under-specified and its meaning varies across
// implementations. As of Kubernetes v1.24, users are encouraged to use
// implementation-specific annotations when available.
func SetServiceLoadBalancerIP(service *corev1.Service, ip string) error {
	if service == nil {
		return errors.ErrNilService
	}
	service.Spec.LoadBalancerIP = ip
	return nil
}

// SetServiceExternalTrafficPolicy sets the external traffic policy on the
// Service spec.
func SetServiceExternalTrafficPolicy(service *corev1.Service, trafficPolicy corev1.ServiceExternalTrafficPolicy) error {
	if service == nil {
		return errors.ErrNilService
	}
	service.Spec.ExternalTrafficPolicy = trafficPolicy
	return nil
}

// SetServiceSessionAffinity sets the session affinity on the Service spec.
func SetServiceSessionAffinity(service *corev1.Service, affinity corev1.ServiceAffinity) error {
	if service == nil {
		return errors.ErrNilService
	}
	service.Spec.SessionAffinity = affinity
	return nil
}

// SetServiceLoadBalancerClass sets the load balancer class on the Service spec.
func SetServiceLoadBalancerClass(service *corev1.Service, class string) error {
	if service == nil {
		return errors.ErrNilService
	}
	service.Spec.LoadBalancerClass = &class
	return nil
}

// AddServiceLabel adds a single label to the Service metadata.
func AddServiceLabel(svc *corev1.Service, key, value string) error {
	if svc == nil {
		return errors.ErrNilService
	}
	if svc.Labels == nil {
		svc.Labels = make(map[string]string)
	}
	svc.Labels[key] = value
	return nil
}

// AddServiceAnnotation adds a single annotation to the Service metadata.
func AddServiceAnnotation(svc *corev1.Service, key, value string) error {
	if svc == nil {
		return errors.ErrNilService
	}
	if svc.Annotations == nil {
		svc.Annotations = make(map[string]string)
	}
	svc.Annotations[key] = value
	return nil
}

// SetServiceLabels replaces the labels on the Service with the provided map.
func SetServiceLabels(svc *corev1.Service, labels map[string]string) error {
	if svc == nil {
		return errors.ErrNilService
	}
	svc.Labels = labels
	return nil
}

// SetServiceAnnotations replaces the annotations on the Service with the
// provided map.
func SetServiceAnnotations(svc *corev1.Service, anns map[string]string) error {
	if svc == nil {
		return errors.ErrNilService
	}
	svc.Annotations = anns
	return nil
}

// SetServicePublishNotReadyAddresses sets whether endpoints for not-ready pods
// are published.
func SetServicePublishNotReadyAddresses(svc *corev1.Service, publish bool) error {
	if svc == nil {
		return errors.ErrNilService
	}
	svc.Spec.PublishNotReadyAddresses = publish
	return nil
}

// AddServiceLoadBalancerSourceRange appends a CIDR to the allowed source ranges
// for a load balancer Service.
func AddServiceLoadBalancerSourceRange(svc *corev1.Service, cidr string) error {
	if svc == nil {
		return errors.ErrNilService
	}
	svc.Spec.LoadBalancerSourceRanges = append(svc.Spec.LoadBalancerSourceRanges, cidr)
	return nil
}

// SetServiceLoadBalancerSourceRanges replaces the load balancer source ranges
// on the Service spec.
func SetServiceLoadBalancerSourceRanges(svc *corev1.Service, ranges []string) error {
	if svc == nil {
		return errors.ErrNilService
	}
	svc.Spec.LoadBalancerSourceRanges = ranges
	return nil
}

// SetServiceIPFamilies sets the IP families on the Service spec.
func SetServiceIPFamilies(svc *corev1.Service, fams []corev1.IPFamily) error {
	if svc == nil {
		return errors.ErrNilService
	}
	svc.Spec.IPFamilies = fams
	return nil
}

// SetServiceIPFamilyPolicy sets the IP family policy on the Service spec.
func SetServiceIPFamilyPolicy(svc *corev1.Service, policy *corev1.IPFamilyPolicyType) error {
	if svc == nil {
		return errors.ErrNilService
	}
	svc.Spec.IPFamilyPolicy = policy
	return nil
}

// SetServiceInternalTrafficPolicy sets the internal traffic policy on the
// Service spec.
func SetServiceInternalTrafficPolicy(svc *corev1.Service, policy *corev1.ServiceInternalTrafficPolicyType) error {
	if svc == nil {
		return errors.ErrNilService
	}
	svc.Spec.InternalTrafficPolicy = policy
	return nil
}

// SetServiceAllocateLoadBalancerNodePorts controls whether node ports are
// allocated for a LoadBalancer Service.
func SetServiceAllocateLoadBalancerNodePorts(svc *corev1.Service, allocate bool) error {
	if svc == nil {
		return errors.ErrNilService
	}
	svc.Spec.AllocateLoadBalancerNodePorts = &allocate
	return nil
}

// SetServiceExternalName sets the externalName field for ExternalName services.
func SetServiceExternalName(svc *corev1.Service, name string) error {
	if svc == nil {
		return errors.ErrNilService
	}
	svc.Spec.ExternalName = name
	return nil
}

// SetServiceHealthCheckNodePort sets the healthCheckNodePort field for
// LoadBalancer services.
func SetServiceHealthCheckNodePort(svc *corev1.Service, port int32) error {
	if svc == nil {
		return errors.ErrNilService
	}
	svc.Spec.HealthCheckNodePort = port
	return nil
}

// SetServiceSessionAffinityConfig configures the session affinity options.
func SetServiceSessionAffinityConfig(svc *corev1.Service, cfg *corev1.SessionAffinityConfig) error {
	if svc == nil {
		return errors.ErrNilService
	}
	svc.Spec.SessionAffinityConfig = cfg
	return nil
}
