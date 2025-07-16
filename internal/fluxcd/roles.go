package fluxcd

import (
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateRole returns a basic Role object.
func CreateRole(name, namespace string) *rbacv1.Role {
	obj := &rbacv1.Role{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Role",
			APIVersion: rbacv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Rules: []rbacv1.PolicyRule{},
	}
	return obj
}

// AddRoleRule appends a PolicyRule to the Role.
func AddRoleRule(role *rbacv1.Role, rule rbacv1.PolicyRule) {
	role.Rules = append(role.Rules, rule)
}

// SetRoleRules replaces all PolicyRules on the Role.
func SetRoleRules(role *rbacv1.Role, rules []rbacv1.PolicyRule) {
	role.Rules = rules
}

// AddRoleLabel adds a label to the Role.
func AddRoleLabel(role *rbacv1.Role, key, value string) {
	if role.Labels == nil {
		role.Labels = make(map[string]string)
	}
	role.Labels[key] = value
}

// AddRoleAnnotation adds an annotation to the Role.
func AddRoleAnnotation(role *rbacv1.Role, key, value string) {
	if role.Annotations == nil {
		role.Annotations = make(map[string]string)
	}
	role.Annotations[key] = value
}

// SetRoleLabels replaces all labels on the Role.
func SetRoleLabels(role *rbacv1.Role, labels map[string]string) {
	role.Labels = labels
}

// SetRoleAnnotations replaces all annotations on the Role.
func SetRoleAnnotations(role *rbacv1.Role, annotations map[string]string) {
	role.Annotations = annotations
}

// CreateRoleBinding returns a basic RoleBinding object.
func CreateRoleBinding(name, namespace string, roleRef rbacv1.RoleRef) *rbacv1.RoleBinding {
	obj := &rbacv1.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			Kind:       "RoleBinding",
			APIVersion: rbacv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Subjects: []rbacv1.Subject{},
		RoleRef:  roleRef,
	}
	return obj
}

// AddRoleBindingSubject appends a subject to the RoleBinding.
func AddRoleBindingSubject(rb *rbacv1.RoleBinding, subject rbacv1.Subject) {
	rb.Subjects = append(rb.Subjects, subject)
}

// SetRoleBindingSubjects replaces all subjects on the RoleBinding.
func SetRoleBindingSubjects(rb *rbacv1.RoleBinding, subjects []rbacv1.Subject) {
	rb.Subjects = subjects
}

// SetRoleBindingRoleRef updates the RoleRef on the RoleBinding.
func SetRoleBindingRoleRef(rb *rbacv1.RoleBinding, roleRef rbacv1.RoleRef) {
	rb.RoleRef = roleRef
}

// AddRoleBindingLabel adds a label to the RoleBinding.
func AddRoleBindingLabel(rb *rbacv1.RoleBinding, key, value string) {
	if rb.Labels == nil {
		rb.Labels = make(map[string]string)
	}
	rb.Labels[key] = value
}

// AddRoleBindingAnnotation adds an annotation to the RoleBinding.
func AddRoleBindingAnnotation(rb *rbacv1.RoleBinding, key, value string) {
	if rb.Annotations == nil {
		rb.Annotations = make(map[string]string)
	}
	rb.Annotations[key] = value
}

// SetRoleBindingLabels replaces all labels on the RoleBinding.
func SetRoleBindingLabels(rb *rbacv1.RoleBinding, labels map[string]string) {
	rb.Labels = labels
}

// SetRoleBindingAnnotations replaces all annotations on the RoleBinding.
func SetRoleBindingAnnotations(rb *rbacv1.RoleBinding, annotations map[string]string) {
	rb.Annotations = annotations
}

// CreateClusterRole returns a basic ClusterRole object.
func CreateClusterRole(name string) *rbacv1.ClusterRole {
	obj := &rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterRole",
			APIVersion: rbacv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Rules: []rbacv1.PolicyRule{},
	}
	return obj
}

