package kubernetes

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestConfigMapHelpers_PanicOnNil(t *testing.T) {
	assertPanics(t, func() { AddConfigMapData(nil, "k", "v") })
	assertPanics(t, func() { AddConfigMapBinaryData(nil, "k", nil) })
	assertPanics(t, func() { SetConfigMapImmutable(nil, true) })
	assertPanics(t, func() { AddConfigMapLabel(nil, "k", "v") })
	assertPanics(t, func() { AddConfigMapAnnotation(nil, "k", "v") })
	assertPanics(t, func() { SetConfigMapLabels(nil, nil) })
	assertPanics(t, func() { SetConfigMapAnnotations(nil, nil) })
}

// Container setter tests
func TestAddContainerPort_Success(t *testing.T) {
	container := &corev1.Container{Name: "test", Image: "nginx"}
	port := corev1.ContainerPort{ContainerPort: 8080}
	AddContainerPort(container, port)
	if len(container.Ports) != 1 {
		t.Fatal("expected Port to be added")
	}
}

func TestAddContainerEnv_Success(t *testing.T) {
	container := &corev1.Container{Name: "test", Image: "nginx"}
	env := corev1.EnvVar{Name: "KEY", Value: "value"}
	AddContainerEnv(container, env)
	if len(container.Env) != 1 {
		t.Fatal("expected Env to be added")
	}
}

func TestAddContainerEnvFrom_Success(t *testing.T) {
	container := &corev1.Container{Name: "test", Image: "nginx"}
	envFrom := corev1.EnvFromSource{}
	AddContainerEnvFrom(container, envFrom)
	if len(container.EnvFrom) != 1 {
		t.Fatal("expected EnvFrom to be added")
	}
}

func TestAddContainerVolumeMount_Success(t *testing.T) {
	container := &corev1.Container{Name: "test", Image: "nginx"}
	mount := corev1.VolumeMount{Name: "vol", MountPath: "/data"}
	AddContainerVolumeMount(container, mount)
	if len(container.VolumeMounts) != 1 {
		t.Fatal("expected VolumeMount to be added")
	}
}

func TestAddContainerVolumeDevice_Success(t *testing.T) {
	container := &corev1.Container{Name: "test", Image: "nginx"}
	device := corev1.VolumeDevice{Name: "dev", DevicePath: "/dev/sda"}
	AddContainerVolumeDevice(container, device)
	if len(container.VolumeDevices) != 1 {
		t.Fatal("expected VolumeDevice to be added")
	}
}

func TestSetContainerLivenessProbe_Success(t *testing.T) {
	container := &corev1.Container{Name: "test", Image: "nginx"}
	probe := corev1.Probe{}
	SetContainerLivenessProbe(container, probe)
	if container.LivenessProbe == nil {
		t.Fatal("expected LivenessProbe to be set")
	}
}

func TestSetContainerReadinessProbe_Success(t *testing.T) {
	container := &corev1.Container{Name: "test", Image: "nginx"}
	probe := corev1.Probe{}
	SetContainerReadinessProbe(container, probe)
	if container.ReadinessProbe == nil {
		t.Fatal("expected ReadinessProbe to be set")
	}
}

func TestSetContainerStartupProbe_Success(t *testing.T) {
	container := &corev1.Container{Name: "test", Image: "nginx"}
	probe := corev1.Probe{}
	SetContainerStartupProbe(container, probe)
	if container.StartupProbe == nil {
		t.Fatal("expected StartupProbe to be set")
	}
}

func TestSetContainerSecurityContext_Success(t *testing.T) {
	container := &corev1.Container{Name: "test", Image: "nginx"}
	sc := corev1.SecurityContext{}
	SetContainerSecurityContext(container, sc)
	if container.SecurityContext == nil {
		t.Fatal("expected SecurityContext to be set")
	}
}

