package cnpg

import (
	corev1 "k8s.io/api/core/v1"

	barmanapi "github.com/cloudnative-pg/barman-cloud/pkg/api"
	cnpgv1 "github.com/cloudnative-pg/cloudnative-pg/api/v1"
	barmanv1 "github.com/cloudnative-pg/plugin-barman-cloud/api/v1"

	intcnpg "github.com/go-kure/kure/internal/cnpg"
)

// AddClusterLabel delegates to the internal helper.
func AddClusterLabel(obj *cnpgv1.Cluster, key, value string) {
	intcnpg.AddClusterLabel(obj, key, value)
}

// AddClusterAnnotation delegates to the internal helper.
func AddClusterAnnotation(obj *cnpgv1.Cluster, key, value string) {
	intcnpg.AddClusterAnnotation(obj, key, value)
}

// AddClusterManagedRole delegates to the internal helper.
func AddClusterManagedRole(obj *cnpgv1.Cluster, role cnpgv1.RoleConfiguration) {
	intcnpg.AddClusterManagedRole(obj, role)
}

// AddDatabaseLabel delegates to the internal helper.
func AddDatabaseLabel(obj *cnpgv1.Database, key, value string) {
	intcnpg.AddDatabaseLabel(obj, key, value)
}

// AddDatabaseAnnotation delegates to the internal helper.
func AddDatabaseAnnotation(obj *cnpgv1.Database, key, value string) {
	intcnpg.AddDatabaseAnnotation(obj, key, value)
}

// AddDatabaseExtension delegates to the internal helper.
func AddDatabaseExtension(obj *cnpgv1.Database, ext cnpgv1.ExtensionSpec) {
	intcnpg.AddDatabaseExtension(obj, ext)
}

// SetDatabaseClusterRef delegates to the internal helper.
func SetDatabaseClusterRef(obj *cnpgv1.Database, clusterName string) {
	intcnpg.SetDatabaseClusterRef(obj, clusterName)
}

// SetDatabaseOwner delegates to the internal helper.
func SetDatabaseOwner(obj *cnpgv1.Database, owner string) {
	intcnpg.SetDatabaseOwner(obj, owner)
}

// SetDatabaseReclaimPolicy delegates to the internal helper.
func SetDatabaseReclaimPolicy(obj *cnpgv1.Database, policy cnpgv1.DatabaseReclaimPolicy) {
	intcnpg.SetDatabaseReclaimPolicy(obj, policy)
}

// SetDatabaseEnsure delegates to the internal helper.
func SetDatabaseEnsure(obj *cnpgv1.Database, ensure cnpgv1.EnsureOption) {
	intcnpg.SetDatabaseEnsure(obj, ensure)
}

// AddObjectStoreLabel delegates to the internal helper.
func AddObjectStoreLabel(obj *barmanv1.ObjectStore, key, value string) {
	intcnpg.AddObjectStoreLabel(obj, key, value)
}

// AddObjectStoreAnnotation delegates to the internal helper.
func AddObjectStoreAnnotation(obj *barmanv1.ObjectStore, key, value string) {
	intcnpg.AddObjectStoreAnnotation(obj, key, value)
}

// AddObjectStoreEnvVar delegates to the internal helper.
func AddObjectStoreEnvVar(obj *barmanv1.ObjectStore, envVar corev1.EnvVar) {
	intcnpg.AddObjectStoreEnvVar(obj, envVar)
}

// SetObjectStoreDestinationPath delegates to the internal helper.
func SetObjectStoreDestinationPath(obj *barmanv1.ObjectStore, path string) {
	intcnpg.SetObjectStoreDestinationPath(obj, path)
}

// SetObjectStoreEndpointURL delegates to the internal helper.
func SetObjectStoreEndpointURL(obj *barmanv1.ObjectStore, url string) {
	intcnpg.SetObjectStoreEndpointURL(obj, url)
}

// SetObjectStoreS3Credentials delegates to the internal helper.
func SetObjectStoreS3Credentials(obj *barmanv1.ObjectStore, creds *barmanapi.S3Credentials) {
	intcnpg.SetObjectStoreS3Credentials(obj, creds)
}

// SetObjectStoreRetentionPolicy delegates to the internal helper.
func SetObjectStoreRetentionPolicy(obj *barmanv1.ObjectStore, policy string) {
	intcnpg.SetObjectStoreRetentionPolicy(obj, policy)
}

// SetObjectStoreWalConfig delegates to the internal helper.
func SetObjectStoreWalConfig(obj *barmanv1.ObjectStore, wal *barmanapi.WalBackupConfiguration) {
	intcnpg.SetObjectStoreWalConfig(obj, wal)
}

// SetObjectStoreDataConfig delegates to the internal helper.
func SetObjectStoreDataConfig(obj *barmanv1.ObjectStore, data *barmanapi.DataBackupConfiguration) {
	intcnpg.SetObjectStoreDataConfig(obj, data)
}

// AddScheduledBackupLabel delegates to the internal helper.
func AddScheduledBackupLabel(obj *cnpgv1.ScheduledBackup, key, value string) {
	intcnpg.AddScheduledBackupLabel(obj, key, value)
}

// AddScheduledBackupAnnotation delegates to the internal helper.
func AddScheduledBackupAnnotation(obj *cnpgv1.ScheduledBackup, key, value string) {
	intcnpg.AddScheduledBackupAnnotation(obj, key, value)
}

// SetScheduledBackupMethod delegates to the internal helper.
func SetScheduledBackupMethod(obj *cnpgv1.ScheduledBackup, method cnpgv1.BackupMethod) {
	intcnpg.SetScheduledBackupMethod(obj, method)
}

// SetScheduledBackupPluginConfiguration delegates to the internal helper.
func SetScheduledBackupPluginConfiguration(obj *cnpgv1.ScheduledBackup, name string, params map[string]string) {
	intcnpg.SetScheduledBackupPluginConfiguration(obj, name, params)
}

// SetScheduledBackupImmediate delegates to the internal helper.
func SetScheduledBackupImmediate(obj *cnpgv1.ScheduledBackup, immediate bool) {
	intcnpg.SetScheduledBackupImmediate(obj, immediate)
}

// SetScheduledBackupBackupOwnerReference delegates to the internal helper.
func SetScheduledBackupBackupOwnerReference(obj *cnpgv1.ScheduledBackup, ref string) {
	intcnpg.SetScheduledBackupBackupOwnerReference(obj, ref)
}

// SetScheduledBackupSuspend delegates to the internal helper.
func SetScheduledBackupSuspend(obj *cnpgv1.ScheduledBackup, suspend bool) {
	intcnpg.SetScheduledBackupSuspend(obj, suspend)
}
