package kubernetes

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// CreateResourceRequirements returns a ResourceRequirements initialized with
// empty Requests and Limits maps.
func CreateResourceRequirements() *corev1.ResourceRequirements {
	return &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{},
		Limits:   corev1.ResourceList{},
	}
}

// SetResourceRequest sets a named resource request. Parse the quantity at the
// call site (resource.MustParse("100m") for a literal, resource.ParseQuantity
// when the text comes from configuration).
func SetResourceRequest(rr *corev1.ResourceRequirements, name corev1.ResourceName, qty resource.Quantity) {
	if rr == nil {
		panic("SetResourceRequest: rr must not be nil")
	}
	if rr.Requests == nil {
		rr.Requests = corev1.ResourceList{}
	}
	rr.Requests[name] = qty
}

// SetResourceLimit sets a named resource limit. Parse the quantity at the call
// site, as for SetResourceRequest.
func SetResourceLimit(rr *corev1.ResourceRequirements, name corev1.ResourceName, qty resource.Quantity) {
	if rr == nil {
		panic("SetResourceLimit: rr must not be nil")
	}
	if rr.Limits == nil {
		rr.Limits = corev1.ResourceList{}
	}
	rr.Limits[name] = qty
}

// AddResourceClaim appends a ResourceClaim to the ResourceRequirements.
func AddResourceClaim(rr *corev1.ResourceRequirements, claim corev1.ResourceClaim) {
	if rr == nil {
		panic("AddResourceClaim: rr must not be nil")
	}
	rr.Claims = append(rr.Claims, claim)
}
