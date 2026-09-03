package kubernetes

import (
	netv1 "k8s.io/api/networking/v1"
)

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
func AddIngressRule(ingress *netv1.Ingress, rule *netv1.IngressRule) {
	if ingress == nil {
		panic("AddIngressRule: ingress must not be nil")
	}
	ingress.Spec.Rules = append(ingress.Spec.Rules, *rule)
}

// AddIngressRulePath appends a path to an IngressRule's HTTP paths.
func AddIngressRulePath(rule *netv1.IngressRule, path netv1.HTTPIngressPath) {
	if rule.IngressRuleValue.HTTP == nil {
		rule.IngressRuleValue.HTTP = &netv1.HTTPIngressRuleValue{}
	}
	rule.IngressRuleValue.HTTP.Paths = append(rule.IngressRuleValue.HTTP.Paths, path)
}

// AddIngressTLS appends a TLS configuration to the Ingress spec.
func AddIngressTLS(ingress *netv1.Ingress, tls netv1.IngressTLS) {
	if ingress == nil {
		panic("AddIngressTLS: ingress must not be nil")
	}
	ingress.Spec.TLS = append(ingress.Spec.TLS, tls)
}

// SetIngressDefaultBackend sets the default backend on the Ingress spec.
func SetIngressDefaultBackend(ingress *netv1.Ingress, backend netv1.IngressBackend) {
	if ingress == nil {
		panic("SetIngressDefaultBackend: ingress must not be nil")
	}
	ingress.Spec.DefaultBackend = &backend
}

// SetIngressClassName sets the ingress class name on the Ingress spec.
func SetIngressClassName(ingress *netv1.Ingress, class string) {
	if ingress == nil {
		panic("SetIngressClassName: ingress must not be nil")
	}
	ingress.Spec.IngressClassName = &class
}
