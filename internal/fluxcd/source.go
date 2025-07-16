package fluxcd

import (
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	sourcev1beta2 "github.com/fluxcd/source-controller/api/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateGitRepository(name string, namespace string, spec sourcev1.GitRepositorySpec) *sourcev1.GitRepository {
	obj := &sourcev1.GitRepository{
		TypeMeta: metav1.TypeMeta{
			Kind:       "GitRepository",
			APIVersion: sourcev1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: spec,
	}
	return obj

}
func CreateHelmRepository(name string, namespace string, spec sourcev1.HelmRepositorySpec) *sourcev1.HelmRepository {
	obj := &sourcev1.HelmRepository{
		TypeMeta: metav1.TypeMeta{
			Kind:       "HelmRepository",
			APIVersion: sourcev1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: spec,
	}
	return obj
}
func CreateOCIRepository(name string, namespace string, spec sourcev1beta2.OCIRepositorySpec) *sourcev1beta2.OCIRepository {
	obj := &sourcev1beta2.OCIRepository{
		TypeMeta: metav1.TypeMeta{
			Kind:       "OCIRepository",
			APIVersion: sourcev1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: spec,
	}
	return obj
}
