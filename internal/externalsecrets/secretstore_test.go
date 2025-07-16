package externalsecrets

import (
	esv1beta1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1beta1"
	"testing"
)

func TestSecretStoreHelpers(t *testing.T) {
	ss := CreateSecretStore("demo", "ns", esv1beta1.SecretStoreSpec{})
	if ss.Name != "demo" || ss.Namespace != "ns" {
		t.Fatalf("metadata mismatch")
	}
	AddSecretStoreLabel(ss, "env", "prod")
	if ss.Labels["env"] != "prod" {
		t.Errorf("label not set")
	}
	AddSecretStoreAnnotation(ss, "team", "dev")
	if ss.Annotations["team"] != "dev" {
		t.Errorf("annotation not set")
	}
	provider := &esv1beta1.SecretStoreProvider{}
	SetSecretStoreProvider(ss, provider)
	if ss.Spec.Provider == nil {
		t.Errorf("provider not set")
	}
}
