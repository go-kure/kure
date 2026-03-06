package cnpg

import (
	barmanapi "github.com/cloudnative-pg/barman-cloud/pkg/api"
	barmanv1 "github.com/cloudnative-pg/plugin-barman-cloud/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/go-kure/kure/internal/validation"
)

// CreateObjectStore returns a new CNPG ObjectStore object with the provided
// name, namespace and spec.
func CreateObjectStore(name, namespace string, spec barmanv1.ObjectStoreSpec) *barmanv1.ObjectStore {
	return &barmanv1.ObjectStore{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ObjectStore",
			APIVersion: barmanv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: spec,
	}
}

// AddObjectStoreLabel adds or updates a label on the ObjectStore metadata.
func AddObjectStoreLabel(obj *barmanv1.ObjectStore, key, value string) error {
	v := validation.NewValidator()
	if err := v.ValidateObjectStore(obj); err != nil {
		return err
	}
	if obj.Labels == nil {
		obj.Labels = make(map[string]string)
	}
	obj.Labels[key] = value
	return nil
}

// AddObjectStoreAnnotation adds or updates an annotation on the ObjectStore metadata.
func AddObjectStoreAnnotation(obj *barmanv1.ObjectStore, key, value string) error {
	v := validation.NewValidator()
	if err := v.ValidateObjectStore(obj); err != nil {
		return err
	}
	if obj.Annotations == nil {
		obj.Annotations = make(map[string]string)
	}
	obj.Annotations[key] = value
	return nil
}

// AddObjectStoreEnvVar appends an environment variable to the instance
// sidecar configuration.
func AddObjectStoreEnvVar(obj *barmanv1.ObjectStore, envVar corev1.EnvVar) error {
	v := validation.NewValidator()
	if err := v.ValidateObjectStore(obj); err != nil {
		return err
	}
	obj.Spec.InstanceSidecarConfiguration.Env = append(
		obj.Spec.InstanceSidecarConfiguration.Env, envVar,
	)
	return nil
}

// SetObjectStoreDestinationPath sets the destination path on the ObjectStore
// configuration (e.g. s3://bucket/path/to/folder).
func SetObjectStoreDestinationPath(obj *barmanv1.ObjectStore, path string) error {
	v := validation.NewValidator()
	if err := v.ValidateObjectStore(obj); err != nil {
		return err
	}
	obj.Spec.Configuration.DestinationPath = path
	return nil
}

// SetObjectStoreEndpointURL sets the endpoint URL on the ObjectStore
// configuration.
func SetObjectStoreEndpointURL(obj *barmanv1.ObjectStore, url string) error {
	v := validation.NewValidator()
	if err := v.ValidateObjectStore(obj); err != nil {
		return err
	}
	obj.Spec.Configuration.EndpointURL = url
	return nil
}

// SetObjectStoreS3Credentials sets S3 credentials on the ObjectStore
// configuration using secret key selectors.
func SetObjectStoreS3Credentials(obj *barmanv1.ObjectStore, creds *barmanapi.S3Credentials) error {
	v := validation.NewValidator()
	if err := v.ValidateObjectStore(obj); err != nil {
		return err
	}
	obj.Spec.Configuration.AWS = creds
	return nil
}

// SetObjectStoreRetentionPolicy sets the retention policy on the ObjectStore
// spec (e.g. "60d", "4w", "3m").
func SetObjectStoreRetentionPolicy(obj *barmanv1.ObjectStore, policy string) error {
	v := validation.NewValidator()
	if err := v.ValidateObjectStore(obj); err != nil {
		return err
	}
	obj.Spec.RetentionPolicy = policy
	return nil
}

// SetObjectStoreWalConfig sets the WAL backup configuration on the ObjectStore.
func SetObjectStoreWalConfig(obj *barmanv1.ObjectStore, wal *barmanapi.WalBackupConfiguration) error {
	v := validation.NewValidator()
	if err := v.ValidateObjectStore(obj); err != nil {
		return err
	}
	obj.Spec.Configuration.Wal = wal
	return nil
}

// SetObjectStoreDataConfig sets the data backup configuration on the ObjectStore.
func SetObjectStoreDataConfig(obj *barmanv1.ObjectStore, data *barmanapi.DataBackupConfiguration) error {
	v := validation.NewValidator()
	if err := v.ValidateObjectStore(obj); err != nil {
		return err
	}
	obj.Spec.Configuration.Data = data
	return nil
}
