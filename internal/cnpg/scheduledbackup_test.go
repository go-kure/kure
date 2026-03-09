package cnpg

import (
	"testing"

	cnpgv1 "github.com/cloudnative-pg/cloudnative-pg/api/v1"
)

func TestCreateScheduledBackup(t *testing.T) {
	spec := cnpgv1.ScheduledBackupSpec{
		Schedule: "0 0 0 * * *",
		Cluster:  cnpgv1.LocalObjectReference{Name: "my-cluster"},
	}
	obj := CreateScheduledBackup("daily-backup", "db-ns", spec)

	if obj == nil {
		t.Fatal("expected non-nil ScheduledBackup")
	}
	if obj.Name != "daily-backup" {
		t.Errorf("unexpected name %q", obj.Name)
	}
	if obj.Namespace != "db-ns" {
		t.Errorf("unexpected namespace %q", obj.Namespace)
	}
	if obj.Kind != "ScheduledBackup" {
		t.Errorf("unexpected kind %q", obj.Kind)
	}
	if obj.APIVersion != "postgresql.cnpg.io/v1" {
		t.Errorf("unexpected apiVersion %q", obj.APIVersion)
	}
	if obj.Spec.Schedule != "0 0 0 * * *" {
		t.Errorf("unexpected schedule %q", obj.Spec.Schedule)
	}
	if obj.Spec.Cluster.Name != "my-cluster" {
		t.Errorf("unexpected cluster name %q", obj.Spec.Cluster.Name)
	}
}

func TestAddScheduledBackupLabel(t *testing.T) {
	obj := CreateScheduledBackup("test", "ns", cnpgv1.ScheduledBackupSpec{})
	AddScheduledBackupLabel(obj, "app", "postgres")
	if obj.Labels["app"] != "postgres" {
		t.Errorf("label not set")
	}
}

func TestAddScheduledBackupAnnotation(t *testing.T) {
	obj := CreateScheduledBackup("test", "ns", cnpgv1.ScheduledBackupSpec{})
	AddScheduledBackupAnnotation(obj, "team", "dba")
	if obj.Annotations["team"] != "dba" {
		t.Errorf("annotation not set")
	}
}

func TestSetScheduledBackupMethod(t *testing.T) {
	obj := CreateScheduledBackup("test", "ns", cnpgv1.ScheduledBackupSpec{})
	SetScheduledBackupMethod(obj, cnpgv1.BackupMethodPlugin)
	if obj.Spec.Method != cnpgv1.BackupMethodPlugin {
		t.Errorf("unexpected method %q", obj.Spec.Method)
	}
}

func TestSetScheduledBackupMethodVolumeSnapshot(t *testing.T) {
	obj := CreateScheduledBackup("test", "ns", cnpgv1.ScheduledBackupSpec{})
	SetScheduledBackupMethod(obj, cnpgv1.BackupMethodVolumeSnapshot)
	if obj.Spec.Method != cnpgv1.BackupMethodVolumeSnapshot {
		t.Errorf("unexpected method %q", obj.Spec.Method)
	}
}

func TestSetScheduledBackupMethodBarmanObjectStore(t *testing.T) {
	obj := CreateScheduledBackup("test", "ns", cnpgv1.ScheduledBackupSpec{})
	SetScheduledBackupMethod(obj, cnpgv1.BackupMethodBarmanObjectStore)
	if obj.Spec.Method != cnpgv1.BackupMethodBarmanObjectStore {
		t.Errorf("unexpected method %q", obj.Spec.Method)
	}
}

func TestSetScheduledBackupPluginConfiguration(t *testing.T) {
	obj := CreateScheduledBackup("test", "ns", cnpgv1.ScheduledBackupSpec{})
	params := map[string]string{"bucket": "my-bucket", "region": "eu-west-1"}
	SetScheduledBackupPluginConfiguration(obj, "barman-cloud.cloudnative-pg.io", params)
	if obj.Spec.PluginConfiguration == nil {
		t.Fatal("expected non-nil pluginConfiguration")
	}
	if obj.Spec.PluginConfiguration.Name != "barman-cloud.cloudnative-pg.io" {
		t.Errorf("unexpected plugin name %q", obj.Spec.PluginConfiguration.Name)
	}
	if obj.Spec.PluginConfiguration.Parameters["bucket"] != "my-bucket" {
		t.Errorf("unexpected parameter bucket %q", obj.Spec.PluginConfiguration.Parameters["bucket"])
	}
	if obj.Spec.PluginConfiguration.Parameters["region"] != "eu-west-1" {
		t.Errorf("unexpected parameter region %q", obj.Spec.PluginConfiguration.Parameters["region"])
	}
}

func TestSetScheduledBackupPluginConfigurationNilParams(t *testing.T) {
	obj := CreateScheduledBackup("test", "ns", cnpgv1.ScheduledBackupSpec{})
	SetScheduledBackupPluginConfiguration(obj, "barman-cloud.cloudnative-pg.io", nil)
	if obj.Spec.PluginConfiguration == nil {
		t.Fatal("expected non-nil pluginConfiguration")
	}
	if obj.Spec.PluginConfiguration.Name != "barman-cloud.cloudnative-pg.io" {
		t.Errorf("unexpected plugin name %q", obj.Spec.PluginConfiguration.Name)
	}
	if obj.Spec.PluginConfiguration.Parameters != nil {
		t.Errorf("expected nil parameters, got %v", obj.Spec.PluginConfiguration.Parameters)
	}
}

func TestSetScheduledBackupImmediate(t *testing.T) {
	obj := CreateScheduledBackup("test", "ns", cnpgv1.ScheduledBackupSpec{})
	SetScheduledBackupImmediate(obj, true)
	if obj.Spec.Immediate == nil || !*obj.Spec.Immediate {
		t.Errorf("expected immediate to be true")
	}
}

func TestSetScheduledBackupImmediateFalse(t *testing.T) {
	obj := CreateScheduledBackup("test", "ns", cnpgv1.ScheduledBackupSpec{})
	SetScheduledBackupImmediate(obj, false)
	if obj.Spec.Immediate == nil || *obj.Spec.Immediate {
		t.Errorf("expected immediate to be false")
	}
}

func TestSetScheduledBackupBackupOwnerReference(t *testing.T) {
	tests := []struct {
		name string
		ref  string
	}{
		{"none", "none"},
		{"self", "self"},
		{"cluster", "cluster"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			obj := CreateScheduledBackup("test", "ns", cnpgv1.ScheduledBackupSpec{})
			SetScheduledBackupBackupOwnerReference(obj, tc.ref)
			if obj.Spec.BackupOwnerReference != tc.ref {
				t.Errorf("unexpected backupOwnerReference %q, want %q", obj.Spec.BackupOwnerReference, tc.ref)
			}
		})
	}
}

func TestSetScheduledBackupSuspend(t *testing.T) {
	obj := CreateScheduledBackup("test", "ns", cnpgv1.ScheduledBackupSpec{})
	SetScheduledBackupSuspend(obj, true)
	if obj.Spec.Suspend == nil || !*obj.Spec.Suspend {
		t.Errorf("expected suspend to be true")
	}
}

func TestSetScheduledBackupSuspendFalse(t *testing.T) {
	obj := CreateScheduledBackup("test", "ns", cnpgv1.ScheduledBackupSpec{})
	SetScheduledBackupSuspend(obj, false)
	if obj.Spec.Suspend == nil || *obj.Spec.Suspend {
		t.Errorf("expected suspend to be false")
	}
}
