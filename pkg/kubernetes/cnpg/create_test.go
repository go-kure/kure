package cnpg

import (
	"testing"

	cnpgv1 "github.com/cloudnative-pg/cloudnative-pg/api/v1"
)

func TestCluster_Success(t *testing.T) {
	obj, err := Cluster(&ClusterConfig{
		Name:      "pg-main",
		Namespace: "databases",
		Options: &ClusterOptions{
			Instances:   3,
			ImageName:   "ghcr.io/cloudnative-pg/postgresql:16",
			StorageSize: "10Gi",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if obj == nil {
		t.Fatal("expected non-nil Cluster")
	}
	if obj.Name != "pg-main" {
		t.Errorf("expected Name 'pg-main', got %s", obj.Name)
	}
	if obj.Namespace != "databases" {
		t.Errorf("expected Namespace 'databases', got %s", obj.Namespace)
	}
	if obj.Spec.Instances != 3 {
		t.Errorf("expected Instances 3, got %d", obj.Spec.Instances)
	}
	if obj.Spec.StorageConfiguration.Size != "10Gi" {
		t.Errorf("expected StorageSize '10Gi', got %s", obj.Spec.StorageConfiguration.Size)
	}
}

func TestCluster_EnablePDB(t *testing.T) {
	single, err := Cluster(&ClusterConfig{
		Name: "pg", Namespace: "ns",
		Options: &ClusterOptions{Instances: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if *single.Spec.EnablePDB {
		t.Error("single-instance cluster should have EnablePDB=false")
	}

	multi, err := Cluster(&ClusterConfig{
		Name: "pg", Namespace: "ns",
		Options: &ClusterOptions{Instances: 3},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !*multi.Spec.EnablePDB {
		t.Error("multi-instance cluster should have EnablePDB=true")
	}
}

func TestCluster_ManagedRoles(t *testing.T) {
	obj, err := Cluster(&ClusterConfig{
		Name: "pg", Namespace: "ns",
		Options: &ClusterOptions{
			Instances: 1,
			ManagedRoles: []ManagedRoleOptions{
				{Name: "app_user", Login: true, PasswordSecret: "app-creds", Comment: "Application user"},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if obj.Spec.Managed == nil || len(obj.Spec.Managed.Roles) != 1 {
		t.Fatal("expected 1 managed role")
	}
	role := obj.Spec.Managed.Roles[0]
	if role.Name != "app_user" || !role.Login || role.Comment != "Application user" {
		t.Errorf("unexpected role: %+v", role)
	}
	if role.PasswordSecret == nil || role.PasswordSecret.Name != "app-creds" {
		t.Errorf("expected PasswordSecret 'app-creds', got %v", role.PasswordSecret)
	}
}

func TestCluster_ObjectStorePlugin(t *testing.T) {
	obj, err := Cluster(&ClusterConfig{
		Name: "pg", Namespace: "ns",
		Options: &ClusterOptions{
			Instances:       1,
			ObjectStoreName: "pg-backup",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(obj.Spec.Plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(obj.Spec.Plugins))
	}
	if obj.Spec.Plugins[0].Parameters["objectStoreName"] != "pg-backup" {
		t.Errorf("unexpected plugin params: %v", obj.Spec.Plugins[0].Parameters)
	}
}

func TestCluster_NilConfig(t *testing.T) {
	obj, err := Cluster(nil)
	if err != nil {
		t.Fatal(err)
	}
	if obj != nil {
		t.Error("expected nil result for nil config")
	}
}

func TestDatabase_Success(t *testing.T) {
	obj := Database(&DatabaseConfig{
		Name:      "pg-appdb",
		Namespace: "databases",
		Options: &DatabaseOptions{
			ClusterName: "pg",
			DBName:      "appdb",
			Owner:       "app_user",
		},
	})
	if obj == nil {
		t.Fatal("expected non-nil Database")
	}
	if obj.Name != "pg-appdb" {
		t.Errorf("expected Name 'pg-appdb', got %s", obj.Name)
	}
	if obj.Spec.Name != "appdb" {
		t.Errorf("expected Spec.Name 'appdb', got %s", obj.Spec.Name)
	}
	if obj.Spec.Owner != "app_user" {
		t.Errorf("expected Owner 'app_user', got %s", obj.Spec.Owner)
	}
}

func TestDatabase_Extensions(t *testing.T) {
	obj := Database(&DatabaseConfig{
		Name: "pg-db", Namespace: "ns",
		Options: &DatabaseOptions{
			ClusterName: "pg",
			DBName:      "db",
			Extensions: []ExtensionOptions{
				{Name: "pg_stat_statements", Ensure: ""},
				{Name: "pgvector", Ensure: "absent"},
			},
		},
	})
	if len(obj.Spec.Extensions) != 2 {
		t.Fatalf("expected 2 extensions, got %d", len(obj.Spec.Extensions))
	}
	if obj.Spec.Extensions[0].Ensure != cnpgv1.EnsurePresent {
		t.Errorf("expected first extension ensure=present, got %s", obj.Spec.Extensions[0].Ensure)
	}
	if obj.Spec.Extensions[1].Ensure != cnpgv1.EnsureAbsent {
		t.Errorf("expected second extension ensure=absent, got %s", obj.Spec.Extensions[1].Ensure)
	}
}

func TestDatabase_NilConfig(t *testing.T) {
	if Database(nil) != nil {
		t.Error("expected nil result for nil config")
	}
}

func TestObjectStore_Success(t *testing.T) {
	obj := ObjectStore(&ObjectStoreConfig{
		Name:      "backup-store",
		Namespace: "databases",
		Options: &ObjectStoreOptions{
			DestinationPath: "s3://my-bucket/pg/",
			RetentionPolicy: "30d",
			SecretName:      "backup-creds",
		},
	})
	if obj == nil {
		t.Fatal("expected non-nil ObjectStore")
	}
	if obj.Name != "backup-store" {
		t.Errorf("expected Name 'backup-store', got %s", obj.Name)
	}
	if obj.Spec.Configuration.DestinationPath != "s3://my-bucket/pg/" {
		t.Errorf("unexpected DestinationPath: %s", obj.Spec.Configuration.DestinationPath)
	}
	if obj.Spec.Configuration.AWS == nil {
		t.Fatal("expected S3 credentials to be set")
	}
	if obj.Spec.Configuration.AWS.AccessKeyIDReference.Key != "ACCESS_KEY_ID" {
		t.Errorf("unexpected access key ID key: %s", obj.Spec.Configuration.AWS.AccessKeyIDReference.Key)
	}
}

func TestObjectStore_NoCredentials(t *testing.T) {
	obj := ObjectStore(&ObjectStoreConfig{
		Name: "store", Namespace: "ns",
		Options: &ObjectStoreOptions{DestinationPath: "s3://bucket/path/"},
	})
	if obj.Spec.Configuration.AWS != nil {
		t.Error("expected no S3 credentials when SecretName is empty")
	}
}

func TestObjectStore_NilConfig(t *testing.T) {
	if ObjectStore(nil) != nil {
		t.Error("expected nil result for nil config")
	}
}

func TestScheduledBackup_Success(t *testing.T) {
	cfg := &ScheduledBackupConfig{
		Name:      "daily-backup",
		Namespace: "databases",
		Spec:      cnpgv1.ScheduledBackupSpec{Schedule: "0 2 * * *"},
	}

	obj := ScheduledBackup(cfg)

	if obj == nil {
		t.Fatal("expected non-nil ScheduledBackup")
	}
	if obj.Name != "daily-backup" {
		t.Errorf("expected Name 'daily-backup', got %s", obj.Name)
	}
	if obj.Spec.Schedule != "0 2 * * *" {
		t.Errorf("expected Schedule '0 2 * * *', got %s", obj.Spec.Schedule)
	}
}

func TestScheduledBackup_NilConfig(t *testing.T) {
	if ScheduledBackup(nil) != nil {
		t.Error("expected nil result for nil config")
	}
}
