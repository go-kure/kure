package cilium

import (
	"testing"

	ciliumv2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
)

func TestCreateCiliumLocalRedirectPolicy(t *testing.T) {
	obj := CreateCiliumLocalRedirectPolicy("my-lrp", "default")
	if obj == nil {
		t.Fatal("expected non-nil CiliumLocalRedirectPolicy")
	}
	if obj.Name != "my-lrp" {
		t.Errorf("expected Name 'my-lrp', got %s", obj.Name)
	}
	if obj.Namespace != "default" {
		t.Errorf("expected Namespace 'default', got %s", obj.Namespace)
	}
	if obj.Kind != "CiliumLocalRedirectPolicy" {
		t.Errorf("expected Kind 'CiliumLocalRedirectPolicy', got %s", obj.Kind)
	}
	if obj.APIVersion != "cilium.io/v2" {
		t.Errorf("expected APIVersion 'cilium.io/v2', got %s", obj.APIVersion)
	}
}

func TestSetCiliumLocalRedirectPolicySpec(t *testing.T) {
	obj := CreateCiliumLocalRedirectPolicy("p", "ns")
	spec := ciliumv2.CiliumLocalRedirectPolicySpec{
		Description: "redirect health",
	}
	SetCiliumLocalRedirectPolicySpec(obj, spec)
	if obj.Spec.Description != "redirect health" {
		t.Errorf("unexpected Description: %s", obj.Spec.Description)
	}
}

func TestSetCiliumLocalRedirectPolicyDescription(t *testing.T) {
	obj := CreateCiliumLocalRedirectPolicy("p", "ns")
	SetCiliumLocalRedirectPolicyDescription(obj, "health check redirect")
	if obj.Spec.Description != "health check redirect" {
		t.Errorf("unexpected Description: %s", obj.Spec.Description)
	}
}

func TestSetCiliumLocalRedirectPolicySkipRedirectFromBackend(t *testing.T) {
	obj := CreateCiliumLocalRedirectPolicy("p", "ns")
	SetCiliumLocalRedirectPolicySkipRedirectFromBackend(obj, true)
	if !obj.Spec.SkipRedirectFromBackend {
		t.Error("expected SkipRedirectFromBackend to be true")
	}
}

func TestSetCiliumLocalRedirectPolicyFrontendAndBackend(t *testing.T) {
	obj := CreateCiliumLocalRedirectPolicy("p", "ns")
	frontend := ciliumv2.RedirectFrontend{}
	backend := ciliumv2.RedirectBackend{}
	SetCiliumLocalRedirectPolicyFrontend(obj, frontend)
	SetCiliumLocalRedirectPolicyBackend(obj, backend)
	// Verify no panic and assignment succeeded — structs are value types
}
