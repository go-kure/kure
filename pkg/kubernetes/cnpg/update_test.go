package cnpg

import (
	"testing"

	corev1 "k8s.io/api/core/v1"

	barmanapi "github.com/cloudnative-pg/barman-cloud/pkg/api"
	cnpgv1 "github.com/cloudnative-pg/cloudnative-pg/api/v1"
	barmanv1 "github.com/cloudnative-pg/plugin-barman-cloud/api/v1"
)

func TestAddClusterLabel(t *testing.T) {
	obj := Cluster(&ClusterConfig{Name: "pg", Namespace: "db", Spec: cnpgv1.ClusterSpec{}})
	AddClusterLabel(obj, "env", "prod")
	if obj.Labels["env"] != "prod" {
		t.Error("expected label 'env' to be 'prod'")
	}
}

func TestAddClusterAnnotation(t *testing.T) {
	obj := Cluster(&ClusterConfig{Name: "pg", Namespace: "db", Spec: cnpgv1.ClusterSpec{}})
	AddClusterAnnotation(obj, "note", "value")
	if obj.Annotations["note"] != "value" {
		t.Error("expected annotation 'note' to be 'value'")
	}
}

func TestAddClusterManagedRole(t *testing.T) {
	obj := Cluster(&ClusterConfig{Name: "pg", Namespace: "db", Spec: cnpgv1.ClusterSpec{}})
	role := cnpgv1.RoleConfiguration{Name: "app"}
	AddClusterManagedRole(obj, role)
	if len(obj.Spec.Managed.Roles) != 1 {
		t.Fatalf("expected 1 managed role, got %d", len(obj.Spec.Managed.Roles))
	}
	if obj.Spec.Managed.Roles[0].Name != "app" {
		t.Errorf("expected role name 'app', got %s", obj.Spec.Managed.Roles[0].Name)
	}
}

func TestAddDatabaseLabel(t *testing.T) {
	obj := Database(&DatabaseConfig{Name: "db", Namespace: "ns", Spec: cnpgv1.DatabaseSpec{}})
	AddDatabaseLabel(obj, "env", "prod")
	if obj.Labels["env"] != "prod" {
		t.Error("expected label 'env' to be 'prod'")
	}
}

func TestAddDatabaseAnnotation(t *testing.T) {
	obj := Database(&DatabaseConfig{Name: "db", Namespace: "ns", Spec: cnpgv1.DatabaseSpec{}})
	AddDatabaseAnnotation(obj, "note", "value")
	if obj.Annotations["note"] != "value" {
		t.Error("expected annotation 'note' to be 'value'")
	}
}

func TestAddDatabaseExtension(t *testing.T) {
	obj := Database(&DatabaseConfig{Name: "db", Namespace: "ns", Spec: cnpgv1.DatabaseSpec{}})
	ext := cnpgv1.ExtensionSpec{DatabaseObjectSpec: cnpgv1.DatabaseObjectSpec{Name: "pgcrypto"}}
	AddDatabaseExtension(obj, ext)
	if len(obj.Spec.Extensions) != 1 {
		t.Fatalf("expected 1 extension, got %d", len(obj.Spec.Extensions))
	}
	if obj.Spec.Extensions[0].Name != "pgcrypto" {
		t.Errorf("expected extension name 'pgcrypto', got %s", obj.Spec.Extensions[0].Name)
	}
}

func TestSetDatabaseClusterRef(t *testing.T) {
	obj := Database(&DatabaseConfig{Name: "db", Namespace: "ns", Spec: cnpgv1.DatabaseSpec{}})
	SetDatabaseClusterRef(obj, "pg-main")
	if obj.Spec.ClusterRef.Name != "pg-main" {
		t.Errorf("expected cluster ref 'pg-main', got %s", obj.Spec.ClusterRef.Name)
	}
}

func TestSetDatabaseOwner(t *testing.T) {
	obj := Database(&DatabaseConfig{Name: "db", Namespace: "ns", Spec: cnpgv1.DatabaseSpec{}})
	SetDatabaseOwner(obj, "appuser")
	if obj.Spec.Owner != "appuser" {
		t.Errorf("expected owner 'appuser', got %s", obj.Spec.Owner)
	}
}

func TestSetDatabaseReclaimPolicy(t *testing.T) {
	obj := Database(&DatabaseConfig{Name: "db", Namespace: "ns", Spec: cnpgv1.DatabaseSpec{}})
	SetDatabaseReclaimPolicy(obj, cnpgv1.DatabaseReclaimDelete)
	if obj.Spec.ReclaimPolicy != cnpgv1.DatabaseReclaimDelete {
		t.Errorf("expected reclaim policy Delete, got %s", obj.Spec.ReclaimPolicy)
	}
}

