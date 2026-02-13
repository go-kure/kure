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
	if err := AddServicePort(nil, corev1.ServicePort{}); err == nil {
		t.Error("expected error for nil Service on AddServicePort")
	}
	if err := SetServiceSelector(nil, map[string]string{}); err == nil {
		t.Error("expected error for nil Service on SetServiceSelector")
	}
	if err := SetServiceType(nil, corev1.ServiceTypeClusterIP); err == nil {
		t.Error("expected error for nil Service on SetServiceType")
	}
	if err := SetServiceClusterIP(nil, "10.0.0.1"); err == nil {
		t.Error("expected error for nil Service on SetServiceClusterIP")
	}
	if err := AddServiceExternalIP(nil, "1.2.3.4"); err == nil {
		t.Error("expected error for nil Service on AddServiceExternalIP")
	}
	if err := SetServiceLoadBalancerIP(nil, "1.2.3.4"); err == nil {
		t.Error("expected error for nil Service on SetServiceLoadBalancerIP")
	}
	if err := SetServiceExternalTrafficPolicy(nil, corev1.ServiceExternalTrafficPolicyLocal); err == nil {
		t.Error("expected error for nil Service on SetServiceExternalTrafficPolicy")
	}
	if err := SetServiceSessionAffinity(nil, corev1.ServiceAffinityClientIP); err == nil {
		t.Error("expected error for nil Service on SetServiceSessionAffinity")
	}
	if err := SetServiceLoadBalancerClass(nil, "x"); err == nil {
		t.Error("expected error for nil Service on SetServiceLoadBalancerClass")
	}
	if err := AddServiceLabel(nil, "k", "v"); err == nil {
		t.Error("expected error for nil Service on AddServiceLabel")
	}
	if err := AddServiceAnnotation(nil, "k", "v"); err == nil {
		t.Error("expected error for nil Service on AddServiceAnnotation")
	}
	if err := SetServiceLabels(nil, nil); err == nil {
		t.Error("expected error for nil Service on SetServiceLabels")
	}
	if err := SetServiceAnnotations(nil, nil); err == nil {
		t.Error("expected error for nil Service on SetServiceAnnotations")
	}
	if err := SetServicePublishNotReadyAddresses(nil, true); err == nil {
		t.Error("expected error for nil Service on SetServicePublishNotReadyAddresses")
	}
	if err := AddServiceLoadBalancerSourceRange(nil, "10.0.0.0/24"); err == nil {
		t.Error("expected error for nil Service on AddServiceLoadBalancerSourceRange")
	}
	if err := SetServiceLoadBalancerSourceRanges(nil, nil); err == nil {
		t.Error("expected error for nil Service on SetServiceLoadBalancerSourceRanges")
	}
	if err := SetServiceIPFamilies(nil, nil); err == nil {
		t.Error("expected error for nil Service on SetServiceIPFamilies")
	}
	if err := SetServiceIPFamilyPolicy(nil, nil); err == nil {
		t.Error("expected error for nil Service on SetServiceIPFamilyPolicy")
	}
	if err := SetServiceInternalTrafficPolicy(nil, nil); err == nil {
		t.Error("expected error for nil Service on SetServiceInternalTrafficPolicy")
	}
	if err := SetServiceAllocateLoadBalancerNodePorts(nil, false); err == nil {
		t.Error("expected error for nil Service on SetServiceAllocateLoadBalancerNodePorts")
	}
	if err := SetServiceExternalName(nil, "example.com"); err == nil {
		t.Error("expected error for nil Service on SetServiceExternalName")
	}
	if err := SetServiceHealthCheckNodePort(nil, 30000); err == nil {
		t.Error("expected error for nil Service on SetServiceHealthCheckNodePort")
	}
	if err := SetServiceSessionAffinityConfig(nil, nil); err == nil {
		t.Error("expected error for nil Service on SetServiceSessionAffinityConfig")
	}
}

