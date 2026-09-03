package kubernetes

import (
	corev1 "k8s.io/api/core/v1"
)

// AddContainerPort appends a container port to the Ports slice.
func AddContainerPort(container *corev1.Container, port corev1.ContainerPort) {
	if container == nil {
		panic("AddContainerPort: container must not be nil")
	}
	container.Ports = append(container.Ports, port)
}

// AddContainerEnv appends an environment variable to the container.
func AddContainerEnv(container *corev1.Container, env corev1.EnvVar) {
	if container == nil {
		panic("AddContainerEnv: container must not be nil")
	}
	container.Env = append(container.Env, env)
}

// AddContainerEnvFrom appends an EnvFromSource entry to the container.
func AddContainerEnvFrom(container *corev1.Container, envFrom corev1.EnvFromSource) {
	if container == nil {
		panic("AddContainerEnvFrom: container must not be nil")
	}
	container.EnvFrom = append(container.EnvFrom, envFrom)
}

// AddContainerVolumeMount appends a volume mount to the container.
func AddContainerVolumeMount(container *corev1.Container, volumeMount corev1.VolumeMount) {
	if container == nil {
		panic("AddContainerVolumeMount: container must not be nil")
	}
	container.VolumeMounts = append(container.VolumeMounts, volumeMount)
}

// AddContainerVolumeDevice appends a volume device to the container.
func AddContainerVolumeDevice(container *corev1.Container, volumeDevice corev1.VolumeDevice) {
	if container == nil {
		panic("AddContainerVolumeDevice: container must not be nil")
	}
	container.VolumeDevices = append(container.VolumeDevices, volumeDevice)
}

// SetContainerLivenessProbe sets the container's liveness probe.
func SetContainerLivenessProbe(container *corev1.Container, livenessProbe corev1.Probe) {
	if container == nil {
		panic("SetContainerLivenessProbe: container must not be nil")
	}
	container.LivenessProbe = &livenessProbe
}

// SetContainerReadinessProbe sets the container's readiness probe.
func SetContainerReadinessProbe(container *corev1.Container, readinessProbe corev1.Probe) {
	if container == nil {
		panic("SetContainerReadinessProbe: container must not be nil")
	}
	container.ReadinessProbe = &readinessProbe
}

// SetContainerStartupProbe sets the container's startup probe.
func SetContainerStartupProbe(container *corev1.Container, startupProbe corev1.Probe) {
	if container == nil {
		panic("SetContainerStartupProbe: container must not be nil")
	}
	container.StartupProbe = &startupProbe
}

// SetContainerSecurityContext sets the security context on the container.
func SetContainerSecurityContext(container *corev1.Container, securityContext corev1.SecurityContext) {
	if container == nil {
		panic("SetContainerSecurityContext: container must not be nil")
	}
	container.SecurityContext = &securityContext
}

func SetContainerLifecycle(container *corev1.Container, lifecycle *corev1.Lifecycle) {
	container.Lifecycle = lifecycle
}
