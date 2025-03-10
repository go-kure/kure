package k8s

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func CreateContainer(name string, image string, command []string, args []string) *corev1.Container {
	obj := corev1.Container{
		Name:    name,
		Image:   image,
		Command: command,
		Args:    args,
		Ports:   []corev1.ContainerPort{},
		EnvFrom: []corev1.EnvFromSource{},
		Env:     []corev1.EnvVar{},
		Resources: corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				"memory": resource.MustParse("256Mi"),
			},
			Requests: corev1.ResourceList{
				"cpu":    resource.MustParse("100m"),
				"memory": resource.MustParse("256Mi"),
			},
		},
		VolumeMounts:    []corev1.VolumeMount{},
		VolumeDevices:   []corev1.VolumeDevice{},
		LivenessProbe:   &corev1.Probe{},
		ReadinessProbe:  &corev1.Probe{},
		StartupProbe:    &corev1.Probe{},
		ImagePullPolicy: corev1.PullIfNotPresent,
		SecurityContext: &corev1.SecurityContext{},
	}
	return &obj
}

func AddContainerPort(container *corev1.Container, port corev1.ContainerPort) {
	container.Ports = append(container.Ports, port)
}

func AddContainerEnv(container *corev1.Container, env corev1.EnvVar) {
	container.Env = append(container.Env, env)
}

func AddContainerEnvFrom(container *corev1.Container, envFrom corev1.EnvFromSource) {
	container.EnvFrom = append(container.EnvFrom, envFrom)
}

func AddContainerVolumeMount(container *corev1.Container, volumeMount corev1.VolumeMount) {
	container.VolumeMounts = append(container.VolumeMounts, volumeMount)
}

func AddContainerVolumeDevice(container *corev1.Container, volumeDevice corev1.VolumeDevice) {
	container.VolumeDevices = append(container.VolumeDevices, volumeDevice)
}

func SetContainerLivenessProbe(container *corev1.Container, livenessProbe corev1.Probe) {
	container.LivenessProbe = &livenessProbe
}

func SetContainerReadinessProbe(container *corev1.Container, readinessProbe corev1.Probe) {
	container.ReadinessProbe = &readinessProbe
}

func SetContainerStartupProbe(container *corev1.Container, startupProbe corev1.Probe) {
	container.StartupProbe = &startupProbe
}

func SetContainerResources(container *corev1.Container, resources corev1.ResourceRequirements) {
	container.Resources = resources
}

func SetContainerImagePullPolicy(container *corev1.Container, imagePullPolicy corev1.PullPolicy) {
	container.ImagePullPolicy = imagePullPolicy
}

func SetContainerSecurityContext(container *corev1.Container, securityContext corev1.SecurityContext) {
	container.SecurityContext = &securityContext
}
