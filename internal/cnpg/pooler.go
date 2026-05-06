package cnpg

import (
	cnpgv1 "github.com/cloudnative-pg/cloudnative-pg/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreatePooler returns a CNPG Pooler with TypeMeta and ObjectMeta preset.
func CreatePooler(name, namespace string, spec cnpgv1.PoolerSpec) *cnpgv1.Pooler {
	return &cnpgv1.Pooler{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pooler",
			APIVersion: cnpgv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: spec,
	}
}

// SetPoolerInstances sets the number of replicas on the Pooler spec.
func SetPoolerInstances(obj *cnpgv1.Pooler, instances int32) {
	obj.Spec.Instances = &instances
}

// SetPoolerClusterRef sets the cluster reference on the Pooler spec.
func SetPoolerClusterRef(obj *cnpgv1.Pooler, clusterName string) {
	obj.Spec.Cluster = cnpgv1.LocalObjectReference{Name: clusterName}
}

// SetPoolerPgBouncerSpec sets the PgBouncer configuration on the Pooler spec.
func SetPoolerPgBouncerSpec(obj *cnpgv1.Pooler, spec cnpgv1.PgBouncerSpec) {
	obj.Spec.PgBouncer = &spec
}

// AddPoolerLabel adds or updates a label on the Pooler metadata.
func AddPoolerLabel(obj *cnpgv1.Pooler, key, value string) {
	if obj.Labels == nil {
		obj.Labels = make(map[string]string)
	}
	obj.Labels[key] = value
}

// AddPoolerAnnotation adds or updates an annotation on the Pooler metadata.
func AddPoolerAnnotation(obj *cnpgv1.Pooler, key, value string) {
	if obj.Annotations == nil {
		obj.Annotations = make(map[string]string)
	}
	obj.Annotations[key] = value
}
