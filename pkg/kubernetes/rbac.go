package kubernetes

import (
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/go-kure/kure/pkg/errors"
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

func AddRoleRule(role *rbacv1.Role, rule rbacv1.PolicyRule) error {
	if role == nil {
		return errors.ErrNilRole
	}
	role.Rules = append(role.Rules, rule)
	return nil
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

func SetRoleBindingRoleRef(rb *rbacv1.RoleBinding, roleRef rbacv1.RoleRef) error {
	if rb == nil {
		return errors.ErrNilRoleBinding
	}
	rb.RoleRef = roleRef
	return nil
}

func AddRoleBindingSubject(rb *rbacv1.RoleBinding, subject rbacv1.Subject) error {
	if rb == nil {
		return errors.ErrNilRoleBinding
	}
	rb.Subjects = append(rb.Subjects, subject)
	return nil
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

func AddClusterRoleRule(cr *rbacv1.ClusterRole, rule rbacv1.PolicyRule) error {
	if cr == nil {
		return errors.ErrNilClusterRole
	}
	cr.Rules = append(cr.Rules, rule)
	return nil
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

func SetClusterRoleBindingRoleRef(crb *rbacv1.ClusterRoleBinding, roleRef rbacv1.RoleRef) error {
	if crb == nil {
		return errors.ErrNilClusterRoleBinding
	}
	crb.RoleRef = roleRef
	return nil
}

func AddClusterRoleBindingSubject(crb *rbacv1.ClusterRoleBinding, subject rbacv1.Subject) error {
	if crb == nil {
		return errors.ErrNilClusterRoleBinding
	}
	crb.Subjects = append(crb.Subjects, subject)
	return nil
}
