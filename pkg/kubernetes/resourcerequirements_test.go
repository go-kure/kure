package kubernetes

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestCreateResourceRequirements(t *testing.T) {
	rr := CreateResourceRequirements()
	if rr == nil {
		t.Fatal("expected non-nil ResourceRequirements")
	}
	if rr.Requests == nil {
		t.Error("expected non-nil Requests")
	}
	if rr.Limits == nil {
		t.Error("expected non-nil Limits")
	}
}

func TestSetResourceRequestCPU(t *testing.T) {
	rr := CreateResourceRequirements()
	if err := SetResourceRequestCPU(rr, "100m"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := resource.MustParse("100m")
	if !rr.Requests.Cpu().Equal(expected) {
		t.Errorf("expected CPU request 100m, got %s", rr.Requests.Cpu())
	}
}

func TestSetResourceRequestMemory(t *testing.T) {
	rr := CreateResourceRequirements()
	if err := SetResourceRequestMemory(rr, "256Mi"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := resource.MustParse("256Mi")
	if !rr.Requests.Memory().Equal(expected) {
		t.Errorf("expected memory request 256Mi, got %s", rr.Requests.Memory())
	}
}

func TestSetResourceLimitCPU(t *testing.T) {
	rr := CreateResourceRequirements()
	if err := SetResourceLimitCPU(rr, "500m"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := resource.MustParse("500m")
	if !rr.Limits.Cpu().Equal(expected) {
		t.Errorf("expected CPU limit 500m, got %s", rr.Limits.Cpu())
	}
}

func TestSetResourceLimitMemory(t *testing.T) {
	rr := CreateResourceRequirements()
	if err := SetResourceLimitMemory(rr, "1Gi"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := resource.MustParse("1Gi")
	if !rr.Limits.Memory().Equal(expected) {
		t.Errorf("expected memory limit 1Gi, got %s", rr.Limits.Memory())
	}
}

func TestSetResourceRequestEphemeralStorage(t *testing.T) {
	rr := CreateResourceRequirements()
	if err := SetResourceRequestEphemeralStorage(rr, "10Gi"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := rr.Requests[corev1.ResourceEphemeralStorage]
	expected := resource.MustParse("10Gi")
	if !got.Equal(expected) {
		t.Errorf("expected ephemeral storage request 10Gi, got %s", got.String())
	}
}

func TestSetResourceLimitEphemeralStorage(t *testing.T) {
	rr := CreateResourceRequirements()
	if err := SetResourceLimitEphemeralStorage(rr, "20Gi"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := rr.Limits[corev1.ResourceEphemeralStorage]
	expected := resource.MustParse("20Gi")
	if !got.Equal(expected) {
		t.Errorf("expected ephemeral storage limit 20Gi, got %s", got.String())
	}
}

func TestSetResourceRequest(t *testing.T) {
	rr := CreateResourceRequirements()
	if err := SetResourceRequest(rr, "nvidia.com/gpu", "1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := rr.Requests[corev1.ResourceName("nvidia.com/gpu")]
	expected := resource.MustParse("1")
	if !got.Equal(expected) {
		t.Errorf("expected gpu request 1, got %s", got.String())
	}
}

func TestSetResourceLimit(t *testing.T) {
	rr := CreateResourceRequirements()
	if err := SetResourceLimit(rr, "nvidia.com/gpu", "2"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := rr.Limits[corev1.ResourceName("nvidia.com/gpu")]
	expected := resource.MustParse("2")
	if !got.Equal(expected) {
		t.Errorf("expected gpu limit 2, got %s", got.String())
	}
}

func TestAddResourceClaim(t *testing.T) {
	rr := CreateResourceRequirements()
	claim := corev1.ResourceClaim{Name: "my-gpu-claim"}
	if err := AddResourceClaim(rr, claim); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rr.Claims) != 1 {
		t.Fatalf("expected 1 claim, got %d", len(rr.Claims))
	}
	if rr.Claims[0].Name != "my-gpu-claim" {
		t.Errorf("expected claim name my-gpu-claim, got %s", rr.Claims[0].Name)
	}
}

func TestResourceRequirementsNilErrors(t *testing.T) {
	if err := SetResourceRequestCPU(nil, "100m"); err == nil {
		t.Error("expected error for nil ResourceRequirements")
	}
	if err := SetResourceLimitMemory(nil, "256Mi"); err == nil {
		t.Error("expected error for nil ResourceRequirements")
	}
	if err := AddResourceClaim(nil, corev1.ResourceClaim{Name: "test"}); err == nil {
		t.Error("expected error for nil ResourceRequirements")
	}
}

func TestResourceRequirementsInvalidQuantity(t *testing.T) {
	rr := CreateResourceRequirements()
	if err := SetResourceRequestCPU(rr, "not-a-quantity"); err == nil {
		t.Error("expected error for invalid quantity")
	}
}

func TestResourceRequirementsMultipleValues(t *testing.T) {
	rr := CreateResourceRequirements()
	if err := SetResourceRequestCPU(rr, "100m"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := SetResourceRequestMemory(rr, "256Mi"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := SetResourceLimitCPU(rr, "500m"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := SetResourceLimitMemory(rr, "1Gi"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(rr.Requests) != 2 {
		t.Errorf("expected 2 requests, got %d", len(rr.Requests))
	}
	if len(rr.Limits) != 2 {
		t.Errorf("expected 2 limits, got %d", len(rr.Limits))
	}
}
