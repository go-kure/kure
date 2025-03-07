package k8s

import (
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateService(name string) *apiv1.Service {
	service := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"app": name,
			},
			Annotations: map[string]string{
				"app": name,
			},
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: apiv1.SchemeGroupVersion.String(),
		},
		Spec: apiv1.ServiceSpec{
			Selector: map[string]string{},
			Ports:    []apiv1.ServicePort{},
		},
	}
	return service
}

func AddServicePort(service *apiv1.Service, port apiv1.ServicePort) {
	service.Spec.Ports = append(service.Spec.Ports, port)
}

func SetServiceSelector(service *apiv1.Service, selector map[string]string) {
	service.Spec.Selector = selector
}

func SetServiceType(service *apiv1.Service, type_ apiv1.ServiceType) {
	service.Spec.Type = type_
}

func SetServiceLoadBalancerIP(service *apiv1.Service, ip string) {
	service.Spec.LoadBalancerIP = ip
}

func SetServiceExternalTrafficPolicy(service *apiv1.Service, trafficPolicy apiv1.ServiceExternalTrafficPolicy) {
	service.Spec.ExternalTrafficPolicy = trafficPolicy
}
