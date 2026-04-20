package cnpg

import (
	cnpgv1 "github.com/cloudnative-pg/cloudnative-pg/api/v1"
	barmanv1 "github.com/cloudnative-pg/plugin-barman-cloud/api/v1"
)

// ClusterConfig describes a CNPG Cluster resource.
type ClusterConfig struct {
	Name      string             `yaml:"name"`
	Namespace string             `yaml:"namespace"`
	Spec      cnpgv1.ClusterSpec `yaml:"spec"`
}

// DatabaseConfig describes a CNPG Database resource.
type DatabaseConfig struct {
	Name      string              `yaml:"name"`
	Namespace string              `yaml:"namespace"`
	Spec      cnpgv1.DatabaseSpec `yaml:"spec"`
}

// ObjectStoreConfig describes a Barman Cloud ObjectStore resource.
type ObjectStoreConfig struct {
	Name      string                   `yaml:"name"`
	Namespace string                   `yaml:"namespace"`
	Spec      barmanv1.ObjectStoreSpec `yaml:"spec"`
}

// ScheduledBackupConfig describes a CNPG ScheduledBackup resource.
type ScheduledBackupConfig struct {
	Name      string                     `yaml:"name"`
	Namespace string                     `yaml:"namespace"`
	Spec      cnpgv1.ScheduledBackupSpec `yaml:"spec"`
}
