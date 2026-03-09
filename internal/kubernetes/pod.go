package kubernetes

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreatePod returns a Pod with the provided name and namespace. The object is
// populated with sensible defaults for metadata and spec fields.
func CreatePod(name string, namespace string) *corev1.Pod {
	obj := corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"app": name,
			},
			Annotations: map[string]string{
				"app": name,
			},
		},
		Spec: corev1.PodSpec{},
	}
	return &obj
}

// SetPodSpec assigns a pod spec to the Pod.
func SetPodSpec(pod *corev1.Pod, spec *corev1.PodSpec) {
	pod.Spec = *spec
}

// AddPodContainer appends a container to the Pod spec.
func AddPodContainer(pod *corev1.Pod, container *corev1.Container) {
	pod.Spec.Containers = append(pod.Spec.Containers, *container)
}

// AddPodInitContainer appends an init container to the Pod spec.
func AddPodInitContainer(pod *corev1.Pod, container *corev1.Container) {
	pod.Spec.InitContainers = append(pod.Spec.InitContainers, *container)
}

// AddPodEphemeralContainer appends an ephemeral container to the Pod spec.
func AddPodEphemeralContainer(pod *corev1.Pod, container *corev1.EphemeralContainer) {
	pod.Spec.EphemeralContainers = append(pod.Spec.EphemeralContainers, *container)
}

// AddPodVolume appends a volume to the Pod spec.
func AddPodVolume(pod *corev1.Pod, volume *corev1.Volume) {
	pod.Spec.Volumes = append(pod.Spec.Volumes, *volume)
}

// AddPodImagePullSecret appends an image pull secret to the Pod spec.
func AddPodImagePullSecret(pod *corev1.Pod, imagePullSecret *corev1.LocalObjectReference) {
	pod.Spec.ImagePullSecrets = append(pod.Spec.ImagePullSecrets, *imagePullSecret)
}

// AddPodToleration appends a toleration to the Pod spec.
func AddPodToleration(pod *corev1.Pod, toleration *corev1.Toleration) {
	pod.Spec.Tolerations = append(pod.Spec.Tolerations, *toleration)
}

// AddPodTopologySpreadConstraints appends a topology spread constraint if provided.
func AddPodTopologySpreadConstraints(pod *corev1.Pod, topologySpreadConstraint *corev1.TopologySpreadConstraint) {
	pod.Spec.TopologySpreadConstraints = append(pod.Spec.TopologySpreadConstraints, *topologySpreadConstraint)
}

// SetPodServiceAccountName sets the service account used by the Pod.
func SetPodServiceAccountName(pod *corev1.Pod, serviceAccountName string) {
	pod.Spec.ServiceAccountName = serviceAccountName
}

// SetPodSecurityContext sets the pod-level security context.
func SetPodSecurityContext(pod *corev1.Pod, securityContext *corev1.PodSecurityContext) {
	pod.Spec.SecurityContext = securityContext
}

// SetPodAffinity assigns affinity rules to the Pod.
func SetPodAffinity(pod *corev1.Pod, affinity *corev1.Affinity) {
	pod.Spec.Affinity = affinity
}

// SetPodNodeSelector sets the node selector map.
func SetPodNodeSelector(pod *corev1.Pod, nodeSelector map[string]string) {
	pod.Spec.NodeSelector = nodeSelector
}

// SetPodPriorityClassName sets the priority class name.
func SetPodPriorityClassName(pod *corev1.Pod, class string) {
	pod.Spec.PriorityClassName = class
}

// SetPodHostNetwork configures host networking for the Pod.
func SetPodHostNetwork(pod *corev1.Pod, hostNetwork bool) {
	pod.Spec.HostNetwork = hostNetwork
}

// SetPodHostPID configures host PID namespace usage for the Pod.
func SetPodHostPID(pod *corev1.Pod, hostPID bool) {
	pod.Spec.HostPID = hostPID
}

// SetPodHostIPC configures host IPC namespace usage for the Pod.
func SetPodHostIPC(pod *corev1.Pod, hostIPC bool) {
	pod.Spec.HostIPC = hostIPC
}

// SetPodDNSPolicy sets the DNS policy for the Pod.
func SetPodDNSPolicy(pod *corev1.Pod, policy corev1.DNSPolicy) {
	pod.Spec.DNSPolicy = policy
}

func SetPodDNSConfig(pod *corev1.Pod, dnsConfig *corev1.PodDNSConfig) {
	pod.Spec.DNSConfig = dnsConfig
}

func SetPodHostname(pod *corev1.Pod, hostname string) {
	pod.Spec.Hostname = hostname
}

func SetPodSubdomain(pod *corev1.Pod, subdomain string) {
	pod.Spec.Subdomain = subdomain
}

func SetPodRestartPolicy(pod *corev1.Pod, policy corev1.RestartPolicy) {
	pod.Spec.RestartPolicy = policy
}

func SetPodTerminationGracePeriod(pod *corev1.Pod, secs int64) {
	if pod.Spec.TerminationGracePeriodSeconds == nil {
		pod.Spec.TerminationGracePeriodSeconds = new(int64)
	}
	*pod.Spec.TerminationGracePeriodSeconds = secs
}

func SetPodSchedulerName(pod *corev1.Pod, scheduler string) {
	pod.Spec.SchedulerName = scheduler
}
