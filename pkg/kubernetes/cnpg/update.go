package cnpg

import (
	barmanapi "github.com/cloudnative-pg/barman-cloud/pkg/api"
	cnpgv1 "github.com/cloudnative-pg/cloudnative-pg/api/v1"
	barmanv1 "github.com/cloudnative-pg/plugin-barman-cloud/api/v1"
	corev1 "k8s.io/api/core/v1"
)

// AddClusterLabel adds or updates a label on the Cluster metadata.
func AddClusterLabel(obj *cnpgv1.Cluster, key, value string) {
	if obj.Labels == nil {
		obj.Labels = make(map[string]string)
	}
	obj.Labels[key] = value
}

// AddClusterAnnotation adds or updates an annotation on the Cluster metadata.
func AddClusterAnnotation(obj *cnpgv1.Cluster, key, value string) {
	if obj.Annotations == nil {
		obj.Annotations = make(map[string]string)
	}
	obj.Annotations[key] = value
}

// AddClusterManagedRole adds a managed role to the Cluster spec.
func AddClusterManagedRole(obj *cnpgv1.Cluster, role cnpgv1.RoleConfiguration) {
	if obj.Spec.Managed == nil {
		obj.Spec.Managed = &cnpgv1.ManagedConfiguration{}
	}
	obj.Spec.Managed.Roles = append(obj.Spec.Managed.Roles, role)
}

// AddDatabaseLabel adds or updates a label on the Database metadata.
func AddDatabaseLabel(obj *cnpgv1.Database, key, value string) {
	if obj.Labels == nil {
		obj.Labels = make(map[string]string)
	}
	obj.Labels[key] = value
}

// AddDatabaseAnnotation adds or updates an annotation on the Database metadata.
func AddDatabaseAnnotation(obj *cnpgv1.Database, key, value string) {
	if obj.Annotations == nil {
		obj.Annotations = make(map[string]string)
	}
	obj.Annotations[key] = value
}

// AddDatabaseExtension appends an extension to the Database spec.
func AddDatabaseExtension(obj *cnpgv1.Database, ext cnpgv1.ExtensionSpec) {
	obj.Spec.Extensions = append(obj.Spec.Extensions, ext)
}

// AddObjectStoreLabel adds or updates a label on the ObjectStore metadata.
func AddObjectStoreLabel(obj *barmanv1.ObjectStore, key, value string) {
	if obj.Labels == nil {
		obj.Labels = make(map[string]string)
	}
	obj.Labels[key] = value
}

// AddObjectStoreAnnotation adds or updates an annotation on the ObjectStore metadata.
func AddObjectStoreAnnotation(obj *barmanv1.ObjectStore, key, value string) {
	if obj.Annotations == nil {
		obj.Annotations = make(map[string]string)
	}
	obj.Annotations[key] = value
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

// AddScheduledBackupLabel adds or updates a label on the ScheduledBackup metadata.
func AddScheduledBackupLabel(obj *cnpgv1.ScheduledBackup, key, value string) {
	if obj.Labels == nil {
		obj.Labels = make(map[string]string)
	}
	obj.Labels[key] = value
}

// AddScheduledBackupAnnotation adds or updates an annotation on the ScheduledBackup metadata.
func AddScheduledBackupAnnotation(obj *cnpgv1.ScheduledBackup, key, value string) {
	if obj.Annotations == nil {
		obj.Annotations = make(map[string]string)
	}
	obj.Annotations[key] = value
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
