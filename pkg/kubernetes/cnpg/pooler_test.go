package cnpg

import (
	"testing"

	cnpgv1 "github.com/cloudnative-pg/cloudnative-pg/api/v1"
)

func TestPooler_Success(t *testing.T) {
	obj := Pooler(&PoolerConfig{
		Name:      "pg-pooler",
		Namespace: "databases",
		Options: &PoolerOptions{
			ClusterName: "pg-main",
			Instances:   2,
			Type:        "rw",
		},
	})
	if obj == nil {
		t.Fatal("expected non-nil Pooler")
	}
	if obj.Name != "pg-pooler" {
		t.Errorf("expected Name 'pg-pooler', got %s", obj.Name)
	}
	if obj.Namespace != "databases" {
		t.Errorf("expected Namespace 'databases', got %s", obj.Namespace)
	}
	if obj.Spec.Cluster.Name != "pg-main" {
		t.Errorf("expected Cluster 'pg-main', got %s", obj.Spec.Cluster.Name)
	}
	if obj.Spec.Type != cnpgv1.PoolerTypeRW {
		t.Errorf("expected Type 'rw', got %s", obj.Spec.Type)
	}
	if obj.Spec.Instances == nil || *obj.Spec.Instances != 2 {
		t.Errorf("expected Instances 2, got %v", obj.Spec.Instances)
	}
	if obj.Spec.PgBouncer == nil {
		t.Error("expected non-nil PgBouncer")
	}
}

func TestPooler_TypeRO(t *testing.T) {
	obj := Pooler(&PoolerConfig{
		Name:      "pg-pooler-ro",
		Namespace: "databases",
		Options: &PoolerOptions{
			ClusterName: "pg-main",
			Type:        "ro",
		},
	})
	if obj == nil {
		t.Fatal("expected non-nil Pooler")
	}
	if obj.Spec.Type != cnpgv1.PoolerTypeRO {
		t.Errorf("expected Type 'ro', got %s", obj.Spec.Type)
	}
}

func TestPooler_DefaultTypeRW(t *testing.T) {
	obj := Pooler(&PoolerConfig{
		Name:      "pg-pooler",
		Namespace: "ns",
		Options: &PoolerOptions{
			ClusterName: "pg-main",
		},
	})
	if obj.Spec.Type != cnpgv1.PoolerTypeRW {
		t.Errorf("expected default Type 'rw', got %s", obj.Spec.Type)
	}
}

func TestPooler_WithPgBouncerSpec(t *testing.T) {
	pgBouncer := &cnpgv1.PgBouncerSpec{
		PoolMode: cnpgv1.PgBouncerPoolModeTransaction,
	}
	obj := Pooler(&PoolerConfig{
		Name:      "pg-pooler",
		Namespace: "ns",
		Options: &PoolerOptions{
			ClusterName: "pg-main",
			PgBouncer:   pgBouncer,
		},
	})
	if obj.Spec.PgBouncer == nil {
		t.Fatal("expected non-nil PgBouncer")
	}
	if obj.Spec.PgBouncer.PoolMode != cnpgv1.PgBouncerPoolModeTransaction {
		t.Errorf("expected PoolMode transaction, got %s", obj.Spec.PgBouncer.PoolMode)
	}
}

func TestPooler_ZeroInstances(t *testing.T) {
	obj := Pooler(&PoolerConfig{
		Name:      "pg-pooler",
		Namespace: "ns",
		Options: &PoolerOptions{
			ClusterName: "pg-main",
			Instances:   0,
		},
	})
	if obj.Spec.Instances != nil {
		t.Errorf("expected nil Instances for zero value, got %v", obj.Spec.Instances)
	}
}

func TestPooler_NilOptions(t *testing.T) {
	obj := Pooler(&PoolerConfig{
		Name:      "pg-pooler",
		Namespace: "ns",
		Options:   nil,
	})
	if obj == nil {
		t.Fatal("expected non-nil Pooler")
	}
	if obj.Spec.Type != cnpgv1.PoolerTypeRW {
		t.Errorf("expected default Type 'rw', got %s", obj.Spec.Type)
	}
}

func TestPooler_NilConfig(t *testing.T) {
	if Pooler(nil) != nil {
		t.Error("expected nil result for nil config")
	}
}

func TestPooler_TypeMeta(t *testing.T) {
	obj := Pooler(&PoolerConfig{
		Name:      "pg-pooler",
		Namespace: "ns",
		Options:   &PoolerOptions{ClusterName: "pg"},
	})
	if obj.Kind != "Pooler" {
		t.Errorf("expected Kind 'Pooler', got %s", obj.Kind)
	}
	if obj.APIVersion != "postgresql.cnpg.io/v1" {
		t.Errorf("expected APIVersion 'postgresql.cnpg.io/v1', got %s", obj.APIVersion)
	}
}
