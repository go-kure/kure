package kubernetes

import (
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
)

func TestCreateRole(t *testing.T) {
	r := CreateRole("my-role", "default")
	if r.Name != "my-role" {
		t.Errorf("expected name my-role, got %s", r.Name)
	}
	if r.Namespace != "default" {
		t.Errorf("expected namespace default, got %s", r.Namespace)
	}
	if r.Kind != "Role" {
		t.Errorf("unexpected kind %q", r.Kind)
	}
	if r.APIVersion != "rbac.authorization.k8s.io/v1" {
		t.Errorf("unexpected apiVersion %q", r.APIVersion)
	}
}

func TestAddRoleRule(t *testing.T) {
	r := CreateRole("r", "ns")
	rule := rbacv1.PolicyRule{
		APIGroups: []string{""},
		Resources: []string{"pods"},
		Verbs:     []string{"get", "list"},
	}
	AddRoleRule(r, rule)
	if len(r.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(r.Rules))
	}
	if r.Rules[0].Resources[0] != "pods" {
		t.Errorf("rule not added correctly")
	}
}

func TestCreateRoleBinding(t *testing.T) {
	rb := CreateRoleBinding("my-rb", "default")
	if rb.Name != "my-rb" {
		t.Errorf("expected name my-rb, got %s", rb.Name)
	}
	if rb.Namespace != "default" {
		t.Errorf("expected namespace default, got %s", rb.Namespace)
	}
	if rb.Kind != "RoleBinding" {
		t.Errorf("unexpected kind %q", rb.Kind)
	}
}

func TestSetRoleBindingRoleRef(t *testing.T) {
	rb := CreateRoleBinding("rb", "ns")
	ref := rbacv1.RoleRef{
		APIGroup: "rbac.authorization.k8s.io",
		Kind:     "Role",
		Name:     "my-role",
	}
	SetRoleBindingRoleRef(rb, ref)
	if rb.RoleRef != ref {
		t.Errorf("role ref not set correctly")
	}
}

func TestAddRoleBindingSubject(t *testing.T) {
	rb := CreateRoleBinding("rb", "ns")
	subj := rbacv1.Subject{
		Kind:      "ServiceAccount",
		Name:      "my-sa",
		Namespace: "ns",
	}
	AddRoleBindingSubject(rb, subj)
	if len(rb.Subjects) != 1 || rb.Subjects[0] != subj {
		t.Errorf("subject not added correctly")
	}
}

func TestCreateClusterRole(t *testing.T) {
	cr := CreateClusterRole("my-cr")
	if cr.Name != "my-cr" {
		t.Errorf("expected name my-cr, got %s", cr.Name)
	}
	if cr.Namespace != "" {
		t.Errorf("expected empty namespace for ClusterRole, got %s", cr.Namespace)
	}
	if cr.Kind != "ClusterRole" {
		t.Errorf("unexpected kind %q", cr.Kind)
	}
}

func TestAddClusterRoleRule(t *testing.T) {
	cr := CreateClusterRole("cr")
	rule := rbacv1.PolicyRule{
		APIGroups: []string{"apps"},
		Resources: []string{"deployments"},
		Verbs:     []string{"get", "list", "watch"},
	}
	AddClusterRoleRule(cr, rule)
	if len(cr.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(cr.Rules))
	}
	if cr.Rules[0].Resources[0] != "deployments" {
		t.Errorf("rule not added correctly")
	}
}

func TestCreateClusterRoleBinding(t *testing.T) {
	crb := CreateClusterRoleBinding("my-crb")
	if crb.Name != "my-crb" {
		t.Errorf("expected name my-crb, got %s", crb.Name)
	}
	if crb.Namespace != "" {
		t.Errorf("expected empty namespace for ClusterRoleBinding, got %s", crb.Namespace)
	}
	if crb.Kind != "ClusterRoleBinding" {
		t.Errorf("unexpected kind %q", crb.Kind)
	}
}

func TestSetClusterRoleBindingRoleRef(t *testing.T) {
	crb := CreateClusterRoleBinding("crb")
	ref := rbacv1.RoleRef{
		APIGroup: "rbac.authorization.k8s.io",
		Kind:     "ClusterRole",
		Name:     "my-cr",
	}
	SetClusterRoleBindingRoleRef(crb, ref)
	if crb.RoleRef != ref {
		t.Errorf("role ref not set correctly")
	}
}

func TestAddClusterRoleBindingSubject(t *testing.T) {
	crb := CreateClusterRoleBinding("crb")
	subj := rbacv1.Subject{
		Kind:      "ServiceAccount",
		Name:      "my-sa",
		Namespace: "ns",
	}
	AddClusterRoleBindingSubject(crb, subj)
	if len(crb.Subjects) != 1 || crb.Subjects[0] != subj {
		t.Errorf("subject not added correctly")
	}
}

func TestRBACNilGuards(t *testing.T) {
	rule := rbacv1.PolicyRule{}
	ref := rbacv1.RoleRef{}
	subj := rbacv1.Subject{}

	// All RBAC functions now panic on nil receiver
	assertPanics(t, func() { AddRoleRule(nil, rule) })
	assertPanics(t, func() { SetRoleBindingRoleRef(nil, ref) })
	assertPanics(t, func() { AddRoleBindingSubject(nil, subj) })
	assertPanics(t, func() { AddClusterRoleRule(nil, rule) })
	assertPanics(t, func() { SetClusterRoleBindingRoleRef(nil, ref) })
	assertPanics(t, func() { AddClusterRoleBindingSubject(nil, subj) })
}
