package k8s

import (
	"errors"

	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateIngress(name string, namespace string, classname string) *netv1.Ingress {
	obj := &netv1.Ingress{
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
			Kind:       "Ingress",
			APIVersion: netv1.SchemeGroupVersion.String(),
		},
		Spec: netv1.IngressSpec{
			IngressClassName: &classname,
			Rules:            []netv1.IngressRule{},
			TLS:              []netv1.IngressTLS{},
		},
	}
	return obj
}

func CreateIngressRule(host string) *netv1.IngressRule {
	return &netv1.IngressRule{
		Host: host,
		IngressRuleValue: netv1.IngressRuleValue{
			HTTP: &netv1.HTTPIngressRuleValue{
				Paths: []netv1.HTTPIngressPath{},
			},
		},
	}
}
func CreateIngressPath(path string, pathType *netv1.PathType, servicename string, serviceportname string) netv1.HTTPIngressPath {
	return netv1.HTTPIngressPath{
		Path:     path,
		PathType: pathType,
		Backend: netv1.IngressBackend{
			Service: &netv1.IngressServiceBackend{
				Name: servicename,
				Port: netv1.ServiceBackendPort{
					Name: serviceportname,
				},
			},
		},
	}
}
func AddIngressRule(ingress *netv1.Ingress, rule *netv1.IngressRule) {
	ingress.Spec.Rules = append(ingress.Spec.Rules, *rule)
}
func AddIngressRulePath(rule *netv1.IngressRule, path netv1.HTTPIngressPath) {
	if rule.IngressRuleValue.HTTP == nil {
		rule.IngressRuleValue.HTTP = &netv1.HTTPIngressRuleValue{}
	}
	rule.IngressRuleValue.HTTP.Paths = append(rule.IngressRuleValue.HTTP.Paths, path)
}

func AddIngressTLS(ingress *netv1.Ingress, tls netv1.IngressTLS) {
	ingress.Spec.TLS = append(ingress.Spec.TLS, tls)
}

func SetIngressDefaultBackend(ingress *netv1.Ingress, backend netv1.IngressBackend) {
	ingress.Spec.DefaultBackend = &backend
}

func SetIngressClassName(ingress *netv1.Ingress, class string) error {
	if ingress == nil {
		return errors.New("nil ingress")
	}
	ingress.Spec.IngressClassName = &class
	return nil
}
