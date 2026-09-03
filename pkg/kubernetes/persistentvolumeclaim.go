package kubernetes

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AddPVCAccessMode appends an access mode to the claim.
func AddPVCAccessMode(pvc *corev1.PersistentVolumeClaim, mode corev1.PersistentVolumeAccessMode) {
	pvc.Spec.AccessModes = append(pvc.Spec.AccessModes, mode)
}

// SetPVCStorageClassName sets the storage class name for the claim.
func SetPVCStorageClassName(pvc *corev1.PersistentVolumeClaim, class string) {
	pvc.Spec.StorageClassName = &class
}

// SetPVCVolumeMode sets the volume mode of the claim.
func SetPVCVolumeMode(pvc *corev1.PersistentVolumeClaim, mode corev1.PersistentVolumeMode) {
	pvc.Spec.VolumeMode = &mode
}

// SetPVCSelector sets the selector for the claim.
func SetPVCSelector(pvc *corev1.PersistentVolumeClaim, selector *metav1.LabelSelector) {
	pvc.Spec.Selector = selector
}

// SetPVCDataSource sets the data source for the claim.
func SetPVCDataSource(pvc *corev1.PersistentVolumeClaim, src *corev1.TypedLocalObjectReference) {
	pvc.Spec.DataSource = src
}

// SetPVCDataSourceRef sets the data source reference for the claim.
func SetPVCDataSourceRef(pvc *corev1.PersistentVolumeClaim, src *corev1.TypedObjectReference) {
	pvc.Spec.DataSourceRef = src
}

// VolumeClaimTemplateOptions holds fields needed to construct a PVC for
// embedding in StatefulSet.Spec.VolumeClaimTemplates.
type VolumeClaimTemplateOptions struct {
	// StorageClassName is the name of the StorageClass. When empty, the cluster
	// default StorageClass is used.
	StorageClassName string
	// AccessModes defines the desired access modes for the volume.
	AccessModes []corev1.PersistentVolumeAccessMode
	// StorageRequest is the minimum storage capacity requested.
	StorageRequest resource.Quantity
	// Labels are optional metadata labels applied to the PVC template.
	Labels map[string]string
}
