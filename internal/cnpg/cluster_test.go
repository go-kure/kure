package cnpg

import (
	"testing"

	cnpgv1 "github.com/cloudnative-pg/cloudnative-pg/api/v1"
)

func TestCreateCluster(t *testing.T) {
	spec := cnpgv1.ClusterSpec{
		Instances: 3,
	}
	obj := CreateCluster("my-cluster", "db-ns", spec)

	if obj == nil {
		t.Fatal("expected non-nil Cluster")
	}
	if obj.Name != "my-cluster" {
		t.Errorf("unexpected name %q", obj.Name)
	}
	if obj.Namespace != "db-ns" {
		t.Errorf("unexpected namespace %q", obj.Namespace)
	}
	if obj.Kind != "Cluster" {
		t.Errorf("unexpected kind %q", obj.Kind)
	}
	if obj.APIVersion != "postgresql.cnpg.io/v1" {
		t.Errorf("unexpected apiVersion %q", obj.APIVersion)
	}
	if obj.Spec.Instances != 3 {
		t.Errorf("unexpected instances %d", obj.Spec.Instances)
	}
}

func TestAddClusterLabel(t *testing.T) {
	obj := CreateCluster("test", "ns", cnpgv1.ClusterSpec{})
	if err := AddClusterLabel(obj, "app", "postgres"); err != nil {
		t.Fatalf("AddClusterLabel failed: %v", err)
	}
	if obj.Labels["app"] != "postgres" {
		t.Errorf("label not set")
	}
}

func TestAddClusterAnnotation(t *testing.T) {
	obj := CreateCluster("test", "ns", cnpgv1.ClusterSpec{})
	if err := AddClusterAnnotation(obj, "team", "dba"); err != nil {
		t.Fatalf("AddClusterAnnotation failed: %v", err)
	}
	if obj.Annotations["team"] != "dba" {
		t.Errorf("annotation not set")
	}
}

func TestAddClusterManagedRole(t *testing.T) {
	obj := CreateCluster("test", "ns", cnpgv1.ClusterSpec{})

	loginTrue := true
	role := cnpgv1.RoleConfiguration{
		Name:     "app_user",
		Ensure:   cnpgv1.EnsurePresent,
		Login:    true,
		CreateDB: false,
		PasswordSecret: &cnpgv1.LocalObjectReference{
			Name: "app-user-password",
		},
		Inherit: &loginTrue,
	}
	if err := AddClusterManagedRole(obj, role); err != nil {
		t.Fatalf("AddClusterManagedRole failed: %v", err)
	}
	if obj.Spec.Managed == nil {
		t.Fatal("expected non-nil Managed")
	}
	if len(obj.Spec.Managed.Roles) != 1 {
		t.Fatalf("expected 1 role, got %d", len(obj.Spec.Managed.Roles))
	}
	r := obj.Spec.Managed.Roles[0]
	if r.Name != "app_user" {
		t.Errorf("unexpected role name %q", r.Name)
	}
	if r.Ensure != cnpgv1.EnsurePresent {
		t.Errorf("unexpected ensure %q", r.Ensure)
	}
	if !r.Login {
		t.Error("expected login to be true")
	}
	if r.PasswordSecret == nil || r.PasswordSecret.Name != "app-user-password" {
		t.Error("unexpected passwordSecret")
	}
}

func TestAddClusterManagedRoleSuperuser(t *testing.T) {
	obj := CreateCluster("test", "ns", cnpgv1.ClusterSpec{})

	role := cnpgv1.RoleConfiguration{
		Name:      "admin",
		Ensure:    cnpgv1.EnsurePresent,
		Login:     true,
		Superuser: true,
		CreateDB:  true,
	}
	if err := AddClusterManagedRole(obj, role); err != nil {
		t.Fatalf("AddClusterManagedRole failed: %v", err)
	}
	r := obj.Spec.Managed.Roles[0]
	if !r.Superuser {
		t.Error("expected superuser to be true")
	}
	if !r.CreateDB {
		t.Error("expected createdb to be true")
	}
}

func TestAddClusterManagedRoleAbsent(t *testing.T) {
	obj := CreateCluster("test", "ns", cnpgv1.ClusterSpec{})

	role := cnpgv1.RoleConfiguration{
		Name:   "old_user",
		Ensure: cnpgv1.EnsureAbsent,
	}
	if err := AddClusterManagedRole(obj, role); err != nil {
		t.Fatalf("AddClusterManagedRole failed: %v", err)
	}
	if obj.Spec.Managed.Roles[0].Ensure != cnpgv1.EnsureAbsent {
		t.Errorf("unexpected ensure %q", obj.Spec.Managed.Roles[0].Ensure)
	}
}

func TestAddClusterManagedRoleMultiple(t *testing.T) {
	obj := CreateCluster("test", "ns", cnpgv1.ClusterSpec{})

	roles := []cnpgv1.RoleConfiguration{
		{Name: "app_user", Ensure: cnpgv1.EnsurePresent, Login: true},
		{Name: "readonly", Ensure: cnpgv1.EnsurePresent, Login: true},
		{Name: "admin", Ensure: cnpgv1.EnsurePresent, Login: true, Superuser: true},
	}
	for _, role := range roles {
		if err := AddClusterManagedRole(obj, role); err != nil {
			t.Fatalf("AddClusterManagedRole failed for %s: %v", role.Name, err)
		}
	}
	if len(obj.Spec.Managed.Roles) != 3 {
		t.Fatalf("expected 3 roles, got %d", len(obj.Spec.Managed.Roles))
	}
	if obj.Spec.Managed.Roles[0].Name != "app_user" {
		t.Errorf("unexpected first role %q", obj.Spec.Managed.Roles[0].Name)
	}
	if obj.Spec.Managed.Roles[2].Superuser != true {
		t.Error("expected third role to be superuser")
	}
}

func TestAddClusterManagedRoleInitializesManaged(t *testing.T) {
	obj := CreateCluster("test", "ns", cnpgv1.ClusterSpec{})
	if obj.Spec.Managed != nil {
		t.Fatal("expected nil Managed before adding role")
	}

	role := cnpgv1.RoleConfiguration{Name: "test_user"}
	if err := AddClusterManagedRole(obj, role); err != nil {
		t.Fatalf("AddClusterManagedRole failed: %v", err)
	}
	if obj.Spec.Managed == nil {
		t.Fatal("expected non-nil Managed after adding role")
	}
}

func TestClusterNilGuards(t *testing.T) {
	if err := AddClusterLabel(nil, "key", "value"); err == nil {
		t.Error("expected error for nil Cluster")
	}
	if err := AddClusterAnnotation(nil, "key", "value"); err == nil {
		t.Error("expected error for nil Cluster")
	}
	role := cnpgv1.RoleConfiguration{Name: "test"}
	if err := AddClusterManagedRole(nil, role); err == nil {
		t.Error("expected error for nil Cluster")
	}
}
