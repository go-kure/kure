package kubernetes

import (
	corev1 "k8s.io/api/core/v1"
)

// AddServicePort appends a port to the Service spec.
func AddServicePort(service *corev1.Service, port corev1.ServicePort) {
	if service == nil {
		panic("AddServicePort: service must not be nil")
	}
	service.Spec.Ports = append(service.Spec.Ports, port)
}

// AddServiceExternalIP appends an external IP address to the Service spec.
func AddServiceExternalIP(service *corev1.Service, ip string) {
	if service == nil {
		panic("AddServiceExternalIP: service must not be nil")
	}
	service.Spec.ExternalIPs = append(service.Spec.ExternalIPs, ip)
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

// AddServiceLoadBalancerSourceRange appends a CIDR to the allowed source ranges
// for a load balancer Service.
func AddServiceLoadBalancerSourceRange(svc *corev1.Service, cidr string) {
	if svc == nil {
		panic("AddServiceLoadBalancerSourceRange: svc must not be nil")
	}
	svc.Spec.LoadBalancerSourceRanges = append(svc.Spec.LoadBalancerSourceRanges, cidr)
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

// SetServiceSessionAffinityConfig configures the session affinity options.
func SetServiceSessionAffinityConfig(svc *corev1.Service, cfg *corev1.SessionAffinityConfig) {
	if svc == nil {
		panic("SetServiceSessionAffinityConfig: svc must not be nil")
	}
	svc.Spec.SessionAffinityConfig = cfg
}
