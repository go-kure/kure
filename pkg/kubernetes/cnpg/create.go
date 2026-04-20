package cnpg

import (
	cnpgv1 "github.com/cloudnative-pg/cloudnative-pg/api/v1"
	barmanv1 "github.com/cloudnative-pg/plugin-barman-cloud/api/v1"

	intcnpg "github.com/go-kure/kure/internal/cnpg"
)

// Cluster converts the config to a CNPG Cluster object.
func Cluster(cfg *ClusterConfig) *cnpgv1.Cluster {
	if cfg == nil {
		return nil
	}
	return intcnpg.CreateCluster(cfg.Name, cfg.Namespace, cfg.Spec)
}

// Database converts the config to a CNPG Database object.
func Database(cfg *DatabaseConfig) *cnpgv1.Database {
	if cfg == nil {
		return nil
	}
	return intcnpg.CreateDatabase(cfg.Name, cfg.Namespace, cfg.Spec)
}

// ObjectStore converts the config to a Barman Cloud ObjectStore object.
func ObjectStore(cfg *ObjectStoreConfig) *barmanv1.ObjectStore {
	if cfg == nil {
		return nil
	}
	return intcnpg.CreateObjectStore(cfg.Name, cfg.Namespace, cfg.Spec)
}

// ScheduledBackup converts the config to a CNPG ScheduledBackup object.
func ScheduledBackup(cfg *ScheduledBackupConfig) *cnpgv1.ScheduledBackup {
	if cfg == nil {
		return nil
	}
	return intcnpg.CreateScheduledBackup(cfg.Name, cfg.Namespace, cfg.Spec)
}
