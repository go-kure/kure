package cnpg

import (
	cnpgv1 "github.com/cloudnative-pg/cloudnative-pg/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/go-kure/kure/internal/validation"
)

// CreateDatabase returns a new CNPG Database object with the provided name,
// namespace and spec.
func CreateDatabase(name, namespace string, spec cnpgv1.DatabaseSpec) *cnpgv1.Database {
	return &cnpgv1.Database{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Database",
			APIVersion: cnpgv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: spec,
	}
}

// AddDatabaseLabel adds or updates a label on the Database metadata.
func AddDatabaseLabel(obj *cnpgv1.Database, key, value string) error {
	v := validation.NewValidator()
	if err := v.ValidateDatabase(obj); err != nil {
		return err
	}
	if obj.Labels == nil {
		obj.Labels = make(map[string]string)
	}
	obj.Labels[key] = value
	return nil
}

// AddDatabaseAnnotation adds or updates an annotation on the Database metadata.
func AddDatabaseAnnotation(obj *cnpgv1.Database, key, value string) error {
	v := validation.NewValidator()
	if err := v.ValidateDatabase(obj); err != nil {
		return err
	}
	if obj.Annotations == nil {
		obj.Annotations = make(map[string]string)
	}
	obj.Annotations[key] = value
	return nil
}

// AddDatabaseExtension appends an extension to the Database spec.
func AddDatabaseExtension(obj *cnpgv1.Database, ext cnpgv1.ExtensionSpec) error {
	v := validation.NewValidator()
	if err := v.ValidateDatabase(obj); err != nil {
		return err
	}
	obj.Spec.Extensions = append(obj.Spec.Extensions, ext)
	return nil
}

// SetDatabaseClusterRef sets the cluster reference on the Database spec.
func SetDatabaseClusterRef(obj *cnpgv1.Database, clusterName string) error {
	v := validation.NewValidator()
	if err := v.ValidateDatabase(obj); err != nil {
		return err
	}
	obj.Spec.ClusterRef = corev1.LocalObjectReference{Name: clusterName}
	return nil
}

// SetDatabaseOwner sets the owner role on the Database spec.
func SetDatabaseOwner(obj *cnpgv1.Database, owner string) error {
	v := validation.NewValidator()
	if err := v.ValidateDatabase(obj); err != nil {
		return err
	}
	obj.Spec.Owner = owner
	return nil
}

// SetDatabaseReclaimPolicy sets the reclaim policy on the Database spec.
func SetDatabaseReclaimPolicy(obj *cnpgv1.Database, policy cnpgv1.DatabaseReclaimPolicy) error {
	v := validation.NewValidator()
	if err := v.ValidateDatabase(obj); err != nil {
		return err
	}
	obj.Spec.ReclaimPolicy = policy
	return nil
}

// SetDatabaseEnsure sets the ensure option (present/absent) on the Database spec.
func SetDatabaseEnsure(obj *cnpgv1.Database, ensure cnpgv1.EnsureOption) error {
	v := validation.NewValidator()
	if err := v.ValidateDatabase(obj); err != nil {
		return err
	}
	obj.Spec.Ensure = ensure
	return nil
}
