package externalsecrets_test

import (
	"fmt"

	esv1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1"

	"github.com/go-kure/kure/internal/externalsecrets"
)

// This example demonstrates composing a SecretStore backed by AWS
// Secrets Manager and an ExternalSecret that syncs credentials into
// a Kubernetes Secret.
func Example_composeSecretStoreAndExternalSecret() {
	// --- SecretStore (AWS Secrets Manager) ---
	store := externalsecrets.CreateSecretStore("aws-store", "default", esv1.SecretStoreSpec{})
	externalsecrets.SetSecretStoreProvider(store, &esv1.SecretStoreProvider{
		AWS: &esv1.AWSProvider{
			Service: esv1.AWSServiceSecretsManager,
			Region:  "eu-west-1",
		},
	})
	externalsecrets.AddSecretStoreLabel(store, "env", "production")

	// --- ExternalSecret referencing the store ---
	es := externalsecrets.CreateExternalSecret("db-credentials", "default", esv1.ExternalSecretSpec{})
	externalsecrets.SetExternalSecretSecretStoreRef(es, esv1.SecretStoreRef{
		Name: store.Name,
		Kind: "SecretStore",
	})
	externalsecrets.AddExternalSecretData(es, esv1.ExternalSecretData{
		SecretKey: "username",
		RemoteRef: esv1.ExternalSecretDataRemoteRef{Key: "prod/db/username"},
	})
	externalsecrets.AddExternalSecretData(es, esv1.ExternalSecretData{
		SecretKey: "password",
		RemoteRef: esv1.ExternalSecretDataRemoteRef{Key: "prod/db/password"},
	})
	externalsecrets.AddExternalSecretLabel(es, "app", "backend")

	fmt.Println("Store:", store.Name)
	fmt.Println("Store Kind:", store.Kind)
	fmt.Println("Store Namespace:", store.Namespace)
	fmt.Println("ExternalSecret:", es.Name)
	fmt.Println("ExternalSecret Namespace:", es.Namespace)
	fmt.Println("Store Ref:", es.Spec.SecretStoreRef.Name)
	fmt.Println("Data Keys:", es.Spec.Data[0].SecretKey, es.Spec.Data[1].SecretKey)
	// Output:
	// Store: aws-store
	// Store Kind: SecretStore
	// Store Namespace: default
	// ExternalSecret: db-credentials
	// ExternalSecret Namespace: default
	// Store Ref: aws-store
	// Data Keys: username password
}
