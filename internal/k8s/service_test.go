package k8s

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestServiceFunctions(t *testing.T) {
	svc := CreateService("svc", "ns")
	if svc.Name != "svc" || svc.Namespace != "ns" {
		t.Fatalf("unexpected metadata: %s/%s", svc.Namespace, svc.Name)
	}
	if svc.Kind != "Service" {
		t.Errorf("unexpected kind %q", svc.Kind)
	}
	if len(svc.Spec.Ports) != 0 {
		t.Errorf("expected no ports got %d", len(svc.Spec.Ports))
	}

	port := corev1.ServicePort{Name: "http", Port: 80}
	AddServicePort(svc, port)
	if len(svc.Spec.Ports) != 1 || svc.Spec.Ports[0] != port {
		t.Errorf("port not added correctly: %+v", svc.Spec.Ports)
	}

	selector := map[string]string{"app": "svc"}
	SetServiceSelector(svc, selector)
	if !reflect.DeepEqual(svc.Spec.Selector, selector) {
		t.Errorf("selector not set: %+v", svc.Spec.Selector)
	}

	SetServiceType(svc, corev1.ServiceTypeNodePort)
	if svc.Spec.Type != corev1.ServiceTypeNodePort {
		t.Errorf("service type not set")
	}

	SetServiceExternalTrafficPolicy(svc, corev1.ServiceExternalTrafficPolicyLocal)
	if svc.Spec.ExternalTrafficPolicy != corev1.ServiceExternalTrafficPolicyLocal {
		t.Errorf("external traffic policy not set")
	}
}

func TestServiceMetadataFunctions(t *testing.T) {
	svc := CreateService("svc", "ns")

	AddServiceLabel(svc, "k", "v")
	if svc.Labels["k"] != "v" {
		t.Errorf("label not added")
	}

	AddServiceAnnotation(svc, "a", "b")
	if svc.Annotations["a"] != "b" {
		t.Errorf("annotation not added")
	}

	labels := map[string]string{"x": "y"}
	SetServiceLabels(svc, labels)
	if !reflect.DeepEqual(svc.Labels, labels) {
		t.Errorf("labels not set")
	}

	anns := map[string]string{"c": "d"}
	SetServiceAnnotations(svc, anns)
	if !reflect.DeepEqual(svc.Annotations, anns) {
		t.Errorf("annotations not set")
	}
}
