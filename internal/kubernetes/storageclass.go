package kubernetes

import (
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateStorageClass returns a StorageClass object with sane defaults.
func CreateStorageClass(name string, provisioner string) *storagev1.StorageClass {
	policy := corev1.PersistentVolumeReclaimDelete
	binding := storagev1.VolumeBindingImmediate
	obj := &storagev1.StorageClass{
		TypeMeta: metav1.TypeMeta{
			Kind:       "StorageClass",
			APIVersion: storagev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"app": name,
			},
			Annotations: map[string]string{
				"app": name,
			},
		},
		Provisioner:          provisioner,
		Parameters:           map[string]string{},
		MountOptions:         []string{},
		ReclaimPolicy:        &policy,
		AllowVolumeExpansion: new(bool),
		VolumeBindingMode:    &binding,
		AllowedTopologies:    []corev1.TopologySelectorTerm{},
	}
	return obj
}

// AddStorageClassParameter inserts a single parameter into the StorageClass.
func AddStorageClassParameter(sc *storagev1.StorageClass, key, value string) {
	if sc.Parameters == nil {
		sc.Parameters = make(map[string]string)
	}
	sc.Parameters[key] = value
}

// AddStorageClassParameters merges the provided parameters into the StorageClass.
func AddStorageClassParameters(sc *storagev1.StorageClass, params map[string]string) {
	if sc.Parameters == nil {
		sc.Parameters = make(map[string]string)
	}
	for k, v := range params {
		sc.Parameters[k] = v
	}
}

// SetStorageClassParameters replaces the parameters map entirely.
func SetStorageClassParameters(sc *storagev1.StorageClass, params map[string]string) {
	sc.Parameters = params
}

// AddStorageClassMountOption appends a mount option to the StorageClass.
func AddStorageClassMountOption(sc *storagev1.StorageClass, option string) {
	sc.MountOptions = append(sc.MountOptions, option)
}

// AddStorageClassMountOptions appends multiple mount options to the StorageClass.
func AddStorageClassMountOptions(sc *storagev1.StorageClass, options []string) {
	sc.MountOptions = append(sc.MountOptions, options...)
}

// SetStorageClassMountOptions replaces all mount options.
func SetStorageClassMountOptions(sc *storagev1.StorageClass, options []string) {
	sc.MountOptions = options
}

// SetStorageClassProvisioner sets the provisioner field.
func SetStorageClassProvisioner(sc *storagev1.StorageClass, provisioner string) {
	sc.Provisioner = provisioner
}

// SetStorageClassReclaimPolicy sets the reclaim policy.
func SetStorageClassReclaimPolicy(sc *storagev1.StorageClass, policy corev1.PersistentVolumeReclaimPolicy) {
	sc.ReclaimPolicy = &policy
}

// SetStorageClassAllowVolumeExpansion sets the allowVolumeExpansion field.
func SetStorageClassAllowVolumeExpansion(sc *storagev1.StorageClass, allow bool) {
	if sc.AllowVolumeExpansion == nil {
		sc.AllowVolumeExpansion = new(bool)
	}
	*sc.AllowVolumeExpansion = allow
}

// SetStorageClassVolumeBindingMode sets the volume binding mode.
func SetStorageClassVolumeBindingMode(sc *storagev1.StorageClass, mode storagev1.VolumeBindingMode) {
	sc.VolumeBindingMode = &mode
}

// AddStorageClassAllowedTopology appends an allowed topology term.
func AddStorageClassAllowedTopology(sc *storagev1.StorageClass, topo corev1.TopologySelectorTerm) {
	sc.AllowedTopologies = append(sc.AllowedTopologies, topo)
}

// AddStorageClassAllowedTopologies appends multiple allowed topology terms.
func AddStorageClassAllowedTopologies(sc *storagev1.StorageClass, topos []corev1.TopologySelectorTerm) {
	sc.AllowedTopologies = append(sc.AllowedTopologies, topos...)
}

// SetStorageClassAllowedTopologies replaces the allowed topologies slice.
func SetStorageClassAllowedTopologies(sc *storagev1.StorageClass, topos []corev1.TopologySelectorTerm) {
	sc.AllowedTopologies = topos
}

// SetPVCStorageClass sets the StorageClass for the claim by name.
func SetPVCStorageClass(pvc *corev1.PersistentVolumeClaim, sc *storagev1.StorageClass) {
	if sc == nil {
		return
	}
	pvc.Spec.StorageClassName = &sc.Name
}
