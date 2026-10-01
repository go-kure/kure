package kubernetes_test

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/go-kure/kure/internal/kuretest"
	. "github.com/go-kure/kure/pkg/kubernetes"
)

func testInitContainer() *corev1.Container {
	return &corev1.Container{
		Name:    "init-data",
		Image:   "busybox:1.36",
		Command: []string{"/bin/sh", "-c"},
		Args:    []string{"cp /src/config.yaml /data/config.yaml"},
		VolumeMounts: []corev1.VolumeMount{
			{Name: "data-vol", MountPath: "/data"},
		},
		Env: []corev1.EnvVar{
			{Name: "INIT_MODE", Value: "copy"},
		},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("50m"),
				corev1.ResourceMemory: resource.MustParse("64Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("128Mi"),
			},
		},
	}
}

func testAppContainer() *corev1.Container {
	return &corev1.Container{
		Name:  "app",
		Image: "myapp:latest",
	}
}

func testDataVolume() *corev1.Volume {
	return &corev1.Volume{
		Name: "data-vol",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
}

func TestDeploymentInitContainer_Golden(t *testing.T) {
	dep := CreateDeployment("my-app", "default")
	spec := &dep.Spec.Template.Spec
	AddPodSpecContainer(spec, testAppContainer())
	AddPodSpecInitContainer(spec, testInitContainer())
	AddPodSpecVolume(spec, testDataVolume())

	kuretest.Golden(t, "deployment-with-init-container.yaml", dep)
}

func TestStatefulSetInitContainer_Golden(t *testing.T) {
	sts := CreateStatefulSet("my-app", "default")
	spec := &sts.Spec.Template.Spec
	AddPodSpecContainer(spec, testAppContainer())
	AddPodSpecInitContainer(spec, testInitContainer())
	AddPodSpecVolume(spec, testDataVolume())

	kuretest.Golden(t, "statefulset-with-init-container.yaml", sts)
}

func TestDaemonSetInitContainer_Golden(t *testing.T) {
	ds := CreateDaemonSet("my-app", "default")
	spec := &ds.Spec.Template.Spec
	AddPodSpecContainer(spec, testAppContainer())
	AddPodSpecInitContainer(spec, testInitContainer())
	AddPodSpecVolume(spec, testDataVolume())

	kuretest.Golden(t, "daemonset-with-init-container.yaml", ds)
}
