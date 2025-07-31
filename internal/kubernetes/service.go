package kubernetes

import (
	"errors"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateService(name string, namespace string) *corev1.Service {
	obj := &corev1.Service{
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
	return obj
}

func AddServicePort(service *corev1.Service, port corev1.ServicePort) error {
	if service == nil {
		return errors.New("nil service")
	}
	service.Spec.Ports = append(service.Spec.Ports, port)
	return nil
}

func SetServiceSelector(service *corev1.Service, selector map[string]string) error {
	if service == nil {
		return errors.New("nil service")
	}
	service.Spec.Selector = selector
	return nil
}

func SetServiceType(service *corev1.Service, type_ corev1.ServiceType) error {
	if service == nil {
		return errors.New("nil service")
	}
	service.Spec.Type = type_
	return nil
}

// SetServiceClusterIP sets the clusterIP on the Service spec.
func SetServiceClusterIP(service *corev1.Service, ip string) error {
	if service == nil {
		return errors.New("nil service")
	}
	service.Spec.ClusterIP = ip
	return nil
}

// AddServiceExternalIP appends an external IP address to the Service spec.
func AddServiceExternalIP(service *corev1.Service, ip string) error {
	if service == nil {
		return errors.New("nil service")
	}
	service.Spec.ExternalIPs = append(service.Spec.ExternalIPs, ip)
	return nil
}

// SetServiceLoadBalancerIP sets the load balancer IP on the Service spec.
func SetServiceLoadBalancerIP(service *corev1.Service, ip string) error {
	if service == nil {
		return errors.New("nil service")
	}
	service.Spec.LoadBalancerIP = ip
	return nil
}

/*
   service.Spec.LoadBalancerIP

   Deprecated: This field was under-specified and
   its meaning varies across implementations, and it cannot support dual-stack.
   As of Kubernetes v1.24, users are encouraged to use implementation-specific
   annotations when available.
*/

func SetServiceExternalTrafficPolicy(service *corev1.Service, trafficPolicy corev1.ServiceExternalTrafficPolicy) error {
	if service == nil {
		return errors.New("nil service")
	}
	service.Spec.ExternalTrafficPolicy = trafficPolicy
	return nil
}

func SetServiceSessionAffinity(service *corev1.Service, affinity corev1.ServiceAffinity) error {
	if service == nil {
		return errors.New("nil service")
	}
	service.Spec.SessionAffinity = affinity
	return nil
}

func SetServiceLoadBalancerClass(service *corev1.Service, class string) error {
	if service == nil {
		return errors.New("nil service")
	}
	service.Spec.LoadBalancerClass = &class
	return nil
}

func AddServiceLabel(svc *corev1.Service, key, value string) error {
	if svc == nil {
		return errors.New("nil service")
	}
	if svc.Labels == nil {
		svc.Labels = make(map[string]string)
	}
	svc.Labels[key] = value
	return nil
}

func AddServiceAnnotation(svc *corev1.Service, key, value string) error {
	if svc == nil {
		return errors.New("nil service")
	}
	if svc.Annotations == nil {
		svc.Annotations = make(map[string]string)
	}
	svc.Annotations[key] = value
	return nil
}

func SetServiceLabels(svc *corev1.Service, labels map[string]string) error {
	if svc == nil {
		return errors.New("nil service")
	}
	svc.Labels = labels
	return nil
}

func SetServiceAnnotations(svc *corev1.Service, anns map[string]string) error {
	if svc == nil {
		return errors.New("nil service")
	}
	svc.Annotations = anns
	return nil
}

func SetServicePublishNotReadyAddresses(svc *corev1.Service, publish bool) error {
	if svc == nil {
		return errors.New("nil service")
	}
	svc.Spec.PublishNotReadyAddresses = publish
	return nil
}

func AddServiceLoadBalancerSourceRange(svc *corev1.Service, cidr string) error {
	if svc == nil {
		return errors.New("nil service")
	}
	svc.Spec.LoadBalancerSourceRanges = append(svc.Spec.LoadBalancerSourceRanges, cidr)
	return nil
}

func SetServiceLoadBalancerSourceRanges(svc *corev1.Service, ranges []string) error {
	if svc == nil {
		return errors.New("nil service")
	}
	svc.Spec.LoadBalancerSourceRanges = ranges
	return nil
}

func SetServiceIPFamilies(svc *corev1.Service, fams []corev1.IPFamily) error {
	if svc == nil {
		return errors.New("nil service")
	}
	svc.Spec.IPFamilies = fams
	return nil
}

func SetServiceIPFamilyPolicy(svc *corev1.Service, policy *corev1.IPFamilyPolicyType) error {
	if svc == nil {
		return errors.New("nil service")
	}
	svc.Spec.IPFamilyPolicy = policy
	return nil
}

func SetServiceInternalTrafficPolicy(svc *corev1.Service, policy *corev1.ServiceInternalTrafficPolicyType) error {
	if svc == nil {
		return errors.New("nil service")
	}
	svc.Spec.InternalTrafficPolicy = policy
	return nil
}

func SetServiceAllocateLoadBalancerNodePorts(svc *corev1.Service, allocate bool) error {
	if svc == nil {
		return errors.New("nil service")
	}
	svc.Spec.AllocateLoadBalancerNodePorts = &allocate
	return nil
}

// SetServiceExternalName sets the externalName field for ExternalName services.
func SetServiceExternalName(svc *corev1.Service, name string) error {
	if svc == nil {
		return errors.New("nil service")
	}
	svc.Spec.ExternalName = name
	return nil
}

// SetServiceHealthCheckNodePort sets the healthCheckNodePort field for LoadBalancer services.
func SetServiceHealthCheckNodePort(svc *corev1.Service, port int32) error {
	if svc == nil {
		return errors.New("nil service")
	}
	svc.Spec.HealthCheckNodePort = port
	return nil
}

// SetServiceSessionAffinityConfig configures the session affinity options.
func SetServiceSessionAffinityConfig(svc *corev1.Service, cfg *corev1.SessionAffinityConfig) error {
	if svc == nil {
		return errors.New("nil service")
	}
	svc.Spec.SessionAffinityConfig = cfg
	return nil
}
