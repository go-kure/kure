package kubernetes

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

// AddStatefulSetVolumeClaimTemplate appends a PVC template to the StatefulSet.
func AddStatefulSetVolumeClaimTemplate(sts *appsv1.StatefulSet, pvc corev1.PersistentVolumeClaim) {
	if sts == nil {
		panic("AddStatefulSetVolumeClaimTemplate: sts must not be nil")
	}
	sts.Spec.VolumeClaimTemplates = append(sts.Spec.VolumeClaimTemplates, pvc)
}

// SetStatefulSetReplicas sets the replica count.
func SetStatefulSetReplicas(sts *appsv1.StatefulSet, replicas int32) {
	if sts == nil {
		panic("SetStatefulSetReplicas: sts must not be nil")
	}
	if sts.Spec.Replicas == nil {
		sts.Spec.Replicas = new(int32)
	}
	*sts.Spec.Replicas = replicas
}

// SetStatefulSetRevisionHistoryLimit sets the revision history limit.
func SetStatefulSetRevisionHistoryLimit(sts *appsv1.StatefulSet, limit *int32) {
	if sts == nil {
		panic("SetStatefulSetRevisionHistoryLimit: sts must not be nil")
	}
	sts.Spec.RevisionHistoryLimit = limit
}
