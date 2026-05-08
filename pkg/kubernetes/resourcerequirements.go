package kubernetes

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/go-kure/kure/pkg/errors"
)

// CreateResourceRequirements returns a ResourceRequirements initialized with
// empty Requests and Limits maps.
func CreateResourceRequirements() *corev1.ResourceRequirements {
	return &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{},
		Limits:   corev1.ResourceList{},
	}
}

// SetResourceRequestCPU sets the CPU request on the ResourceRequirements.
// The value is parsed as a Kubernetes resource.Quantity (e.g. "100m", "0.5", "2").
func SetResourceRequestCPU(rr *corev1.ResourceRequirements, value string) error {
	return setResourceQuantity(rr, corev1.ResourceCPU, value, true)
}

// SetResourceRequestMemory sets the memory request on the ResourceRequirements.
// The value is parsed as a Kubernetes resource.Quantity (e.g. "128Mi", "1Gi").
func SetResourceRequestMemory(rr *corev1.ResourceRequirements, value string) error {
	return setResourceQuantity(rr, corev1.ResourceMemory, value, true)
}

// SetResourceRequestEphemeralStorage sets the ephemeral storage request.
func SetResourceRequestEphemeralStorage(rr *corev1.ResourceRequirements, value string) error {
	return setResourceQuantity(rr, corev1.ResourceEphemeralStorage, value, true)
}

// SetResourceLimitCPU sets the CPU limit on the ResourceRequirements.
func SetResourceLimitCPU(rr *corev1.ResourceRequirements, value string) error {
	return setResourceQuantity(rr, corev1.ResourceCPU, value, false)
}

// SetResourceLimitMemory sets the memory limit on the ResourceRequirements.
func SetResourceLimitMemory(rr *corev1.ResourceRequirements, value string) error {
	return setResourceQuantity(rr, corev1.ResourceMemory, value, false)
}

// SetResourceLimitEphemeralStorage sets the ephemeral storage limit.
func SetResourceLimitEphemeralStorage(rr *corev1.ResourceRequirements, value string) error {
	return setResourceQuantity(rr, corev1.ResourceEphemeralStorage, value, false)
}

// SetResourceRequest sets a named resource request.
func SetResourceRequest(rr *corev1.ResourceRequirements, name corev1.ResourceName, value string) error {
	return setResourceQuantity(rr, name, value, true)
}

// SetResourceLimit sets a named resource limit.
func SetResourceLimit(rr *corev1.ResourceRequirements, name corev1.ResourceName, value string) error {
	return setResourceQuantity(rr, name, value, false)
}

// AddResourceClaim appends a ResourceClaim to the ResourceRequirements.
func AddResourceClaim(rr *corev1.ResourceRequirements, claim corev1.ResourceClaim) {
	if rr == nil {
		panic("AddResourceClaim: rr must not be nil")
	}
	rr.Claims = append(rr.Claims, claim)
}

func setResourceQuantity(rr *corev1.ResourceRequirements, name corev1.ResourceName, value string, isRequest bool) error {
	if rr == nil {
		return errors.ErrNilResourceRequirements
	}
	qty, err := resource.ParseQuantity(value)
	if err != nil {
		return errors.Wrapf(err, "invalid quantity %q for resource %s", value, name)
	}
	if isRequest {
		if rr.Requests == nil {
			rr.Requests = corev1.ResourceList{}
		}
		rr.Requests[name] = qty
	} else {
		if rr.Limits == nil {
			rr.Limits = corev1.ResourceList{}
		}
		rr.Limits[name] = qty
	}
	return nil
}
