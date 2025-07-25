package k8s

import (
	"errors"

	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/yaml"
)

// CreateKustomizationFile returns a types.Kustomization with
// apiVersion and kind set to the values used by the kustomize project.
// All list fields are initialized so entries can be appended safely.
func CreateKustomizationFile() *types.Kustomization {
	obj := &types.Kustomization{
		TypeMeta: types.TypeMeta{
			Kind:       types.KustomizationKind,
			APIVersion: types.KustomizationVersion,
		},
		Resources:  []string{},
		Components: []string{},
		Crds:       []string{},
		Patches:    []types.Patch{},
		Images:     []types.Image{},
	}
	return obj
}

// AddKustomizationResource appends a resource path to the kustomization.
func AddKustomizationResource(k *types.Kustomization, path string) error {
	if k == nil {
		return errors.New("nil kustomization")
	}
	k.Resources = append(k.Resources, path)
	return nil
}

// AddKustomizationComponent appends a component path to the kustomization.
func AddKustomizationComponent(k *types.Kustomization, path string) error {
	if k == nil {
		return errors.New("nil kustomization")
	}
	k.Components = append(k.Components, path)
	return nil
}

// AddKustomizationCRD appends a CRD path to the kustomization.
func AddKustomizationCRD(k *types.Kustomization, path string) error {
	if k == nil {
		return errors.New("nil kustomization")
	}
	k.Crds = append(k.Crds, path)
	return nil
}

// AddKustomizationImage appends an image transformer entry.
func AddKustomizationImage(k *types.Kustomization, img types.Image) error {
	if k == nil {
		return errors.New("nil kustomization")
	}
	k.Images = append(k.Images, img)
	return nil
}

// AddKustomizationPatch appends a patch entry.
func AddKustomizationPatch(k *types.Kustomization, p types.Patch) error {
	if k == nil {
		return errors.New("nil kustomization")
	}
	k.Patches = append(k.Patches, p)
	return nil
}

// SetKustomizationNamespace sets the namespace for all resources.
func SetKustomizationNamespace(k *types.Kustomization, ns string) error {
	if k == nil {
		return errors.New("nil kustomization")
	}
	k.Namespace = ns
	return nil
}

// MarshalKustomization returns the YAML encoding of the kustomization object.
func MarshalKustomization(k *types.Kustomization) ([]byte, error) {
	if k == nil {
		return nil, errors.New("nil kustomization")
	}
	return yaml.Marshal(k)
}
