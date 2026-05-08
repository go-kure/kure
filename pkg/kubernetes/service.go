package kubernetes

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
func AddServicePort(service *corev1.Service, port corev1.ServicePort) {
	if service == nil {
		panic("AddServicePort: service must not be nil")
	}
	service.Spec.Ports = append(service.Spec.Ports, port)
}

// SetServiceSelector sets the selector map on the Service spec.
func SetServiceSelector(service *corev1.Service, selector map[string]string) {
	if service == nil {
		panic("SetServiceSelector: service must not be nil")
	}
	service.Spec.Selector = selector
}

// SetServiceType sets the service type (ClusterIP, NodePort, LoadBalancer, etc.).
func SetServiceType(service *corev1.Service, type_ corev1.ServiceType) {
	if service == nil {
		panic("SetServiceType: service must not be nil")
	}
	service.Spec.Type = type_
}

// SetServiceClusterIP sets the clusterIP on the Service spec.
func SetServiceClusterIP(service *corev1.Service, ip string) {
	if service == nil {
		panic("SetServiceClusterIP: service must not be nil")
	}
	service.Spec.ClusterIP = ip
}

// AddServiceExternalIP appends an external IP address to the Service spec.
func AddServiceExternalIP(service *corev1.Service, ip string) {
	if service == nil {
		panic("AddServiceExternalIP: service must not be nil")
	}
	service.Spec.ExternalIPs = append(service.Spec.ExternalIPs, ip)
}

// SetServiceExternalTrafficPolicy sets the external traffic policy on the
// Service spec.
func SetServiceExternalTrafficPolicy(service *corev1.Service, trafficPolicy corev1.ServiceExternalTrafficPolicy) {
	if service == nil {
		panic("SetServiceExternalTrafficPolicy: service must not be nil")
	}
	service.Spec.ExternalTrafficPolicy = trafficPolicy
}

// SetServiceSessionAffinity sets the session affinity on the Service spec.
func SetServiceSessionAffinity(service *corev1.Service, affinity corev1.ServiceAffinity) {
	if service == nil {
		panic("SetServiceSessionAffinity: service must not be nil")
	}
	service.Spec.SessionAffinity = affinity
}

// SetServiceLoadBalancerClass sets the load balancer class on the Service spec.
func SetServiceLoadBalancerClass(service *corev1.Service, class string) {
	if service == nil {
		panic("SetServiceLoadBalancerClass: service must not be nil")
	}
	service.Spec.LoadBalancerClass = &class
}

// AddServiceLabel adds a single label to the Service metadata.
func AddServiceLabel(svc *corev1.Service, key, value string) {
	if svc == nil {
		panic("AddServiceLabel: svc must not be nil")
	}
	if svc.Labels == nil {
		svc.Labels = make(map[string]string)
	}
	svc.Labels[key] = value
}

// AddServiceAnnotation adds a single annotation to the Service metadata.
func AddServiceAnnotation(svc *corev1.Service, key, value string) {
	if svc == nil {
		panic("AddServiceAnnotation: svc must not be nil")
	}
	if svc.Annotations == nil {
		svc.Annotations = make(map[string]string)
	}
	svc.Annotations[key] = value
}

// SetServiceLabels replaces the labels on the Service with the provided map.
func SetServiceLabels(svc *corev1.Service, labels map[string]string) {
	if svc == nil {
		panic("SetServiceLabels: svc must not be nil")
	}
	svc.Labels = labels
}

// SetServiceAnnotations replaces the annotations on the Service with the
// provided map.
func SetServiceAnnotations(svc *corev1.Service, anns map[string]string) {
	if svc == nil {
		panic("SetServiceAnnotations: svc must not be nil")
	}
	svc.Annotations = anns
}

// SetServicePublishNotReadyAddresses sets whether endpoints for not-ready pods
// are published.
func SetServicePublishNotReadyAddresses(svc *corev1.Service, publish bool) {
	if svc == nil {
		panic("SetServicePublishNotReadyAddresses: svc must not be nil")
	}
	svc.Spec.PublishNotReadyAddresses = publish
}

// AddServiceLoadBalancerSourceRange appends a CIDR to the allowed source ranges
// for a load balancer Service.
func AddServiceLoadBalancerSourceRange(svc *corev1.Service, cidr string) {
	if svc == nil {
		panic("AddServiceLoadBalancerSourceRange: svc must not be nil")
	}
	svc.Spec.LoadBalancerSourceRanges = append(svc.Spec.LoadBalancerSourceRanges, cidr)
}

// SetServiceLoadBalancerSourceRanges replaces the load balancer source ranges
// on the Service spec.
func SetServiceLoadBalancerSourceRanges(svc *corev1.Service, ranges []string) {
	if svc == nil {
		panic("SetServiceLoadBalancerSourceRanges: svc must not be nil")
	}
	svc.Spec.LoadBalancerSourceRanges = ranges
}

// SetServiceIPFamilies sets the IP families on the Service spec.
func SetServiceIPFamilies(svc *corev1.Service, fams []corev1.IPFamily) {
	if svc == nil {
		panic("SetServiceIPFamilies: svc must not be nil")
	}
	svc.Spec.IPFamilies = fams
}

// SetServiceIPFamilyPolicy sets the IP family policy on the Service spec.
func SetServiceIPFamilyPolicy(svc *corev1.Service, policy *corev1.IPFamilyPolicy) {
	if svc == nil {
		panic("SetServiceIPFamilyPolicy: svc must not be nil")
	}
	svc.Spec.IPFamilyPolicy = policy
}

// SetServiceInternalTrafficPolicy sets the internal traffic policy on the
// Service spec.
func SetServiceInternalTrafficPolicy(svc *corev1.Service, policy *corev1.ServiceInternalTrafficPolicy) {
	if svc == nil {
		panic("SetServiceInternalTrafficPolicy: svc must not be nil")
	}
	svc.Spec.InternalTrafficPolicy = policy
}

// SetServiceAllocateLoadBalancerNodePorts controls whether node ports are
// allocated for a LoadBalancer Service.
func SetServiceAllocateLoadBalancerNodePorts(svc *corev1.Service, allocate bool) {
	if svc == nil {
		panic("SetServiceAllocateLoadBalancerNodePorts: svc must not be nil")
	}
	svc.Spec.AllocateLoadBalancerNodePorts = &allocate
}

// SetServiceExternalName sets the externalName field for ExternalName services.
func SetServiceExternalName(svc *corev1.Service, name string) {
	if svc == nil {
		panic("SetServiceExternalName: svc must not be nil")
	}
	svc.Spec.ExternalName = name
}

// SetServiceHealthCheckNodePort sets the healthCheckNodePort field for
// LoadBalancer services.
func SetServiceHealthCheckNodePort(svc *corev1.Service, port int32) {
	if svc == nil {
		panic("SetServiceHealthCheckNodePort: svc must not be nil")
	}
	svc.Spec.HealthCheckNodePort = port
}

// SetServiceSessionAffinityConfig configures the session affinity options.
func SetServiceSessionAffinityConfig(svc *corev1.Service, cfg *corev1.SessionAffinityConfig) {
	if svc == nil {
		panic("SetServiceSessionAffinityConfig: svc must not be nil")
	}
	svc.Spec.SessionAffinityConfig = cfg
}
