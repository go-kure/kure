package kubernetes

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestServiceNilErrors(t *testing.T) {
	// All Service functions now panic on nil receiver
	assertPanics(t, func() { AddServicePort(nil, corev1.ServicePort{}) })
	assertPanics(t, func() { AddServiceExternalIP(nil, "1.2.3.4") })
	assertPanics(t, func() { SetServiceLoadBalancerClass(nil, "x") })
	assertPanics(t, func() { AddServiceLoadBalancerSourceRange(nil, "10.0.0.0/24") })
	assertPanics(t, func() { SetServiceIPFamilyPolicy(nil, nil) })
	assertPanics(t, func() { SetServiceInternalTrafficPolicy(nil, nil) })
	assertPanics(t, func() { SetServiceAllocateLoadBalancerNodePorts(nil, false) })
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

	SetServiceLoadBalancerClass(svc, "lb-class")
	if svc.Spec.LoadBalancerClass == nil || *svc.Spec.LoadBalancerClass != "lb-class" {
		t.Errorf("load balancer class not set")
	}

	AddServiceExternalIP(svc, "192.168.1.2")
	if len(svc.Spec.ExternalIPs) != 1 || svc.Spec.ExternalIPs[0] != "192.168.1.2" {
		t.Errorf("external IP not added")
	}

	AddServiceLoadBalancerSourceRange(svc, "10.0.0.0/24")
	if len(svc.Spec.LoadBalancerSourceRanges) != 1 || svc.Spec.LoadBalancerSourceRanges[0] != "10.0.0.0/24" {
		t.Errorf("source range not added")
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

	cfg := &corev1.SessionAffinityConfig{ClientIP: &corev1.ClientIPConfig{TimeoutSeconds: new(int32)}}
	SetServiceSessionAffinityConfig(svc, cfg)
	if svc.Spec.SessionAffinityConfig != cfg {
		t.Errorf("session affinity config not set")
	}
}

// TestServiceMetadataViaGenericHelpers covers what AddServiceLabel and
// AddServiceAnnotation used to do, including the nil-map init path on a Service
// not built by CreateService.
func TestServiceMetadataViaGenericHelpers(t *testing.T) {
	svc := CreateService("svc", "ns")
	AddLabel(svc, "k", "v")
	AddAnnotation(svc, "a", "b")
	if svc.Labels["k"] != "v" {
		t.Errorf("label not added: %v", svc.Labels)
	}
	if svc.Annotations["a"] != "b" {
		t.Errorf("annotation not added: %v", svc.Annotations)
	}

	bare := &corev1.Service{}
	AddLabel(bare, "env", "prod")
	AddAnnotation(bare, "note", "test")
	if bare.Labels["env"] != "prod" {
		t.Errorf("expected label env=prod, got %v", bare.Labels)
	}
	if bare.Annotations["note"] != "test" {
		t.Errorf("expected annotation note=test, got %v", bare.Annotations)
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
