package kuretest

import (
	"k8s.io/apimachinery/pkg/api/validate/content"
	"k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilvalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// nameRules is the name rule of every built-in kind the apiserver does not
// hold to the DNS subdomain a custom resource gets, each as the kind's own
// validation (k8s.io/kubernetes/pkg/apis/<group>/validation) passes it to
// ValidateObjectMeta. A kind absent here gets the subdomain.
var nameRules = map[schema.GroupKind]validation.ValidateNameFunc{
	// rbac: ValidateRBACName, any path segment
	{Group: "rbac.authorization.k8s.io", Kind: "Role"}:               pathSegmentName,
	{Group: "rbac.authorization.k8s.io", Kind: "RoleBinding"}:        pathSegmentName,
	{Group: "rbac.authorization.k8s.io", Kind: "ClusterRole"}:        pathSegmentName,
	{Group: "rbac.authorization.k8s.io", Kind: "ClusterRoleBinding"}: pathSegmentName,
	// core: ValidateNamespaceName; ValidateService under
	// RelaxedServiceNameValidation, locked on since 1.37
	{Kind: "Namespace"}: validation.ValidateNamespaceName,
	{Kind: "Service"}:   validation.NameIsDNSLabel,
	// apps: ValidateStatefulSetName
	{Group: "apps", Kind: "StatefulSet"}: validation.NameIsDNSLabel,
	// networking: ValidateIPAddressName
	{Group: "networking.k8s.io", Kind: "IPAddress"}: ipAddressName,
}

// definitionKind's rule is built per object: ValidateCustomResourceDefinition
// requires the name to be spec.names.plural "." spec.group.
var definitionKind = schema.GroupKind{Group: "apiextensions.k8s.io", Kind: "CustomResourceDefinition"}

// nameRule is the rule u's name is held to; nil is the default.
func nameRule(u *unstructured.Unstructured) validation.ValidateNameFunc {
	gk := u.GroupVersionKind().GroupKind()
	if gk == definitionKind {
		return definitionName(u)
	}
	return nameRules[gk]
}

func pathSegmentName(name string, _ bool) []string {
	return content.IsPathSegmentName(name)
}

func ipAddressName(name string, _ bool) []string {
	var msgs []string
	for _, e := range utilvalidation.IsValidIP(&field.Path{}, name) {
		msgs = append(msgs, e.Detail)
	}
	return msgs
}

// definitionName closes over the plural and group the definition declares;
// one that is missing or not a string reads as empty, which no name matches.
func definitionName(u *unstructured.Unstructured) validation.ValidateNameFunc {
	plural, _, _ := unstructured.NestedString(u.Object, "spec", "names", "plural")
	group, _, _ := unstructured.NestedString(u.Object, "spec", "group")
	required := plural + "." + group
	return func(name string, prefix bool) []string {
		msgs := validation.NameIsDNSSubdomain(name, prefix)
		if name != required {
			msgs = append(msgs, `must be spec.names.plural+"."+spec.group`)
		}
		return msgs
	}
}
