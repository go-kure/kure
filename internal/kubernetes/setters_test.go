package kubernetes

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
)

// ConfigMap setter tests
func TestAddConfigMapData_Success(t *testing.T) {
	cm := CreateConfigMap("test", "default")
	AddConfigMapData(cm, "key", "value")
	if cm.Data["key"] != "value" {
		t.Fatal("expected Data to be set")
	}
}

func TestAddConfigMapDataMap_Success(t *testing.T) {
	cm := CreateConfigMap("test", "default")
	data := map[string]string{"key1": "value1", "key2": "value2"}
	AddConfigMapDataMap(cm, data)
	if len(cm.Data) != 2 {
		t.Fatal("expected Data to be set")
	}
}

func TestAddConfigMapBinaryData_Success(t *testing.T) {
	cm := CreateConfigMap("test", "default")
	AddConfigMapBinaryData(cm, "key", []byte("value"))
	if string(cm.BinaryData["key"]) != "value" {
		t.Fatal("expected BinaryData to be set")
	}
}

func TestAddConfigMapBinaryDataMap_Success(t *testing.T) {
	cm := CreateConfigMap("test", "default")
	data := map[string][]byte{"key1": []byte("value1")}
	AddConfigMapBinaryDataMap(cm, data)
	if len(cm.BinaryData) != 1 {
		t.Fatal("expected BinaryData to be set")
	}
}

func TestSetConfigMapData_Success(t *testing.T) {
	cm := CreateConfigMap("test", "default")
	data := map[string]string{"new": "data"}
	SetConfigMapData(cm, data)
	if cm.Data["new"] != "data" {
		t.Fatal("expected Data to be replaced")
	}
}

func TestSetConfigMapBinaryData_Success(t *testing.T) {
	cm := CreateConfigMap("test", "default")
	data := map[string][]byte{"new": []byte("data")}
	SetConfigMapBinaryData(cm, data)
	if string(cm.BinaryData["new"]) != "data" {
		t.Fatal("expected BinaryData to be replaced")
	}
}

func TestSetConfigMapImmutable_Success(t *testing.T) {
	cm := CreateConfigMap("test", "default")
	SetConfigMapImmutable(cm, true)
	if cm.Immutable == nil || !*cm.Immutable {
		t.Fatal("expected Immutable to be true")
	}
}

func TestAddConfigMapLabel_Success(t *testing.T) {
	cm := CreateConfigMap("test", "default")
	AddConfigMapLabel(cm, "key", "value")
	if cm.Labels["key"] != "value" {
		t.Fatal("expected Label to be set")
	}
}

func TestAddConfigMapAnnotation_Success(t *testing.T) {
	cm := CreateConfigMap("test", "default")
	AddConfigMapAnnotation(cm, "key", "value")
	if cm.Annotations["key"] != "value" {
		t.Fatal("expected Annotation to be set")
	}
}

func TestSetConfigMapLabels_Success(t *testing.T) {
	cm := CreateConfigMap("test", "default")
	labels := map[string]string{"new": "label"}
	SetConfigMapLabels(cm, labels)
	if cm.Labels["new"] != "label" {
		t.Fatal("expected Labels to be replaced")
	}
}

func TestSetConfigMapAnnotations_Success(t *testing.T) {
	cm := CreateConfigMap("test", "default")
	anns := map[string]string{"new": "annotation"}
	SetConfigMapAnnotations(cm, anns)
	if cm.Annotations["new"] != "annotation" {
		t.Fatal("expected Annotations to be replaced")
	}
}

// Secret setter tests
func TestAddSecretData_Success(t *testing.T) {
	secret := CreateSecret("test", "default")
	AddSecretData(secret, "key", []byte("value"))
	if string(secret.Data["key"]) != "value" {
		t.Fatal("expected Data to be set")
	}
}

func TestAddSecretStringData_Success(t *testing.T) {
	secret := CreateSecret("test", "default")
	AddSecretStringData(secret, "key", "value")
	if secret.StringData["key"] != "value" {
		t.Fatal("expected StringData to be set")
	}
}

func TestSetSecretType_Success(t *testing.T) {
	secret := CreateSecret("test", "default")
	SetSecretType(secret, corev1.SecretTypeTLS)
	if secret.Type != corev1.SecretTypeTLS {
		t.Fatal("expected Type to be set")
	}
}

func TestSetSecretImmutable_Success(t *testing.T) {
	secret := CreateSecret("test", "default")
	SetSecretImmutable(secret, true)
	if secret.Immutable == nil || !*secret.Immutable {
		t.Fatal("expected Immutable to be true")
	}
}

func TestAddSecretLabel_Success(t *testing.T) {
	secret := CreateSecret("test", "default")
	AddSecretLabel(secret, "key", "value")
	if secret.Labels["key"] != "value" {
		t.Fatal("expected Label to be set")
	}
}

