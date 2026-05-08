package kubernetes

import (
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

// SetContainerResources sets resource requirements on the container.
func SetContainerResources(container *corev1.Container, resources corev1.ResourceRequirements) {
	if container == nil {
		panic("SetContainerResources: container must not be nil")
	}
	container.Resources = resources
}

// SetContainerImagePullPolicy sets the image pull policy.
func SetContainerImagePullPolicy(container *corev1.Container, imagePullPolicy corev1.PullPolicy) {
	if container == nil {
		panic("SetContainerImagePullPolicy: container must not be nil")
	}
	container.ImagePullPolicy = imagePullPolicy
}

// SetContainerSecurityContext sets the security context on the container.
func SetContainerSecurityContext(container *corev1.Container, securityContext corev1.SecurityContext) {
	if container == nil {
		panic("SetContainerSecurityContext: container must not be nil")
	}
	container.SecurityContext = &securityContext
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

// SetContainerImage sets the image on the container.
func SetContainerImage(container *corev1.Container, image string) {
	container.Image = image
}

// SetContainerCommand replaces the command slice on the container.
func SetContainerCommand(container *corev1.Container, command []string) {
	container.Command = command
}

// SetContainerArgs replaces the args slice on the container.
func SetContainerArgs(container *corev1.Container, args []string) {
	container.Args = args
}
