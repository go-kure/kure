package kubernetes

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestCreateService(t *testing.T) {
	svc := CreateService("my-svc", "default")
	if svc.Name != "my-svc" || svc.Namespace != "default" {
		t.Fatalf("metadata mismatch: %s/%s", svc.Namespace, svc.Name)
	}
	if svc.Kind != "Service" {
		t.Errorf("unexpected kind %q", svc.Kind)
	}
	if svc.Labels["app"] != "my-svc" {
		t.Errorf("expected label app=my-svc, got %v", svc.Labels)
	}
	if len(svc.Spec.Ports) != 0 {
		t.Errorf("expected no ports, got %d", len(svc.Spec.Ports))
	}
}

func TestServiceNilErrors(t *testing.T) {
	// All Service functions now panic on nil receiver
	assertPanics(t, func() { AddServicePort(nil, corev1.ServicePort{}) })
	assertPanics(t, func() { SetServiceSelector(nil, map[string]string{}) })
	assertPanics(t, func() { SetServiceType(nil, corev1.ServiceTypeClusterIP) })
	assertPanics(t, func() { SetServiceClusterIP(nil, "10.0.0.1") })
	assertPanics(t, func() { AddServiceExternalIP(nil, "1.2.3.4") })
	assertPanics(t, func() { SetServiceExternalTrafficPolicy(nil, corev1.ServiceExternalTrafficPolicyLocal) })
	assertPanics(t, func() { SetServiceSessionAffinity(nil, corev1.ServiceAffinityClientIP) })
	assertPanics(t, func() { SetServiceLoadBalancerClass(nil, "x") })
	assertPanics(t, func() { AddServiceLabel(nil, "k", "v") })
	assertPanics(t, func() { AddServiceAnnotation(nil, "k", "v") })
	assertPanics(t, func() { SetServiceLabels(nil, nil) })
	assertPanics(t, func() { SetServiceAnnotations(nil, nil) })
	assertPanics(t, func() { SetServicePublishNotReadyAddresses(nil, true) })
	assertPanics(t, func() { AddServiceLoadBalancerSourceRange(nil, "10.0.0.0/24") })
	assertPanics(t, func() { SetServiceLoadBalancerSourceRanges(nil, nil) })
	assertPanics(t, func() { SetServiceIPFamilies(nil, nil) })
	assertPanics(t, func() { SetServiceIPFamilyPolicy(nil, nil) })
	assertPanics(t, func() { SetServiceInternalTrafficPolicy(nil, nil) })
	assertPanics(t, func() { SetServiceAllocateLoadBalancerNodePorts(nil, false) })
	assertPanics(t, func() { SetServiceExternalName(nil, "example.com") })
	assertPanics(t, func() { SetServiceHealthCheckNodePort(nil, 30000) })
	assertPanics(t, func() { SetServiceSessionAffinityConfig(nil, nil) })
}

func TestServiceFunctions(t *testing.T) {
	svc := CreateService("svc", "ns")
	if svc.Name != "svc" || svc.Namespace != "ns" {
		t.Fatalf("unexpected metadata: %s/%s", svc.Namespace, svc.Name)
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

	SetServiceSessionAffinity(svc, corev1.ServiceAffinityClientIP)
	if svc.Spec.SessionAffinity != corev1.ServiceAffinityClientIP {
		t.Errorf("session affinity not set")
	}

	SetServiceLoadBalancerClass(svc, "lb-class")
	if svc.Spec.LoadBalancerClass == nil || *svc.Spec.LoadBalancerClass != "lb-class" {
		t.Errorf("load balancer class not set")
	}

	SetServiceClusterIP(svc, "10.0.0.1")
	if svc.Spec.ClusterIP != "10.0.0.1" {
		t.Errorf("clusterIP not set")
	}

	AddServiceExternalIP(svc, "192.168.1.2")
	if len(svc.Spec.ExternalIPs) != 1 || svc.Spec.ExternalIPs[0] != "192.168.1.2" {
		t.Errorf("external IP not added")
	}

	SetServicePublishNotReadyAddresses(svc, true)
	if !svc.Spec.PublishNotReadyAddresses {
		t.Errorf("publish not ready addresses not set")
	}

	AddServiceLoadBalancerSourceRange(svc, "10.0.0.0/24")
	if len(svc.Spec.LoadBalancerSourceRanges) != 1 || svc.Spec.LoadBalancerSourceRanges[0] != "10.0.0.0/24" {
		t.Errorf("source range not added")
	}

	ranges := []string{"10.0.1.0/24", "10.0.2.0/24"}
	SetServiceLoadBalancerSourceRanges(svc, ranges)
	if !reflect.DeepEqual(svc.Spec.LoadBalancerSourceRanges, ranges) {
		t.Errorf("source ranges not set")
	}

	SetServiceIPFamilies(svc, []corev1.IPFamily{corev1.IPv4Protocol})
	if len(svc.Spec.IPFamilies) != 1 || svc.Spec.IPFamilies[0] != corev1.IPv4Protocol {
		t.Errorf("ip families not set")
	}

	policy := corev1.IPFamilyPolicyPreferDualStack
	SetServiceIPFamilyPolicy(svc, &policy)
	if svc.Spec.IPFamilyPolicy == nil || *svc.Spec.IPFamilyPolicy != policy {
		t.Errorf("ip family policy not set")
	}

	itp := corev1.ServiceInternalTrafficPolicyLocal
	SetServiceInternalTrafficPolicy(svc, &itp)
	if svc.Spec.InternalTrafficPolicy == nil || *svc.Spec.InternalTrafficPolicy != itp {
		t.Errorf("internal traffic policy not set")
	}

	SetServiceAllocateLoadBalancerNodePorts(svc, false)
	if svc.Spec.AllocateLoadBalancerNodePorts == nil || *svc.Spec.AllocateLoadBalancerNodePorts {
		t.Errorf("allocate LB node ports not set")
	}

	SetServiceExternalName(svc, "example.com")
	if svc.Spec.ExternalName != "example.com" {
		t.Errorf("external name not set")
	}

	SetServiceHealthCheckNodePort(svc, 30000)
	if svc.Spec.HealthCheckNodePort != 30000 {
		t.Errorf("health check node port not set")
	}

	cfg := &corev1.SessionAffinityConfig{ClientIP: &corev1.ClientIPConfig{TimeoutSeconds: new(int32)}}
	SetServiceSessionAffinityConfig(svc, cfg)
	if svc.Spec.SessionAffinityConfig != cfg {
		t.Errorf("session affinity config not set")
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

func TestAddServicePort_Success(t *testing.T) {
	svc := CreateService("test", "default")
	port := corev1.ServicePort{
		Name:       "http",
		Port:       80,
		TargetPort: intstr.FromInt(8080),
	}
	AddServicePort(svc, port)
	if len(svc.Spec.Ports) != 1 {
		t.Fatal("expected Port to be added")
	}
}
