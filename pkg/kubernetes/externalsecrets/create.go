package externalsecrets

import (
	esv1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1"
)

// ExternalSecret converts the config to an ExternalSecret object.
func ExternalSecret(cfg *ExternalSecretConfig) *esv1.ExternalSecret {
	if cfg == nil {
		return nil
	}
	obj := CreateExternalSecret(cfg.Name, cfg.Namespace)
	SetExternalSecretSecretStoreRef(obj, cfg.SecretStoreRef)
	for _, d := range cfg.Data {
		AddExternalSecretData(obj, d)
	}
	return obj
}

// SecretStore converts the config to a SecretStore object.
func SecretStore(cfg *SecretStoreConfig) *esv1.SecretStore {
	if cfg == nil {
		return nil
	}
	obj := CreateSecretStore(cfg.Name, cfg.Namespace)
	if cfg.Provider != nil {
		SetSecretStoreProvider(obj, cfg.Provider)
	}
	if cfg.Controller != "" {
		SetSecretStoreController(obj, cfg.Controller)
	}
	return obj
}

// ClusterSecretStore converts the config to a ClusterSecretStore object.
func ClusterSecretStore(cfg *ClusterSecretStoreConfig) *esv1.ClusterSecretStore {
	if cfg == nil {
		return nil
	}
	obj := CreateClusterSecretStore(cfg.Name)
	if cfg.Provider != nil {
		SetClusterSecretStoreProvider(obj, cfg.Provider)
	}
	if cfg.Controller != "" {
		SetClusterSecretStoreController(obj, cfg.Controller)
	}
	return obj
}
