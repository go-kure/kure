package externalsecrets

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	esv1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kureio "github.com/go-kure/kure/pkg/io"
)

var update = flag.Bool("update", false, "update golden files")

func goldenTest(t *testing.T, filename string, obj client.Object) {
	t.Helper()
	objects := []*client.Object{&obj}
	got, err := kureio.EncodeObjectsToYAMLWithOptions(objects, kureio.EncodeOptions{
		KubernetesFieldOrder: true,
	})
	if err != nil {
		t.Fatalf("encoding to YAML: %v", err)
	}
	golden := filepath.Join("testdata", filename)
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatalf("creating testdata dir: %v", err)
		}
		if err := os.WriteFile(golden, got, 0o644); err != nil {
			t.Fatalf("updating golden file: %v", err)
		}
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("reading golden file (run with -update to create): %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("output does not match golden file %s\n\ngot:\n%s\nwant:\n%s", golden, got, want)
	}
}

func TestGolden_ExternalSecret(t *testing.T) {
	obj := ExternalSecret(&ExternalSecretConfig{
		Name:           "db-credentials",
		Namespace:      "default",
		SecretStoreRef: esv1.SecretStoreRef{Name: "vault", Kind: "ClusterSecretStore"},
		Data: []esv1.ExternalSecretData{
			{SecretKey: "password", RemoteRef: esv1.ExternalSecretDataRemoteRef{Key: "secret/data/db", Property: "password"}},
			{SecretKey: "username", RemoteRef: esv1.ExternalSecretDataRemoteRef{Key: "secret/data/db", Property: "username"}},
		},
	})
	goldenTest(t, "externalsecret.yaml", obj)
}

func TestGolden_SecretStore(t *testing.T) {
	obj := SecretStore(&SecretStoreConfig{
		Name:       "aws-store",
		Namespace:  "default",
		Provider:   &esv1.SecretStoreProvider{AWS: &esv1.AWSProvider{Service: esv1.AWSServiceSecretsManager, Region: "us-east-1"}},
		Controller: "my-controller",
	})
	goldenTest(t, "secretstore.yaml", obj)
}

func TestGolden_ClusterSecretStore(t *testing.T) {
	obj := ClusterSecretStore(&ClusterSecretStoreConfig{
		Name:       "global-vault",
		Provider:   &esv1.SecretStoreProvider{AWS: &esv1.AWSProvider{Service: esv1.AWSServiceSecretsManager, Region: "eu-west-1"}},
		Controller: "global-controller",
	})
	goldenTest(t, "clustersecretstore.yaml", obj)
}
