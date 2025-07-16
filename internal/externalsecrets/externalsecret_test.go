package externalsecrets

import (
	"testing"

	esv1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1"
)

func TestCreateExternalSecret(t *testing.T) {
	es := CreateExternalSecret("es", "ns", esv1.ExternalSecretSpec{})
	if es.Name != "es" || es.Namespace != "ns" {
		t.Fatalf("unexpected metadata")
	}
	if es.Kind != "ExternalSecret" {
		t.Errorf("unexpected kind %s", es.Kind)
	}
}

func TestExternalSecretHelpers(t *testing.T) {
	es := CreateExternalSecret("es", "ns", esv1.ExternalSecretSpec{})
	AddExternalSecretLabel(es, "app", "demo")
	AddExternalSecretAnnotation(es, "team", "dev")
	data := esv1.ExternalSecretData{SecretKey: "key", RemoteRef: esv1.ExternalSecretDataRemoteRef{Key: "remote"}}
	AddExternalSecretData(es, data)
	ref := esv1.SecretStoreRef{Name: "store"}
	SetExternalSecretSecretStoreRef(es, ref)

	if es.Labels["app"] != "demo" {
		t.Errorf("label not set")
	}
	if es.Annotations["team"] != "dev" {
		t.Errorf("annotation not set")
	}
	if len(es.Spec.Data) != 1 || es.Spec.Data[0].SecretKey != "key" {
		t.Errorf("data not added")
	}
	if es.Spec.SecretStoreRef.Name != "store" {
		t.Errorf("store ref not set")
	}
}