func TestSetDatabaseEnsure(t *testing.T) {
	obj := Database(&DatabaseConfig{Name: "db", Namespace: "ns", Spec: cnpgv1.DatabaseSpec{}})
	SetDatabaseEnsure(obj, cnpgv1.EnsurePresent)
	if obj.Spec.Ensure != cnpgv1.EnsurePresent {
		t.Errorf("expected ensure Present, got %s", obj.Spec.Ensure)
	}
}

func TestAddObjectStoreLabel(t *testing.T) {
	obj := ObjectStore(&ObjectStoreConfig{Name: "store", Namespace: "ns", Spec: barmanv1.ObjectStoreSpec{}})
	AddObjectStoreLabel(obj, "env", "prod")
	if obj.Labels["env"] != "prod" {
		t.Error("expected label 'env' to be 'prod'")
	}
}

func TestAddObjectStoreAnnotation(t *testing.T) {
	obj := ObjectStore(&ObjectStoreConfig{Name: "store", Namespace: "ns", Spec: barmanv1.ObjectStoreSpec{}})
	AddObjectStoreAnnotation(obj, "note", "value")
	if obj.Annotations["note"] != "value" {
		t.Error("expected annotation 'note' to be 'value'")
	}
}

func TestAddObjectStoreEnvVar(t *testing.T) {
	obj := ObjectStore(&ObjectStoreConfig{Name: "store", Namespace: "ns", Spec: barmanv1.ObjectStoreSpec{}})
	env := corev1.EnvVar{Name: "AWS_REGION", Value: "us-east-1"}
	AddObjectStoreEnvVar(obj, env)
	if len(obj.Spec.InstanceSidecarConfiguration.Env) != 1 {
		t.Fatalf("expected 1 env var, got %d", len(obj.Spec.InstanceSidecarConfiguration.Env))
	}
	if obj.Spec.InstanceSidecarConfiguration.Env[0].Name != "AWS_REGION" {
		t.Errorf("expected env var 'AWS_REGION', got %s", obj.Spec.InstanceSidecarConfiguration.Env[0].Name)
	}
}

func TestSetObjectStoreDestinationPath(t *testing.T) {
	obj := ObjectStore(&ObjectStoreConfig{Name: "store", Namespace: "ns", Spec: barmanv1.ObjectStoreSpec{}})
	SetObjectStoreDestinationPath(obj, "s3://my-bucket/backups")
	if obj.Spec.Configuration.DestinationPath != "s3://my-bucket/backups" {
		t.Errorf("expected DestinationPath 's3://my-bucket/backups', got %s", obj.Spec.Configuration.DestinationPath)
	}
}

func TestSetObjectStoreEndpointURL(t *testing.T) {
	obj := ObjectStore(&ObjectStoreConfig{Name: "store", Namespace: "ns", Spec: barmanv1.ObjectStoreSpec{}})
	SetObjectStoreEndpointURL(obj, "https://s3.example.com")
	if obj.Spec.Configuration.EndpointURL != "https://s3.example.com" {
		t.Errorf("expected EndpointURL 'https://s3.example.com', got %s", obj.Spec.Configuration.EndpointURL)
	}
}

func TestSetObjectStoreS3Credentials(t *testing.T) {
	obj := ObjectStore(&ObjectStoreConfig{Name: "store", Namespace: "ns", Spec: barmanv1.ObjectStoreSpec{}})
	creds := &barmanapi.S3Credentials{}
	SetObjectStoreS3Credentials(obj, creds)
	if obj.Spec.Configuration.AWS == nil {
		t.Error("expected non-nil S3Credentials (AWS field)")
	}
}

func TestSetObjectStoreRetentionPolicy(t *testing.T) {
	obj := ObjectStore(&ObjectStoreConfig{Name: "store", Namespace: "ns", Spec: barmanv1.ObjectStoreSpec{}})
	SetObjectStoreRetentionPolicy(obj, "30d")
	if obj.Spec.RetentionPolicy != "30d" {
		t.Errorf("expected RetentionPolicy '30d', got %s", obj.Spec.RetentionPolicy)
	}
}

