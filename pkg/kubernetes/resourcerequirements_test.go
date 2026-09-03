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

func TestSetResourceRequest(t *testing.T) {
	rr := CreateResourceRequirements()

	SetResourceRequest(rr, corev1.ResourceCPU, resource.MustParse("100m"))
	if !rr.Requests.Cpu().Equal(resource.MustParse("100m")) {
		t.Errorf("expected CPU request 100m, got %s", rr.Requests.Cpu())
	}

	SetResourceRequest(rr, corev1.ResourceMemory, resource.MustParse("256Mi"))
	if !rr.Requests.Memory().Equal(resource.MustParse("256Mi")) {
		t.Errorf("expected memory request 256Mi, got %s", rr.Requests.Memory())
	}

	SetResourceRequest(rr, corev1.ResourceEphemeralStorage, resource.MustParse("10Gi"))
	got := rr.Requests[corev1.ResourceEphemeralStorage]
	if !got.Equal(resource.MustParse("10Gi")) {
		t.Errorf("expected ephemeral storage request 10Gi, got %s", got.String())
	}

	SetResourceRequest(rr, "nvidia.com/gpu", resource.MustParse("1"))
	gpu := rr.Requests[corev1.ResourceName("nvidia.com/gpu")]
	if !gpu.Equal(resource.MustParse("1")) {
		t.Errorf("expected gpu request 1, got %s", gpu.String())
	}
}

func TestSetResourceLimit(t *testing.T) {
	rr := CreateResourceRequirements()

	SetResourceLimit(rr, corev1.ResourceCPU, resource.MustParse("500m"))
	if !rr.Limits.Cpu().Equal(resource.MustParse("500m")) {
		t.Errorf("expected CPU limit 500m, got %s", rr.Limits.Cpu())
	}

	SetResourceLimit(rr, corev1.ResourceMemory, resource.MustParse("1Gi"))
	if !rr.Limits.Memory().Equal(resource.MustParse("1Gi")) {
		t.Errorf("expected memory limit 1Gi, got %s", rr.Limits.Memory())
	}

	SetResourceLimit(rr, corev1.ResourceEphemeralStorage, resource.MustParse("20Gi"))
	got := rr.Limits[corev1.ResourceEphemeralStorage]
	if !got.Equal(resource.MustParse("20Gi")) {
		t.Errorf("expected ephemeral storage limit 20Gi, got %s", got.String())
	}

	SetResourceLimit(rr, "nvidia.com/gpu", resource.MustParse("2"))
	gpu := rr.Limits[corev1.ResourceName("nvidia.com/gpu")]
	if !gpu.Equal(resource.MustParse("2")) {
		t.Errorf("expected gpu limit 2, got %s", gpu.String())
	}
}

// TestSetResourceOverwrites covers a second write to the same resource name:
// the map insert replaces the entry rather than accumulating.
func TestSetResourceOverwrites(t *testing.T) {
	rr := CreateResourceRequirements()
	SetResourceRequest(rr, corev1.ResourceCPU, resource.MustParse("100m"))
	SetResourceRequest(rr, corev1.ResourceCPU, resource.MustParse("200m"))
	if len(rr.Requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(rr.Requests))
	}
	if !rr.Requests.Cpu().Equal(resource.MustParse("200m")) {
		t.Errorf("expected CPU request 200m, got %s", rr.Requests.Cpu())
	}
}

func TestAddResourceClaim(t *testing.T) {
	rr := CreateResourceRequirements()
	claim := corev1.ResourceClaim{Name: "my-gpu-claim"}
	AddResourceClaim(rr, claim)
	if len(rr.Claims) != 1 {
		t.Fatalf("expected 1 claim, got %d", len(rr.Claims))
	}
	if rr.Claims[0].Name != "my-gpu-claim" {
		t.Errorf("expected claim name my-gpu-claim, got %s", rr.Claims[0].Name)
	}
}

func TestResourceRequirementsNilErrors(t *testing.T) {
	assertPanics(t, func() { SetResourceRequest(nil, corev1.ResourceCPU, resource.MustParse("100m")) })
	assertPanics(t, func() { SetResourceLimit(nil, corev1.ResourceMemory, resource.MustParse("256Mi")) })
	assertPanics(t, func() { AddResourceClaim(nil, corev1.ResourceClaim{Name: "test"}) })
}

// TestSetResource_NilMaps exercises the Requests==nil and Limits==nil init
// paths when rr was constructed without CreateResourceRequirements.
func TestSetResource_NilMaps(t *testing.T) {
	rr := &corev1.ResourceRequirements{}
	SetResourceRequest(rr, corev1.ResourceCPU, resource.MustParse("100m"))
	if rr.Requests == nil {
		t.Error("expected Requests to be initialized")
	}

	rr2 := &corev1.ResourceRequirements{}
	SetResourceLimit(rr2, corev1.ResourceCPU, resource.MustParse("500m"))
	if rr2.Limits == nil {
		t.Error("expected Limits to be initialized")
	}
}

func TestResourceRequirementsMultipleValues(t *testing.T) {
	rr := CreateResourceRequirements()
	SetResourceRequest(rr, corev1.ResourceCPU, resource.MustParse("100m"))
	SetResourceRequest(rr, corev1.ResourceMemory, resource.MustParse("256Mi"))
	SetResourceLimit(rr, corev1.ResourceCPU, resource.MustParse("500m"))
	SetResourceLimit(rr, corev1.ResourceMemory, resource.MustParse("1Gi"))

	if len(rr.Requests) != 2 {
		t.Errorf("expected 2 requests, got %d", len(rr.Requests))
	}
	if len(rr.Limits) != 2 {
		t.Errorf("expected 2 limits, got %d", len(rr.Limits))
	}
}
