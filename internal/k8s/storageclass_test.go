package k8s

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
)

func TestCreateStorageClass(t *testing.T) {
	sc := CreateStorageClass("standard", "kubernetes.io/no-provisioner")

	if sc.Name != "standard" {
		t.Errorf("expected name standard got %s", sc.Name)
	}
	if sc.Provisioner != "kubernetes.io/no-provisioner" {
		t.Errorf("unexpected provisioner %s", sc.Provisioner)
	}
	if sc.Kind != "StorageClass" {
		t.Errorf("unexpected kind %q", sc.Kind)
	}
	if sc.ReclaimPolicy == nil || *sc.ReclaimPolicy != corev1.PersistentVolumeReclaimDelete {
		t.Errorf("unexpected reclaim policy")
	}
	if sc.AllowVolumeExpansion == nil {
		t.Errorf("expected allowVolumeExpansion pointer set")
	}
	if sc.VolumeBindingMode == nil || *sc.VolumeBindingMode != storagev1.VolumeBindingImmediate {
		t.Errorf("unexpected volume binding mode")
	}
	if len(sc.Parameters) != 0 {
		t.Errorf("expected no parameters")
	}
	if len(sc.MountOptions) != 0 {
		t.Errorf("expected no mount options")
	}
	if len(sc.AllowedTopologies) != 0 {
		t.Errorf("expected no allowed topologies")
	}
}

func TestStorageClassFunctions(t *testing.T) {
	sc := CreateStorageClass("standard", "prov")

	AddStorageClassParameter(sc, "fstype", "xfs")
	if sc.Parameters["fstype"] != "xfs" {
		t.Errorf("parameter not added")
	}

	moreParams := map[string]string{"a": "b"}
	AddStorageClassParameters(sc, moreParams)
	for k, v := range moreParams {
		if sc.Parameters[k] != v {
			t.Errorf("parameter %s not merged", k)
		}
	}

	SetStorageClassParameters(sc, map[string]string{"x": "y"})
	if !reflect.DeepEqual(sc.Parameters, map[string]string{"x": "y"}) {
		t.Errorf("parameters not set")
	}

	AddStorageClassMountOption(sc, "ro")
	if sc.MountOptions[0] != "ro" {
		t.Errorf("mount option not added")
	}

	AddStorageClassMountOptions(sc, []string{"sync"})
	if !reflect.DeepEqual(sc.MountOptions, []string{"ro", "sync"}) {
		t.Errorf("mount options not appended")
	}

	SetStorageClassMountOptions(sc, []string{"rw"})
	if !reflect.DeepEqual(sc.MountOptions, []string{"rw"}) {
		t.Errorf("mount options not set")
	}

	SetStorageClassProvisioner(sc, "other")
	if sc.Provisioner != "other" {
		t.Errorf("provisioner not set")
	}

	SetStorageClassReclaimPolicy(sc, corev1.PersistentVolumeReclaimRetain)
	if sc.ReclaimPolicy == nil || *sc.ReclaimPolicy != corev1.PersistentVolumeReclaimRetain {
		t.Errorf("reclaim policy not set")
	}

	SetStorageClassAllowVolumeExpansion(sc, true)
	if sc.AllowVolumeExpansion == nil || !*sc.AllowVolumeExpansion {
		t.Errorf("allow expansion not set")
	}

	SetStorageClassVolumeBindingMode(sc, storagev1.VolumeBindingWaitForFirstConsumer)
	if sc.VolumeBindingMode == nil || *sc.VolumeBindingMode != storagev1.VolumeBindingWaitForFirstConsumer {
		t.Errorf("binding mode not set")
	}

	topo := corev1.TopologySelectorTerm{MatchLabelExpressions: []corev1.TopologySelectorLabelRequirement{{Key: "k", Values: []string{"v"}}}}
	AddStorageClassAllowedTopology(sc, topo)
	if len(sc.AllowedTopologies) != 1 {
		t.Errorf("topology not added")
	}

	AddStorageClassAllowedTopologies(sc, []corev1.TopologySelectorTerm{topo})
	if len(sc.AllowedTopologies) != 2 {
		t.Errorf("topologies not appended")
	}

	SetStorageClassAllowedTopologies(sc, []corev1.TopologySelectorTerm{})
	if len(sc.AllowedTopologies) != 0 {
		t.Errorf("topologies not set")
	}

	pvc := CreatePersistentVolumeClaim("pvc", "ns")
	SetPVCStorageClass(pvc, sc)
	if pvc.Spec.StorageClassName == nil || *pvc.Spec.StorageClassName != sc.Name {
		t.Errorf("pvc storage class not set")
	}
}
