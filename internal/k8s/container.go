package k8s

import (
	"errors"

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
		ImagePullPolicy: corev1.PullIfNotPresent,
	}
	return &obj
}

func AddContainerPort(container *corev1.Container, port corev1.ContainerPort) error {
	if container == nil {
		return errors.New("nil container")
	}
	container.Ports = append(container.Ports, port)
	return nil
}

func AddContainerEnv(container *corev1.Container, env corev1.EnvVar) error {
	if container == nil {
		return errors.New("nil container")
	}
	container.Env = append(container.Env, env)
	return nil
}

func AddContainerEnvFrom(container *corev1.Container, envFrom corev1.EnvFromSource) error {
	if container == nil {
		return errors.New("nil container")
	}
	container.EnvFrom = append(container.EnvFrom, envFrom)
	return nil
}

func AddContainerVolumeMount(container *corev1.Container, volumeMount corev1.VolumeMount) error {
	if container == nil {
		return errors.New("nil container")
	}
	container.VolumeMounts = append(container.VolumeMounts, volumeMount)
	return nil
}

func AddContainerVolumeDevice(container *corev1.Container, volumeDevice corev1.VolumeDevice) error {
	if container == nil {
		return errors.New("nil container")
	}
	container.VolumeDevices = append(container.VolumeDevices, volumeDevice)
	return nil
}

func SetContainerLivenessProbe(container *corev1.Container, livenessProbe corev1.Probe) error {
	if container == nil {
		return errors.New("nil container")
	}
	container.LivenessProbe = &livenessProbe
	return nil
}

func SetContainerReadinessProbe(container *corev1.Container, readinessProbe corev1.Probe) error {
	if container == nil {
		return errors.New("nil container")
	}
	container.ReadinessProbe = &readinessProbe
	return nil
}

func SetContainerStartupProbe(container *corev1.Container, startupProbe corev1.Probe) error {
	if container == nil {
		return errors.New("nil container")
	}
	container.StartupProbe = &startupProbe
	return nil
}

func SetContainerResources(container *corev1.Container, resources corev1.ResourceRequirements) error {
	if container == nil {
		return errors.New("nil container")
	}
	container.Resources = resources
	return nil
}

func SetContainerImagePullPolicy(container *corev1.Container, imagePullPolicy corev1.PullPolicy) error {
	if container == nil {
		return errors.New("nil container")
	}
	container.ImagePullPolicy = imagePullPolicy
	return nil
}

func SetContainerSecurityContext(container *corev1.Container, securityContext corev1.SecurityContext) error {
	if container == nil {
		return errors.New("nil container")
	}
	container.SecurityContext = &securityContext
	return nil
}
