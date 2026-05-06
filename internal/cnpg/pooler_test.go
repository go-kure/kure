package cnpg

import (
	"testing"

	cnpgv1 "github.com/cloudnative-pg/cloudnative-pg/api/v1"
)

func TestCreatePooler(t *testing.T) {
	instances := int32(2)
	spec := cnpgv1.PoolerSpec{
		Cluster:   cnpgv1.LocalObjectReference{Name: "pg-main"},
		Type:      cnpgv1.PoolerTypeRW,
		Instances: &instances,
		PgBouncer: &cnpgv1.PgBouncerSpec{},
	}
	obj := CreatePooler("my-pooler", "db-ns", spec)

	if obj == nil {
		t.Fatal("expected non-nil Pooler")
	}
	if obj.Name != "my-pooler" {
		t.Errorf("unexpected name %q", obj.Name)
	}
	if obj.Namespace != "db-ns" {
		t.Errorf("unexpected namespace %q", obj.Namespace)
	}
	if obj.Kind != "Pooler" {
		t.Errorf("unexpected kind %q", obj.Kind)
	}
	if obj.APIVersion != "postgresql.cnpg.io/v1" {
		t.Errorf("unexpected apiVersion %q", obj.APIVersion)
	}
	if obj.Spec.Cluster.Name != "pg-main" {
		t.Errorf("unexpected cluster ref %q", obj.Spec.Cluster.Name)
	}
}

func TestSetPoolerInstances(t *testing.T) {
	obj := CreatePooler("test", "ns", cnpgv1.PoolerSpec{})
	SetPoolerInstances(obj, 3)
	if obj.Spec.Instances == nil {
		t.Fatal("expected non-nil Instances")
	}
	if *obj.Spec.Instances != 3 {
		t.Errorf("unexpected instances %d", *obj.Spec.Instances)
	}
}

func TestSetPoolerClusterRef(t *testing.T) {
	obj := CreatePooler("test", "ns", cnpgv1.PoolerSpec{})
	SetPoolerClusterRef(obj, "pg-cluster")
	if obj.Spec.Cluster.Name != "pg-cluster" {
		t.Errorf("unexpected cluster ref %q", obj.Spec.Cluster.Name)
	}
}

func TestSetPoolerPgBouncerSpec(t *testing.T) {
	obj := CreatePooler("test", "ns", cnpgv1.PoolerSpec{})
	spec := cnpgv1.PgBouncerSpec{
		PoolMode: cnpgv1.PgBouncerPoolModeTransaction,
	}
	SetPoolerPgBouncerSpec(obj, spec)
	if obj.Spec.PgBouncer == nil {
		t.Fatal("expected non-nil PgBouncer")
	}
	if obj.Spec.PgBouncer.PoolMode != cnpgv1.PgBouncerPoolModeTransaction {
		t.Errorf("unexpected pool mode %q", obj.Spec.PgBouncer.PoolMode)
	}
}

func TestAddPoolerLabel(t *testing.T) {
	obj := CreatePooler("test", "ns", cnpgv1.PoolerSpec{})
	AddPoolerLabel(obj, "app", "pgbouncer")
	if obj.Labels["app"] != "pgbouncer" {
		t.Errorf("label not set")
	}
}

func TestAddPoolerAnnotation(t *testing.T) {
	obj := CreatePooler("test", "ns", cnpgv1.PoolerSpec{})
	AddPoolerAnnotation(obj, "team", "dba")
	if obj.Annotations["team"] != "dba" {
		t.Errorf("annotation not set")
	}
}

func TestAddPoolerLabel_InitializesMap(t *testing.T) {
	obj := CreatePooler("test", "ns", cnpgv1.PoolerSpec{})
	if obj.Labels != nil {
		t.Fatal("expected nil Labels before adding label")
	}
	AddPoolerLabel(obj, "key", "val")
	if obj.Labels == nil {
		t.Fatal("expected non-nil Labels after adding label")
	}
}

func TestAddPoolerAnnotation_InitializesMap(t *testing.T) {
	obj := CreatePooler("test", "ns", cnpgv1.PoolerSpec{})
	if obj.Annotations != nil {
		t.Fatal("expected nil Annotations before adding annotation")
	}
	AddPoolerAnnotation(obj, "key", "val")
	if obj.Annotations == nil {
		t.Fatal("expected non-nil Annotations after adding annotation")
	}
}
