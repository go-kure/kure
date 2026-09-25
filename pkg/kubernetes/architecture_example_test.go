package kubernetes_test

import (
	"fmt"

	rbacv1 "k8s.io/api/rbac/v1"

	"github.com/go-kure/kure/pkg/kubernetes"
)

// The RBAC Integration block of docs/ARCHITECTURE.md is generated from this
// function (scripts/gen-doc-examples.sh).

func Example_architectureRBAC() {
	// Create minimal privilege roles
	role := kubernetes.CreateRole("app-reader", "default")
	kubernetes.AddRoleRule(role, rbacv1.PolicyRule{
		APIGroups: []string{""},
		Resources: []string{"pods"},
		Verbs:     []string{"get", "list"},
	})

	// Bind to specific accounts. RoleRef is a single struct field, so it is written
	// directly - a helper for it would be the assignment with more words.
	binding := kubernetes.CreateRoleBinding("app-reader", "default")
	binding.RoleRef = rbacv1.RoleRef{
		APIGroup: rbacv1.GroupName,
		Kind:     "Role",
		Name:     "app-reader",
	}
	kubernetes.AddRoleBindingSubject(binding, rbacv1.Subject{
		Kind: "ServiceAccount",
		Name: "app-sa",
	})
	fmt.Println(role.Rules[0].Verbs, binding.RoleRef.Kind, binding.Subjects[0].Name)
	// Output: [get list] Role app-sa
}
