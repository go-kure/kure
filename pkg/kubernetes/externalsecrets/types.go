package externalsecrets

import (
	esv1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1"
)

// ExternalSecretConfig contains the configuration for an ExternalSecret.
type ExternalSecretConfig struct {
	Name           string                    `yaml:"name"`
	Namespace      string                    `yaml:"namespace"`
	SecretStoreRef esv1.SecretStoreRef       `yaml:"secretStoreRef"`
	Data           []esv1.ExternalSecretData `yaml:"data,omitempty"`
}

// SecretStoreConfig contains the configuration for a SecretStore.
type SecretStoreConfig struct {
	Name       string                    `yaml:"name"`
	Namespace  string                    `yaml:"namespace"`
	Provider   *esv1.SecretStoreProvider `yaml:"provider,omitempty"`
	Controller string                    `yaml:"controller,omitempty"`
}

// ClusterSecretStoreConfig contains the configuration for a ClusterSecretStore.
type ClusterSecretStoreConfig struct {
	Name       string                    `yaml:"name"`
	Provider   *esv1.SecretStoreProvider `yaml:"provider,omitempty"`
	Controller string                    `yaml:"controller,omitempty"`
}