func TestAddSecretAnnotation_Success(t *testing.T) {
	secret := CreateSecret("test", "default")
	AddSecretAnnotation(secret, "key", "value")
	if secret.Annotations["key"] != "value" {
		t.Fatal("expected Annotation to be set")
	}
}

func TestSetSecretLabels_Success(t *testing.T) {
	secret := CreateSecret("test", "default")
	labels := map[string]string{"new": "label"}
	SetSecretLabels(secret, labels)
	if secret.Labels["new"] != "label" {
		t.Fatal("expected Labels to be replaced")
	}
}

func TestSetSecretAnnotations_Success(t *testing.T) {
	secret := CreateSecret("test", "default")
	anns := map[string]string{"new": "annotation"}
	SetSecretAnnotations(secret, anns)
	if secret.Annotations["new"] != "annotation" {
		t.Fatal("expected Annotations to be replaced")
	}
}

func TestAddRoleRule_Success(t *testing.T) {
	role := CreateRole("test", "default")
	rule := rbacv1.PolicyRule{
		Verbs:     []string{"get"},
		APIGroups: []string{""},
		Resources: []string{"pods"},
	}
	AddRoleRule(role, rule)
	if len(role.Rules) != 1 {
		t.Fatal("expected PolicyRule to be added")
	}
}

func TestAddClusterRoleRule_Success(t *testing.T) {
	cr := CreateClusterRole("test")
	rule := rbacv1.PolicyRule{
		Verbs:     []string{"get"},
		APIGroups: []string{""},
		Resources: []string{"nodes"},
	}
	AddClusterRoleRule(cr, rule)
	if len(cr.Rules) != 1 {
		t.Fatal("expected PolicyRule to be added")
	}
}

// StorageClass setter tests
func TestSetStorageClassProvisioner_Success(t *testing.T) {
	sc := CreateStorageClass("test", "kubernetes.io/gce-pd")
	SetStorageClassProvisioner(sc, "kubernetes.io/aws-ebs")
	if sc.Provisioner != "kubernetes.io/aws-ebs" {
		t.Fatal("expected Provisioner to be set")
	}
}

func TestSetStorageClassReclaimPolicy_Success(t *testing.T) {
	sc := CreateStorageClass("test", "kubernetes.io/gce-pd")
	policy := corev1.PersistentVolumeReclaimRetain
	SetStorageClassReclaimPolicy(sc, policy)
	if sc.ReclaimPolicy == nil || *sc.ReclaimPolicy != policy {
		t.Fatal("expected ReclaimPolicy to be set")
	}
}

func TestSetStorageClassVolumeBindingMode_Success(t *testing.T) {
	sc := CreateStorageClass("test", "kubernetes.io/gce-pd")
	mode := storagev1.VolumeBindingWaitForFirstConsumer
	SetStorageClassVolumeBindingMode(sc, mode)
	if sc.VolumeBindingMode == nil || *sc.VolumeBindingMode != mode {
		t.Fatal("expected VolumeBindingMode to be set")
	}
}

func TestSetStorageClassAllowVolumeExpansion_Success(t *testing.T) {
	sc := CreateStorageClass("test", "kubernetes.io/gce-pd")
	SetStorageClassAllowVolumeExpansion(sc, true)
	if sc.AllowVolumeExpansion == nil || !*sc.AllowVolumeExpansion {
		t.Fatal("expected AllowVolumeExpansion to be true")
	}
}

func TestAddStorageClassParameter_Success(t *testing.T) {
	sc := CreateStorageClass("test", "kubernetes.io/gce-pd")
	AddStorageClassParameter(sc, "type", "gp2")
	if sc.Parameters["type"] != "gp2" {
		t.Fatal("expected Parameter to be added")
	}
}

func TestAddStorageClassAllowedTopology_Success(t *testing.T) {
	sc := CreateStorageClass("test", "kubernetes.io/gce-pd")
	topology := corev1.TopologySelectorTerm{
		MatchLabelExpressions: []corev1.TopologySelectorLabelRequirement{
			{Key: "zone", Values: []string{"us-west-1a"}},
		},
	}
	AddStorageClassAllowedTopology(sc, topology)
	if len(sc.AllowedTopologies) != 1 {
		t.Fatal("expected AllowedTopology to be added")
	}
}

func TestSetStorageClassMountOptions_Success(t *testing.T) {
	sc := CreateStorageClass("test", "kubernetes.io/gce-pd")
	opts := []string{"ro", "noatime"}
	SetStorageClassMountOptions(sc, opts)
	if len(sc.MountOptions) != 2 {
		t.Fatal("expected MountOptions to be set")
	}
}
