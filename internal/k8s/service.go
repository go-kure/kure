package k8s

import (
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

func AddServicePort(service *corev1.Service, port corev1.ServicePort) {
	service.Spec.Ports = append(service.Spec.Ports, port)
}

func SetServiceSelector(service *corev1.Service, selector map[string]string) {
	service.Spec.Selector = selector
}

func SetServiceType(service *corev1.Service, type_ corev1.ServiceType) {
	service.Spec.Type = type_
}

/*
   service.Spec.LoadBalancerIP

   Deprecated: This field was under-specified and
   its meaning varies across implementations, and it cannot support dual-stack.
   As of Kubernetes v1.24, users are encouraged to use implementation-specific
   annotations when available.
*/

func SetServiceExternalTrafficPolicy(service *corev1.Service, trafficPolicy corev1.ServiceExternalTrafficPolicy) {
	service.Spec.ExternalTrafficPolicy = trafficPolicy
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
