package cnpg

import (
	cnpgv1 "github.com/cloudnative-pg/cloudnative-pg/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
func AddScheduledBackupLabel(obj *cnpgv1.ScheduledBackup, key, value string) {
	if obj.Labels == nil {
		obj.Labels = make(map[string]string)
	}
	obj.Labels[key] = value
}

// AddScheduledBackupAnnotation adds or updates an annotation on the ScheduledBackup metadata.
func AddScheduledBackupAnnotation(obj *cnpgv1.ScheduledBackup, key, value string) {
	if obj.Annotations == nil {
		obj.Annotations = make(map[string]string)
	}
	obj.Annotations[key] = value
}

// SetScheduledBackupMethod sets the backup method on the ScheduledBackup spec.
func SetScheduledBackupMethod(obj *cnpgv1.ScheduledBackup, method cnpgv1.BackupMethod) {
	obj.Spec.Method = method
}

// SetScheduledBackupPluginConfiguration sets the plugin configuration on the ScheduledBackup spec.
func SetScheduledBackupPluginConfiguration(obj *cnpgv1.ScheduledBackup, name string, params map[string]string) {
	obj.Spec.PluginConfiguration = &cnpgv1.BackupPluginConfiguration{
		Name:       name,
		Parameters: params,
	}
}

// SetScheduledBackupImmediate sets the immediate flag on the ScheduledBackup spec.
func SetScheduledBackupImmediate(obj *cnpgv1.ScheduledBackup, immediate bool) {
	obj.Spec.Immediate = &immediate
}

// SetScheduledBackupBackupOwnerReference sets the backupOwnerReference on the ScheduledBackup spec.
func SetScheduledBackupBackupOwnerReference(obj *cnpgv1.ScheduledBackup, ref string) {
	obj.Spec.BackupOwnerReference = ref
}

// SetScheduledBackupSuspend sets the suspend flag on the ScheduledBackup spec.
func SetScheduledBackupSuspend(obj *cnpgv1.ScheduledBackup, suspend bool) {
	obj.Spec.Suspend = &suspend
}
