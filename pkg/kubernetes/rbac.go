package kubernetes

import (
	rbacv1 "k8s.io/api/rbac/v1"
)

func AddRoleRule(role *rbacv1.Role, rule rbacv1.PolicyRule) {
	if role == nil {
		panic("AddRoleRule: role must not be nil")
	}
	role.Rules = append(role.Rules, rule)
}

func AddRoleBindingSubject(rb *rbacv1.RoleBinding, subject rbacv1.Subject) {
	if rb == nil {
		panic("AddRoleBindingSubject: rb must not be nil")
	}
	rb.Subjects = append(rb.Subjects, subject)
}

func AddClusterRoleRule(cr *rbacv1.ClusterRole, rule rbacv1.PolicyRule) {
	if cr == nil {
		panic("AddClusterRoleRule: cr must not be nil")
	}
	cr.Rules = append(cr.Rules, rule)
}

func AddClusterRoleBindingSubject(crb *rbacv1.ClusterRoleBinding, subject rbacv1.Subject) {
	if crb == nil {
		panic("AddClusterRoleBindingSubject: crb must not be nil")
	}
	crb.Subjects = append(crb.Subjects, subject)
}
