package k8s

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestCreateSecret(t *testing.T) {
	sec := CreateSecret("sec", "ns")
	if sec.Name != "sec" {
		t.Errorf("expected name sec got %s", sec.Name)
	}
	if sec.Namespace != "ns" {
		t.Errorf("expected namespace ns got %s", sec.Namespace)
	}
	if sec.Kind != "Secret" {
		t.Errorf("unexpected kind %q", sec.Kind)
	}
	if sec.Type != corev1.SecretTypeOpaque {
		t.Errorf("unexpected default type %s", sec.Type)
	}
	if sec.Immutable == nil {
		t.Errorf("expected immutable pointer set")
	}
	if len(sec.Data) != 0 || len(sec.StringData) != 0 {
		t.Errorf("expected empty data fields")
	}
}

func TestAddSecretData(t *testing.T) {
	sec := CreateSecret("s", "ns")
	AddSecretData(sec, "key", []byte("val"))
	if v, ok := sec.Data["key"]; !ok || string(v) != "val" {
		t.Errorf("data not added correctly")
	}
}

func TestAddSecretStringData(t *testing.T) {
	sec := CreateSecret("s", "ns")
	AddSecretStringData(sec, "key", "val")
	if v, ok := sec.StringData["key"]; !ok || v != "val" {
		t.Errorf("stringData not added correctly")
	}
}

func TestSetSecretType(t *testing.T) {
	sec := CreateSecret("s", "ns")
	SetSecretType(sec, corev1.SecretTypeDockercfg)
	if sec.Type != corev1.SecretTypeDockercfg {
		t.Errorf("secret type not set")
	}
}

func TestSetSecretImmutable(t *testing.T) {
	sec := CreateSecret("s", "ns")
	SetSecretImmutable(sec, true)
	if sec.Immutable == nil || !*sec.Immutable {
		t.Errorf("immutable not set to true")
	}
	SetSecretImmutable(sec, false)
	if sec.Immutable == nil || *sec.Immutable {
		t.Errorf("immutable not updated to false")
	}
}
