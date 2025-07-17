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
	if err := AddServicePort(svc, port); err != nil {
		t.Fatalf("AddServicePort returned error: %v", err)
	}
	if len(svc.Spec.Ports) != 1 || svc.Spec.Ports[0] != port {
		t.Errorf("port not added correctly: %+v", svc.Spec.Ports)
	}

	selector := map[string]string{"app": "svc"}
	if err := SetServiceSelector(svc, selector); err != nil {
		t.Fatalf("SetServiceSelector returned error: %v", err)
	}
	if !reflect.DeepEqual(svc.Spec.Selector, selector) {
		t.Errorf("selector not set: %+v", svc.Spec.Selector)
	}

	if err := SetServiceType(svc, corev1.ServiceTypeNodePort); err != nil {
		t.Fatalf("SetServiceType returned error: %v", err)
	}
	if svc.Spec.Type != corev1.ServiceTypeNodePort {
		t.Errorf("service type not set")
	}

	if err := SetServiceExternalTrafficPolicy(svc, corev1.ServiceExternalTrafficPolicyLocal); err != nil {
		t.Fatalf("SetServiceExternalTrafficPolicy returned error: %v", err)
	}
	if svc.Spec.ExternalTrafficPolicy != corev1.ServiceExternalTrafficPolicyLocal {
		t.Errorf("external traffic policy not set")
	}

	SetServiceSessionAffinity(svc, corev1.ServiceAffinityClientIP)
	if svc.Spec.SessionAffinity != corev1.ServiceAffinityClientIP {
		t.Errorf("session affinity not set")
	}

	SetServiceLoadBalancerClass(svc, "lb-class")
	if svc.Spec.LoadBalancerClass == nil || *svc.Spec.LoadBalancerClass != "lb-class" {
		t.Errorf("load balancer class not set")
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
