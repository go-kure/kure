package kubernetes

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// Container setter tests
func TestAddContainerPort_Success(t *testing.T) {
	container := CreateContainer("test", "nginx", nil, nil)
	port := corev1.ContainerPort{ContainerPort: 8080}
	err := AddContainerPort(container, port)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(container.Ports) != 1 {
		t.Fatal("expected Port to be added")
	}
}

func TestAddContainerEnv_Success(t *testing.T) {
	container := CreateContainer("test", "nginx", nil, nil)
	env := corev1.EnvVar{Name: "KEY", Value: "value"}
	err := AddContainerEnv(container, env)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(container.Env) != 1 {
		t.Fatal("expected Env to be added")
	}
}

func TestAddContainerEnvFrom_Success(t *testing.T) {
	container := CreateContainer("test", "nginx", nil, nil)
	envFrom := corev1.EnvFromSource{}
	err := AddContainerEnvFrom(container, envFrom)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(container.EnvFrom) != 1 {
		t.Fatal("expected EnvFrom to be added")
	}
}

func TestAddContainerVolumeMount_Success(t *testing.T) {
	container := CreateContainer("test", "nginx", nil, nil)
	mount := corev1.VolumeMount{Name: "vol", MountPath: "/data"}
	err := AddContainerVolumeMount(container, mount)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(container.VolumeMounts) != 1 {
		t.Fatal("expected VolumeMount to be added")
	}
}

func TestAddContainerVolumeDevice_Success(t *testing.T) {
	container := CreateContainer("test", "nginx", nil, nil)
	device := corev1.VolumeDevice{Name: "dev", DevicePath: "/dev/sda"}
	err := AddContainerVolumeDevice(container, device)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(container.VolumeDevices) != 1 {
		t.Fatal("expected VolumeDevice to be added")
	}
}

func TestSetContainerLivenessProbe_Success(t *testing.T) {
	container := CreateContainer("test", "nginx", nil, nil)
	probe := corev1.Probe{}
	err := SetContainerLivenessProbe(container, probe)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if container.LivenessProbe == nil {
		t.Fatal("expected LivenessProbe to be set")
	}
}

func TestSetContainerReadinessProbe_Success(t *testing.T) {
	container := CreateContainer("test", "nginx", nil, nil)
	probe := corev1.Probe{}
	err := SetContainerReadinessProbe(container, probe)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if container.ReadinessProbe == nil {
		t.Fatal("expected ReadinessProbe to be set")
	}
}

func TestSetContainerStartupProbe_Success(t *testing.T) {
	container := CreateContainer("test", "nginx", nil, nil)
	probe := corev1.Probe{}
	err := SetContainerStartupProbe(container, probe)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if container.StartupProbe == nil {
		t.Fatal("expected StartupProbe to be set")
	}
}

func TestSetContainerResources_Success(t *testing.T) {
	container := CreateContainer("test", "nginx", nil, nil)
	resources := corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			"cpu": resource.MustParse("1"),
		},
	}
	err := SetContainerResources(container, resources)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestSetContainerImagePullPolicy_Success(t *testing.T) {
	container := CreateContainer("test", "nginx", nil, nil)
	err := SetContainerImagePullPolicy(container, corev1.PullAlways)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if container.ImagePullPolicy != corev1.PullAlways {
		t.Fatal("expected ImagePullPolicy to be set")
	}
}

