package cnpg

import (
	cnpgv1 "github.com/cloudnative-pg/cloudnative-pg/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/go-kure/kure/internal/validation"
)

// CreateScheduledBackup returns a new ScheduledBackup object with the provided name, namespace and spec.
func CreateScheduledBackup(name, namespace string, spec cnpgv1.ScheduledBackupSpec) *cnpgv1.ScheduledBackup {
	obj := &cnpgv1.ScheduledBackup{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ScheduledBackup",
			APIVersion: cnpgv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: spec,
	}
	return obj
}

// AddScheduledBackupLabel adds or updates a label on the ScheduledBackup metadata.
func AddScheduledBackupLabel(obj *cnpgv1.ScheduledBackup, key, value string) error {
	v := validation.NewValidator()
	if err := v.ValidateScheduledBackup(obj); err != nil {
		return err
	}
	if obj.Labels == nil {
		obj.Labels = make(map[string]string)
	}
	obj.Labels[key] = value
	return nil
}

// AddScheduledBackupAnnotation adds or updates an annotation on the ScheduledBackup metadata.
func AddScheduledBackupAnnotation(obj *cnpgv1.ScheduledBackup, key, value string) error {
	v := validation.NewValidator()
	if err := v.ValidateScheduledBackup(obj); err != nil {
		return err
	}
	if obj.Annotations == nil {
		obj.Annotations = make(map[string]string)
	}
	obj.Annotations[key] = value
	return nil
}

// SetScheduledBackupMethod sets the backup method on the ScheduledBackup spec.
func SetScheduledBackupMethod(obj *cnpgv1.ScheduledBackup, method cnpgv1.BackupMethod) error {
	v := validation.NewValidator()
	if err := v.ValidateScheduledBackup(obj); err != nil {
		return err
	}
	obj.Spec.Method = method
	return nil
}

// SetScheduledBackupPluginConfiguration sets the plugin configuration on the ScheduledBackup spec.
func SetScheduledBackupPluginConfiguration(obj *cnpgv1.ScheduledBackup, name string, params map[string]string) error {
	v := validation.NewValidator()
	if err := v.ValidateScheduledBackup(obj); err != nil {
		return err
	}
	obj.Spec.PluginConfiguration = &cnpgv1.BackupPluginConfiguration{
		Name:       name,
		Parameters: params,
	}
	return nil
}

// SetScheduledBackupImmediate sets the immediate flag on the ScheduledBackup spec.
func SetScheduledBackupImmediate(obj *cnpgv1.ScheduledBackup, immediate bool) error {
	v := validation.NewValidator()
	if err := v.ValidateScheduledBackup(obj); err != nil {
		return err
	}
	obj.Spec.Immediate = &immediate
	return nil
}

// SetScheduledBackupBackupOwnerReference sets the backupOwnerReference on the ScheduledBackup spec.
func SetScheduledBackupBackupOwnerReference(obj *cnpgv1.ScheduledBackup, ref string) error {
	v := validation.NewValidator()
	if err := v.ValidateScheduledBackup(obj); err != nil {
		return err
	}
	obj.Spec.BackupOwnerReference = ref
	return nil
}

// SetScheduledBackupSuspend sets the suspend flag on the ScheduledBackup spec.
func SetScheduledBackupSuspend(obj *cnpgv1.ScheduledBackup, suspend bool) error {
	v := validation.NewValidator()
	if err := v.ValidateScheduledBackup(obj); err != nil {
		return err
	}
	obj.Spec.Suspend = &suspend
	return nil
}
