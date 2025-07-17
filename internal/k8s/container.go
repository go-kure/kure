package k8s

import (
	"errors"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// CreateContainer returns a Container populated with the provided name, image,
// command and arguments. All collection fields are initialized and basic
// resource requests and limits are set.
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

// AddContainerPort appends a container port to the Ports slice.
func AddContainerPort(container *corev1.Container, port corev1.ContainerPort) error {
	if container == nil {
		return errors.New("nil container")
	}
	container.Ports = append(container.Ports, port)
	return nil
}

// AddContainerEnv appends an environment variable to the container.
func AddContainerEnv(container *corev1.Container, env corev1.EnvVar) error {
	if container == nil {
		return errors.New("nil container")
	}
	container.Env = append(container.Env, env)
	return nil
}

// AddContainerEnvFrom appends an EnvFromSource entry to the container.
func AddContainerEnvFrom(container *corev1.Container, envFrom corev1.EnvFromSource) error {
	if container == nil {
		return errors.New("nil container")
	}
	container.EnvFrom = append(container.EnvFrom, envFrom)
	return nil
}

// AddContainerVolumeMount appends a volume mount to the container.
func AddContainerVolumeMount(container *corev1.Container, volumeMount corev1.VolumeMount) error {
	if container == nil {
		return errors.New("nil container")
	}
	container.VolumeMounts = append(container.VolumeMounts, volumeMount)
	return nil
}

// AddContainerVolumeDevice appends a volume device to the container.
func AddContainerVolumeDevice(container *corev1.Container, volumeDevice corev1.VolumeDevice) error {
	if container == nil {
		return errors.New("nil container")
	}
	container.VolumeDevices = append(container.VolumeDevices, volumeDevice)
	return nil
}

// SetContainerLivenessProbe sets the container's liveness probe.
func SetContainerLivenessProbe(container *corev1.Container, livenessProbe corev1.Probe) error {
	if container == nil {
		return errors.New("nil container")
	}
	container.LivenessProbe = &livenessProbe
	return nil
}

// SetContainerReadinessProbe sets the container's readiness probe.
func SetContainerReadinessProbe(container *corev1.Container, readinessProbe corev1.Probe) error {
	if container == nil {
		return errors.New("nil container")
	}
	container.ReadinessProbe = &readinessProbe
	return nil
}

// SetContainerStartupProbe sets the container's startup probe.
func SetContainerStartupProbe(container *corev1.Container, startupProbe corev1.Probe) error {
	if container == nil {
		return errors.New("nil container")
	}
	container.StartupProbe = &startupProbe
	return nil
}

// SetContainerResources sets resource requirements on the container.
func SetContainerResources(container *corev1.Container, resources corev1.ResourceRequirements) error {
	if container == nil {
		return errors.New("nil container")
	}
	container.Resources = resources
	return nil
}

// SetContainerImagePullPolicy sets the image pull policy.
func SetContainerImagePullPolicy(container *corev1.Container, imagePullPolicy corev1.PullPolicy) error {
	if container == nil {
		return errors.New("nil container")
	}
	container.ImagePullPolicy = imagePullPolicy
	return nil
}

// SetContainerSecurityContext sets the security context on the container.
func SetContainerSecurityContext(container *corev1.Container, securityContext corev1.SecurityContext) error {
	if container == nil {
		return errors.New("nil container")
	}
	container.SecurityContext = &securityContext
	return nil
}

func SetContainerWorkingDir(container *corev1.Container, dir string) {
	container.WorkingDir = dir
}

func SetContainerLifecycle(container *corev1.Container, lifecycle *corev1.Lifecycle) {
	container.Lifecycle = lifecycle
}

func SetContainerTerminationMessagePath(container *corev1.Container, path string) {
	container.TerminationMessagePath = path
}

func SetContainerTerminationMessagePolicy(container *corev1.Container, policy corev1.TerminationMessagePolicy) {
	container.TerminationMessagePolicy = policy
}

func SetContainerStdin(container *corev1.Container, stdin bool) {
	container.Stdin = stdin
}

func SetContainerStdinOnce(container *corev1.Container, once bool) {
	container.StdinOnce = once
}

func SetContainerTTY(container *corev1.Container, tty bool) {
	container.TTY = tty
}
