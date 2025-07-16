package externalsecrets

import (
	"testing"

	esv1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1"
)

func TestCreateSecretStore(t *testing.T) {
	ss := CreateSecretStore("store", "ns", esv1.SecretStoreSpec{})
	if ss.Name != "store" || ss.Namespace != "ns" {
		t.Fatalf("unexpected metadata %s %s", ss.Name, ss.Namespace)
	}
	if ss.Kind != "SecretStore" {
		t.Errorf("unexpected kind %s", ss.Kind)
	}
}

func TestSecretStoreHelpers(t *testing.T) {
	ss := CreateSecretStore("s", "ns", esv1.SecretStoreSpec{})
	AddSecretStoreLabel(ss, "app", "demo")
	AddSecretStoreAnnotation(ss, "team", "dev")
	provider := &esv1.SecretStoreProvider{}
	SetSecretStoreProvider(ss, provider)
	SetSecretStoreController(ss, "custom")

	if ss.Labels["app"] != "demo" {
		t.Errorf("label not set")
	}
	if ss.Annotations["team"] != "dev" {
		t.Errorf("annotation not set")
	}
	if ss.Spec.Provider == nil {
		t.Errorf("provider not set")
	}
	if ss.Spec.Controller != "custom" {
		t.Errorf("controller not set")
	}
}
