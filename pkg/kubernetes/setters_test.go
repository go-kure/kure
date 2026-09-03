package kubernetes

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

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

// DaemonSet setter tests
func TestSetDaemonSetPodSpec_Success(t *testing.T) {
	ds := CreateDaemonSet("test", "default")
	spec := &corev1.PodSpec{
		Containers: []corev1.Container{
			{Name: "test", Image: "nginx"},
		},
	}
	err := SetDaemonSetPodSpec(ds, spec)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestAddDaemonSetContainer_Success(t *testing.T) {
	ds := CreateDaemonSet("test", "default")
	container := &corev1.Container{Name: "app", Image: "nginx"}
	err := AddDaemonSetContainer(ds, container)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestAddDaemonSetInitContainer_Success(t *testing.T) {
	ds := CreateDaemonSet("test", "default")
	container := &corev1.Container{Name: "init", Image: "busybox"}
	err := AddDaemonSetInitContainer(ds, container)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestAddDaemonSetVolume_Success(t *testing.T) {
	ds := CreateDaemonSet("test", "default")
	vol := &corev1.Volume{Name: "vol"}
	err := AddDaemonSetVolume(ds, vol)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestAddDaemonSetImagePullSecret_Success(t *testing.T) {
	ds := CreateDaemonSet("test", "default")
	secret := &corev1.LocalObjectReference{Name: "secret"}
	err := AddDaemonSetImagePullSecret(ds, secret)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestAddDaemonSetToleration_Success(t *testing.T) {
	ds := CreateDaemonSet("test", "default")
	toleration := &corev1.Toleration{Key: "key"}
	err := AddDaemonSetToleration(ds, toleration)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestAddDaemonSetTopologySpreadConstraints_Success(t *testing.T) {
	ds := CreateDaemonSet("test", "default")
	constraint := &corev1.TopologySpreadConstraint{
		MaxSkew:           1,
		TopologyKey:       "zone",
		WhenUnsatisfiable: corev1.DoNotSchedule,
	}
	err := AddDaemonSetTopologySpreadConstraints(ds, constraint)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestSetDaemonSetServiceAccountName_Success(t *testing.T) {
	ds := CreateDaemonSet("test", "default")
	SetDaemonSetServiceAccountName(ds, "test-sa")
}

func TestSetDaemonSetSecurityContext_Success(t *testing.T) {
	ds := CreateDaemonSet("test", "default")
	sc := &corev1.PodSecurityContext{}
	SetDaemonSetSecurityContext(ds, sc)
}

func TestSetDaemonSetAffinity_Success(t *testing.T) {
	ds := CreateDaemonSet("test", "default")
	affinity := &corev1.Affinity{}
	SetDaemonSetAffinity(ds, affinity)
}

func TestSetDaemonSetNodeSelector_Success(t *testing.T) {
	ds := CreateDaemonSet("test", "default")
	selector := map[string]string{"key": "value"}
	SetDaemonSetNodeSelector(ds, selector)
}

func TestSetDaemonSetRevisionHistoryLimit_Success(t *testing.T) {
	ds := CreateDaemonSet("test", "default")
	limit := int32(5)
	SetDaemonSetRevisionHistoryLimit(ds, &limit)
	if ds.Spec.RevisionHistoryLimit == nil || *ds.Spec.RevisionHistoryLimit != 5 {
		t.Fatal("expected RevisionHistoryLimit to be 5")
	}
}

// Job setter tests
func TestSetJobPodSpec_Success(t *testing.T) {
	job := CreateJob("test", "default")
	spec := &corev1.PodSpec{
		Containers: []corev1.Container{
			{Name: "test", Image: "nginx"},
		},
	}
	err := SetJobPodSpec(job, spec)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestAddJobContainer_Success(t *testing.T) {
	job := CreateJob("test", "default")
	container := &corev1.Container{Name: "app", Image: "nginx"}
	err := AddJobContainer(job, container)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestAddJobInitContainer_Success(t *testing.T) {
	job := CreateJob("test", "default")
	container := &corev1.Container{Name: "init", Image: "busybox"}
	err := AddJobInitContainer(job, container)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestAddJobVolume_Success(t *testing.T) {
	job := CreateJob("test", "default")
	vol := &corev1.Volume{Name: "vol"}
	err := AddJobVolume(job, vol)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestAddJobImagePullSecret_Success(t *testing.T) {
	job := CreateJob("test", "default")
	secret := &corev1.LocalObjectReference{Name: "secret"}
	err := AddJobImagePullSecret(job, secret)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestAddJobToleration_Success(t *testing.T) {
	job := CreateJob("test", "default")
	toleration := &corev1.Toleration{Key: "key"}
	err := AddJobToleration(job, toleration)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestAddJobTopologySpreadConstraint_Success(t *testing.T) {
	job := CreateJob("test", "default")
	constraint := &corev1.TopologySpreadConstraint{
		MaxSkew:           1,
		TopologyKey:       "zone",
		WhenUnsatisfiable: corev1.DoNotSchedule,
	}
	err := AddJobTopologySpreadConstraint(job, constraint)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestSetJobServiceAccountName_Success(t *testing.T) {
	job := CreateJob("test", "default")
	SetJobServiceAccountName(job, "test-sa")
}

func TestSetJobSecurityContext_Success(t *testing.T) {
	job := CreateJob("test", "default")
	sc := &corev1.PodSecurityContext{}
	SetJobSecurityContext(job, sc)
}

func TestSetJobAffinity_Success(t *testing.T) {
	job := CreateJob("test", "default")
	affinity := &corev1.Affinity{}
	SetJobAffinity(job, affinity)
}

func TestSetJobNodeSelector_Success(t *testing.T) {
	job := CreateJob("test", "default")
	selector := map[string]string{"key": "value"}
	SetJobNodeSelector(job, selector)
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
