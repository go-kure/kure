package kubernetes

import (
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateRole(name, namespace string) *rbacv1.Role {
	return &rbacv1.Role{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Role",
			APIVersion: rbacv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func AddRoleRule(role *rbacv1.Role, rule rbacv1.PolicyRule) {
	if role == nil {
		panic("AddRoleRule: role must not be nil")
	}
	role.Rules = append(role.Rules, rule)
}

func CreateRoleBinding(name, namespace string) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			Kind:       "RoleBinding",
			APIVersion: rbacv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func SetRoleBindingRoleRef(rb *rbacv1.RoleBinding, roleRef rbacv1.RoleRef) {
	if rb == nil {
		panic("SetRoleBindingRoleRef: rb must not be nil")
	}
	rb.RoleRef = roleRef
}

func AddRoleBindingSubject(rb *rbacv1.RoleBinding, subject rbacv1.Subject) {
	if rb == nil {
		panic("AddRoleBindingSubject: rb must not be nil")
	}
	rb.Subjects = append(rb.Subjects, subject)
}

func CreateClusterRole(name string) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterRole",
			APIVersion: rbacv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

func AddClusterRoleRule(cr *rbacv1.ClusterRole, rule rbacv1.PolicyRule) {
	if cr == nil {
		panic("AddClusterRoleRule: cr must not be nil")
	}
	cr.Rules = append(cr.Rules, rule)
}

func CreateClusterRoleBinding(name string) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterRoleBinding",
			APIVersion: rbacv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

func SetClusterRoleBindingRoleRef(crb *rbacv1.ClusterRoleBinding, roleRef rbacv1.RoleRef) {
	if crb == nil {
		panic("SetClusterRoleBindingRoleRef: crb must not be nil")
	}
	crb.RoleRef = roleRef
}

func AddClusterRoleBindingSubject(crb *rbacv1.ClusterRoleBinding, subject rbacv1.Subject) {
	if crb == nil {
		panic("AddClusterRoleBindingSubject: crb must not be nil")
	}
	crb.Subjects = append(crb.Subjects, subject)
}
