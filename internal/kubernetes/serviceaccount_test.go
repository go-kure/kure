package kubernetes

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestCreateServiceAccount(t *testing.T) {
	sa := CreateServiceAccount("sa", "default")
	if sa.Name != "sa" {
		t.Errorf("expected name sa got %s", sa.Name)
	}
	if sa.Namespace != "default" {
		t.Errorf("expected namespace default got %s", sa.Namespace)
	}
	if sa.Kind != "ServiceAccount" {
		t.Errorf("unexpected kind %q", sa.Kind)
	}
	if len(sa.Secrets) != 0 {
		t.Errorf("expected no secrets got %d", len(sa.Secrets))
	}
	if len(sa.ImagePullSecrets) != 0 {
		t.Errorf("expected no image pull secrets got %d", len(sa.ImagePullSecrets))
	}
	if sa.AutomountServiceAccountToken == nil {
		t.Errorf("expected automount token pointer set")
	}
}

func TestAddServiceAccountSecret(t *testing.T) {
	sa := CreateServiceAccount("sa", "ns")
	ref := corev1.ObjectReference{Name: "secret"}
	if err := AddServiceAccountSecret(sa, ref); err != nil {
		t.Fatalf("AddServiceAccountSecret returned error: %v", err)
	}
	if len(sa.Secrets) != 1 || sa.Secrets[0] != ref {
		t.Errorf("secret not added")
	}
}

func TestAddServiceAccountImagePullSecret(t *testing.T) {
	sa := CreateServiceAccount("sa", "ns")
	ref := corev1.LocalObjectReference{Name: "pullsecret"}
	if err := AddServiceAccountImagePullSecret(sa, ref); err != nil {
		t.Fatalf("AddServiceAccountImagePullSecret returned error: %v", err)
	}
	if len(sa.ImagePullSecrets) != 1 || sa.ImagePullSecrets[0] != ref {
		t.Errorf("image pull secret not added")
	}
}

func TestSetServiceAccountSecrets(t *testing.T) {
	sa := CreateServiceAccount("sa", "ns")
	secrets := []corev1.ObjectReference{{Name: "a"}, {Name: "b"}}
	if err := SetServiceAccountSecrets(sa, secrets); err != nil {
		t.Fatalf("SetServiceAccountSecrets returned error: %v", err)
	}
	if !reflect.DeepEqual(sa.Secrets, secrets) {
		t.Errorf("secrets not set")
	}
}

func TestSetServiceAccountImagePullSecrets(t *testing.T) {
	sa := CreateServiceAccount("sa", "ns")
	pulls := []corev1.LocalObjectReference{{Name: "x"}, {Name: "y"}}
	if err := SetServiceAccountImagePullSecrets(sa, pulls); err != nil {
		t.Fatalf("SetServiceAccountImagePullSecrets returned error: %v", err)
	}
	if !reflect.DeepEqual(sa.ImagePullSecrets, pulls) {
		t.Errorf("image pull secrets not set")
	}
}

func TestSetServiceAccountAutomountToken(t *testing.T) {
	sa := CreateServiceAccount("sa", "ns")
	if err := SetServiceAccountAutomountToken(sa, true); err != nil {
		t.Fatalf("SetServiceAccountAutomountToken returned error: %v", err)
	}
	if sa.AutomountServiceAccountToken == nil || !*sa.AutomountServiceAccountToken {
		t.Errorf("automount token not set to true")
	}
	if err := SetServiceAccountAutomountToken(sa, false); err != nil {
		t.Fatalf("SetServiceAccountAutomountToken returned error: %v", err)
	}
	if sa.AutomountServiceAccountToken == nil || *sa.AutomountServiceAccountToken {
		t.Errorf("automount token not updated to false")
	}
}

func TestServiceAccountMetadataFunctions(t *testing.T) {
	sa := CreateServiceAccount("sa", "ns")
	AddServiceAccountLabel(sa, "team", "dev")
	if sa.Labels["team"] != "dev" {
		t.Errorf("label not added")
	}
	AddServiceAccountAnnotation(sa, "owner", "bob")
	if sa.Annotations["owner"] != "bob" {
		t.Errorf("annotation not added")
	}

	newLabels := map[string]string{"a": "b"}
	SetServiceAccountLabels(sa, newLabels)
	if !reflect.DeepEqual(sa.Labels, newLabels) {
		t.Errorf("labels not set")
	}

	newAnn := map[string]string{"x": "y"}
	SetServiceAccountAnnotations(sa, newAnn)
	if !reflect.DeepEqual(sa.Annotations, newAnn) {
		t.Errorf("annotations not set")
	}
}
