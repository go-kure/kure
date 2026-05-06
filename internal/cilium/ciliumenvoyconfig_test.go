package cilium

import (
	"testing"

	ciliumv2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	slimv1 "github.com/cilium/cilium/pkg/k8s/slim/k8s/apis/meta/v1"
)

func TestCreateCiliumEnvoyConfig(t *testing.T) {
	obj := CreateCiliumEnvoyConfig("my-cec", "default")
	if obj == nil {
		t.Fatal("expected non-nil CiliumEnvoyConfig")
	}
	if obj.Name != "my-cec" {
		t.Errorf("expected Name 'my-cec', got %s", obj.Name)
	}
	if obj.Namespace != "default" {
		t.Errorf("expected Namespace 'default', got %s", obj.Namespace)
	}
	if obj.Kind != "CiliumEnvoyConfig" {
		t.Errorf("expected Kind 'CiliumEnvoyConfig', got %s", obj.Kind)
	}
	if obj.APIVersion != "cilium.io/v2" {
		t.Errorf("expected APIVersion 'cilium.io/v2', got %s", obj.APIVersion)
	}
}

func TestSetCiliumEnvoyConfigSpec(t *testing.T) {
	obj := CreateCiliumEnvoyConfig("p", "ns")
	spec := ciliumv2.CiliumEnvoyConfigSpec{
		Resources: []ciliumv2.XDSResource{{}},
	}
	SetCiliumEnvoyConfigSpec(obj, spec)
	if len(obj.Spec.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(obj.Spec.Resources))
	}
}

func TestAddCiliumEnvoyConfigService(t *testing.T) {
	obj := CreateCiliumEnvoyConfig("p", "ns")
	svc := &ciliumv2.ServiceListener{Name: "my-service", Namespace: "default"}
	AddCiliumEnvoyConfigService(obj, svc)
	AddCiliumEnvoyConfigService(obj, svc)
	if len(obj.Spec.Services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(obj.Spec.Services))
	}
}

func TestAddCiliumEnvoyConfigBackendService(t *testing.T) {
	obj := CreateCiliumEnvoyConfig("p", "ns")
	svc := &ciliumv2.Service{Name: "backend", Namespace: "default"}
	AddCiliumEnvoyConfigBackendService(obj, svc)
	if len(obj.Spec.BackendServices) != 1 {
		t.Fatalf("expected 1 backend service, got %d", len(obj.Spec.BackendServices))
	}
}

func TestAddCiliumEnvoyConfigResource(t *testing.T) {
	obj := CreateCiliumEnvoyConfig("p", "ns")
	AddCiliumEnvoyConfigResource(obj, ciliumv2.XDSResource{})
	if len(obj.Spec.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(obj.Spec.Resources))
	}
}

func TestSetCiliumEnvoyConfigNodeSelector(t *testing.T) {
	obj := CreateCiliumEnvoyConfig("p", "ns")
	sel := &slimv1.LabelSelector{
		MatchLabels: map[string]string{"node": "worker"},
	}
	SetCiliumEnvoyConfigNodeSelector(obj, sel)
	if obj.Spec.NodeSelector == nil {
		t.Fatal("expected non-nil NodeSelector")
	}
}

func TestCreateCiliumClusterwideEnvoyConfig(t *testing.T) {
	obj := CreateCiliumClusterwideEnvoyConfig("my-ccec")
	if obj == nil {
		t.Fatal("expected non-nil CiliumClusterwideEnvoyConfig")
	}
	if obj.Name != "my-ccec" {
		t.Errorf("expected Name 'my-ccec', got %s", obj.Name)
	}
	if obj.Namespace != "" {
		t.Errorf("expected empty Namespace for cluster-scoped resource, got %s", obj.Namespace)
	}
	if obj.Kind != "CiliumClusterwideEnvoyConfig" {
		t.Errorf("expected Kind 'CiliumClusterwideEnvoyConfig', got %s", obj.Kind)
	}
	if obj.APIVersion != "cilium.io/v2" {
		t.Errorf("expected APIVersion 'cilium.io/v2', got %s", obj.APIVersion)
	}
}

func TestSetCiliumClusterwideEnvoyConfigSpec(t *testing.T) {
	obj := CreateCiliumClusterwideEnvoyConfig("p")
	spec := ciliumv2.CiliumEnvoyConfigSpec{
		Resources: []ciliumv2.XDSResource{{}, {}},
	}
	SetCiliumClusterwideEnvoyConfigSpec(obj, spec)
	if len(obj.Spec.Resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(obj.Spec.Resources))
	}
}

func TestAddCiliumClusterwideEnvoyConfigService(t *testing.T) {
	obj := CreateCiliumClusterwideEnvoyConfig("p")
	svc := &ciliumv2.ServiceListener{Name: "svc", Namespace: "ns"}
	AddCiliumClusterwideEnvoyConfigService(obj, svc)
	if len(obj.Spec.Services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(obj.Spec.Services))
	}
}

func TestSetCiliumClusterwideEnvoyConfigNodeSelector(t *testing.T) {
	obj := CreateCiliumClusterwideEnvoyConfig("p")
	sel := &slimv1.LabelSelector{MatchLabels: map[string]string{"node": "worker"}}
	SetCiliumClusterwideEnvoyConfigNodeSelector(obj, sel)
	if obj.Spec.NodeSelector == nil {
		t.Fatal("expected non-nil NodeSelector")
	}
}
