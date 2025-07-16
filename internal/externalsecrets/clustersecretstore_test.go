package externalsecrets

import (
	esv1beta1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1beta1"
	"testing"
)

func TestClusterSecretStoreHelpers(t *testing.T) {
	css := CreateClusterSecretStore("demo", esv1beta1.SecretStoreSpec{})
	if css.Name != "demo" {
		t.Fatalf("name mismatch")
	}
	AddClusterSecretStoreLabel(css, "env", "prod")
	if css.Labels["env"] != "prod" {
		t.Errorf("label not set")
	}
	AddClusterSecretStoreAnnotation(css, "team", "dev")
	if css.Annotations["team"] != "dev" {
		t.Errorf("annotation not set")
	}
	provider := &esv1beta1.SecretStoreProvider{}
	SetClusterSecretStoreProvider(css, provider)
	if css.Spec.Provider == nil {
		t.Errorf("provider not set")
	}
}