func TestSetObjectStoreWalConfig(t *testing.T) {
	obj := ObjectStore(&ObjectStoreConfig{Name: "store", Namespace: "ns", Spec: barmanv1.ObjectStoreSpec{}})
	wal := &barmanapi.WalBackupConfiguration{Compression: barmanapi.CompressionTypeGzip}
	SetObjectStoreWalConfig(obj, wal)
	if obj.Spec.Configuration.Wal == nil {
		t.Fatal("expected non-nil Wal config")
	}
	if obj.Spec.Configuration.Wal.Compression != barmanapi.CompressionTypeGzip {
		t.Error("expected Wal Compression to be Gzip")
	}
}

func TestSetObjectStoreDataConfig(t *testing.T) {
	obj := ObjectStore(&ObjectStoreConfig{Name: "store", Namespace: "ns", Spec: barmanv1.ObjectStoreSpec{}})
	data := &barmanapi.DataBackupConfiguration{Compression: barmanapi.CompressionTypeGzip}
	SetObjectStoreDataConfig(obj, data)
	if obj.Spec.Configuration.Data == nil {
		t.Fatal("expected non-nil Data config")
	}
	if obj.Spec.Configuration.Data.Compression != barmanapi.CompressionTypeGzip {
		t.Error("expected Data Compression to be Gzip")
	}
}

func TestAddScheduledBackupLabel(t *testing.T) {
	obj := ScheduledBackup(&ScheduledBackupConfig{Name: "bk", Namespace: "ns", Spec: cnpgv1.ScheduledBackupSpec{}})
	AddScheduledBackupLabel(obj, "env", "prod")
	if obj.Labels["env"] != "prod" {
		t.Error("expected label 'env' to be 'prod'")
	}
}

func TestAddScheduledBackupAnnotation(t *testing.T) {
	obj := ScheduledBackup(&ScheduledBackupConfig{Name: "bk", Namespace: "ns", Spec: cnpgv1.ScheduledBackupSpec{}})
	AddScheduledBackupAnnotation(obj, "note", "value")
	if obj.Annotations["note"] != "value" {
		t.Error("expected annotation 'note' to be 'value'")
	}
}

func TestSetScheduledBackupMethod(t *testing.T) {
	obj := ScheduledBackup(&ScheduledBackupConfig{Name: "bk", Namespace: "ns", Spec: cnpgv1.ScheduledBackupSpec{}})
	SetScheduledBackupMethod(obj, cnpgv1.BackupMethodBarmanObjectStore)
	if obj.Spec.Method != cnpgv1.BackupMethodBarmanObjectStore {
		t.Errorf("expected method BarmanObjectStore, got %s", obj.Spec.Method)
	}
}

func TestSetScheduledBackupPluginConfiguration(t *testing.T) {
	obj := ScheduledBackup(&ScheduledBackupConfig{Name: "bk", Namespace: "ns", Spec: cnpgv1.ScheduledBackupSpec{}})
	params := map[string]string{"key": "value"}
	SetScheduledBackupPluginConfiguration(obj, "barman-cloud.cloudnative-pg.io", params)
	if obj.Spec.PluginConfiguration == nil {
		t.Fatal("expected non-nil PluginConfiguration")
	}
	if obj.Spec.PluginConfiguration.Name != "barman-cloud.cloudnative-pg.io" {
		t.Errorf("expected plugin name 'barman-cloud.cloudnative-pg.io', got %s", obj.Spec.PluginConfiguration.Name)
	}
}

func TestSetScheduledBackupImmediate(t *testing.T) {
	obj := ScheduledBackup(&ScheduledBackupConfig{Name: "bk", Namespace: "ns", Spec: cnpgv1.ScheduledBackupSpec{}})
	SetScheduledBackupImmediate(obj, true)
	if obj.Spec.Immediate == nil || !*obj.Spec.Immediate {
		t.Error("expected Immediate to be true")
	}
}

func TestSetScheduledBackupBackupOwnerReference(t *testing.T) {
	obj := ScheduledBackup(&ScheduledBackupConfig{Name: "bk", Namespace: "ns", Spec: cnpgv1.ScheduledBackupSpec{}})
	SetScheduledBackupBackupOwnerReference(obj, "self")
	if obj.Spec.BackupOwnerReference != "self" {
		t.Errorf("expected BackupOwnerReference 'self', got %s", obj.Spec.BackupOwnerReference)
	}
}

func TestSetScheduledBackupSuspend(t *testing.T) {
	obj := ScheduledBackup(&ScheduledBackupConfig{Name: "bk", Namespace: "ns", Spec: cnpgv1.ScheduledBackupSpec{}})
	SetScheduledBackupSuspend(obj, true)
	if obj.Spec.Suspend == nil || !*obj.Spec.Suspend {
		t.Error("expected Suspend to be true")
	}
}
