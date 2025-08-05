package kubernetes

import (
	"github.com/go-kure/kure/internal/validation"

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
func SetPodSpec(pod *corev1.Pod, spec *corev1.PodSpec) error {
	validator := validation.NewValidator()
	if err := validator.ValidatePod(pod); err != nil {
		return err
	}
	if err := validator.ValidatePodSpec(spec); err != nil {
		return err
	}
	pod.Spec = *spec
	return nil
}

// AddPodContainer appends a container to the Pod spec.
func AddPodContainer(pod *corev1.Pod, container *corev1.Container) error {
	validator := validation.NewValidator()
	if err := validator.ValidatePod(pod); err != nil {
		return err
	}
	return AddPodSpecContainer(&pod.Spec, container)
}

// AddPodInitContainer appends an init container to the Pod spec.
func AddPodInitContainer(pod *corev1.Pod, container *corev1.Container) error {
	validator := validation.NewValidator()
	if err := validator.ValidatePod(pod); err != nil {
		return err
	}
	return AddPodSpecInitContainer(&pod.Spec, container)
}

// AddPodEphemeralContainer appends an ephemeral container to the Pod spec.
func AddPodEphemeralContainer(pod *corev1.Pod, container *corev1.EphemeralContainer) error {
	validator := validation.NewValidator()
	if err := validator.ValidatePod(pod); err != nil {
		return err
	}
	return AddPodSpecEphemeralContainer(&pod.Spec, container)
}

// AddPodVolume appends a volume to the Pod spec.
func AddPodVolume(pod *corev1.Pod, volume *corev1.Volume) error {
	validator := validation.NewValidator()
	if err := validator.ValidatePod(pod); err != nil {
		return err
	}
	return AddPodSpecVolume(&pod.Spec, volume)
}

// AddPodImagePullSecret appends an image pull secret to the Pod spec.
func AddPodImagePullSecret(pod *corev1.Pod, imagePullSecret *corev1.LocalObjectReference) error {
	validator := validation.NewValidator()
	if err := validator.ValidatePod(pod); err != nil {
		return err
	}
	return AddPodSpecImagePullSecret(&pod.Spec, imagePullSecret)
}

// AddPodToleration appends a toleration to the Pod spec.
func AddPodToleration(pod *corev1.Pod, toleration *corev1.Toleration) error {
	validator := validation.NewValidator()
	if err := validator.ValidatePod(pod); err != nil {
		return err
	}
	return AddPodSpecToleration(&pod.Spec, toleration)
}

// AddPodTopologySpreadConstraints appends a topology spread constraint if provided.
func AddPodTopologySpreadConstraints(pod *corev1.Pod, topologySpreadConstraint *corev1.TopologySpreadConstraint) error {
	validator := validation.NewValidator()
	if err := validator.ValidatePod(pod); err != nil {
		return err
	}
	return AddPodSpecTopologySpreadConstraints(&pod.Spec, topologySpreadConstraint)
}

// SetPodServiceAccountName sets the service account used by the Pod.
func SetPodServiceAccountName(pod *corev1.Pod, serviceAccountName string) error {
	validator := validation.NewValidator()
	if err := validator.ValidatePod(pod); err != nil {
		return err
	}
	return SetPodSpecServiceAccountName(&pod.Spec, serviceAccountName)
}

// SetPodSecurityContext sets the pod-level security context.
func SetPodSecurityContext(pod *corev1.Pod, securityContext *corev1.PodSecurityContext) error {
	validator := validation.NewValidator()
	if err := validator.ValidatePod(pod); err != nil {
		return err
	}
	return SetPodSpecSecurityContext(&pod.Spec, securityContext)
}

// SetPodAffinity assigns affinity rules to the Pod.
func SetPodAffinity(pod *corev1.Pod, affinity *corev1.Affinity) error {
	validator := validation.NewValidator()
	if err := validator.ValidatePod(pod); err != nil {
		return err
	}
	return SetPodSpecAffinity(&pod.Spec, affinity)
}

// SetPodNodeSelector sets the node selector map.
func SetPodNodeSelector(pod *corev1.Pod, nodeSelector map[string]string) error {
	validator := validation.NewValidator()
	if err := validator.ValidatePod(pod); err != nil {
		return err
	}
	return SetPodSpecNodeSelector(&pod.Spec, nodeSelector)
}

// SetPodPriorityClassName sets the priority class name.
func SetPodPriorityClassName(pod *corev1.Pod, class string) error {
	validator := validation.NewValidator()
	if err := validator.ValidatePod(pod); err != nil {
		return err
	}
	return SetPodSpecPriorityClassName(&pod.Spec, class)
}

// SetPodHostNetwork configures host networking for the Pod.
func SetPodHostNetwork(pod *corev1.Pod, hostNetwork bool) error {
	validator := validation.NewValidator()
	if err := validator.ValidatePod(pod); err != nil {
		return err
	}
	return SetPodSpecHostNetwork(&pod.Spec, hostNetwork)
}

// SetPodHostPID configures host PID namespace usage for the Pod.
func SetPodHostPID(pod *corev1.Pod, hostPID bool) error {
	validator := validation.NewValidator()
	if err := validator.ValidatePod(pod); err != nil {
		return err
	}
	return SetPodSpecHostPID(&pod.Spec, hostPID)
}

// SetPodHostIPC configures host IPC namespace usage for the Pod.
func SetPodHostIPC(pod *corev1.Pod, hostIPC bool) error {
	validator := validation.NewValidator()
	if err := validator.ValidatePod(pod); err != nil {
		return err
	}
	return SetPodSpecHostIPC(&pod.Spec, hostIPC)
}

// SetPodDNSPolicy sets the DNS policy for the Pod.
func SetPodDNSPolicy(pod *corev1.Pod, policy corev1.DNSPolicy) error {
	validator := validation.NewValidator()
	if err := validator.ValidatePod(pod); err != nil {
		return err
	}
	return SetPodSpecDNSPolicy(&pod.Spec, policy)
}

func SetPodDNSConfig(pod *corev1.Pod, dnsConfig *corev1.PodDNSConfig) error {
	validator := validation.NewValidator()
	if err := validator.ValidatePod(pod); err != nil {
		return err
	}
	return SetPodSpecDNSConfig(&pod.Spec, dnsConfig)
}

func SetPodHostname(pod *corev1.Pod, hostname string) error {
	validator := validation.NewValidator()
	if err := validator.ValidatePod(pod); err != nil {
		return err
	}
	return SetPodSpecHostname(&pod.Spec, hostname)
}

func SetPodSubdomain(pod *corev1.Pod, subdomain string) error {
	validator := validation.NewValidator()
	if err := validator.ValidatePod(pod); err != nil {
		return err
	}
	return SetPodSpecSubdomain(&pod.Spec, subdomain)
}

func SetPodRestartPolicy(pod *corev1.Pod, policy corev1.RestartPolicy) error {
	validator := validation.NewValidator()
	if err := validator.ValidatePod(pod); err != nil {
		return err
	}
	return SetPodSpecRestartPolicy(&pod.Spec, policy)
}

func SetPodTerminationGracePeriod(pod *corev1.Pod, secs int64) error {
	validator := validation.NewValidator()
	if err := validator.ValidatePod(pod); err != nil {
		return err
	}
	return SetPodSpecTerminationGracePeriod(&pod.Spec, secs)
}

func SetPodSchedulerName(pod *corev1.Pod, scheduler string) error {
	validator := validation.NewValidator()
	if err := validator.ValidatePod(pod); err != nil {
		return err
	}
	return SetPodSpecSchedulerName(&pod.Spec, scheduler)
}
