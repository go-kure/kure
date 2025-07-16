package externalsecrets

import (
	esv1beta1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1beta1"
	"testing"
)

func TestClusterExternalSecretHelpers(t *testing.T) {
	ces := CreateClusterExternalSecret("demo", esv1beta1.ClusterExternalSecretSpec{})
	if ces.Name != "demo" {
		t.Fatalf("name mismatch: %s", ces.Name)
	}
	AddClusterExternalSecretLabel(ces, "env", "prod")
	if ces.Labels["env"] != "prod" {
		t.Errorf("label not set")
	}
	AddClusterExternalSecretAnnotation(ces, "team", "dev")
	if ces.Annotations["team"] != "dev" {
		t.Errorf("annotation not set")
	}
}
