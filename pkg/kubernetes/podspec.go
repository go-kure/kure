package kubernetes

import (
	corev1 "k8s.io/api/core/v1"

	"github.com/go-kure/kure/pkg/errors"
)

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

// SetPodSpecDNSConfig sets the DNS config.
func SetPodSpecDNSConfig(spec *corev1.PodSpec, cfg *corev1.PodDNSConfig) {
	if spec == nil {
		panic("SetPodSpecDNSConfig: spec must not be nil")
	}
	spec.DNSConfig = cfg
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
