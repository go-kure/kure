package kubernetes

import (
	corev1 "k8s.io/api/core/v1"

	"github.com/go-kure/kure/pkg/errors"
)

// CreatePodSpec returns a PodSpec initialized with sensible defaults.
func CreatePodSpec() *corev1.PodSpec {
	obj := corev1.PodSpec{
		Containers:                    []corev1.Container{},
		InitContainers:                []corev1.Container{},
		Volumes:                       []corev1.Volume{},
		RestartPolicy:                 corev1.RestartPolicyAlways,
		TerminationGracePeriodSeconds: new(int64),
		SecurityContext:               &corev1.PodSecurityContext{},
		ImagePullSecrets:              []corev1.LocalObjectReference{},
		ServiceAccountName:            "",
		NodeSelector:                  map[string]string{},
		Affinity:                      &corev1.Affinity{},
		Tolerations:                   []corev1.Toleration{},
	}
	return &obj
}

// AddPodSpecContainer appends a container to the PodSpec.
func AddPodSpecContainer(spec *corev1.PodSpec, container *corev1.Container) error {
	if spec == nil {
		return errors.ErrNilPodSpec
	}
	if container == nil {
		return errors.ErrNilContainer
	}
	spec.Containers = append(spec.Containers, *container)
	return nil
}

// AddPodSpecInitContainer appends an init container to the PodSpec.
func AddPodSpecInitContainer(spec *corev1.PodSpec, container *corev1.Container) error {
	if spec == nil {
		return errors.ErrNilPodSpec
	}
	if container == nil {
		return errors.ErrNilInitContainer
	}
	spec.InitContainers = append(spec.InitContainers, *container)
	return nil
}

// AddPodSpecEphemeralContainer appends an ephemeral container to the PodSpec.
func AddPodSpecEphemeralContainer(spec *corev1.PodSpec, container *corev1.EphemeralContainer) error {
	if spec == nil {
		return errors.ErrNilPodSpec
	}
	if container == nil {
		return errors.ErrNilEphemeralContainer
	}
	spec.EphemeralContainers = append(spec.EphemeralContainers, *container)
	return nil
}

// AddPodSpecVolume appends a volume to the PodSpec.
func AddPodSpecVolume(spec *corev1.PodSpec, volume *corev1.Volume) error {
	if spec == nil {
		return errors.ErrNilPodSpec
	}
	if volume == nil {
		return errors.ErrNilVolume
	}
	spec.Volumes = append(spec.Volumes, *volume)
	return nil
}

// AddPodSpecImagePullSecret appends an image pull secret to the PodSpec.
func AddPodSpecImagePullSecret(spec *corev1.PodSpec, secret *corev1.LocalObjectReference) error {
	if spec == nil {
		return errors.ErrNilPodSpec
	}
	if secret == nil {
		return errors.ErrNilImagePullSecret
	}
	spec.ImagePullSecrets = append(spec.ImagePullSecrets, *secret)
	return nil
}

// AddPodSpecToleration appends a toleration to the PodSpec.
func AddPodSpecToleration(spec *corev1.PodSpec, toleration *corev1.Toleration) error {
	if spec == nil {
		return errors.ErrNilPodSpec
	}
	if toleration == nil {
		return errors.ErrNilToleration
	}
	spec.Tolerations = append(spec.Tolerations, *toleration)
	return nil
}

// AddPodSpecTopologySpreadConstraints appends a topology spread constraint if provided.
func AddPodSpecTopologySpreadConstraints(spec *corev1.PodSpec, constraint *corev1.TopologySpreadConstraint) error {
	if spec == nil {
		return errors.ErrNilPodSpec
	}
	if constraint == nil {
		return nil
	}
	spec.TopologySpreadConstraints = append(spec.TopologySpreadConstraints, *constraint)
	return nil
}

// SetPodSpecServiceAccountName sets the service account name.
func SetPodSpecServiceAccountName(spec *corev1.PodSpec, name string) {
	if spec == nil {
		panic("SetPodSpecServiceAccountName: spec must not be nil")
	}
	spec.ServiceAccountName = name
}