// AddClusterRoleRule appends a PolicyRule to the ClusterRole.
func AddClusterRoleRule(cr *rbacv1.ClusterRole, rule rbacv1.PolicyRule) {
	cr.Rules = append(cr.Rules, rule)
}

// SetClusterRoleRules replaces all PolicyRules on the ClusterRole.
func SetClusterRoleRules(cr *rbacv1.ClusterRole, rules []rbacv1.PolicyRule) {
	cr.Rules = rules
}

// SetClusterRoleAggregationRule sets the AggregationRule for the ClusterRole.
func SetClusterRoleAggregationRule(cr *rbacv1.ClusterRole, rule *rbacv1.AggregationRule) {
	cr.AggregationRule = rule
}

// AddClusterRoleLabel adds a label to the ClusterRole.
func AddClusterRoleLabel(cr *rbacv1.ClusterRole, key, value string) {
	if cr.Labels == nil {
		cr.Labels = make(map[string]string)
	}
	cr.Labels[key] = value
}

// AddClusterRoleAnnotation adds an annotation to the ClusterRole.
func AddClusterRoleAnnotation(cr *rbacv1.ClusterRole, key, value string) {
	if cr.Annotations == nil {
		cr.Annotations = make(map[string]string)
	}
	cr.Annotations[key] = value
}

// SetClusterRoleLabels replaces all labels on the ClusterRole.
func SetClusterRoleLabels(cr *rbacv1.ClusterRole, labels map[string]string) {
	cr.Labels = labels
}

// SetClusterRoleAnnotations replaces all annotations on the ClusterRole.
func SetClusterRoleAnnotations(cr *rbacv1.ClusterRole, annotations map[string]string) {
	cr.Annotations = annotations
}

// CreateClusterRoleBinding returns a basic ClusterRoleBinding object.
func CreateClusterRoleBinding(name string, roleRef rbacv1.RoleRef) *rbacv1.ClusterRoleBinding {
	obj := &rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterRoleBinding",
			APIVersion: rbacv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Subjects: []rbacv1.Subject{},
		RoleRef:  roleRef,
	}
	return obj
}

// AddClusterRoleBindingSubject appends a subject to the ClusterRoleBinding.
func AddClusterRoleBindingSubject(crb *rbacv1.ClusterRoleBinding, subject rbacv1.Subject) {
	crb.Subjects = append(crb.Subjects, subject)
}

// SetClusterRoleBindingSubjects replaces all subjects on the ClusterRoleBinding.
func SetClusterRoleBindingSubjects(crb *rbacv1.ClusterRoleBinding, subjects []rbacv1.Subject) {
	crb.Subjects = subjects
}

// SetClusterRoleBindingRoleRef updates the RoleRef on the ClusterRoleBinding.
func SetClusterRoleBindingRoleRef(crb *rbacv1.ClusterRoleBinding, roleRef rbacv1.RoleRef) {
	crb.RoleRef = roleRef
}

// AddClusterRoleBindingLabel adds a label to the ClusterRoleBinding.
func AddClusterRoleBindingLabel(crb *rbacv1.ClusterRoleBinding, key, value string) {
	if crb.Labels == nil {
		crb.Labels = make(map[string]string)
	}
	crb.Labels[key] = value
}

// AddClusterRoleBindingAnnotation adds an annotation to the ClusterRoleBinding.
func AddClusterRoleBindingAnnotation(crb *rbacv1.ClusterRoleBinding, key, value string) {
	if crb.Annotations == nil {
		crb.Annotations = make(map[string]string)
	}
	crb.Annotations[key] = value
}

// SetClusterRoleBindingLabels replaces all labels on the ClusterRoleBinding.
func SetClusterRoleBindingLabels(crb *rbacv1.ClusterRoleBinding, labels map[string]string) {
	crb.Labels = labels
}

// SetClusterRoleBindingAnnotations replaces all annotations on the ClusterRoleBinding.
func SetClusterRoleBindingAnnotations(crb *rbacv1.ClusterRoleBinding, annotations map[string]string) {
	crb.Annotations = annotations
}
