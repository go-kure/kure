package kubernetes

import (
	corev1 "k8s.io/api/core/v1"
)

// AddPodSpecContainer appends a container to the PodSpec.
func AddPodSpecContainer(spec *corev1.PodSpec, container *corev1.Container) {
	if spec == nil {
		panic("AddPodSpecContainer: spec must not be nil")
	}
	if container == nil {
		panic("AddPodSpecContainer: container must not be nil")
	}
	spec.Containers = append(spec.Containers, *container)
}

// AddPodSpecInitContainer appends an init container to the PodSpec.
func AddPodSpecInitContainer(spec *corev1.PodSpec, container *corev1.Container) {
	if spec == nil {
		panic("AddPodSpecInitContainer: spec must not be nil")
	}
	if container == nil {
		panic("AddPodSpecInitContainer: container must not be nil")
	}
	spec.InitContainers = append(spec.InitContainers, *container)
}

// AddPodSpecEphemeralContainer appends an ephemeral container to the PodSpec.
func AddPodSpecEphemeralContainer(spec *corev1.PodSpec, container *corev1.EphemeralContainer) {
	if spec == nil {
		panic("AddPodSpecEphemeralContainer: spec must not be nil")
	}
	if container == nil {
		panic("AddPodSpecEphemeralContainer: container must not be nil")
	}
	spec.EphemeralContainers = append(spec.EphemeralContainers, *container)
}

// AddPodSpecVolume appends a volume to the PodSpec.
func AddPodSpecVolume(spec *corev1.PodSpec, volume *corev1.Volume) {
	if spec == nil {
		panic("AddPodSpecVolume: spec must not be nil")
	}
	if volume == nil {
		panic("AddPodSpecVolume: volume must not be nil")
	}
	spec.Volumes = append(spec.Volumes, *volume)
}

// AddPodSpecImagePullSecret appends an image pull secret to the PodSpec.
func AddPodSpecImagePullSecret(spec *corev1.PodSpec, secret *corev1.LocalObjectReference) {
	if spec == nil {
		panic("AddPodSpecImagePullSecret: spec must not be nil")
	}
	if secret == nil {
		panic("AddPodSpecImagePullSecret: secret must not be nil")
	}
	spec.ImagePullSecrets = append(spec.ImagePullSecrets, *secret)
}

// AddPodSpecToleration appends a toleration to the PodSpec.
func AddPodSpecToleration(spec *corev1.PodSpec, toleration *corev1.Toleration) {
	if spec == nil {
		panic("AddPodSpecToleration: spec must not be nil")
	}
	if toleration == nil {
		panic("AddPodSpecToleration: toleration must not be nil")
	}
	spec.Tolerations = append(spec.Tolerations, *toleration)
}

// AddPodSpecTopologySpreadConstraints appends a topology spread constraint to the PodSpec.
func AddPodSpecTopologySpreadConstraints(spec *corev1.PodSpec, constraint *corev1.TopologySpreadConstraint) {
	if spec == nil {
		panic("AddPodSpecTopologySpreadConstraints: spec must not be nil")
	}
	if constraint == nil {
		panic("AddPodSpecTopologySpreadConstraints: constraint must not be nil")
	}
	spec.TopologySpreadConstraints = append(spec.TopologySpreadConstraints, *constraint)
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
