package cnpg

import (
	"testing"

	cnpgv1 "github.com/cloudnative-pg/cloudnative-pg/api/v1"
	corev1 "k8s.io/api/core/v1"
)

func TestCreateDatabase(t *testing.T) {
	spec := cnpgv1.DatabaseSpec{
		Name:       "mydb",
		Owner:      "app",
		ClusterRef: corev1.LocalObjectReference{Name: "pg-cluster"},
	}
	db := CreateDatabase("mydb", "default", spec)

	if db == nil {
		t.Fatal("expected non-nil Database")
	}
	if db.Name != "mydb" {
		t.Errorf("expected name %q, got %q", "mydb", db.Name)
	}
	if db.Namespace != "default" {
		t.Errorf("expected namespace %q, got %q", "default", db.Namespace)
	}
	if db.Kind != "Database" {
		t.Errorf("expected kind %q, got %q", "Database", db.Kind)
	}
	if db.APIVersion != "postgresql.cnpg.io/v1" {
		t.Errorf("expected apiVersion %q, got %q", "postgresql.cnpg.io/v1", db.APIVersion)
	}
	if db.Spec.Name != "mydb" {
		t.Errorf("expected spec.name %q, got %q", "mydb", db.Spec.Name)
	}
	if db.Spec.Owner != "app" {
		t.Errorf("expected spec.owner %q, got %q", "app", db.Spec.Owner)
	}
	if db.Spec.ClusterRef.Name != "pg-cluster" {
		t.Errorf("expected spec.cluster %q, got %q", "pg-cluster", db.Spec.ClusterRef.Name)
	}
}

func TestDatabaseFunctions(t *testing.T) {
	spec := cnpgv1.DatabaseSpec{
		Name:       "testdb",
		Owner:      "owner",
		ClusterRef: corev1.LocalObjectReference{Name: "cluster"},
	}
	db := CreateDatabase("testdb", "ns", spec)

	if err := AddDatabaseLabel(db, "app", "demo"); err != nil {
		t.Errorf("AddDatabaseLabel failed: %v", err)
	}
	if db.Labels["app"] != "demo" {
		t.Errorf("label not set")
	}

	if err := AddDatabaseAnnotation(db, "team", "dev"); err != nil {
		t.Errorf("AddDatabaseAnnotation failed: %v", err)
	}
	if db.Annotations["team"] != "dev" {
		t.Errorf("annotation not set")
	}

	ext := cnpgv1.ExtensionSpec{
		DatabaseObjectSpec: cnpgv1.DatabaseObjectSpec{
			Name:   "pg_stat_statements",
			Ensure: cnpgv1.EnsurePresent,
		},
	}
	if err := AddDatabaseExtension(db, ext); err != nil {
		t.Errorf("AddDatabaseExtension failed: %v", err)
	}
	if len(db.Spec.Extensions) != 1 || db.Spec.Extensions[0].Name != "pg_stat_statements" {
		t.Errorf("extension not added")
	}

	if err := SetDatabaseClusterRef(db, "new-cluster"); err != nil {
		t.Errorf("SetDatabaseClusterRef failed: %v", err)
	}
	if db.Spec.ClusterRef.Name != "new-cluster" {
		t.Errorf("cluster ref not set")
	}

	if err := SetDatabaseOwner(db, "new-owner"); err != nil {
		t.Errorf("SetDatabaseOwner failed: %v", err)
	}
	if db.Spec.Owner != "new-owner" {
		t.Errorf("owner not set")
	}

	if err := SetDatabaseReclaimPolicy(db, cnpgv1.DatabaseReclaimDelete); err != nil {
		t.Errorf("SetDatabaseReclaimPolicy failed: %v", err)
	}
	if db.Spec.ReclaimPolicy != cnpgv1.DatabaseReclaimDelete {
		t.Errorf("reclaim policy not set")
	}

	if err := SetDatabaseEnsure(db, cnpgv1.EnsurePresent); err != nil {
		t.Errorf("SetDatabaseEnsure failed: %v", err)
	}
	if db.Spec.Ensure != cnpgv1.EnsurePresent {
		t.Errorf("ensure not set")
	}
}

func TestDatabaseFunctionsWithNil(t *testing.T) {
	if err := AddDatabaseLabel(nil, "key", "value"); err == nil {
		t.Error("expected error for nil Database")
	}
	if err := AddDatabaseAnnotation(nil, "key", "value"); err == nil {
		t.Error("expected error for nil Database")
	}
	if err := AddDatabaseExtension(nil, cnpgv1.ExtensionSpec{}); err == nil {
		t.Error("expected error for nil Database")
	}
	if err := SetDatabaseClusterRef(nil, "cluster"); err == nil {
		t.Error("expected error for nil Database")
	}
	if err := SetDatabaseOwner(nil, "owner"); err == nil {
		t.Error("expected error for nil Database")
	}
	if err := SetDatabaseReclaimPolicy(nil, cnpgv1.DatabaseReclaimRetain); err == nil {
		t.Error("expected error for nil Database")
	}
	if err := SetDatabaseEnsure(nil, cnpgv1.EnsurePresent); err == nil {
		t.Error("expected error for nil Database")
	}
}
