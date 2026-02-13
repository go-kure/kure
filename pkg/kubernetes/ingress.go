package kubernetes

import (
	"github.com/go-kure/kure/pkg/errors"

	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateIngress creates a new networking/v1 Ingress with the given name,
// namespace, and ingress class name. The returned object has TypeMeta, labels,
// annotations, and empty rules/TLS slices pre-populated so it can be serialized
// to YAML immediately.
func CreateIngress(name string, namespace string, classname string) *netv1.Ingress {
	return &netv1.Ingress{
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
}

// CreateIngressRule creates a new IngressRule for the given host with an empty
// HTTP paths list.
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

// CreateIngressPath creates an HTTPIngressPath with the given path, path type,
// service name, and service port name.
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

// AddIngressRule appends an IngressRule to the Ingress spec.
func AddIngressRule(ingress *netv1.Ingress, rule *netv1.IngressRule) error {
	if ingress == nil {
		return errors.ErrNilIngress
	}
	ingress.Spec.Rules = append(ingress.Spec.Rules, *rule)
	return nil
}

// AddIngressRulePath appends a path to an IngressRule's HTTP paths.
func AddIngressRulePath(rule *netv1.IngressRule, path netv1.HTTPIngressPath) {
	if rule.IngressRuleValue.HTTP == nil {
		rule.IngressRuleValue.HTTP = &netv1.HTTPIngressRuleValue{}
	}
	rule.IngressRuleValue.HTTP.Paths = append(rule.IngressRuleValue.HTTP.Paths, path)
}

// AddIngressTLS appends a TLS configuration to the Ingress spec.
func AddIngressTLS(ingress *netv1.Ingress, tls netv1.IngressTLS) error {
	if ingress == nil {
		return errors.ErrNilIngress
	}
	ingress.Spec.TLS = append(ingress.Spec.TLS, tls)
	return nil
}

// SetIngressDefaultBackend sets the default backend on the Ingress spec.
func SetIngressDefaultBackend(ingress *netv1.Ingress, backend netv1.IngressBackend) error {
	if ingress == nil {
		return errors.ErrNilIngress
	}
	ingress.Spec.DefaultBackend = &backend
	return nil
}

// SetIngressClassName sets the ingress class name on the Ingress spec.
func SetIngressClassName(ingress *netv1.Ingress, class string) error {
	if ingress == nil {
		return errors.ErrNilIngress
	}
	ingress.Spec.IngressClassName = &class
	return nil
}