func TestSetDaemonSetRevisionHistoryLimit_Success(t *testing.T) {
	ds := CreateDaemonSet("test", "default")
	limit := int32(5)
	SetDaemonSetRevisionHistoryLimit(ds, &limit)
	if ds.Spec.RevisionHistoryLimit == nil || *ds.Spec.RevisionHistoryLimit != 5 {
		t.Fatal("expected RevisionHistoryLimit to be 5")
	}
}

func TestSetJobCompletions_Success(t *testing.T) {
	job := CreateJob("test", "default")
	SetJobCompletions(job, 1)
	if job.Spec.Completions == nil || *job.Spec.Completions != 1 {
		t.Fatal("expected Completions to be 1")
	}
}

func TestSetJobParallelism_Success(t *testing.T) {
	job := CreateJob("test", "default")
	SetJobParallelism(job, 2)
	if job.Spec.Parallelism == nil || *job.Spec.Parallelism != 2 {
		t.Fatal("expected Parallelism to be 2")
	}
}

func TestSetJobBackoffLimit_Success(t *testing.T) {
	job := CreateJob("test", "default")
	SetJobBackoffLimit(job, 3)
	if job.Spec.BackoffLimit == nil || *job.Spec.BackoffLimit != 3 {
		t.Fatal("expected BackoffLimit to be 3")
	}
}

func TestSetJobTTLSecondsAfterFinished_Success(t *testing.T) {
	job := CreateJob("test", "default")
	SetJobTTLSecondsAfterFinished(job, 100)
	if job.Spec.TTLSecondsAfterFinished == nil || *job.Spec.TTLSecondsAfterFinished != 100 {
		t.Fatal("expected TTLSecondsAfterFinished to be 100")
	}
}

func TestSetJobActiveDeadlineSeconds_Success(t *testing.T) {
	job := CreateJob("test", "default")
	secs := int64(600)
	SetJobActiveDeadlineSeconds(job, &secs)
	if job.Spec.ActiveDeadlineSeconds == nil || *job.Spec.ActiveDeadlineSeconds != 600 {
		t.Fatal("expected ActiveDeadlineSeconds to be 600")
	}
}

// StatefulSet setter tests
func TestSetStatefulSetReplicas_Success(t *testing.T) {
	ss := CreateStatefulSet("test", "default")
	SetStatefulSetReplicas(ss, 3)
	if ss.Spec.Replicas == nil || *ss.Spec.Replicas != 3 {
		t.Fatal("expected Replicas to be 3")
	}
}

func TestSetStatefulSetRevisionHistoryLimit_Success(t *testing.T) {
	ss := CreateStatefulSet("test", "default")
	limit := int32(5)
	SetStatefulSetRevisionHistoryLimit(ss, &limit)
	if ss.Spec.RevisionHistoryLimit == nil || *ss.Spec.RevisionHistoryLimit != 5 {
		t.Fatal("expected RevisionHistoryLimit to be 5")
	}
}

// PVC setter tests
func TestSetPVCStorageClassName_Success(t *testing.T) {
	pvc := CreatePersistentVolumeClaim("test", "default")
	SetPVCStorageClassName(pvc, "fast")
	if pvc.Spec.StorageClassName == nil || *pvc.Spec.StorageClassName != "fast" {
		t.Fatal("expected StorageClassName to be set")
	}
}

func TestSetPVCVolumeMode_Success(t *testing.T) {
	pvc := CreatePersistentVolumeClaim("test", "default")
	mode := corev1.PersistentVolumeBlock
	SetPVCVolumeMode(pvc, mode)
	if pvc.Spec.VolumeMode == nil || *pvc.Spec.VolumeMode != mode {
		t.Fatal("expected VolumeMode to be set")
	}
}

func TestSetPVCDataSource_Success(t *testing.T) {
	pvc := CreatePersistentVolumeClaim("test", "default")
	dataSource := &corev1.TypedLocalObjectReference{Name: "snapshot"}
	SetPVCDataSource(pvc, dataSource)
	if pvc.Spec.DataSource == nil {
		t.Fatal("expected DataSource to be set")
	}
}
