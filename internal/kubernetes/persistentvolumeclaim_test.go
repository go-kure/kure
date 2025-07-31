package kubernetes

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCreatePersistentVolumeClaim(t *testing.T) {
	pvc := CreatePersistentVolumeClaim("data", "ns")

	if pvc.Name != "data" {
		t.Errorf("expected name data got %s", pvc.Name)
	}
	if pvc.Namespace != "ns" {
		t.Errorf("expected namespace ns got %s", pvc.Namespace)
	}
	if pvc.Kind != "PersistentVolumeClaim" {
		t.Errorf("unexpected kind %q", pvc.Kind)
	}
	if len(pvc.Spec.AccessModes) != 0 {
		t.Errorf("expected no access modes")
	}
	if pvc.Spec.VolumeMode == nil || *pvc.Spec.VolumeMode != corev1.PersistentVolumeFilesystem {
		t.Errorf("unexpected volume mode")
	}
	exp := resource.MustParse("1Gi")
	req := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
	if req.Cmp(exp) != 0 {
		t.Errorf("unexpected storage request %v", pvc.Spec.Resources.Requests)
	}
}

func TestPersistentVolumeClaimFunctions(t *testing.T) {
	pvc := CreatePersistentVolumeClaim("data", "ns")

	AddPVCAccessMode(pvc, corev1.ReadWriteOnce)
	if len(pvc.Spec.AccessModes) != 1 || pvc.Spec.AccessModes[0] != corev1.ReadWriteOnce {
		t.Errorf("access mode not added")
	}

	SetPVCStorageClassName(pvc, "fast")
	if pvc.Spec.StorageClassName == nil || *pvc.Spec.StorageClassName != "fast" {
		t.Errorf("storage class name not set")
	}

	SetPVCVolumeMode(pvc, corev1.PersistentVolumeBlock)
	if pvc.Spec.VolumeMode == nil || *pvc.Spec.VolumeMode != corev1.PersistentVolumeBlock {
		t.Errorf("volume mode not updated")
	}

	res := corev1.VolumeResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceStorage: resource.MustParse("2Gi"),
		},
	}
	SetPVCResources(pvc, res)
	req := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
	if req.Cmp(res.Requests[corev1.ResourceStorage]) != 0 {
		t.Errorf("resources not set")
	}

	sel := &metav1.LabelSelector{MatchLabels: map[string]string{"app": "data"}}
	SetPVCSelector(pvc, sel)
	if pvc.Spec.Selector == nil || !reflect.DeepEqual(pvc.Spec.Selector, sel) {
		t.Errorf("selector not set")
	}

	SetPVCVolumeName(pvc, "pv1")
	if pvc.Spec.VolumeName != "pv1" {
		t.Errorf("volume name not set")
	}

	ds := &corev1.TypedLocalObjectReference{Kind: "PersistentVolumeClaim", Name: "source"}
	SetPVCDataSource(pvc, ds)
	if pvc.Spec.DataSource != ds {
		t.Errorf("data source not set")
	}

	dor := &corev1.TypedObjectReference{Kind: "VolumeSnapshot", Name: "snap"}
	SetPVCDataSourceRef(pvc, dor)
	if pvc.Spec.DataSourceRef != dor {
		t.Errorf("data source ref not set")
	}
}