func TestSetContainerSecurityContext_Success(t *testing.T) {
	container := CreateContainer("test", "nginx", nil, nil)
	sc := corev1.SecurityContext{}
	err := SetContainerSecurityContext(container, sc)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
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
	container := CreateContainer("app", "nginx", nil, nil)
	err := AddDaemonSetContainer(ds, container)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestAddDaemonSetInitContainer_Success(t *testing.T) {
	ds := CreateDaemonSet("test", "default")
	container := CreateContainer("init", "busybox", nil, nil)
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
	err := SetDaemonSetServiceAccountName(ds, "test-sa")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestSetDaemonSetSecurityContext_Success(t *testing.T) {
	ds := CreateDaemonSet("test", "default")
	sc := &corev1.PodSecurityContext{}
	err := SetDaemonSetSecurityContext(ds, sc)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestSetDaemonSetAffinity_Success(t *testing.T) {
	ds := CreateDaemonSet("test", "default")
	affinity := &corev1.Affinity{}
	err := SetDaemonSetAffinity(ds, affinity)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestSetDaemonSetNodeSelector_Success(t *testing.T) {
	ds := CreateDaemonSet("test", "default")
	selector := map[string]string{"key": "value"}
	err := SetDaemonSetNodeSelector(ds, selector)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestSetDaemonSetUpdateStrategy_Success(t *testing.T) {
	ds := CreateDaemonSet("test", "default")
	strategy := appsv1.DaemonSetUpdateStrategy{}
	err := SetDaemonSetUpdateStrategy(ds, strategy)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestSetDaemonSetRevisionHistoryLimit_Success(t *testing.T) {
	ds := CreateDaemonSet("test", "default")
	limit := int32(5)
	err := SetDaemonSetRevisionHistoryLimit(ds, &limit)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
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
	container := CreateContainer("app", "nginx", nil, nil)
	err := AddJobContainer(job, container)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestAddJobInitContainer_Success(t *testing.T) {
	job := CreateJob("test", "default")
	container := CreateContainer("init", "busybox", nil, nil)
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
	err := SetJobServiceAccountName(job, "test-sa")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestSetJobSecurityContext_Success(t *testing.T) {
	job := CreateJob("test", "default")
	sc := &corev1.PodSecurityContext{}
	err := SetJobSecurityContext(job, sc)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestSetJobAffinity_Success(t *testing.T) {
	job := CreateJob("test", "default")
	affinity := &corev1.Affinity{}
	err := SetJobAffinity(job, affinity)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestSetJobNodeSelector_Success(t *testing.T) {
	job := CreateJob("test", "default")
	selector := map[string]string{"key": "value"}
	err := SetJobNodeSelector(job, selector)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestSetJobCompletions_Success(t *testing.T) {
	job := CreateJob("test", "default")
	err := SetJobCompletions(job, 1)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if job.Spec.Completions == nil || *job.Spec.Completions != 1 {
		t.Fatal("expected Completions to be 1")
	}
}

func TestSetJobParallelism_Success(t *testing.T) {
	job := CreateJob("test", "default")
	err := SetJobParallelism(job, 2)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if job.Spec.Parallelism == nil || *job.Spec.Parallelism != 2 {
		t.Fatal("expected Parallelism to be 2")
	}
}

func TestSetJobBackoffLimit_Success(t *testing.T) {
	job := CreateJob("test", "default")
	err := SetJobBackoffLimit(job, 3)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if job.Spec.BackoffLimit == nil || *job.Spec.BackoffLimit != 3 {
		t.Fatal("expected BackoffLimit to be 3")
	}
}

func TestSetJobTTLSecondsAfterFinished_Success(t *testing.T) {
	job := CreateJob("test", "default")
	err := SetJobTTLSecondsAfterFinished(job, 100)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if job.Spec.TTLSecondsAfterFinished == nil || *job.Spec.TTLSecondsAfterFinished != 100 {
		t.Fatal("expected TTLSecondsAfterFinished to be 100")
	}
}

func TestSetJobActiveDeadlineSeconds_Success(t *testing.T) {
	job := CreateJob("test", "default")
	secs := int64(600)
	err := SetJobActiveDeadlineSeconds(job, &secs)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if job.Spec.ActiveDeadlineSeconds == nil || *job.Spec.ActiveDeadlineSeconds != 600 {
		t.Fatal("expected ActiveDeadlineSeconds to be 600")
	}
}

// StatefulSet setter tests
func TestSetStatefulSetReplicas_Success(t *testing.T) {
	ss := CreateStatefulSet("test", "default")
	err := SetStatefulSetReplicas(ss, 3)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if ss.Spec.Replicas == nil || *ss.Spec.Replicas != 3 {
		t.Fatal("expected Replicas to be 3")
	}
}

func TestSetStatefulSetServiceName_Success(t *testing.T) {
	ss := CreateStatefulSet("test", "default")
	err := SetStatefulSetServiceName(ss, "headless")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if ss.Spec.ServiceName != "headless" {
		t.Fatal("expected ServiceName to be set")
	}
}

func TestSetStatefulSetUpdateStrategy_Success(t *testing.T) {
	ss := CreateStatefulSet("test", "default")
	strategy := appsv1.StatefulSetUpdateStrategy{Type: appsv1.RollingUpdateStatefulSetStrategyType}
	err := SetStatefulSetUpdateStrategy(ss, strategy)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestSetStatefulSetRevisionHistoryLimit_Success(t *testing.T) {
	ss := CreateStatefulSet("test", "default")
	limit := int32(5)
	err := SetStatefulSetRevisionHistoryLimit(ss, &limit)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if ss.Spec.RevisionHistoryLimit == nil || *ss.Spec.RevisionHistoryLimit != 5 {
		t.Fatal("expected RevisionHistoryLimit to be 5")
	}
}

func TestSetStatefulSetPodManagementPolicy_Success(t *testing.T) {
	ss := CreateStatefulSet("test", "default")
	err := SetStatefulSetPodManagementPolicy(ss, appsv1.ParallelPodManagement)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if ss.Spec.PodManagementPolicy != appsv1.ParallelPodManagement {
		t.Fatal("expected PodManagementPolicy to be set")
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
