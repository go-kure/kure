package fluxcd

import (
	fluxv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateResourceSet returns a new ResourceSet object.
func CreateResourceSet(name, namespace string, spec fluxv1.ResourceSetSpec) *fluxv1.ResourceSet {
	obj := &fluxv1.ResourceSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       fluxv1.ResourceSetKind,
			APIVersion: fluxv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: spec,
	}
	return obj
}

// AddResourceSetInput appends an input to the ResourceSet.
func AddResourceSetInput(rs *fluxv1.ResourceSet, in fluxv1.ResourceSetInput) {
	rs.Spec.Inputs = append(rs.Spec.Inputs, in)
}

// AddResourceSetInputFrom appends an input provider reference.
func AddResourceSetInputFrom(rs *fluxv1.ResourceSet, ref fluxv1.InputProviderReference) {
	rs.Spec.InputsFrom = append(rs.Spec.InputsFrom, ref)
}

// AddResourceSetResource appends a resource to reconcile.
func AddResourceSetResource(rs *fluxv1.ResourceSet, r *apiextensionsv1.JSON) {
	rs.Spec.Resources = append(rs.Spec.Resources, r)
}

// SetResourceSetResourcesTemplate sets the resources template.
func SetResourceSetResourcesTemplate(rs *fluxv1.ResourceSet, tpl string) {
	rs.Spec.ResourcesTemplate = tpl
}

// AddResourceSetDependency appends a dependency.
func AddResourceSetDependency(rs *fluxv1.ResourceSet, dep fluxv1.Dependency) {
	rs.Spec.DependsOn = append(rs.Spec.DependsOn, dep)
}

// SetResourceSetServiceAccountName sets the service account name.
func SetResourceSetServiceAccountName(rs *fluxv1.ResourceSet, name string) {
	rs.Spec.ServiceAccountName = name
}

// SetResourceSetWait sets the wait flag.
func SetResourceSetWait(rs *fluxv1.ResourceSet, wait bool) {
	rs.Spec.Wait = wait
}

// SetResourceSetCommonMetadata sets the common metadata.
func SetResourceSetCommonMetadata(rs *fluxv1.ResourceSet, cm *fluxv1.CommonMetadata) {
	rs.Spec.CommonMetadata = cm
}
