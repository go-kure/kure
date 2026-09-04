package cnpg

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	barmanapi "github.com/cloudnative-pg/barman-cloud/pkg/api"
	cnpgv1 "github.com/cloudnative-pg/cloudnative-pg/api/v1"

	"github.com/go-kure/kure/pkg/kubernetes"
)

func mustCluster(t *testing.T, cfg *ClusterConfig) *cnpgv1.Cluster {
	t.Helper()
	obj, err := Cluster(cfg)
	if err != nil {
		t.Fatalf("Cluster: unexpected error: %v", err)
	}
	return obj
}

// TestMetadataViaGenericHelpers covers what the per-kind
// Add<Kind>Label/Add<Kind>Annotation helpers used to do for every kind in this
// package: the generic helpers work over metav1.Object, so one pair reaches all
// four.
func TestMetadataViaGenericHelpers(t *testing.T) {
	objs := []metav1.Object{
		mustCluster(t, &ClusterConfig{Name: "pg", Namespace: "db", Options: &ClusterOptions{}}),
		Database(&DatabaseConfig{Name: "db", Namespace: "ns", Options: &DatabaseOptions{}}),
		ObjectStore(&ObjectStoreConfig{Name: "store", Namespace: "ns", Options: &ObjectStoreOptions{}}),
		ScheduledBackup(&ScheduledBackupConfig{Name: "bk", Namespace: "ns", Spec: cnpgv1.ScheduledBackupSpec{}}),
	}
	for _, obj := range objs {
		kubernetes.AddLabel(obj, "env", "prod")
		kubernetes.AddAnnotation(obj, "note", "value")
		if obj.GetLabels()["env"] != "prod" {
			t.Errorf("%T: expected label env=prod, got %v", obj, obj.GetLabels())
		}
		if obj.GetAnnotations()["note"] != "value" {
			t.Errorf("%T: expected annotation note=value, got %v", obj, obj.GetAnnotations())
		}
	}
}

func TestAddClusterManagedRole(t *testing.T) {
	obj := mustCluster(t, &ClusterConfig{Name: "pg", Namespace: "db", Options: &ClusterOptions{}})
	role := cnpgv1.RoleConfiguration{Name: "app"}
	AddClusterManagedRole(obj, role)
	if len(obj.Spec.Managed.Roles) != 1 {
		t.Fatalf("expected 1 managed role, got %d", len(obj.Spec.Managed.Roles))
	}
	if obj.Spec.Managed.Roles[0].Name != "app" {
		t.Errorf("expected role name 'app', got %s", obj.Spec.Managed.Roles[0].Name)
	}
}

func TestAddDatabaseExtension(t *testing.T) {
	obj := Database(&DatabaseConfig{Name: "db", Namespace: "ns", Options: &DatabaseOptions{}})
	ext := cnpgv1.ExtensionSpec{DatabaseObjectSpec: cnpgv1.DatabaseObjectSpec{Name: "pgcrypto"}}
	AddDatabaseExtension(obj, ext)
	if len(obj.Spec.Extensions) != 1 {
		t.Fatalf("expected 1 extension, got %d", len(obj.Spec.Extensions))
	}
	if obj.Spec.Extensions[0].Name != "pgcrypto" {
		t.Errorf("expected extension name 'pgcrypto', got %s", obj.Spec.Extensions[0].Name)
	}
}

func TestAddObjectStoreEnvVar(t *testing.T) {
	obj := ObjectStore(&ObjectStoreConfig{Name: "store", Namespace: "ns", Options: &ObjectStoreOptions{}})
	env := corev1.EnvVar{Name: "AWS_REGION", Value: "us-east-1"}
	AddObjectStoreEnvVar(obj, env)
	if len(obj.Spec.InstanceSidecarConfiguration.Env) != 1 {
		t.Fatalf("expected 1 env var, got %d", len(obj.Spec.InstanceSidecarConfiguration.Env))
	}
	if obj.Spec.InstanceSidecarConfiguration.Env[0].Name != "AWS_REGION" {
		t.Errorf("expected env var 'AWS_REGION', got %s", obj.Spec.InstanceSidecarConfiguration.Env[0].Name)
	}
}

func TestSetObjectStoreS3Credentials(t *testing.T) {
	obj := ObjectStore(&ObjectStoreConfig{Name: "store", Namespace: "ns", Options: &ObjectStoreOptions{}})
	creds := &barmanapi.S3Credentials{}
	SetObjectStoreS3Credentials(obj, creds)
	if obj.Spec.Configuration.AWS == nil {
		t.Error("expected non-nil S3Credentials (AWS field)")
	}
}

func TestSetObjectStoreWalConfig(t *testing.T) {
	obj := ObjectStore(&ObjectStoreConfig{Name: "store", Namespace: "ns", Options: &ObjectStoreOptions{}})
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
	obj := ObjectStore(&ObjectStoreConfig{Name: "store", Namespace: "ns", Options: &ObjectStoreOptions{}})
	data := &barmanapi.DataBackupConfiguration{Compression: barmanapi.CompressionTypeGzip}
	SetObjectStoreDataConfig(obj, data)
	if obj.Spec.Configuration.Data == nil {
		t.Fatal("expected non-nil Data config")
	}
	if obj.Spec.Configuration.Data.Compression != barmanapi.CompressionTypeGzip {
		t.Error("expected Data Compression to be Gzip")
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

func TestSetScheduledBackupSuspend(t *testing.T) {
	obj := ScheduledBackup(&ScheduledBackupConfig{Name: "bk", Namespace: "ns", Spec: cnpgv1.ScheduledBackupSpec{}})
	SetScheduledBackupSuspend(obj, true)
	if obj.Spec.Suspend == nil || !*obj.Spec.Suspend {
		t.Error("expected Suspend to be true")
	}
}
