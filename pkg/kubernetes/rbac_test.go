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
	if err := AddRoleRule(r, rule); err != nil {
		t.Fatalf("AddRoleRule returned error: %v", err)
	}
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
	if err := SetRoleBindingRoleRef(rb, ref); err != nil {
		t.Fatalf("SetRoleBindingRoleRef returned error: %v", err)
	}
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
	if err := AddRoleBindingSubject(rb, subj); err != nil {
		t.Fatalf("AddRoleBindingSubject returned error: %v", err)
	}
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
	if err := AddClusterRoleRule(cr, rule); err != nil {
		t.Fatalf("AddClusterRoleRule returned error: %v", err)
	}
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
	if err := SetClusterRoleBindingRoleRef(crb, ref); err != nil {
		t.Fatalf("SetClusterRoleBindingRoleRef returned error: %v", err)
	}
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
	if err := AddClusterRoleBindingSubject(crb, subj); err != nil {
		t.Fatalf("AddClusterRoleBindingSubject returned error: %v", err)
	}
	if len(crb.Subjects) != 1 || crb.Subjects[0] != subj {
		t.Errorf("subject not added correctly")
	}
}

func TestRBACNilGuards(t *testing.T) {
	rule := rbacv1.PolicyRule{}
	ref := rbacv1.RoleRef{}
	subj := rbacv1.Subject{}

	tests := []struct {
		name string
		fn   func() error
	}{
		{"AddRoleRule", func() error { return AddRoleRule(nil, rule) }},
		{"SetRoleBindingRoleRef", func() error { return SetRoleBindingRoleRef(nil, ref) }},
		{"AddRoleBindingSubject", func() error { return AddRoleBindingSubject(nil, subj) }},
		{"AddClusterRoleRule", func() error { return AddClusterRoleRule(nil, rule) }},
		{"SetClusterRoleBindingRoleRef", func() error { return SetClusterRoleBindingRoleRef(nil, ref) }},
		{"AddClusterRoleBindingSubject", func() error { return AddClusterRoleBindingSubject(nil, subj) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.fn(); err == nil {
				t.Errorf("%s(nil) should return error", tt.name)
			}
		})
	}
}
