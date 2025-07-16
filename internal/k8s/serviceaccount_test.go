package k8s

import (
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
	AddServiceAccountSecret(sa, ref)
	if len(sa.Secrets) != 1 || sa.Secrets[0] != ref {
		t.Errorf("secret not added")
	}
}

func TestAddServiceAccountImagePullSecret(t *testing.T) {
	sa := CreateServiceAccount("sa", "ns")
	ref := corev1.LocalObjectReference{Name: "pullsecret"}
	AddServiceAccountImagePullSecret(sa, ref)
	if len(sa.ImagePullSecrets) != 1 || sa.ImagePullSecrets[0] != ref {
		t.Errorf("image pull secret not added")
	}
}

func TestSetServiceAccountAutomountToken(t *testing.T) {
	sa := CreateServiceAccount("sa", "ns")
	SetServiceAccountAutomountToken(sa, true)
	if sa.AutomountServiceAccountToken == nil || !*sa.AutomountServiceAccountToken {
		t.Errorf("automount token not set to true")
	}
	SetServiceAccountAutomountToken(sa, false)
	if sa.AutomountServiceAccountToken == nil || *sa.AutomountServiceAccountToken {
		t.Errorf("automount token not updated to false")
	}
}
