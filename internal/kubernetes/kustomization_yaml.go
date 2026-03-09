package kubernetes

import (
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
func AddKustomizationResource(k *types.Kustomization, path string) {
	k.Resources = append(k.Resources, path)
}

// AddKustomizationComponent appends a component path to the kustomization.
func AddKustomizationComponent(k *types.Kustomization, path string) {
	k.Components = append(k.Components, path)
}

// AddKustomizationCRD appends a CRD path to the kustomization.
func AddKustomizationCRD(k *types.Kustomization, path string) {
	k.Crds = append(k.Crds, path)
}

// AddKustomizationImage appends an image transformer entry.
func AddKustomizationImage(k *types.Kustomization, img types.Image) {
	k.Images = append(k.Images, img)
}

// AddKustomizationPatch appends a patch entry.
func AddKustomizationPatch(k *types.Kustomization, p types.Patch) {
	k.Patches = append(k.Patches, p)
}

// SetKustomizationNamespace sets the namespace for all resources.
func SetKustomizationNamespace(k *types.Kustomization, ns string) {
	k.Namespace = ns
}

// MarshalKustomization returns the YAML encoding of the kustomization object.
func MarshalKustomization(k *types.Kustomization) ([]byte, error) {
	return yaml.Marshal(k)
}
