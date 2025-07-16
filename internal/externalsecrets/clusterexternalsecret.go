package externalsecrets

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	esv1beta1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1beta1"
)

// CreateClusterExternalSecret returns a new ClusterExternalSecret object.
func CreateClusterExternalSecret(name string, spec esv1beta1.ClusterExternalSecretSpec) *esv1beta1.ClusterExternalSecret {
	obj := &esv1beta1.ClusterExternalSecret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterExternalSecret",
			APIVersion: esv1beta1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: spec,
	}
	return obj
}

// AddClusterExternalSecretLabel adds a label to the ClusterExternalSecret metadata.
func AddClusterExternalSecretLabel(obj *esv1beta1.ClusterExternalSecret, key, value string) {
	if obj.Labels == nil {
		obj.Labels = map[string]string{}
	}
	obj.Labels[key] = value
}

// AddClusterExternalSecretAnnotation adds an annotation to the ClusterExternalSecret metadata.
func AddClusterExternalSecretAnnotation(obj *esv1beta1.ClusterExternalSecret, key, value string) {
	if obj.Annotations == nil {
		obj.Annotations = map[string]string{}
	}
	obj.Annotations[key] = value
}
