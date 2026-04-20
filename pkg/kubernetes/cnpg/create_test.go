package cnpg

import (
	"testing"

	cnpgv1 "github.com/cloudnative-pg/cloudnative-pg/api/v1"
	barmanv1 "github.com/cloudnative-pg/plugin-barman-cloud/api/v1"
)

func TestCluster_Success(t *testing.T) {
	cfg := &ClusterConfig{
		Name:      "pg-main",
		Namespace: "databases",
		Spec:      cnpgv1.ClusterSpec{Instances: 3},
	}

	obj := Cluster(cfg)

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
}

func TestCluster_NilConfig(t *testing.T) {
	obj := Cluster(nil)
	if obj != nil {
		t.Error("expected nil result for nil config")
	}
}

func TestDatabase_Success(t *testing.T) {
	cfg := &DatabaseConfig{
		Name:      "app-db",
		Namespace: "databases",
		Spec:      cnpgv1.DatabaseSpec{Name: "appdb"},
	}

	obj := Database(cfg)

	if obj == nil {
		t.Fatal("expected non-nil Database")
	}
	if obj.Name != "app-db" {
		t.Errorf("expected Name 'app-db', got %s", obj.Name)
	}
	if obj.Namespace != "databases" {
		t.Errorf("expected Namespace 'databases', got %s", obj.Namespace)
	}
	if obj.Spec.Name != "appdb" {
		t.Errorf("expected Spec.Name 'appdb', got %s", obj.Spec.Name)
	}
}

func TestDatabase_NilConfig(t *testing.T) {
	obj := Database(nil)
	if obj != nil {
		t.Error("expected nil result for nil config")
	}
}

func TestObjectStore_Success(t *testing.T) {
	cfg := &ObjectStoreConfig{
		Name:      "backup-store",
		Namespace: "databases",
		Spec:      barmanv1.ObjectStoreSpec{},
	}

	obj := ObjectStore(cfg)

	if obj == nil {
		t.Fatal("expected non-nil ObjectStore")
	}
	if obj.Name != "backup-store" {
		t.Errorf("expected Name 'backup-store', got %s", obj.Name)
	}
	if obj.Namespace != "databases" {
		t.Errorf("expected Namespace 'databases', got %s", obj.Namespace)
	}
}

func TestObjectStore_NilConfig(t *testing.T) {
	obj := ObjectStore(nil)
	if obj != nil {
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
	if obj.Namespace != "databases" {
		t.Errorf("expected Namespace 'databases', got %s", obj.Namespace)
	}
	if obj.Spec.Schedule != "0 2 * * *" {
		t.Errorf("expected Schedule '0 2 * * *', got %s", obj.Spec.Schedule)
	}
}

func TestScheduledBackup_NilConfig(t *testing.T) {
	obj := ScheduledBackup(nil)
	if obj != nil {
		t.Error("expected nil result for nil config")
	}
}
