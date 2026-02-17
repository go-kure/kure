package fluxcd

import (
	"github.com/go-kure/kure/pkg/errors"

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
func AddResourceSetInput(rs *fluxv1.ResourceSet, in fluxv1.ResourceSetInput) error {
	if rs == nil {
		return errors.New("nil ResourceSet")
	}
	rs.Spec.Inputs = append(rs.Spec.Inputs, in)
	return nil
}

// AddResourceSetInputFrom appends an input provider reference.
func AddResourceSetInputFrom(rs *fluxv1.ResourceSet, ref fluxv1.InputProviderReference) error {
	if rs == nil {
		return errors.New("nil ResourceSet")
	}
	rs.Spec.InputsFrom = append(rs.Spec.InputsFrom, ref)
	return nil
}

// AddResourceSetResource appends a resource to reconcile.
func AddResourceSetResource(rs *fluxv1.ResourceSet, r *apiextensionsv1.JSON) error {
	if rs == nil || r == nil {
		return errors.New("nil ResourceSet or resource")
	}
	rs.Spec.Resources = append(rs.Spec.Resources, r)
	return nil
}

// SetResourceSetResourcesTemplate sets the resources template.
func SetResourceSetResourcesTemplate(rs *fluxv1.ResourceSet, tpl string) error {
	if rs == nil {
		return errors.New("nil ResourceSet")
	}
	rs.Spec.ResourcesTemplate = tpl
	return nil
}

// AddResourceSetDependency appends a dependency.
func AddResourceSetDependency(rs *fluxv1.ResourceSet, dep fluxv1.Dependency) error {
	if rs == nil {
		return errors.New("nil ResourceSet")
	}
	rs.Spec.DependsOn = append(rs.Spec.DependsOn, dep)
	return nil
}

// SetResourceSetServiceAccountName sets the service account name.
func SetResourceSetServiceAccountName(rs *fluxv1.ResourceSet, name string) error {
	if rs == nil {
		return errors.New("nil ResourceSet")
	}
	rs.Spec.ServiceAccountName = name
	return nil
}

// SetResourceSetWait sets the wait flag.
func SetResourceSetWait(rs *fluxv1.ResourceSet, wait bool) error {
	if rs == nil {
		return errors.New("nil ResourceSet")
	}
	rs.Spec.Wait = wait
	return nil
}

// SetResourceSetCommonMetadata sets the common metadata.
func SetResourceSetCommonMetadata(rs *fluxv1.ResourceSet, cm *fluxv1.CommonMetadata) error {
	if rs == nil {
		return errors.New("nil ResourceSet")
	}
	rs.Spec.CommonMetadata = cm
	return nil
}
