package kubernetes

import (
	corev1 "k8s.io/api/core/v1"

	"github.com/go-kure/kure/internal/validation"
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
	validator := validation.NewValidator()
	if err := validator.ValidatePodSpec(spec); err != nil {
		return err
	}
	if err := validator.ValidateContainer(container); err != nil {
		return err
	}
	spec.Containers = append(spec.Containers, *container)
	return nil
}

// AddPodSpecInitContainer appends an init container to the PodSpec.
func AddPodSpecInitContainer(spec *corev1.PodSpec, container *corev1.Container) error {
	validator := validation.NewValidator()
	if err := validator.ValidatePodSpec(spec); err != nil {
		return err
	}
	if err := validator.ValidateInitContainer(container); err != nil {
		return err
	}
	spec.InitContainers = append(spec.InitContainers, *container)
	return nil
}

// AddPodSpecEphemeralContainer appends an ephemeral container to the PodSpec.
func AddPodSpecEphemeralContainer(spec *corev1.PodSpec, container *corev1.EphemeralContainer) error {
	validator := validation.NewValidator()
	if err := validator.ValidatePodSpec(spec); err != nil {
		return err
	}
	if err := validator.ValidateEphemeralContainer(container); err != nil {
		return err
	}
	spec.EphemeralContainers = append(spec.EphemeralContainers, *container)
	return nil
}

// AddPodSpecVolume appends a volume to the PodSpec.
func AddPodSpecVolume(spec *corev1.PodSpec, volume *corev1.Volume) error {
	validator := validation.NewValidator()
	if err := validator.ValidatePodSpec(spec); err != nil {
		return err
	}
	if err := validator.ValidateVolume(volume); err != nil {
		return err
	}
	spec.Volumes = append(spec.Volumes, *volume)
	return nil
}

// AddPodSpecImagePullSecret appends an image pull secret to the PodSpec.
func AddPodSpecImagePullSecret(spec *corev1.PodSpec, secret *corev1.LocalObjectReference) error {
	validator := validation.NewValidator()
	if err := validator.ValidatePodSpec(spec); err != nil {
		return err
	}
	if err := validator.ValidateImagePullSecret(secret); err != nil {
		return err
	}
	spec.ImagePullSecrets = append(spec.ImagePullSecrets, *secret)
	return nil
}

// AddPodSpecToleration appends a toleration to the PodSpec.
func AddPodSpecToleration(spec *corev1.PodSpec, toleration *corev1.Toleration) error {
	validator := validation.NewValidator()
	if err := validator.ValidatePodSpec(spec); err != nil {
		return err
	}
	if err := validator.ValidateToleration(toleration); err != nil {
		return err
	}
	spec.Tolerations = append(spec.Tolerations, *toleration)
	return nil
}

// AddPodSpecTopologySpreadConstraints appends a topology spread constraint if provided.
func AddPodSpecTopologySpreadConstraints(spec *corev1.PodSpec, constraint *corev1.TopologySpreadConstraint) error {
	validator := validation.NewValidator()
	if err := validator.ValidatePodSpec(spec); err != nil {
		return err
	}
	if constraint == nil {
		return nil
	}
	spec.TopologySpreadConstraints = append(spec.TopologySpreadConstraints, *constraint)
	return nil
}

// SetPodSpecServiceAccountName sets the service account name.
func SetPodSpecServiceAccountName(spec *corev1.PodSpec, name string) error {
	validator := validation.NewValidator()
	if err := validator.ValidatePodSpec(spec); err != nil {
		return err
	}
	spec.ServiceAccountName = name
	return nil
}

// SetPodSpecSecurityContext sets the security context for the PodSpec.
func SetPodSpecSecurityContext(spec *corev1.PodSpec, sc *corev1.PodSecurityContext) error {
	validator := validation.NewValidator()
	if err := validator.ValidatePodSpec(spec); err != nil {
		return err
	}
	spec.SecurityContext = sc
	return nil
}

// SetPodSpecAffinity assigns affinity rules to the PodSpec.
func SetPodSpecAffinity(spec *corev1.PodSpec, aff *corev1.Affinity) error {
	validator := validation.NewValidator()
	if err := validator.ValidatePodSpec(spec); err != nil {
		return err
	}
	spec.Affinity = aff
	return nil
}

