package k8s

import (
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func CreateContainer(name string, image string, command []string, args []string) *apiv1.Container {
	container := apiv1.Container{
		Name:    name,
		Image:   image,
		Command: command,
		Args:    args,
		Ports:   []apiv1.ContainerPort{},
		EnvFrom: []apiv1.EnvFromSource{},
		Env:     []apiv1.EnvVar{},
		Resources: apiv1.ResourceRequirements{
			Limits: apiv1.ResourceList{
				"memory": resource.MustParse("256Mi"),
			},
			Requests: apiv1.ResourceList{
				"cpu":    resource.MustParse("100m"),
				"memory": resource.MustParse("256Mi"),
			},
		},
		VolumeMounts:    []apiv1.VolumeMount{},
		VolumeDevices:   []apiv1.VolumeDevice{},
		LivenessProbe:   &apiv1.Probe{},
		ReadinessProbe:  &apiv1.Probe{},
		StartupProbe:    &apiv1.Probe{},
		ImagePullPolicy: apiv1.PullIfNotPresent,
		SecurityContext: &apiv1.SecurityContext{},
	}
	return &container
}

func AddContainerPort(container *apiv1.Container, port apiv1.ContainerPort) {
	container.Ports = append(container.Ports, port)
}

func AddContainerEnv(container *apiv1.Container, env apiv1.EnvVar) {
	container.Env = append(container.Env, env)
}

func AddContainerEnvFrom(container *apiv1.Container, envFrom apiv1.EnvFromSource) {
	container.EnvFrom = append(container.EnvFrom, envFrom)
}

func AddContainerVolumeMount(container *apiv1.Container, volumeMount apiv1.VolumeMount) {
	container.VolumeMounts = append(container.VolumeMounts, volumeMount)
}

func AddContainerVolumeDevice(container *apiv1.Container, volumeDevice apiv1.VolumeDevice) {
	container.VolumeDevices = append(container.VolumeDevices, volumeDevice)
}

func SetContainerLivenessProbe(container *apiv1.Container, livenessProbe apiv1.Probe) {
	container.LivenessProbe = &livenessProbe
}

func SetContainerReadinessProbe(container *apiv1.Container, readinessProbe apiv1.Probe) {
	container.ReadinessProbe = &readinessProbe
}

func SetContainerStartupProbe(container *apiv1.Container, startupProbe apiv1.Probe) {
	container.StartupProbe = &startupProbe
}

func SetContainerResources(container *apiv1.Container, resources apiv1.ResourceRequirements) {
	container.Resources = resources
}

func SetContainerImagePullPolicy(container *apiv1.Container, imagePullPolicy apiv1.PullPolicy) {
	container.ImagePullPolicy = imagePullPolicy
}

func SetContainerSecurityContext(container *apiv1.Container, securityContext apiv1.SecurityContext) {
	container.SecurityContext = &securityContext
}
