package kubernetes

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreatePersistentVolumeClaim returns a PersistentVolumeClaim object with sane defaults.
func CreatePersistentVolumeClaim(name string, namespace string) *corev1.PersistentVolumeClaim {
	mode := corev1.PersistentVolumeFilesystem
	obj := &corev1.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PersistentVolumeClaim",
			APIVersion: corev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"app": name,
			},
			Annotations: map[string]string{
				"app": name,
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
			VolumeMode: &mode,
		},
	}
	return obj
}

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

// SetPVCResources sets the resource requirements for the claim.
func SetPVCResources(pvc *corev1.PersistentVolumeClaim, resources corev1.VolumeResourceRequirements) {
	pvc.Spec.Resources = resources
}

// SetPVCSelector sets the selector for the claim.
func SetPVCSelector(pvc *corev1.PersistentVolumeClaim, selector *metav1.LabelSelector) {
	pvc.Spec.Selector = selector
}

// SetPVCVolumeName sets the bound volume name for the claim.
func SetPVCVolumeName(pvc *corev1.PersistentVolumeClaim, volumeName string) {
	pvc.Spec.VolumeName = volumeName
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

// CreateVolumeClaimTemplate returns a PersistentVolumeClaim suitable for
// embedding in StatefulSet.Spec.VolumeClaimTemplates. Only ObjectMeta.Name and
// Spec are set; TypeMeta and Namespace are intentionally omitted because
// StatefulSet embeds PVCs by name only and the owning StatefulSet provides the
// namespace.
func CreateVolumeClaimTemplate(name string, opts VolumeClaimTemplateOptions) corev1.PersistentVolumeClaim {
	pvc := corev1.PersistentVolumeClaim{}
	pvc.Name = name
	if opts.Labels != nil {
		pvc.Labels = opts.Labels
	}
	pvc.Spec = corev1.PersistentVolumeClaimSpec{
		AccessModes: opts.AccessModes,
		Resources: corev1.VolumeResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceStorage: opts.StorageRequest,
			},
		},
	}
	if opts.StorageClassName != "" {
		sc := opts.StorageClassName
		pvc.Spec.StorageClassName = &sc
	}
	return pvc
}
