package cnpg

import (
	barmanapi "github.com/cloudnative-pg/barman-cloud/pkg/api"
	cnpgv1 "github.com/cloudnative-pg/cloudnative-pg/api/v1"
	barmanv1 "github.com/cloudnative-pg/plugin-barman-cloud/api/v1"
	corev1 "k8s.io/api/core/v1"
)

// Labels and annotations use the generic kubernetes.AddLabel /
// kubernetes.AddAnnotation over metav1.Object; this package carries no per-kind
// metadata helpers.

// AddClusterManagedRole adds a managed role to the Cluster spec.
func AddClusterManagedRole(obj *cnpgv1.Cluster, role cnpgv1.RoleConfiguration) {
	if obj.Spec.Managed == nil {
		obj.Spec.Managed = &cnpgv1.ManagedConfiguration{}
	}
	obj.Spec.Managed.Roles = append(obj.Spec.Managed.Roles, role)
}

// AddDatabaseExtension appends an extension to the Database spec.
func AddDatabaseExtension(obj *cnpgv1.Database, ext cnpgv1.ExtensionSpec) {
	obj.Spec.Extensions = append(obj.Spec.Extensions, ext)
}

// AddObjectStoreEnvVar appends an environment variable to the instance sidecar configuration.
func AddObjectStoreEnvVar(obj *barmanv1.ObjectStore, envVar corev1.EnvVar) {
	obj.Spec.InstanceSidecarConfiguration.Env = append(
		obj.Spec.InstanceSidecarConfiguration.Env, envVar,
	)
}

// SetObjectStoreS3Credentials sets S3 credentials on the ObjectStore configuration.
func SetObjectStoreS3Credentials(obj *barmanv1.ObjectStore, creds *barmanapi.S3Credentials) {
	obj.Spec.Configuration.AWS = creds
}

// SetObjectStoreWalConfig sets the WAL backup configuration on the ObjectStore.
func SetObjectStoreWalConfig(obj *barmanv1.ObjectStore, wal *barmanapi.WalBackupConfiguration) {
	obj.Spec.Configuration.Wal = wal
}

// SetObjectStoreDataConfig sets the data backup configuration on the ObjectStore.
func SetObjectStoreDataConfig(obj *barmanv1.ObjectStore, data *barmanapi.DataBackupConfiguration) {
	obj.Spec.Configuration.Data = data
}

// SetScheduledBackupPluginConfiguration sets the plugin configuration on the ScheduledBackup spec.
func SetScheduledBackupPluginConfiguration(obj *cnpgv1.ScheduledBackup, name string, params map[string]string) {
	obj.Spec.PluginConfiguration = &cnpgv1.BackupPluginConfiguration{
		Name:       name,
		Parameters: params,
	}
}

// SetScheduledBackupImmediate sets the immediate flag on the ScheduledBackup spec.
func SetScheduledBackupImmediate(obj *cnpgv1.ScheduledBackup, immediate bool) {
	obj.Spec.Immediate = &immediate
}

// SetScheduledBackupSuspend sets the suspend flag on the ScheduledBackup spec.
func SetScheduledBackupSuspend(obj *cnpgv1.ScheduledBackup, suspend bool) {
	obj.Spec.Suspend = &suspend
}
