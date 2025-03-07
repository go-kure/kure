package k8s

import (
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateIngress(name string, classname string) *netv1.Ingress {
	ingress := &netv1.Ingress{
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
			Kind:       "Ingress",
			APIVersion: netv1.SchemeGroupVersion.String(),
		},
		Spec: netv1.IngressSpec{
			IngressClassName: &classname,
			Rules:            []netv1.IngressRule{},
			TLS:              []netv1.IngressTLS{},
		},
	}
	return ingress
}

func AddIngressRule(ingress *netv1.Ingress, rule netv1.IngressRule) {
	ingress.Spec.Rules = append(ingress.Spec.Rules, rule)
}

func AddIngressTls(ingress *netv1.Ingress, tls netv1.IngressTLS) {
	ingress.Spec.TLS = append(ingress.Spec.TLS, tls)
}
