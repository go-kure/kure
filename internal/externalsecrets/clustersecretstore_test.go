package externalsecrets

import (
	"testing"

	esv1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1"
)

func TestCreateClusterSecretStore(t *testing.T) {
	css := CreateClusterSecretStore("css", esv1.SecretStoreSpec{})
	if css.Name != "css" {
		t.Fatalf("unexpected name %s", css.Name)
	}
	if css.Kind != "ClusterSecretStore" {
		t.Errorf("unexpected kind %s", css.Kind)
	}
}

func TestClusterSecretStoreHelpers(t *testing.T) {
	css := CreateClusterSecretStore("css", esv1.SecretStoreSpec{})
	AddClusterSecretStoreLabel(css, "app", "demo")
	AddClusterSecretStoreAnnotation(css, "team", "dev")
	provider := &esv1.SecretStoreProvider{}
	SetClusterSecretStoreProvider(css, provider)
	SetClusterSecretStoreController(css, "controller")

	if css.Labels["app"] != "demo" {
		t.Errorf("label not set")
	}
	if css.Annotations["team"] != "dev" {
		t.Errorf("annotation not set")
	}
	if css.Spec.Provider == nil {
		t.Errorf("provider not set")
	}
	if css.Spec.Controller != "controller" {
		t.Errorf("controller not set")
	}
}
