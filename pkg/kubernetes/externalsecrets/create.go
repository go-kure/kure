package externalsecrets

import (
	esv1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateExternalSecret returns a new ExternalSecret with TypeMeta and ObjectMeta set.
func CreateExternalSecret(name, namespace string) *esv1.ExternalSecret {
	return &esv1.ExternalSecret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ExternalSecret",
			APIVersion: esv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

// CreateSecretStore returns a new SecretStore with TypeMeta and ObjectMeta set.
func CreateSecretStore(name, namespace string) *esv1.SecretStore {
	return &esv1.SecretStore{
		TypeMeta: metav1.TypeMeta{
			Kind:       "SecretStore",
			APIVersion: esv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

// CreateClusterSecretStore returns a new ClusterSecretStore with TypeMeta and ObjectMeta set.
// ClusterSecretStore is cluster-scoped so namespace is not set.
func CreateClusterSecretStore(name string) *esv1.ClusterSecretStore {
	return &esv1.ClusterSecretStore{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterSecretStore",
			APIVersion: esv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

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
