package kubernetes

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// ConfigMap setter tests
func TestAddConfigMapData_Success(t *testing.T) {
	cm := CreateConfigMap("test", "default")
	err := AddConfigMapData(cm, "key", "value")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cm.Data["key"] != "value" {
		t.Fatal("expected Data to be set")
	}
}

func TestAddConfigMapDataMap_Success(t *testing.T) {
	cm := CreateConfigMap("test", "default")
	data := map[string]string{"key1": "value1", "key2": "value2"}
	err := AddConfigMapDataMap(cm, data)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(cm.Data) != 2 {
		t.Fatal("expected Data to be set")
	}
}

func TestAddConfigMapBinaryData_Success(t *testing.T) {
	cm := CreateConfigMap("test", "default")
	err := AddConfigMapBinaryData(cm, "key", []byte("value"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if string(cm.BinaryData["key"]) != "value" {
		t.Fatal("expected BinaryData to be set")
	}
}

func TestAddConfigMapBinaryDataMap_Success(t *testing.T) {
	cm := CreateConfigMap("test", "default")
	data := map[string][]byte{"key1": []byte("value1")}
	err := AddConfigMapBinaryDataMap(cm, data)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(cm.BinaryData) != 1 {
		t.Fatal("expected BinaryData to be set")
	}
}

func TestSetConfigMapData_Success(t *testing.T) {
	cm := CreateConfigMap("test", "default")
	data := map[string]string{"new": "data"}
	err := SetConfigMapData(cm, data)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cm.Data["new"] != "data" {
		t.Fatal("expected Data to be replaced")
	}
}

func TestSetConfigMapBinaryData_Success(t *testing.T) {
	cm := CreateConfigMap("test", "default")
	data := map[string][]byte{"new": []byte("data")}
	err := SetConfigMapBinaryData(cm, data)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if string(cm.BinaryData["new"]) != "data" {
		t.Fatal("expected BinaryData to be replaced")
	}
}

func TestSetConfigMapImmutable_Success(t *testing.T) {
	cm := CreateConfigMap("test", "default")
	err := SetConfigMapImmutable(cm, true)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cm.Immutable == nil || !*cm.Immutable {
		t.Fatal("expected Immutable to be true")
	}
}

func TestAddConfigMapLabel_Success(t *testing.T) {
	cm := CreateConfigMap("test", "default")
	err := AddConfigMapLabel(cm, "key", "value")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cm.Labels["key"] != "value" {
		t.Fatal("expected Label to be set")
	}
}

func TestAddConfigMapAnnotation_Success(t *testing.T) {
	cm := CreateConfigMap("test", "default")
	err := AddConfigMapAnnotation(cm, "key", "value")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cm.Annotations["key"] != "value" {
		t.Fatal("expected Annotation to be set")
	}
}

func TestSetConfigMapLabels_Success(t *testing.T) {
	cm := CreateConfigMap("test", "default")
	labels := map[string]string{"new": "label"}
	err := SetConfigMapLabels(cm, labels)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cm.Labels["new"] != "label" {
		t.Fatal("expected Labels to be replaced")
	}
}

func TestSetConfigMapAnnotations_Success(t *testing.T) {
	cm := CreateConfigMap("test", "default")
	anns := map[string]string{"new": "annotation"}
	err := SetConfigMapAnnotations(cm, anns)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cm.Annotations["new"] != "annotation" {
		t.Fatal("expected Annotations to be replaced")
	}
}

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

// Deployment setter tests
func TestSetDeploymentPodSpec_Success(t *testing.T) {
	dep := CreateDeployment("test", "default")
	spec := &corev1.PodSpec{
		Containers: []corev1.Container{
			{Name: "test", Image: "nginx"},
		},
	}
	err := SetDeploymentPodSpec(dep, spec)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestAddDeploymentContainer_Success(t *testing.T) {
	dep := CreateDeployment("test", "default")
	container := CreateContainer("app", "nginx", nil, nil)
	err := AddDeploymentContainer(dep, container)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestAddDeploymentInitContainer_Success(t *testing.T) {
	dep := CreateDeployment("test", "default")
	container := CreateContainer("init", "busybox", nil, nil)
	err := AddDeploymentInitContainer(dep, container)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestAddDeploymentVolume_Success(t *testing.T) {
	dep := CreateDeployment("test", "default")
	vol := &corev1.Volume{Name: "vol"}
	err := AddDeploymentVolume(dep, vol)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestAddDeploymentImagePullSecret_Success(t *testing.T) {
	dep := CreateDeployment("test", "default")
	secret := &corev1.LocalObjectReference{Name: "secret"}
	err := AddDeploymentImagePullSecret(dep, secret)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestAddDeploymentToleration_Success(t *testing.T) {
	dep := CreateDeployment("test", "default")
	toleration := &corev1.Toleration{Key: "key"}
	err := AddDeploymentToleration(dep, toleration)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestAddDeploymentTopologySpreadConstraints_Success(t *testing.T) {
	dep := CreateDeployment("test", "default")
	constraint := &corev1.TopologySpreadConstraint{
		MaxSkew:           1,
		TopologyKey:       "zone",
		WhenUnsatisfiable: corev1.DoNotSchedule,
	}
	err := AddDeploymentTopologySpreadConstraints(dep, constraint)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestSetDeploymentServiceAccountName_Success(t *testing.T) {
	dep := CreateDeployment("test", "default")
	err := SetDeploymentServiceAccountName(dep, "test-sa")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestSetDeploymentSecurityContext_Success(t *testing.T) {
	dep := CreateDeployment("test", "default")
	sc := &corev1.PodSecurityContext{}
	err := SetDeploymentSecurityContext(dep, sc)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestSetDeploymentAffinity_Success(t *testing.T) {
	dep := CreateDeployment("test", "default")
	affinity := &corev1.Affinity{}
	err := SetDeploymentAffinity(dep, affinity)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestSetDeploymentNodeSelector_Success(t *testing.T) {
	dep := CreateDeployment("test", "default")
	selector := map[string]string{"key": "value"}
	err := SetDeploymentNodeSelector(dep, selector)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestSetDeploymentReplicas_Success(t *testing.T) {
	dep := CreateDeployment("test", "default")
	err := SetDeploymentReplicas(dep, 3)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if dep.Spec.Replicas == nil || *dep.Spec.Replicas != 3 {
		t.Fatal("expected Replicas to be 3")
	}
}

func TestSetDeploymentStrategy_Success(t *testing.T) {
	dep := CreateDeployment("test", "default")
	strategy := appsv1.DeploymentStrategy{Type: appsv1.RollingUpdateDeploymentStrategyType}
	err := SetDeploymentStrategy(dep, strategy)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestSetDeploymentRevisionHistoryLimit_Success(t *testing.T) {
	dep := CreateDeployment("test", "default")
	err := SetDeploymentRevisionHistoryLimit(dep, 5)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if dep.Spec.RevisionHistoryLimit == nil || *dep.Spec.RevisionHistoryLimit != 5 {
		t.Fatal("expected RevisionHistoryLimit to be 5")
	}
}

func TestSetDeploymentMinReadySeconds_Success(t *testing.T) {
	dep := CreateDeployment("test", "default")
	err := SetDeploymentMinReadySeconds(dep, 10)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if dep.Spec.MinReadySeconds != 10 {
		t.Fatal("expected MinReadySeconds to be 10")
	}
}

func TestSetDeploymentProgressDeadlineSeconds_Success(t *testing.T) {
	dep := CreateDeployment("test", "default")
	err := SetDeploymentProgressDeadlineSeconds(dep, 600)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if dep.Spec.ProgressDeadlineSeconds == nil || *dep.Spec.ProgressDeadlineSeconds != 600 {
		t.Fatal("expected ProgressDeadlineSeconds to be 600")
	}
}

// Ingress setter tests
func TestAddIngressRulePath_Success(t *testing.T) {
	rule := &networkingv1.IngressRule{Host: "example.com"}
	pathType := networkingv1.PathTypePrefix
	path := networkingv1.HTTPIngressPath{
		Path:     "/",
		PathType: &pathType,
		Backend: networkingv1.IngressBackend{
			Service: &networkingv1.IngressServiceBackend{
				Name: "svc",
				Port: networkingv1.ServiceBackendPort{Number: 80},
			},
		},
	}
	AddIngressRulePath(rule, path)
	if len(rule.HTTP.Paths) != 1 {
		t.Fatal("expected path to be added")
	}
}

func TestSetIngressClassName_Success(t *testing.T) {
	ing := CreateIngress("test", "default", "nginx")
	err := SetIngressClassName(ing, "traefik")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if ing.Spec.IngressClassName == nil || *ing.Spec.IngressClassName != "traefik" {
		t.Fatal("expected IngressClassName to be traefik")
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

// CronJob setter tests
func TestSetCronJobPodSpec_Success(t *testing.T) {
	cj := CreateCronJob("test", "default", "*/5 * * * *")
	spec := &corev1.PodSpec{
		Containers: []corev1.Container{
			{Name: "test", Image: "nginx"},
		},
	}
	err := SetCronJobPodSpec(cj, spec)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestAddCronJobContainer_Success(t *testing.T) {
	cj := CreateCronJob("test", "default", "*/5 * * * *")
	container := CreateContainer("app", "nginx", nil, nil)
	err := AddCronJobContainer(cj, container)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestAddCronJobInitContainer_Success(t *testing.T) {
	cj := CreateCronJob("test", "default", "*/5 * * * *")
	container := CreateContainer("init", "busybox", nil, nil)
	err := AddCronJobInitContainer(cj, container)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestAddCronJobVolume_Success(t *testing.T) {
	cj := CreateCronJob("test", "default", "*/5 * * * *")
	vol := &corev1.Volume{Name: "vol"}
	err := AddCronJobVolume(cj, vol)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestAddCronJobImagePullSecret_Success(t *testing.T) {
	cj := CreateCronJob("test", "default", "*/5 * * * *")
	secret := &corev1.LocalObjectReference{Name: "secret"}
	err := AddCronJobImagePullSecret(cj, secret)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestAddCronJobToleration_Success(t *testing.T) {
	cj := CreateCronJob("test", "default", "*/5 * * * *")
	toleration := &corev1.Toleration{Key: "key"}
	err := AddCronJobToleration(cj, toleration)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestAddCronJobTopologySpreadConstraint_Success(t *testing.T) {
	cj := CreateCronJob("test", "default", "*/5 * * * *")
	constraint := &corev1.TopologySpreadConstraint{
		MaxSkew:           1,
		TopologyKey:       "zone",
		WhenUnsatisfiable: corev1.DoNotSchedule,
	}
	err := AddCronJobTopologySpreadConstraint(cj, constraint)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestSetCronJobServiceAccountName_Success(t *testing.T) {
	cj := CreateCronJob("test", "default", "*/5 * * * *")
	err := SetCronJobServiceAccountName(cj, "test-sa")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestSetCronJobSecurityContext_Success(t *testing.T) {
	cj := CreateCronJob("test", "default", "*/5 * * * *")
	sc := &corev1.PodSecurityContext{}
	err := SetCronJobSecurityContext(cj, sc)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestSetCronJobAffinity_Success(t *testing.T) {
	cj := CreateCronJob("test", "default", "*/5 * * * *")
	affinity := &corev1.Affinity{}
	err := SetCronJobAffinity(cj, affinity)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestSetCronJobNodeSelector_Success(t *testing.T) {
	cj := CreateCronJob("test", "default", "*/5 * * * *")
	selector := map[string]string{"key": "value"}
	err := SetCronJobNodeSelector(cj, selector)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestSetCronJobSchedule_Success(t *testing.T) {
	cj := CreateCronJob("test", "default", "*/5 * * * *")
	err := SetCronJobSchedule(cj, "0 0 * * *")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cj.Spec.Schedule != "0 0 * * *" {
		t.Fatal("expected Schedule to be set")
	}
}

func TestSetCronJobSuspend_Success(t *testing.T) {
	cj := CreateCronJob("test", "default", "*/5 * * * *")
	err := SetCronJobSuspend(cj, true)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cj.Spec.Suspend == nil || !*cj.Spec.Suspend {
		t.Fatal("expected Suspend to be true")
	}
}

func TestSetCronJobConcurrencyPolicy_Success(t *testing.T) {
	cj := CreateCronJob("test", "default", "*/5 * * * *")
	err := SetCronJobConcurrencyPolicy(cj, batchv1.ForbidConcurrent)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cj.Spec.ConcurrencyPolicy != batchv1.ForbidConcurrent {
		t.Fatal("expected ConcurrencyPolicy to be set")
	}
}

func TestSetCronJobStartingDeadlineSeconds_Success(t *testing.T) {
	cj := CreateCronJob("test", "default", "*/5 * * * *")
	err := SetCronJobStartingDeadlineSeconds(cj, 300)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cj.Spec.StartingDeadlineSeconds == nil || *cj.Spec.StartingDeadlineSeconds != 300 {
		t.Fatal("expected StartingDeadlineSeconds to be 300")
	}
}

func TestSetCronJobSuccessfulJobsHistoryLimit_Success(t *testing.T) {
	cj := CreateCronJob("test", "default", "*/5 * * * *")
	err := SetCronJobSuccessfulJobsHistoryLimit(cj, 5)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cj.Spec.SuccessfulJobsHistoryLimit == nil || *cj.Spec.SuccessfulJobsHistoryLimit != 5 {
		t.Fatal("expected SuccessfulJobsHistoryLimit to be 5")
	}
}

func TestSetCronJobFailedJobsHistoryLimit_Success(t *testing.T) {
	cj := CreateCronJob("test", "default", "*/5 * * * *")
	err := SetCronJobFailedJobsHistoryLimit(cj, 3)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cj.Spec.FailedJobsHistoryLimit == nil || *cj.Spec.FailedJobsHistoryLimit != 3 {
		t.Fatal("expected FailedJobsHistoryLimit to be 3")
	}
}

// Secret setter tests
func TestAddSecretData_Success(t *testing.T) {
	secret := CreateSecret("test", "default")
	err := AddSecretData(secret, "key", []byte("value"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if string(secret.Data["key"]) != "value" {
		t.Fatal("expected Data to be set")
	}
}

func TestAddSecretStringData_Success(t *testing.T) {
	secret := CreateSecret("test", "default")
	err := AddSecretStringData(secret, "key", "value")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if secret.StringData["key"] != "value" {
		t.Fatal("expected StringData to be set")
	}
}

func TestSetSecretType_Success(t *testing.T) {
	secret := CreateSecret("test", "default")
	err := SetSecretType(secret, corev1.SecretTypeTLS)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if secret.Type != corev1.SecretTypeTLS {
		t.Fatal("expected Type to be set")
	}
}

func TestSetSecretImmutable_Success(t *testing.T) {
	secret := CreateSecret("test", "default")
	err := SetSecretImmutable(secret, true)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if secret.Immutable == nil || !*secret.Immutable {
		t.Fatal("expected Immutable to be true")
	}
}

func TestAddSecretLabel_Success(t *testing.T) {
	secret := CreateSecret("test", "default")
	err := AddSecretLabel(secret, "key", "value")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if secret.Labels["key"] != "value" {
		t.Fatal("expected Label to be set")
	}
}

func TestAddSecretAnnotation_Success(t *testing.T) {
	secret := CreateSecret("test", "default")
	err := AddSecretAnnotation(secret, "key", "value")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if secret.Annotations["key"] != "value" {
		t.Fatal("expected Annotation to be set")
	}
}

func TestSetSecretLabels_Success(t *testing.T) {
	secret := CreateSecret("test", "default")
	labels := map[string]string{"new": "label"}
	err := SetSecretLabels(secret, labels)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if secret.Labels["new"] != "label" {
		t.Fatal("expected Labels to be replaced")
	}
}

func TestSetSecretAnnotations_Success(t *testing.T) {
	secret := CreateSecret("test", "default")
	anns := map[string]string{"new": "annotation"}
	err := SetSecretAnnotations(secret, anns)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if secret.Annotations["new"] != "annotation" {
		t.Fatal("expected Annotations to be replaced")
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

// Role/ClusterRole setter tests
func TestAddRoleRule_Success(t *testing.T) {
	role := CreateRole("test", "default")
	rule := rbacv1.PolicyRule{
		Verbs:     []string{"get"},
		APIGroups: []string{""},
		Resources: []string{"pods"},
	}
	AddRoleRule(role, rule)
	if len(role.Rules) != 1 {
		t.Fatal("expected PolicyRule to be added")
	}
}

func TestAddClusterRoleRule_Success(t *testing.T) {
	cr := CreateClusterRole("test")
	rule := rbacv1.PolicyRule{
		Verbs:     []string{"get"},
		APIGroups: []string{""},
		Resources: []string{"nodes"},
	}
	AddClusterRoleRule(cr, rule)
	if len(cr.Rules) != 1 {
		t.Fatal("expected PolicyRule to be added")
	}
}

// StorageClass setter tests
func TestSetStorageClassProvisioner_Success(t *testing.T) {
	sc := CreateStorageClass("test", "kubernetes.io/gce-pd")
	SetStorageClassProvisioner(sc, "kubernetes.io/aws-ebs")
	if sc.Provisioner != "kubernetes.io/aws-ebs" {
		t.Fatal("expected Provisioner to be set")
	}
}

func TestSetStorageClassReclaimPolicy_Success(t *testing.T) {
	sc := CreateStorageClass("test", "kubernetes.io/gce-pd")
	policy := corev1.PersistentVolumeReclaimRetain
	SetStorageClassReclaimPolicy(sc, policy)
	if sc.ReclaimPolicy == nil || *sc.ReclaimPolicy != policy {
		t.Fatal("expected ReclaimPolicy to be set")
	}
}

func TestSetStorageClassVolumeBindingMode_Success(t *testing.T) {
	sc := CreateStorageClass("test", "kubernetes.io/gce-pd")
	mode := storagev1.VolumeBindingWaitForFirstConsumer
	SetStorageClassVolumeBindingMode(sc, mode)
	if sc.VolumeBindingMode == nil || *sc.VolumeBindingMode != mode {
		t.Fatal("expected VolumeBindingMode to be set")
	}
}

func TestSetStorageClassAllowVolumeExpansion_Success(t *testing.T) {
	sc := CreateStorageClass("test", "kubernetes.io/gce-pd")
	SetStorageClassAllowVolumeExpansion(sc, true)
	if sc.AllowVolumeExpansion == nil || !*sc.AllowVolumeExpansion {
		t.Fatal("expected AllowVolumeExpansion to be true")
	}
}

func TestAddStorageClassParameter_Success(t *testing.T) {
	sc := CreateStorageClass("test", "kubernetes.io/gce-pd")
	AddStorageClassParameter(sc, "type", "gp2")
	if sc.Parameters["type"] != "gp2" {
		t.Fatal("expected Parameter to be added")
	}
}

func TestAddStorageClassAllowedTopology_Success(t *testing.T) {
	sc := CreateStorageClass("test", "kubernetes.io/gce-pd")
	topology := corev1.TopologySelectorTerm{
		MatchLabelExpressions: []corev1.TopologySelectorLabelRequirement{
			{Key: "zone", Values: []string{"us-west-1a"}},
		},
	}
	AddStorageClassAllowedTopology(sc, topology)
	if len(sc.AllowedTopologies) != 1 {
		t.Fatal("expected AllowedTopology to be added")
	}
}

func TestSetStorageClassMountOptions_Success(t *testing.T) {
	sc := CreateStorageClass("test", "kubernetes.io/gce-pd")
	opts := []string{"ro", "noatime"}
	SetStorageClassMountOptions(sc, opts)
	if len(sc.MountOptions) != 2 {
		t.Fatal("expected MountOptions to be set")
	}
}

// Service setter tests
func TestAddServicePort_Success(t *testing.T) {
	svc := CreateService("test", "default")
	port := corev1.ServicePort{
		Name:       "http",
		Port:       80,
		TargetPort: intstr.FromInt(8080),
	}
	err := AddServicePort(svc, port)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(svc.Spec.Ports) != 1 {
		t.Fatal("expected Port to be added")
	}
}

func TestSetServiceType_Success(t *testing.T) {
	svc := CreateService("test", "default")
	err := SetServiceType(svc, corev1.ServiceTypeLoadBalancer)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if svc.Spec.Type != corev1.ServiceTypeLoadBalancer {
		t.Fatal("expected Type to be set")
	}
}

func TestSetServiceClusterIP_Success(t *testing.T) {
	svc := CreateService("test", "default")
	err := SetServiceClusterIP(svc, "10.0.0.1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if svc.Spec.ClusterIP != "10.0.0.1" {
		t.Fatal("expected ClusterIP to be set")
	}
}

func TestAddServiceExternalIP_Success(t *testing.T) {
	svc := CreateService("test", "default")
	err := AddServiceExternalIP(svc, "1.2.3.4")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(svc.Spec.ExternalIPs) != 1 {
		t.Fatal("expected ExternalIP to be added")
	}
}

func TestSetServiceLoadBalancerIP_Success(t *testing.T) {
	svc := CreateService("test", "default")
	err := SetServiceLoadBalancerIP(svc, "1.2.3.4")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if svc.Spec.LoadBalancerIP != "1.2.3.4" {
		t.Fatal("expected LoadBalancerIP to be set")
	}
}

func TestSetServiceSessionAffinity_Success(t *testing.T) {
	svc := CreateService("test", "default")
	err := SetServiceSessionAffinity(svc, corev1.ServiceAffinityClientIP)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if svc.Spec.SessionAffinity != corev1.ServiceAffinityClientIP {
		t.Fatal("expected SessionAffinity to be set")
	}
}
