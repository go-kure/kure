package k8s

import (
	"errors"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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

func AddPodContainer(pod *corev1.Pod, container *corev1.Container) error {
	if pod == nil || container == nil {
		return errors.New("nil pod or container")
	}
	pod.Spec.Containers = append(pod.Spec.Containers, *container)
	return nil
}

func AddPodInitContainer(pod *corev1.Pod, container *corev1.Container) error {
	if pod == nil || container == nil {
		return errors.New("nil pod or container")
	}
	pod.Spec.InitContainers = append(pod.Spec.InitContainers, *container)
	return nil
}

func AddPodEphemeralContainer(pod *corev1.Pod, container *corev1.EphemeralContainer) error {
	if pod == nil || container == nil {
		return errors.New("nil pod or container")
	}
	pod.Spec.EphemeralContainers = append(pod.Spec.EphemeralContainers, *container)
	return nil
}

func AddPodVolume(pod *corev1.Pod, volume *corev1.Volume) error {
	if pod == nil || volume == nil {
		return errors.New("nil pod or volume")
	}
	pod.Spec.Volumes = append(pod.Spec.Volumes, *volume)
	return nil
}

func AddPodImagePullSecret(pod *corev1.Pod, imagePullSecret *corev1.LocalObjectReference) error {
	if pod == nil || imagePullSecret == nil {
		return errors.New("nil pod or imagePullSecret")
	}
	pod.Spec.ImagePullSecrets = append(pod.Spec.ImagePullSecrets, *imagePullSecret)
	return nil
}

func AddPodToleration(pod *corev1.Pod, toleration *corev1.Toleration) error {
	if pod == nil || toleration == nil {
		return errors.New("nil pod or toleration")
	}
	pod.Spec.Tolerations = append(pod.Spec.Tolerations, *toleration)
	return nil
}

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

func SetPodServiceAccountName(pod *corev1.Pod, serviceAccountName string) error {
	if pod == nil {
		return errors.New("nil pod")
	}
	pod.Spec.ServiceAccountName = serviceAccountName
	return nil
}

func SetPodSecurityContext(pod *corev1.Pod, securityContext *corev1.PodSecurityContext) error {
	if pod == nil {
		return errors.New("nil pod")
	}
	pod.Spec.SecurityContext = securityContext
	return nil
}

func SetPodAffinity(pod *corev1.Pod, affinity *corev1.Affinity) error {
	if pod == nil {
		return errors.New("nil pod")
	}
	pod.Spec.Affinity = affinity
	return nil
}

func SetPodNodeSelector(pod *corev1.Pod, nodeSelector map[string]string) error {
	if pod == nil {
		return errors.New("nil pod")
	}
	pod.Spec.NodeSelector = nodeSelector
	return nil
}

func SetPodHostNetwork(pod *corev1.Pod, hostNetwork bool) error {
	if pod == nil {
		return errors.New("nil pod")
	}
	pod.Spec.HostNetwork = hostNetwork
	return nil
}

func SetPodDNSPolicy(pod *corev1.Pod, policy corev1.DNSPolicy) error {
	if pod == nil {
		return errors.New("nil pod")
	}
	pod.Spec.DNSPolicy = policy
	return nil
}
