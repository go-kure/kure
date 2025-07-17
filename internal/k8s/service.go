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