// SetPodSpecNodeSelector sets the node selector map.
func SetPodSpecNodeSelector(spec *corev1.PodSpec, selector map[string]string) error {
	validator := validation.NewValidator()
	if err := validator.ValidatePodSpec(spec); err != nil {
		return err
	}
	spec.NodeSelector = selector
	return nil
}

// SetPodSpecPriorityClassName sets the priority class name.
func SetPodSpecPriorityClassName(spec *corev1.PodSpec, class string) error {
	validator := validation.NewValidator()
	if err := validator.ValidatePodSpec(spec); err != nil {
		return err
	}
	spec.PriorityClassName = class
	return nil
}

// SetPodSpecHostNetwork configures host networking.
func SetPodSpecHostNetwork(spec *corev1.PodSpec, hostNetwork bool) error {
	validator := validation.NewValidator()
	if err := validator.ValidatePodSpec(spec); err != nil {
		return err
	}
	spec.HostNetwork = hostNetwork
	return nil
}

// SetPodSpecHostPID configures host PID namespace usage.
func SetPodSpecHostPID(spec *corev1.PodSpec, hostPID bool) error {
	validator := validation.NewValidator()
	if err := validator.ValidatePodSpec(spec); err != nil {
		return err
	}
	spec.HostPID = hostPID
	return nil
}

// SetPodSpecHostIPC configures host IPC namespace usage.
func SetPodSpecHostIPC(spec *corev1.PodSpec, hostIPC bool) error {
	validator := validation.NewValidator()
	if err := validator.ValidatePodSpec(spec); err != nil {
		return err
	}
	spec.HostIPC = hostIPC
	return nil
}

// SetPodSpecDNSPolicy sets the DNS policy.
func SetPodSpecDNSPolicy(spec *corev1.PodSpec, policy corev1.DNSPolicy) error {
	validator := validation.NewValidator()
	if err := validator.ValidatePodSpec(spec); err != nil {
		return err
	}
	spec.DNSPolicy = policy
	return nil
}

// SetPodSpecDNSConfig sets the DNS config.
func SetPodSpecDNSConfig(spec *corev1.PodSpec, cfg *corev1.PodDNSConfig) error {
	validator := validation.NewValidator()
	if err := validator.ValidatePodSpec(spec); err != nil {
		return err
	}
	spec.DNSConfig = cfg
	return nil
}

// SetPodSpecHostname sets the hostname.
func SetPodSpecHostname(spec *corev1.PodSpec, hostname string) error {
	validator := validation.NewValidator()
	if err := validator.ValidatePodSpec(spec); err != nil {
		return err
	}
	spec.Hostname = hostname
	return nil
}

// SetPodSpecSubdomain sets the subdomain.
func SetPodSpecSubdomain(spec *corev1.PodSpec, subdomain string) error {
	validator := validation.NewValidator()
	if err := validator.ValidatePodSpec(spec); err != nil {
		return err
	}
	spec.Subdomain = subdomain
	return nil
}

// SetPodSpecRestartPolicy sets the restart policy.
func SetPodSpecRestartPolicy(spec *corev1.PodSpec, policy corev1.RestartPolicy) error {
	validator := validation.NewValidator()
	if err := validator.ValidatePodSpec(spec); err != nil {
		return err
	}
	spec.RestartPolicy = policy
	return nil
}

// SetPodSpecTerminationGracePeriod sets the termination grace period seconds.
func SetPodSpecTerminationGracePeriod(spec *corev1.PodSpec, secs int64) error {
	validator := validation.NewValidator()
	if err := validator.ValidatePodSpec(spec); err != nil {
		return err
	}
	if spec.TerminationGracePeriodSeconds == nil {
		spec.TerminationGracePeriodSeconds = new(int64)
	}
	*spec.TerminationGracePeriodSeconds = secs
	return nil
}

// SetPodSpecSchedulerName sets the scheduler name.
func SetPodSpecSchedulerName(spec *corev1.PodSpec, scheduler string) error {
	validator := validation.NewValidator()
	if err := validator.ValidatePodSpec(spec); err != nil {
		return err
	}
	spec.SchedulerName = scheduler
	return nil
}