func TestServiceFunctions(t *testing.T) {
	svc := CreateService("svc", "ns")
	if svc.Name != "svc" || svc.Namespace != "ns" {
		t.Fatalf("unexpected metadata: %s/%s", svc.Namespace, svc.Name)
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

	if err := SetServiceSessionAffinity(svc, corev1.ServiceAffinityClientIP); err != nil {
		t.Fatalf("SetServiceSessionAffinity returned error: %v", err)
	}
	if svc.Spec.SessionAffinity != corev1.ServiceAffinityClientIP {
		t.Errorf("session affinity not set")
	}

	if err := SetServiceLoadBalancerClass(svc, "lb-class"); err != nil {
		t.Fatalf("SetServiceLoadBalancerClass returned error: %v", err)
	}
	if svc.Spec.LoadBalancerClass == nil || *svc.Spec.LoadBalancerClass != "lb-class" {
		t.Errorf("load balancer class not set")
	}

	if err := SetServiceClusterIP(svc, "10.0.0.1"); err != nil {
		t.Fatalf("SetServiceClusterIP returned error: %v", err)
	}
	if svc.Spec.ClusterIP != "10.0.0.1" {
		t.Errorf("clusterIP not set")
	}

	if err := AddServiceExternalIP(svc, "192.168.1.2"); err != nil {
		t.Fatalf("AddServiceExternalIP returned error: %v", err)
	}
	if len(svc.Spec.ExternalIPs) != 1 || svc.Spec.ExternalIPs[0] != "192.168.1.2" {
		t.Errorf("external IP not added")
	}

	if err := SetServiceLoadBalancerIP(svc, "1.1.1.1"); err != nil {
		t.Fatalf("SetServiceLoadBalancerIP returned error: %v", err)
	}
	if svc.Spec.LoadBalancerIP != "1.1.1.1" {
		t.Errorf("loadBalancerIP not set")
	}

	if err := SetServicePublishNotReadyAddresses(svc, true); err != nil {
		t.Fatalf("SetServicePublishNotReadyAddresses returned error: %v", err)
	}
	if !svc.Spec.PublishNotReadyAddresses {
		t.Errorf("publish not ready addresses not set")
	}

	if err := AddServiceLoadBalancerSourceRange(svc, "10.0.0.0/24"); err != nil {
		t.Fatalf("AddServiceLoadBalancerSourceRange returned error: %v", err)
	}
	if len(svc.Spec.LoadBalancerSourceRanges) != 1 || svc.Spec.LoadBalancerSourceRanges[0] != "10.0.0.0/24" {
		t.Errorf("source range not added")
	}

	ranges := []string{"10.0.1.0/24", "10.0.2.0/24"}
	if err := SetServiceLoadBalancerSourceRanges(svc, ranges); err != nil {
		t.Fatalf("SetServiceLoadBalancerSourceRanges returned error: %v", err)
	}
	if !reflect.DeepEqual(svc.Spec.LoadBalancerSourceRanges, ranges) {
		t.Errorf("source ranges not set")
	}

	if err := SetServiceIPFamilies(svc, []corev1.IPFamily{corev1.IPv4Protocol}); err != nil {
		t.Fatalf("SetServiceIPFamilies returned error: %v", err)
	}
	if len(svc.Spec.IPFamilies) != 1 || svc.Spec.IPFamilies[0] != corev1.IPv4Protocol {
		t.Errorf("ip families not set")
	}

	policy := corev1.IPFamilyPolicyPreferDualStack
	if err := SetServiceIPFamilyPolicy(svc, &policy); err != nil {
		t.Fatalf("SetServiceIPFamilyPolicy returned error: %v", err)
	}
	if svc.Spec.IPFamilyPolicy == nil || *svc.Spec.IPFamilyPolicy != policy {
		t.Errorf("ip family policy not set")
	}

	itp := corev1.ServiceInternalTrafficPolicyLocal
	if err := SetServiceInternalTrafficPolicy(svc, &itp); err != nil {
		t.Fatalf("SetServiceInternalTrafficPolicy returned error: %v", err)
	}
	if svc.Spec.InternalTrafficPolicy == nil || *svc.Spec.InternalTrafficPolicy != itp {
		t.Errorf("internal traffic policy not set")
	}

	if err := SetServiceAllocateLoadBalancerNodePorts(svc, false); err != nil {
		t.Fatalf("SetServiceAllocateLoadBalancerNodePorts returned error: %v", err)
	}
	if svc.Spec.AllocateLoadBalancerNodePorts == nil || *svc.Spec.AllocateLoadBalancerNodePorts {
		t.Errorf("allocate LB node ports not set")
	}

	if err := SetServiceExternalName(svc, "example.com"); err != nil {
		t.Fatalf("SetServiceExternalName returned error: %v", err)
	}
	if svc.Spec.ExternalName != "example.com" {
		t.Errorf("external name not set")
	}

	if err := SetServiceHealthCheckNodePort(svc, 30000); err != nil {
		t.Fatalf("SetServiceHealthCheckNodePort returned error: %v", err)
	}
	if svc.Spec.HealthCheckNodePort != 30000 {
		t.Errorf("health check node port not set")
	}

	cfg := &corev1.SessionAffinityConfig{ClientIP: &corev1.ClientIPConfig{TimeoutSeconds: new(int32)}}
	if err := SetServiceSessionAffinityConfig(svc, cfg); err != nil {
		t.Fatalf("SetServiceSessionAffinityConfig returned error: %v", err)
	}
	if svc.Spec.SessionAffinityConfig != cfg {
		t.Errorf("session affinity config not set")
	}
}

func TestServiceMetadataFunctions(t *testing.T) {
	svc := CreateService("svc", "ns")

	if err := AddServiceLabel(svc, "k", "v"); err != nil {
		t.Fatalf("AddServiceLabel returned error: %v", err)
	}
	if svc.Labels["k"] != "v" {
		t.Errorf("label not added")
	}

	if err := AddServiceAnnotation(svc, "a", "b"); err != nil {
		t.Fatalf("AddServiceAnnotation returned error: %v", err)
	}
	if svc.Annotations["a"] != "b" {
		t.Errorf("annotation not added")
	}

	labels := map[string]string{"x": "y"}
	if err := SetServiceLabels(svc, labels); err != nil {
		t.Fatalf("SetServiceLabels returned error: %v", err)
	}
	if !reflect.DeepEqual(svc.Labels, labels) {
		t.Errorf("labels not set")
	}

	anns := map[string]string{"c": "d"}
	if err := SetServiceAnnotations(svc, anns); err != nil {
		t.Fatalf("SetServiceAnnotations returned error: %v", err)
	}
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
	if err := AddServicePort(svc, port); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(svc.Spec.Ports) != 1 {
		t.Fatal("expected Port to be added")
	}
}
