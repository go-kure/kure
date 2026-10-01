package externalsecrets

import (
	"testing"

	esv1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1"

	"github.com/go-kure/kure/internal/kuretest"
)

// The fixtures under testdata were written by the config-struct layer this
// package used to carry (ExternalSecret(&ExternalSecretConfig{...}),
// SecretStore, ClusterSecretStore) before it was retired. Each test below
// builds the same object on the generated constructor plus the upstream
// struct and must reproduce that output byte for byte.

func TestGolden_ExternalSecret(t *testing.T) {
	obj := CreateExternalSecret("db-credentials", "default")
	obj.Spec.SecretStoreRef = esv1.SecretStoreRef{Name: "vault", Kind: "ClusterSecretStore"}
	AddExternalSecretData(obj, esv1.ExternalSecretData{
		SecretKey: "password",
		RemoteRef: esv1.ExternalSecretDataRemoteRef{Key: "secret/data/db", Property: "password"},
	})
	AddExternalSecretData(obj, esv1.ExternalSecretData{
		SecretKey: "username",
		RemoteRef: esv1.ExternalSecretDataRemoteRef{Key: "secret/data/db", Property: "username"},
	})
	kuretest.Golden(t, "externalsecret.yaml", obj)
}

func TestGolden_SecretStore(t *testing.T) {
	obj := CreateSecretStore("aws-store", "default")
	obj.Spec.Controller = "my-controller"
	SetSecretStoreProvider(obj, &esv1.SecretStoreProvider{
		AWS: &esv1.AWSProvider{Service: esv1.AWSServiceSecretsManager, Region: "us-east-1"},
	})
	kuretest.Golden(t, "secretstore.yaml", obj)
}

func TestGolden_ClusterSecretStore(t *testing.T) {
	obj := CreateClusterSecretStore("global-vault")
	obj.Spec.Controller = "global-controller"
	SetClusterSecretStoreProvider(obj, &esv1.SecretStoreProvider{
		AWS: &esv1.AWSProvider{Service: esv1.AWSServiceSecretsManager, Region: "eu-west-1"},
	})
	kuretest.Golden(t, "clustersecretstore.yaml", obj)
}
