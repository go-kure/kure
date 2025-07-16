package externalsecrets

import (
	esv1beta1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func TestExternalSecretHelpers(t *testing.T) {
	es := CreateExternalSecret("demo", "ns", esv1beta1.ExternalSecretSpec{})
	if es.Name != "demo" || es.Namespace != "ns" {
		t.Fatalf("metadata mismatch: %s/%s", es.Namespace, es.Name)
	}
	AddExternalSecretLabel(es, "env", "prod")
	if es.Labels["env"] != "prod" {
		t.Errorf("label not set")
	}
	AddExternalSecretAnnotation(es, "team", "dev")
	if es.Annotations["team"] != "dev" {
		t.Errorf("annotation not set")
	}
	AddExternalSecretData(es, esv1beta1.ExternalSecretData{SecretKey: "foo"})
	if len(es.Spec.Data) != 1 || es.Spec.Data[0].SecretKey != "foo" {
		t.Errorf("data not added")
	}
	AddExternalSecretDataFrom(es, esv1beta1.ExternalSecretDataFromRemoteRef{})
	if len(es.Spec.DataFrom) != 1 {
		t.Errorf("dataFrom not added")
	}
	dur := metav1.Duration{Duration: 0}
	SetExternalSecretRefreshInterval(es, dur)
	if es.Spec.RefreshInterval == nil {
		t.Errorf("refresh interval not set")
	}
}
