package kubernetes

import (
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
)

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
	subj := rbacv1.Subject{}

	// All RBAC functions now panic on nil receiver
	assertPanics(t, func() { AddRoleRule(nil, rule) })
	assertPanics(t, func() { AddRoleBindingSubject(nil, subj) })
	assertPanics(t, func() { AddClusterRoleRule(nil, rule) })
	assertPanics(t, func() { AddClusterRoleBindingSubject(nil, subj) })
}
