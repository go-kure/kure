package k8s

import (
	"errors"

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
		Spec: corev1.PodSpec{
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
		},
	}
	return &obj
}

// AddPodContainer appends a container to the Pod spec.
func AddPodContainer(pod *corev1.Pod, container *corev1.Container) error {
	if pod == nil || container == nil {
		return errors.New("nil pod or container")
	}
	pod.Spec.Containers = append(pod.Spec.Containers, *container)
	return nil
}

// AddPodInitContainer appends an init container to the Pod spec.
func AddPodInitContainer(pod *corev1.Pod, container *corev1.Container) error {
	if pod == nil || container == nil {
		return errors.New("nil pod or container")
	}
	pod.Spec.InitContainers = append(pod.Spec.InitContainers, *container)
	return nil
}

// AddPodEphemeralContainer appends an ephemeral container to the Pod spec.
func AddPodEphemeralContainer(pod *corev1.Pod, container *corev1.EphemeralContainer) error {
	if pod == nil || container == nil {
		return errors.New("nil pod or container")
	}
	pod.Spec.EphemeralContainers = append(pod.Spec.EphemeralContainers, *container)
	return nil
}

// AddPodVolume appends a volume to the Pod spec.
func AddPodVolume(pod *corev1.Pod, volume *corev1.Volume) error {
	if pod == nil || volume == nil {
		return errors.New("nil pod or volume")
	}
	pod.Spec.Volumes = append(pod.Spec.Volumes, *volume)
	return nil
}

// AddPodImagePullSecret appends an image pull secret to the Pod spec.
func AddPodImagePullSecret(pod *corev1.Pod, imagePullSecret *corev1.LocalObjectReference) error {
	if pod == nil || imagePullSecret == nil {
		return errors.New("nil pod or imagePullSecret")
	}
	pod.Spec.ImagePullSecrets = append(pod.Spec.ImagePullSecrets, *imagePullSecret)
	return nil
}

// AddPodToleration appends a toleration to the Pod spec.
func AddPodToleration(pod *corev1.Pod, toleration *corev1.Toleration) error {
	if pod == nil || toleration == nil {
		return errors.New("nil pod or toleration")
	}
	pod.Spec.Tolerations = append(pod.Spec.Tolerations, *toleration)
	return nil
}

// AddPodTopologySpreadConstraints appends a topology spread constraint if provided.
func AddPodTopologySpreadConstraints(pod *corev1.Pod, topologySpreadConstraint *corev1.TopologySpreadConstraint) error {
	if pod == nil {
		return errors.New("nil pod")
	}
	if topologySpreadConstraint == nil {
		return nil
	}
	pod.Spec.TopologySpreadConstraints = append(pod.Spec.TopologySpreadConstraints, *topologySpreadConstraint)
	return nil
}

// SetPodServiceAccountName sets the service account used by the Pod.
func SetPodServiceAccountName(pod *corev1.Pod, serviceAccountName string) error {
	if pod == nil {
		return errors.New("nil pod")
	}
	pod.Spec.ServiceAccountName = serviceAccountName
	return nil
}

// SetPodSecurityContext sets the pod-level security context.
func SetPodSecurityContext(pod *corev1.Pod, securityContext *corev1.PodSecurityContext) error {
	if pod == nil {
		return errors.New("nil pod")
	}
	pod.Spec.SecurityContext = securityContext
	return nil
}

// SetPodAffinity assigns affinity rules to the Pod.
func SetPodAffinity(pod *corev1.Pod, affinity *corev1.Affinity) error {
	if pod == nil {
		return errors.New("nil pod")
	}
	pod.Spec.Affinity = affinity
	return nil
}

// SetPodNodeSelector sets the node selector map.
func SetPodNodeSelector(pod *corev1.Pod, nodeSelector map[string]string) error {
	if pod == nil {
		return errors.New("nil pod")
	}
	pod.Spec.NodeSelector = nodeSelector
	return nil
}

// SetPodHostNetwork configures host networking for the Pod.
func SetPodHostNetwork(pod *corev1.Pod, hostNetwork bool) error {
	if pod == nil {
		return errors.New("nil pod")
	}
	pod.Spec.HostNetwork = hostNetwork
	return nil
}

// SetPodDNSPolicy sets the DNS policy for the Pod.
func SetPodDNSPolicy(pod *corev1.Pod, policy corev1.DNSPolicy) error {
	if pod == nil {
		return errors.New("nil pod")
	}
	pod.Spec.DNSPolicy = policy
	return nil
}

func SetPodDNSConfig(pod *corev1.Pod, dnsConfig *corev1.PodDNSConfig) error {
	if pod == nil {
		return errors.New("nil pod")
	}
	pod.Spec.DNSConfig = dnsConfig
	return nil
}

func SetPodHostname(pod *corev1.Pod, hostname string) error {
	if pod == nil {
		return errors.New("nil pod")
	}
	pod.Spec.Hostname = hostname
	return nil
}

func SetPodSubdomain(pod *corev1.Pod, subdomain string) error {
	if pod == nil {
		return errors.New("nil pod")
	}
	pod.Spec.Subdomain = subdomain
	return nil
}

func SetPodRestartPolicy(pod *corev1.Pod, policy corev1.RestartPolicy) error {
	if pod == nil {
		return errors.New("nil pod")
	}
	pod.Spec.RestartPolicy = policy
	return nil
}

func SetPodTerminationGracePeriod(pod *corev1.Pod, secs int64) error {
	if pod == nil {
		return errors.New("nil pod")
	}
	if pod.Spec.TerminationGracePeriodSeconds == nil {
		pod.Spec.TerminationGracePeriodSeconds = new(int64)
	}
	*pod.Spec.TerminationGracePeriodSeconds = secs
	return nil
}

func SetPodSchedulerName(pod *corev1.Pod, scheduler string) error {
	if pod == nil {
		return errors.New("nil pod")
	}
	pod.Spec.SchedulerName = scheduler
	return nil
}
