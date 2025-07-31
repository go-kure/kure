package kubernetes

import (
	"reflect"
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCreateRole(t *testing.T) {
	r := CreateRole("r", "ns")
	if r.Name != "r" || r.Namespace != "ns" {
		t.Fatalf("metadata mismatch: %s/%s", r.Namespace, r.Name)
	}
	if r.Kind != "Role" {
		t.Errorf("unexpected kind %q", r.Kind)
	}
	if len(r.Rules) != 0 {
		t.Errorf("expected no rules")
	}
}

func TestRoleRuleFunctions(t *testing.T) {
	r := CreateRole("r", "ns")
	rule := rbacv1.PolicyRule{APIGroups: []string{""}, Verbs: []string{"get"}, Resources: []string{"pods"}}
	AddRoleRule(r, rule)
	if len(r.Rules) != 1 || !reflect.DeepEqual(r.Rules[0], rule) {
		t.Errorf("rule not added")
	}
	newRules := []rbacv1.PolicyRule{{Verbs: []string{"list"}}}
	SetRoleRules(r, newRules)
	if !reflect.DeepEqual(r.Rules, newRules) {
		t.Errorf("rules not set")
	}
}

func TestRoleMetadataFunctions(t *testing.T) {
	r := CreateRole("r", "ns")
	AddRoleLabel(r, "k", "v")
	if r.Labels["k"] != "v" {
		t.Errorf("label not added")
	}
	AddRoleAnnotation(r, "a", "b")
	if r.Annotations["a"] != "b" {
		t.Errorf("annotation not added")
	}
	SetRoleLabels(r, map[string]string{"x": "y"})
	if !reflect.DeepEqual(r.Labels, map[string]string{"x": "y"}) {
		t.Errorf("labels not set")
	}
	SetRoleAnnotations(r, map[string]string{"c": "d"})
	if !reflect.DeepEqual(r.Annotations, map[string]string{"c": "d"}) {
		t.Errorf("annotations not set")
	}
}

func TestCreateRoleBinding(t *testing.T) {
	rb := CreateRoleBinding("rb", "ns", rbacv1.RoleRef{Kind: "Role", Name: "r"})
	if rb.Name != "rb" || rb.Namespace != "ns" {
		t.Fatalf("metadata mismatch: %s/%s", rb.Namespace, rb.Name)
	}
	if rb.Kind != "RoleBinding" {
		t.Errorf("unexpected kind %q", rb.Kind)
	}
	if rb.RoleRef.Name != "r" {
		t.Errorf("roleRef not set")
	}
	if len(rb.Subjects) != 0 {
		t.Errorf("expected no subjects")
	}
}

func TestRoleBindingFunctions(t *testing.T) {
	rb := CreateRoleBinding("rb", "ns", rbacv1.RoleRef{Kind: "Role", Name: "r"})
	sub := rbacv1.Subject{Kind: "ServiceAccount", Name: "sa"}
	AddRoleBindingSubject(rb, sub)
	if len(rb.Subjects) != 1 || rb.Subjects[0] != sub {
		t.Errorf("subject not added")
	}
	SetRoleBindingSubjects(rb, []rbacv1.Subject{sub})
	if len(rb.Subjects) != 1 || rb.Subjects[0] != sub {
		t.Errorf("subjects not set")
	}
	SetRoleBindingRoleRef(rb, rbacv1.RoleRef{Kind: "ClusterRole", Name: "cr"})
	if rb.RoleRef.Name != "cr" {
		t.Errorf("roleref not updated")
	}
	AddRoleBindingLabel(rb, "k", "v")
	if rb.Labels["k"] != "v" {
		t.Errorf("label not added")
	}
	AddRoleBindingAnnotation(rb, "a", "b")
	if rb.Annotations["a"] != "b" {
		t.Errorf("annotation not added")
	}
	SetRoleBindingLabels(rb, map[string]string{"x": "y"})
	if !reflect.DeepEqual(rb.Labels, map[string]string{"x": "y"}) {
		t.Errorf("labels not set")
	}
	SetRoleBindingAnnotations(rb, map[string]string{"c": "d"})
	if !reflect.DeepEqual(rb.Annotations, map[string]string{"c": "d"}) {
		t.Errorf("annotations not set")
	}
}

func TestCreateClusterRole(t *testing.T) {
	cr := CreateClusterRole("cr")
	if cr.Name != "cr" || cr.Namespace != "" {
		t.Fatalf("metadata mismatch: %s/%s", cr.Namespace, cr.Name)
	}
	if cr.Kind != "ClusterRole" {
		t.Errorf("unexpected kind %q", cr.Kind)
	}
	if len(cr.Rules) != 0 {
		t.Errorf("expected no rules")
	}
}

func TestClusterRoleFunctions(t *testing.T) {
	cr := CreateClusterRole("cr")
	rule := rbacv1.PolicyRule{Verbs: []string{"get"}, Resources: []string{"pods"}}
	AddClusterRoleRule(cr, rule)
	if len(cr.Rules) != 1 || !reflect.DeepEqual(cr.Rules[0], rule) {
		t.Errorf("rule not added")
	}
	SetClusterRoleRules(cr, []rbacv1.PolicyRule{rule})
	if len(cr.Rules) != 1 {
		t.Errorf("rules not set")
	}
	ar := &rbacv1.AggregationRule{ClusterRoleSelectors: []metav1.LabelSelector{{MatchLabels: map[string]string{"k": "v"}}}}
	SetClusterRoleAggregationRule(cr, ar)
	if cr.AggregationRule != ar {
		t.Errorf("aggregation rule not set")
	}
	AddClusterRoleLabel(cr, "k", "v")
	if cr.Labels["k"] != "v" {
		t.Errorf("label not added")
	}
	AddClusterRoleAnnotation(cr, "a", "b")
	if cr.Annotations["a"] != "b" {
		t.Errorf("annotation not added")
	}
	SetClusterRoleLabels(cr, map[string]string{"x": "y"})
	if !reflect.DeepEqual(cr.Labels, map[string]string{"x": "y"}) {
		t.Errorf("labels not set")
	}
	SetClusterRoleAnnotations(cr, map[string]string{"c": "d"})
	if !reflect.DeepEqual(cr.Annotations, map[string]string{"c": "d"}) {
		t.Errorf("annotations not set")
	}
}

func TestCreateClusterRoleBinding(t *testing.T) {
	crb := CreateClusterRoleBinding("crb", rbacv1.RoleRef{Kind: "ClusterRole", Name: "cr"})
	if crb.Name != "crb" {
		t.Fatalf("expected name crb got %s", crb.Name)
	}
	if crb.Kind != "ClusterRoleBinding" {
		t.Errorf("unexpected kind %q", crb.Kind)
	}
	if crb.RoleRef.Name != "cr" {
		t.Errorf("roleRef not set")
	}
	if len(crb.Subjects) != 0 {
		t.Errorf("expected no subjects")
	}
}

func TestClusterRoleBindingFunctions(t *testing.T) {
	crb := CreateClusterRoleBinding("crb", rbacv1.RoleRef{Kind: "ClusterRole", Name: "cr"})
	sub := rbacv1.Subject{Kind: "User", Name: "bob"}
	AddClusterRoleBindingSubject(crb, sub)
	if len(crb.Subjects) != 1 || crb.Subjects[0] != sub {
		t.Errorf("subject not added")
	}
	SetClusterRoleBindingSubjects(crb, []rbacv1.Subject{sub})
	if len(crb.Subjects) != 1 || crb.Subjects[0] != sub {
		t.Errorf("subjects not set")
	}
	SetClusterRoleBindingRoleRef(crb, rbacv1.RoleRef{Kind: "Role", Name: "r"})
	if crb.RoleRef.Name != "r" {
		t.Errorf("roleref not updated")
	}
	AddClusterRoleBindingLabel(crb, "k", "v")
	if crb.Labels["k"] != "v" {
		t.Errorf("label not added")
	}
	AddClusterRoleBindingAnnotation(crb, "a", "b")
	if crb.Annotations["a"] != "b" {
		t.Errorf("annotation not added")
	}
	SetClusterRoleBindingLabels(crb, map[string]string{"x": "y"})
	if !reflect.DeepEqual(crb.Labels, map[string]string{"x": "y"}) {
		t.Errorf("labels not set")
	}
	SetClusterRoleBindingAnnotations(crb, map[string]string{"c": "d"})
	if !reflect.DeepEqual(crb.Annotations, map[string]string{"c": "d"}) {
		t.Errorf("annotations not set")
	}
}
