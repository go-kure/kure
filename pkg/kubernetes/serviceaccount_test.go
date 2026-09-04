package kubernetes

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

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

// TestServiceAccountMetadataViaGenericHelpers covers what
// AddServiceAccountLabel and AddServiceAccountAnnotation used to do.
func TestServiceAccountMetadataViaGenericHelpers(t *testing.T) {
	sa := CreateServiceAccount("sa", "ns")
	AddLabel(sa, "team", "dev")
	AddAnnotation(sa, "owner", "bob")
	if sa.Labels["team"] != "dev" {
		t.Errorf("label not added: %v", sa.Labels)
	}
	if sa.Annotations["owner"] != "bob" {
		t.Errorf("annotation not added: %v", sa.Annotations)
	}
}

func TestServiceAccountNilGuards(t *testing.T) {
	assertPanics(t, func() { AddServiceAccountSecret(nil, corev1.ObjectReference{}) })
	assertPanics(t, func() { AddServiceAccountImagePullSecret(nil, corev1.LocalObjectReference{}) })
	assertPanics(t, func() { SetServiceAccountAutomountToken(nil, true) })

	// Nil-map init guards: plain struct (not CreateServiceAccount) has nil maps and
	// nil AutomountServiceAccountToken pointer.
	bare := &corev1.ServiceAccount{}
	SetServiceAccountAutomountToken(bare, true)
	if bare.AutomountServiceAccountToken == nil || !*bare.AutomountServiceAccountToken {
		t.Error("automount token not initialized on bare ServiceAccount")
	}

	bare2 := &corev1.ServiceAccount{}
	AddLabel(bare2, "team", "ops")
	if bare2.Labels["team"] != "ops" {
		t.Error("label not added to bare ServiceAccount")
	}

	bare3 := &corev1.ServiceAccount{}
	AddAnnotation(bare3, "owner", "ops")
	if bare3.Annotations["owner"] != "ops" {
		t.Error("annotation not added to bare ServiceAccount")
	}
}
