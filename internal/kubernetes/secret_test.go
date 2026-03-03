package kubernetes

import (
	"reflect"
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
	if err := AddSecretData(sec, "key", []byte("val")); err != nil {
		t.Fatalf("AddSecretData returned error: %v", err)
	}
	if v, ok := sec.Data["key"]; !ok || string(v) != "val" {
		t.Errorf("data not added correctly")
	}
}

func TestAddSecretStringData(t *testing.T) {
	sec := CreateSecret("s", "ns")
	if err := AddSecretStringData(sec, "key", "val"); err != nil {
		t.Fatalf("AddSecretStringData returned error: %v", err)
	}
	if v, ok := sec.StringData["key"]; !ok || v != "val" {
		t.Errorf("stringData not added correctly")
	}
}

func TestSetSecretType(t *testing.T) {
	sec := CreateSecret("s", "ns")
	if err := SetSecretType(sec, corev1.SecretTypeDockercfg); err != nil {
		t.Fatalf("SetSecretType returned error: %v", err)
	}
	if sec.Type != corev1.SecretTypeDockercfg {
		t.Errorf("secret type not set")
	}
}

func TestSetSecretImmutable(t *testing.T) {
	sec := CreateSecret("s", "ns")
	if err := SetSecretImmutable(sec, true); err != nil {
		t.Fatalf("SetSecretImmutable returned error: %v", err)
	}
	if sec.Immutable == nil || !*sec.Immutable {
		t.Errorf("immutable not set to true")
	}
	if err := SetSecretImmutable(sec, false); err != nil {
		t.Fatalf("SetSecretImmutable returned error: %v", err)
	}
	if sec.Immutable == nil || *sec.Immutable {
		t.Errorf("immutable not updated to false")
	}
}

func TestSecretLabelFunctions(t *testing.T) {
	sec := CreateSecret("s", "ns")
	if err := AddSecretLabel(sec, "env", "prod"); err != nil {
		t.Fatalf("AddSecretLabel returned error: %v", err)
	}
	if sec.Labels["env"] != "prod" {
		t.Errorf("label not added")
	}
	newLabels := map[string]string{"a": "b"}
	if err := SetSecretLabels(sec, newLabels); err != nil {
		t.Fatalf("SetSecretLabels returned error: %v", err)
	}
	if !reflect.DeepEqual(sec.Labels, newLabels) {
		t.Errorf("labels not set correctly")
	}
}

func TestSecretAnnotationFunctions(t *testing.T) {
	sec := CreateSecret("s", "ns")
	if err := AddSecretAnnotation(sec, "team", "dev"); err != nil {
		t.Fatalf("AddSecretAnnotation returned error: %v", err)
	}
	if sec.Annotations["team"] != "dev" {
		t.Errorf("annotation not added")
	}
	newAnn := map[string]string{"x": "y"}
	if err := SetSecretAnnotations(sec, newAnn); err != nil {
		t.Fatalf("SetSecretAnnotations returned error: %v", err)
	}
	if !reflect.DeepEqual(sec.Annotations, newAnn) {
		t.Errorf("annotations not set correctly")
	}
}

func TestSecretNilGuards(t *testing.T) {
	tests := []struct {
		name string
		fn   func() error
	}{
		{"AddSecretData", func() error { return AddSecretData(nil, "k", []byte("v")) }},
		{"AddSecretStringData", func() error { return AddSecretStringData(nil, "k", "v") }},
		{"SetSecretType", func() error { return SetSecretType(nil, corev1.SecretTypeOpaque) }},
		{"SetSecretImmutable", func() error { return SetSecretImmutable(nil, true) }},
		{"AddSecretLabel", func() error { return AddSecretLabel(nil, "k", "v") }},
		{"AddSecretAnnotation", func() error { return AddSecretAnnotation(nil, "k", "v") }},
		{"SetSecretLabels", func() error { return SetSecretLabels(nil, map[string]string{"k": "v"}) }},
		{"SetSecretAnnotations", func() error { return SetSecretAnnotations(nil, map[string]string{"k": "v"}) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.fn(); err == nil {
				t.Errorf("%s(nil) should return error", tt.name)
			}
		})
	}
}