// SetPodSpecSecurityContext sets the security context for the PodSpec.
func SetPodSpecSecurityContext(spec *corev1.PodSpec, sc *corev1.PodSecurityContext) {
	if spec == nil {
		panic("SetPodSpecSecurityContext: spec must not be nil")
	}
	spec.SecurityContext = sc
}

// SetPodSpecAffinity assigns affinity rules to the PodSpec.
func SetPodSpecAffinity(spec *corev1.PodSpec, aff *corev1.Affinity) {
	if spec == nil {
		panic("SetPodSpecAffinity: spec must not be nil")
	}
	spec.Affinity = aff
}

// SetPodSpecNodeSelector sets the node selector map.
func SetPodSpecNodeSelector(spec *corev1.PodSpec, selector map[string]string) {
	if spec == nil {
		panic("SetPodSpecNodeSelector: spec must not be nil")
	}
	spec.NodeSelector = selector
}

// SetPodSpecPriorityClassName sets the priority class name.
func SetPodSpecPriorityClassName(spec *corev1.PodSpec, class string) {
	if spec == nil {
		panic("SetPodSpecPriorityClassName: spec must not be nil")
	}
	spec.PriorityClassName = class
}

// SetPodSpecHostNetwork configures host networking.
func SetPodSpecHostNetwork(spec *corev1.PodSpec, hostNetwork bool) {
	if spec == nil {
		panic("SetPodSpecHostNetwork: spec must not be nil")
	}
	spec.HostNetwork = hostNetwork
}

// SetPodSpecHostPID configures host PID namespace usage.
func SetPodSpecHostPID(spec *corev1.PodSpec, hostPID bool) {
	if spec == nil {
		panic("SetPodSpecHostPID: spec must not be nil")
	}
	spec.HostPID = hostPID
}

// SetPodSpecHostIPC configures host IPC namespace usage.
func SetPodSpecHostIPC(spec *corev1.PodSpec, hostIPC bool) {
	if spec == nil {
		panic("SetPodSpecHostIPC: spec must not be nil")
	}
	spec.HostIPC = hostIPC
}

// SetPodSpecDNSPolicy sets the DNS policy.
func SetPodSpecDNSPolicy(spec *corev1.PodSpec, policy corev1.DNSPolicy) {
	if spec == nil {
		panic("SetPodSpecDNSPolicy: spec must not be nil")
	}
	spec.DNSPolicy = policy
}

// SetPodSpecDNSConfig sets the DNS config.
func SetPodSpecDNSConfig(spec *corev1.PodSpec, cfg *corev1.PodDNSConfig) {
	if spec == nil {
		panic("SetPodSpecDNSConfig: spec must not be nil")
	}
	spec.DNSConfig = cfg
}

// SetPodSpecHostname sets the hostname.
func SetPodSpecHostname(spec *corev1.PodSpec, hostname string) {
	if spec == nil {
		panic("SetPodSpecHostname: spec must not be nil")
	}
	spec.Hostname = hostname
}

// SetPodSpecSubdomain sets the subdomain.
func SetPodSpecSubdomain(spec *corev1.PodSpec, subdomain string) {
	if spec == nil {
		panic("SetPodSpecSubdomain: spec must not be nil")
	}
	spec.Subdomain = subdomain
}

// SetPodSpecRestartPolicy sets the restart policy.
func SetPodSpecRestartPolicy(spec *corev1.PodSpec, policy corev1.RestartPolicy) {
	if spec == nil {
		panic("SetPodSpecRestartPolicy: spec must not be nil")
	}
	spec.RestartPolicy = policy
}

// SetPodSpecTerminationGracePeriod sets the termination grace period seconds.
func SetPodSpecTerminationGracePeriod(spec *corev1.PodSpec, secs int64) {
	if spec == nil {
		panic("SetPodSpecTerminationGracePeriod: spec must not be nil")
	}
	if spec.TerminationGracePeriodSeconds == nil {
		spec.TerminationGracePeriodSeconds = new(int64)
	}
	*spec.TerminationGracePeriodSeconds = secs
}

// SetPodSpecSchedulerName sets the scheduler name.
func SetPodSpecSchedulerName(spec *corev1.PodSpec, scheduler string) {
	if spec == nil {
		panic("SetPodSpecSchedulerName: spec must not be nil")
	}
	spec.SchedulerName = scheduler
}
