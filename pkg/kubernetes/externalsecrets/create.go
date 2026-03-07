package externalsecrets

import (
	esv1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1"

	intes "github.com/go-kure/kure/internal/externalsecrets"
)

// ExternalSecret converts the config to an ExternalSecret object.
func ExternalSecret(cfg *ExternalSecretConfig) *esv1.ExternalSecret {
	if cfg == nil {
		return nil
	}
	obj := intes.CreateExternalSecret(cfg.Name, cfg.Namespace, esv1.ExternalSecretSpec{})
	intes.SetExternalSecretSecretStoreRef(obj, cfg.SecretStoreRef)
	for _, d := range cfg.Data {
		intes.AddExternalSecretData(obj, d)
	}
	return obj
}

// SecretStore converts the config to a SecretStore object.
func SecretStore(cfg *SecretStoreConfig) *esv1.SecretStore {
	if cfg == nil {
		return nil
	}
	obj := intes.CreateSecretStore(cfg.Name, cfg.Namespace, esv1.SecretStoreSpec{})
	if cfg.Provider != nil {
		intes.SetSecretStoreProvider(obj, cfg.Provider)
	}
	if cfg.Controller != "" {
		intes.SetSecretStoreController(obj, cfg.Controller)
	}
	return obj
}

// ClusterSecretStore converts the config to a ClusterSecretStore object.
func ClusterSecretStore(cfg *ClusterSecretStoreConfig) *esv1.ClusterSecretStore {
	if cfg == nil {
		return nil
	}
	obj := intes.CreateClusterSecretStore(cfg.Name, esv1.SecretStoreSpec{})
	if cfg.Provider != nil {
		intes.SetClusterSecretStoreProvider(obj, cfg.Provider)
	}
	if cfg.Controller != "" {
		intes.SetClusterSecretStoreController(obj, cfg.Controller)
	}
	return obj
}
