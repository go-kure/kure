package k8s

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

func SetServiceSessionAffinity(service *corev1.Service, affinity corev1.ServiceAffinity) {
	service.Spec.SessionAffinity = affinity
}

func SetServiceLoadBalancerClass(service *corev1.Service, class string) {
	service.Spec.LoadBalancerClass = &class
}

func AddServiceLabel(svc *corev1.Service, key, value string) {
	if svc.Labels == nil {
		svc.Labels = make(map[string]string)
	}
	svc.Labels[key] = value
}

func AddServiceAnnotation(svc *corev1.Service, key, value string) {
	if svc.Annotations == nil {
		svc.Annotations = make(map[string]string)
	}
	svc.Annotations[key] = value
}

func SetServiceLabels(svc *corev1.Service, labels map[string]string) {
	svc.Labels = labels
}

func SetServiceAnnotations(svc *corev1.Service, anns map[string]string) {
	svc.Annotations = anns
}
