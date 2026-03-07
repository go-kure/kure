package externalsecrets

import (
	"testing"

	esv1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1"
)

func TestExternalSecret_Success(t *testing.T) {
	cfg := &ExternalSecretConfig{
		Name:      "my-secret",
		Namespace: "default",
		SecretStoreRef: esv1.SecretStoreRef{
			Name: "vault",
			Kind: "ClusterSecretStore",
		},
		Data: []esv1.ExternalSecretData{
			{
				SecretKey: "password",
				RemoteRef: esv1.ExternalSecretDataRemoteRef{
					Key: "secret/data/myapp",
				},
			},
		},
	}

	es := ExternalSecret(cfg)

	if es == nil {
		t.Fatal("expected non-nil ExternalSecret")
	}
	if es.Name != "my-secret" {
		t.Errorf("expected Name 'my-secret', got %s", es.Name)
	}
	if es.Namespace != "default" {
		t.Errorf("expected Namespace 'default', got %s", es.Namespace)
	}
	if es.Spec.SecretStoreRef.Name != "vault" {
		t.Errorf("expected SecretStoreRef.Name 'vault', got %s", es.Spec.SecretStoreRef.Name)
	}
	if es.Spec.SecretStoreRef.Kind != "ClusterSecretStore" {
		t.Errorf("expected SecretStoreRef.Kind 'ClusterSecretStore', got %s", es.Spec.SecretStoreRef.Kind)
	}
	if len(es.Spec.Data) != 1 {
		t.Fatalf("expected 1 data entry, got %d", len(es.Spec.Data))
	}
	if es.Spec.Data[0].SecretKey != "password" {
		t.Errorf("expected SecretKey 'password', got %s", es.Spec.Data[0].SecretKey)
	}
}

func TestExternalSecret_NilConfig(t *testing.T) {
	es := ExternalSecret(nil)
	if es != nil {
		t.Error("expected nil result for nil config")
	}
}

func TestSecretStore_Success(t *testing.T) {
	provider := &esv1.SecretStoreProvider{
		AWS: &esv1.AWSProvider{
			Region: "us-east-1",
		},
	}
	cfg := &SecretStoreConfig{
		Name:       "aws-store",
		Namespace:  "default",
		Provider:   provider,
		Controller: "my-controller",
	}

	ss := SecretStore(cfg)

	if ss == nil {
		t.Fatal("expected non-nil SecretStore")
	}
	if ss.Name != "aws-store" {
		t.Errorf("expected Name 'aws-store', got %s", ss.Name)
	}
	if ss.Namespace != "default" {
		t.Errorf("expected Namespace 'default', got %s", ss.Namespace)
	}
	if ss.Spec.Provider == nil {
		t.Fatal("expected non-nil Provider")
	}
	if ss.Spec.Provider.AWS == nil || ss.Spec.Provider.AWS.Region != "us-east-1" {
		t.Error("expected AWS provider with region us-east-1")
	}
	if ss.Spec.Controller != "my-controller" {
		t.Errorf("expected Controller 'my-controller', got %s", ss.Spec.Controller)
	}
}

func TestSecretStore_NilConfig(t *testing.T) {
	ss := SecretStore(nil)
	if ss != nil {
		t.Error("expected nil result for nil config")
	}
}

func TestSecretStore_MinimalConfig(t *testing.T) {
	cfg := &SecretStoreConfig{
		Name:      "minimal",
		Namespace: "default",
	}

	ss := SecretStore(cfg)

	if ss == nil {
		t.Fatal("expected non-nil SecretStore")
	}
	if ss.Spec.Provider != nil {
		t.Error("expected nil Provider for minimal config")
	}
	if ss.Spec.Controller != "" {
		t.Errorf("expected empty Controller, got %s", ss.Spec.Controller)
	}
}

func TestClusterSecretStore_Success(t *testing.T) {
	provider := &esv1.SecretStoreProvider{
		AWS: &esv1.AWSProvider{
			Region: "eu-west-1",
		},
	}
	cfg := &ClusterSecretStoreConfig{
		Name:       "global-store",
		Provider:   provider,
		Controller: "global-controller",
	}

	css := ClusterSecretStore(cfg)

	if css == nil {
		t.Fatal("expected non-nil ClusterSecretStore")
	}
	if css.Name != "global-store" {
		t.Errorf("expected Name 'global-store', got %s", css.Name)
	}
	if css.Spec.Provider == nil {
		t.Fatal("expected non-nil Provider")
	}
	if css.Spec.Provider.AWS == nil || css.Spec.Provider.AWS.Region != "eu-west-1" {
		t.Error("expected AWS provider with region eu-west-1")
	}
	if css.Spec.Controller != "global-controller" {
		t.Errorf("expected Controller 'global-controller', got %s", css.Spec.Controller)
	}
}

func TestClusterSecretStore_NilConfig(t *testing.T) {
	css := ClusterSecretStore(nil)
	if css != nil {
		t.Error("expected nil result for nil config")
	}
}

func TestClusterSecretStore_MinimalConfig(t *testing.T) {
	cfg := &ClusterSecretStoreConfig{
		Name: "minimal",
	}

	css := ClusterSecretStore(cfg)

	if css == nil {
		t.Fatal("expected non-nil ClusterSecretStore")
	}
	if css.Spec.Provider != nil {
		t.Error("expected nil Provider for minimal config")
	}
	if css.Spec.Controller != "" {
		t.Errorf("expected empty Controller, got %s", css.Spec.Controller)
	}
}
