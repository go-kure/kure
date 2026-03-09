package cnpg

import (
	cnpgv1 "github.com/cloudnative-pg/cloudnative-pg/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateCluster returns a new CNPG Cluster object with the provided name, namespace and spec.
func CreateCluster(name, namespace string, spec cnpgv1.ClusterSpec) *cnpgv1.Cluster {
	return &cnpgv1.Cluster{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Cluster",
			APIVersion: cnpgv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: spec,
	}
}

// AddClusterLabel adds or updates a label on the Cluster metadata.
func AddClusterLabel(obj *cnpgv1.Cluster, key, value string) {
	if obj.Labels == nil {
		obj.Labels = make(map[string]string)
	}
	obj.Labels[key] = value
}

// AddClusterAnnotation adds or updates an annotation on the Cluster metadata.
func AddClusterAnnotation(obj *cnpgv1.Cluster, key, value string) {
	if obj.Annotations == nil {
		obj.Annotations = make(map[string]string)
	}
	obj.Annotations[key] = value
}

// AddClusterManagedRole adds a managed role to the Cluster spec.
// If spec.Managed is nil, it will be initialized.
func AddClusterManagedRole(obj *cnpgv1.Cluster, role cnpgv1.RoleConfiguration) {
	if obj.Spec.Managed == nil {
		obj.Spec.Managed = &cnpgv1.ManagedConfiguration{}
	}
	obj.Spec.Managed.Roles = append(obj.Spec.Managed.Roles, role)
}
